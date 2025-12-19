package broker_test

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"goose/broker"
)

func TestStandalone_Integration(t *testing.T) {
	// 1. Start Server
	tmpFile, _ := os.CreateTemp("", "broker_standalone_*.log")
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b.Close()

	server := httptest.NewServer(broker.NewHandler(b))
	defer server.Close()

	// 2. Setup Client
	client := broker.NewClient(server.URL)

	// 3. Subscribe
	ch, cancel, err := client.Subscribe("client-A", "")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	defer cancel()

	// Wait for sub
	time.Sleep(100 * time.Millisecond)

	// 4. Publish
	err = client.Send("test-stream", "hello-world")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 5. Verify Receive
	select {
	case event := <-ch:
		if event.Stream != "test-stream" {
			t.Errorf("Expected stream test-stream, got %s", event.Stream)
		}
		var payload string
		json.Unmarshal(event.Payload, &payload)
		if payload != "hello-world" {
			t.Errorf("Expected payload hello-world, got %s", payload)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}
