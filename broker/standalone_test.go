package broker_test

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"goose/broker"
	"goose/broker/brokerclient"
)

func TestStandalone_Integration(t *testing.T) {
	// 1. Start Server
	tmpDir, err := os.MkdirTemp("", "broker_standalone_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Setup Manager
	manager, err := broker.NewManager(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	defer manager.Close()
	if _, err := manager.CreateBroker("default"); err != nil {
		t.Fatal(err)
	}

	// 2. Start HTTP Server
	// Note: Standalone test checks the CMD binary or internal http?
	// This test sets up httptest for CLIENT part.
	server := httptest.NewServer(broker.NewHandler(manager))
	defer server.Close()

	// 2. Setup Client
	client := brokerclient.New(server.URL)

	// 3. Subscribe
	ch, cancel := client.Subscribe("test-stream", "client-A", "")
	defer cancel()

	// Wait for sub
	time.Sleep(100 * time.Millisecond)

	// 4. Publish
	// Note: client.Send takes interface{}
	err = client.Send("test-stream", "hello-world")
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// 5. Verify Receive
	select {
	case delivery := <-ch:
		if delivery.Event.Stream != "test-stream" {
			t.Errorf("Expected stream test-stream, got %s", delivery.Event.Stream)
		}
		var payload string
		// Payload is jsontext.Value or json.RawMessage?
		// brokerclient uses jsonv2. broker.Event Payload is jsontext.Value.
		// We need to unmarshal it.
		if err := json.Unmarshal(delivery.Event.Payload, &payload); err != nil {
			t.Errorf("Failed to unmarshal payload: %v", err)
		} else if payload != "hello-world" {
			t.Errorf("Expected payload hello-world, got %s", payload)
		}
		delivery.Ack()
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for event")
	}
}
