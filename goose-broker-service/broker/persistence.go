package broker

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	json "github.com/go-json-experiment/json"
)

// Log handles append-only file operations.
type Log struct {
	path        string
	file        *os.File
	writer      *bufio.Writer
	maxSize     int64
	maxRetained int
	unsafeBatch bool
	syncOnWrite bool
}

// AckEntry represents an acknowledgment of an event by a consumer.
type AckEntry struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	TargetID  string `json:"target_id"`
	Consumer  string `json:"consumer"`
	Timestamp int64  `json:"timestamp"`
}

func openLog(path string, maxSize int64, maxRetained int, unsafeBatch bool, syncOnWrite bool) (*Log, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	return &Log{
		path:        path,
		file:        file,
		writer:      bufio.NewWriter(file),
		maxSize:     maxSize,
		maxRetained: maxRetained,
		unsafeBatch: unsafeBatch,
		syncOnWrite: syncOnWrite,
	}, nil
}

// Append writes an entry to the end of the file.
func (l *Log) Append(entry interface{}) error {
	if l.maxSize > 0 && l.Size() >= l.maxSize {
		if err := l.Rotate(); err != nil {
			return err
		}
	}

	bytes, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	if _, err := l.writer.Write(bytes); err != nil {
		return err
	}
	if err := l.writer.WriteByte('\n'); err != nil {
		return err
	}

	if l.unsafeBatch && !l.syncOnWrite {
		return nil
	}

	if err := l.writer.Flush(); err != nil {
		return err
	}
	if l.syncOnWrite {
		return l.file.Sync()
	}
	return nil
}

// Rotate closes current file, renames it and opens a new one.
func (l *Log) Rotate() error {
	if err := l.writer.Flush(); err != nil {
		return err
	}
	if l.syncOnWrite {
		if err := l.file.Sync(); err != nil {
			return err
		}
	}

	if err := l.file.Close(); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.path, timestamp)
	if err := os.Rename(l.path, rotatedPath); err != nil {
		return err
	}

	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	l.file = file
	l.writer = bufio.NewWriter(file)

	go l.cleanup()
	return nil
}

func (l *Log) cleanup() {
	if l.maxRetained <= 0 {
		return
	}

	files, _ := filepath.Glob(l.path + ".*")
	if len(files) <= l.maxRetained {
		return
	}

	sort.Strings(files)
	toRemove := len(files) - l.maxRetained
	for i := 0; i < toRemove; i++ {
		_ = os.Remove(files[i])
	}
}

// Close flushes and closes the underlying file handle.
func (l *Log) Close() error {
	if err := l.writer.Flush(); err != nil {
		_ = l.file.Close()
		return err
	}
	if l.syncOnWrite {
		if err := l.file.Sync(); err != nil {
			_ = l.file.Close()
			return err
		}
	}
	return l.file.Close()
}

// Size returns the current size of the log file.
func (l *Log) Size() int64 {
	info, err := l.file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// Replay reads the log (including rotated files) from the beginning.
func (l *Log) Replay(fromID string, ch chan<- Event, limitOffset int64) error {
	rotated, _ := filepath.Glob(l.path + ".*")
	sort.Strings(rotated)

	allFiles := append(rotated, l.path)

	foundStart := fromID == ""

	for i, path := range allFiles {
		isCurrent := i == len(allFiles)-1

		file, err := os.Open(path)
		if err != nil {
			continue
		}

		var reader io.Reader = file
		if isCurrent && limitOffset > 0 {
			reader = io.LimitReader(file, limitOffset)
		}

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()

			var header struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			if err := json.Unmarshal(line, &header); err != nil {
				continue
			}

			if !foundStart {
				if header.ID == fromID {
					foundStart = true
				}
				continue
			}

			if header.Type == "ack" {
				continue
			}

			var event Event
			if err := json.Unmarshal(line, &event); err != nil {
				continue
			}
			ch <- event
		}

		_ = file.Close()
	}

	return nil
}
