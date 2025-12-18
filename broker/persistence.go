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

// Append writes an event to the end of the file
func (l *Log) Append(event Event) error {
	return l.encoder.Encode(event)
}

// Close closes the underlying file handle
func (l *Log) Close() error {
	return l.file.Close()
}
