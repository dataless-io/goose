package broker_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"goose/broker"
)

func TestBroker_Rotation(t *testing.T) {
	// 1. Setup with small size
	tmpDir, _ := os.MkdirTemp("", "broker_rot_*")
	defer os.RemoveAll(tmpDir)
	logPath := filepath.Join(tmpDir, "events.log")

	// Set MaxLogSize to ~200 bytes => ~2-3 events (approx 70-80 bytes each)
	b, _ := broker.New(broker.Config{
		PersistencePath: logPath,
		MaxLogSize:      200,
		MaxRetainedLogs: 2,
	})
	defer b.Close()

	// 2. Write Events to Trigger Rotation
	// Subscribe to capture IDs
	recCh, recCancel := b.Subscribe("recorder", "")

	// Event 1
	b.Send("stream", strings.Repeat("a", 50))
	// Event 2 (might be close to limit)
	b.Send("stream", strings.Repeat("b", 50))
	// Event 3 (Should trigger rotation)
	b.Send("stream", strings.Repeat("c", 50))
	// Event 4
	b.Send("stream", strings.Repeat("d", 50))

	// Capture ID of first event
	var startID string
	select {
	case d := <-recCh:
		startID = d.ID
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for recorder")
	}
	recCancel()

	// 3. Verify Files
	files, _ := filepath.Glob(logPath + "*")
	// Expect: events.log + events.log.<timestamp>
	if len(files) < 2 {
		t.Errorf("Expected at least 2 files (rotated), got %d: %v", len(files), files)
	}

	// 4. Test Replay across files
	b.Close()

	b2, _ := broker.New(broker.Config{
		PersistencePath: logPath,
		MaxLogSize:      1000,
	})
	defer b2.Close()

	ch, cancel := b2.Subscribe("replayer", startID)
	defer cancel()

	// Collect
	count := 0
	timeout := time.After(1 * time.Second)
Loop:
	for {
		select {
		case <-ch:
			count++
			if count == 3 { // Expecting 3 events (E2, E3, E4)
				break Loop
			}
		case <-timeout:
			t.Fatalf("Timeout waiting for 3 events, got %d", count)
		}
	}

	if count != 3 {
		t.Errorf("Expected 3 events replayed, got %d", count)
	}
}
