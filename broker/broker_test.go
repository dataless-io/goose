package broker_test

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"goose/broker"
)

func TestBroker_Persistence(t *testing.T) {
	// Setup temporary file
	tmpFile, err := os.CreateTemp("", "broker_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close() // Broker will open it
	defer os.Remove(tmpPath)

	// Initialize Broker
	cfg := broker.Config{PersistencePath: tmpPath}
	b, err := broker.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create broker: %v", err)
	}

	// Send events
	testPayload := map[string]string{"foo": "bar"}
	if err := b.Send("test-stream", testPayload); err != nil {
		t.Fatalf("Failed to send event: %v", err)
	}
	if err := b.Send("test-stream", "second-event"); err != nil {
		t.Fatalf("Failed to send second event: %v", err)
	}

	// Close broker
	if err := b.Close(); err != nil {
		t.Fatalf("Failed to close broker: %v", err)
	}

	// Verify file content
	file, err := os.Open(tmpPath)
	if err != nil {
		t.Fatalf("Failed to open persist file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Check first event
	if !scanner.Scan() {
		t.Fatal("Expected at least one event in log")
	}
	var event1 broker.Event
	if err := json.Unmarshal(scanner.Bytes(), &event1); err != nil {
		t.Fatalf("Failed to unmarshal event 1: %v", err)
	}
	if event1.Stream != "test-stream" {
		t.Errorf("Expected stream 'test-stream', got %s", event1.Stream)
	}

	// Check second event
	if !scanner.Scan() {
		t.Fatal("Expected second event in log")
	}
	var event2 broker.Event
	if err := json.Unmarshal(scanner.Bytes(), &event2); err != nil {
		t.Fatalf("Failed to unmarshal event 2: %v", err)
	}

	// Check that payload strings are contained in the raw message
	if !strings.Contains(string(event1.Payload), "bar") {
		t.Errorf("Payload 1 mismatch")
	}
}
