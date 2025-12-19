package broker_test

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"goose/broker"
)

func TestBroker_HTTPTransport(t *testing.T) {
	// Setup Token Broker
	tmpFile, err := os.CreateTemp("", "broker_http_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, err := broker.New(broker.Config{PersistencePath: tmpPath})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	// Setup Server
	handler := broker.NewHandler(b)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Connect Client
	resp, err := http.Get(server.URL + "/subscribe")
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify Headers
	if ct := resp.Header.Get("Content-Type"); ct != "application/x-ndjson" {
		t.Errorf("Expected Content-Type application/x-ndjson, got %s", ct)
	}

	// Wait a bit to ensure subscription is active
	// (In a real scenario, we might want a way to know when sub is ready,
	// but for this test, we rely on the implementation simplicity)
	time.Sleep(100 * time.Millisecond)

	// Publish Event via Direct Call (simulate internal or add test for POST /publish)
	// For this test, we can use the new POST endpoint to verify it works too.
	go func() {
		// b.Send("http-test", ...) // Old way
		// New way via HTTP
		// ... actually standard test might find it easier to use b.Send directly unless we want to test Post handler.
		// Let's use b.Send to ensure subscribe works first.
		b.Send("http-test", map[string]string{"msg": "hello transport"})
	}()

	// Read Stream
	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		t.Fatal("Failed to read from stream or stream closed")
	}

	var event broker.Event
	if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
		t.Fatalf("Failed to unmarshal received event: %v", err)
	}

	if event.Stream != "http-test" {
		t.Errorf("Expected stream 'http-test', got %s", event.Stream)
	}
}
