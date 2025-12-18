package broker_test

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"goose/broker"
)

func TestBroker_PersistentAck(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "broker_ack_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b.Close()

	consumerName := "critical-logger"
	events, _ := b.Subscribe(consumerName)

	// Consumer routine check
	go func() {
		msg := <-events
		msg.Ack() // This should trigger a disk write
	}()

	// Send event requiring ack
	if err := b.Send("test-stream", "payload", consumerName); err != nil {
		t.Fatal(err)
	}

	// Verify Log File Content
	file, err := os.Open(tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// 1. First line: Event
	if !scanner.Scan() {
		t.Fatal("Expected event line")
	}
	var event broker.Event
	json.Unmarshal(scanner.Bytes(), &event)
	if event.Stream != "test-stream" {
		t.Fatal("Expected event first")
	}

	// 2. Second line: Ack
	if !scanner.Scan() {
		t.Fatal("Expected ack line")
	}
	line := scanner.Text()
	if !strings.Contains(line, `"type":"ack"`) {
		t.Errorf("Expected ack record, got: %s", line)
	}
	if !strings.Contains(line, `"consumer":"critical-logger"`) {
		t.Errorf("Expected consumer name in ack, got: %s", line)
	}
	if !strings.Contains(line, `"target_id":"`+event.ID+`"`) {
		t.Errorf("Expected target_id matching event, got: %s", line)
	}
}
