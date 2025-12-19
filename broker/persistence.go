package broker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Log handles the append-only file operations
type Log struct {
	path        string
	file        *os.File
	encoder     *json.Encoder
	maxSize     int64
	maxRetained int
}

// AckEntry represents an acknowledgment of an event by a consumer
type AckEntry struct {
	Type      string `json:"type"`      // "ack"
	ID        string `json:"id"`        // Unique ID of the ack record
	TargetID  string `json:"target_id"` // ID of the event being acked
	Consumer  string `json:"consumer"`
	Timestamp int64  `json:"timestamp"`
}

// openLog opens or creates the file for appending events
func openLog(path string, maxSize int64, maxRetained int) (*Log, error) {
	// Open file in Append mode, Create if not exists, Write only
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Log{
		path:        path,
		file:        file,
		encoder:     json.NewEncoder(file),
		maxSize:     maxSize,
		maxRetained: maxRetained,
	}, nil
}

// Append writes an entry to the end of the file
func (l *Log) Append(entry interface{}) error {
	// Rotation Check
	configSize := l.maxSize
	if configSize > 0 && l.Size() >= configSize {
		if err := l.Rotate(); err != nil {
			return err
		}
	}
	return l.encoder.Encode(entry)
}

// Rotate closes current file, renames it, and opens a new one
func (l *Log) Rotate() error {
	// Close current
	if err := l.file.Close(); err != nil {
		return err
	}

	// Rename
	timestamp := time.Now().Format("20060102150405")
	oldPath := fmt.Sprintf("%s.%s", l.path, timestamp)
	if err := os.Rename(l.path, oldPath); err != nil {
		return err
	}

	// Re-Open
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	l.file = file
	l.encoder = json.NewEncoder(file)

	// Cleanup
	go l.cleanup() // Async cleanup

	return nil
}

func (l *Log) cleanup() {
	files, _ := filepath.Glob(l.path + ".*") // events.log.2023...
	if len(files) <= l.maxRetained {
		return
	}
	sort.Strings(files) // sort ascending
	toRemove := len(files) - l.maxRetained
	for i := 0; i < toRemove; i++ {
		os.Remove(files[i])
	}
}

// Close closes the underlying file handle
func (l *Log) Close() error {
	return l.file.Close()
}

// Size returns the current size of the log file
func (l *Log) Size() int64 {
	info, err := l.file.Stat()
	if err != nil {
		return 0
	}
	return info.Size()
}

// Replay reads the log (including rotated files) from the beginning
func (l *Log) Replay(fromID string, ch chan<- Event, limitOffset int64) error {
	// Identify all files: Rotated + Current
	rotated, _ := filepath.Glob(l.path + ".*")
	sort.Strings(rotated)

	allFiles := append(rotated, l.path)

	foundStart := false
	if fromID == "" {
		foundStart = true
	}

	for i, path := range allFiles {
		isCurrent := (i == len(allFiles)-1)

		file, err := os.Open(path)
		if err != nil {
			continue // Skip unreadable?
		}

		var reader io.Reader = file
		if isCurrent && limitOffset > 0 {
			reader = io.LimitReader(file, limitOffset)
		}

		scanner := bufio.NewScanner(reader)
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
		file.Close()
	}

	return nil
}
