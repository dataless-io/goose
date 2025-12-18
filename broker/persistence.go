package broker

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// Log handles the append-only file operations
type Log struct {
	file    *os.File
	encoder *json.Encoder
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
func openLog(path string) (*Log, error) {
	// Open file in Append mode, Create if not exists, Write only
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	return &Log{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// Append writes an entry to the end of the file
func (l *Log) Append(entry interface{}) error {
	return l.encoder.Encode(entry)
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

// Replay reads the log from the beginning up to limitOffset and sends events occurring AFTER fromID to the channel.
// If fromID is empty, it replays from the beginning (all history).
// It ignores AckEntries and other types, returning only Events.
func (l *Log) Replay(fromID string, ch chan<- Event, limitOffset int64) error {
	// Open file in Read mode
	file, err := os.Open(l.file.Name())
	if err != nil {
		return err
	}
	defer file.Close()

	// Read only up to the limit to avoid race with concurrent writes
	reader := io.LimitReader(file, limitOffset)
	scanner := bufio.NewScanner(reader)

	startSending := false
	if fromID == "" {
		startSending = true // Send everything
	}

	for scanner.Scan() {
		line := scanner.Bytes()

		// Optimization: Check type before full unmarshal?
		// Or just unmarshal into a generic map to check ID/Type?
		// We know structure.
		// Let's allow simple unmarshal to struct with common fields
		var header struct {
			Type string `json:"type"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal(line, &header); err != nil {
			continue // Skip malformed
		}

		// Logic for resume
		if !startSending {
			if header.ID == fromID {
				startSending = true
			}
			continue
		}

		// Only send Events (Type is empty/default or "event" if we explicitly set it,
		// but currently Event struct doesn't have Type field, so it matches default).
		// Wait, `AckEntry` has `Type: "ack"`. `Event` has no `Type`.
		// So if `header.Type` is "ack", skip.
		if header.Type == "ack" {
			continue
		}

		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		ch <- event
	}

	return scanner.Err()
}
