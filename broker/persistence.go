package broker

import (
	"encoding/json"
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
