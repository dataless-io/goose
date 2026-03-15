package brokerclient_test

import (
	"net/http/httptest"
	"os"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"goose/broker"
	"goose/broker/brokerclient"
)

func TestClient_Integration(t *testing.T) {
	// 1. Setup Broker
	tmpDir, err := os.MkdirTemp("", "client_test")
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

	// 2. Setup HTTP Server
	handler := broker.NewHandler(manager)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 3. Setup Client (pointing to default broker for backward compat or explicit?)
	// Client needs update to handle new URL structure. For now, testing integration implies client update.
	// But client update is NEXT task.
	// To make this compile, I need to update client.go FIRST or patch this test to wait.
	// I will update client.go in parallel/next. For now let's make this test compile even if it fails runtime
	// until client is updated.
	// Actually, client.New(url) just sets base. If base is server.URL, client does POST /publish.
	// Server expects POST /brokers/{name}/publish.
	// So client needs update.
	client := brokerclient.New(server.URL)

	// 4. Subscribe
	streamName := "client-test-stream"
	consumerID := "test-consumer"

	subCh, cancel := client.Subscribe(streamName, consumerID, "")
	defer cancel()

	// Wait for subscription to potentially establish (not strictly needed with async internal but safer)
	time.Sleep(100 * time.Millisecond)

	// 5. Send Events
	payload1 := map[string]string{"msg": "hello"}
	if err := client.Send(streamName, payload1); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	payload2 := map[string]string{"msg": "world"}
	if err := client.Send(streamName, payload2); err != nil {
		t.Fatalf("Send 2 failed: %v", err)
	}

	// 6. Send Event to DIFFERENT stream (should be filtered out)
	if err := client.Send("other-stream", payload2); err != nil {
		t.Fatalf("Send other failed: %v", err)
	}

	// 7. Verify Reception
	timeout := time.After(2 * time.Second)

	// Expect 2 events
	events := []string{}

	for i := 0; i < 2; i++ {
		select {
		case delivery, ok := <-subCh:
			if !ok {
				t.Fatal("Channel closed unexpectedly")
			}
			var p map[string]string
			// Use json.Unmarshal on value
			if err := json.Unmarshal(delivery.Payload, &p); err != nil {
				t.Fatalf("Failed to unmarshal payload: %v", err)
			}
			events = append(events, p["msg"])
			delivery.Ack() // No-op but good practice
		case <-timeout:
			t.Fatalf("Timeout waiting for event %d", i)
		}
	}

	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}
	if events[0] != "hello" || events[1] != "world" {
		t.Errorf("Unexpected events: %v", events)
	}

	// Ensure NO more events (filtered one)
	select {
	case d := <-subCh:
		t.Fatalf("Received unexpected event from wrong stream: %v", d.Event.Stream)
	case <-time.After(200 * time.Millisecond):
		// OK
	}
}
