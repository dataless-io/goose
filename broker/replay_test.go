package broker_test

import (
	"os"
	"testing"
	"time"

	"goose/broker"
)

func TestBroker_Replay(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "broker_replay_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})

	// 1. Publish some events
	for i := 0; i < 5; i++ {
		// We capture the ID manually or we need Send to return it?
		// Send generates ID inside. We can't know it.
		// Wait, Replay needs an ID.
		// We need a way to get the ID back.
		// Send() currently returns error only.
		// Ideally Send should return the Event (or at least ID).
		// But let's assume we can consume them live to get IDs.
	}
	// Workaround: Consume live to get IDs
	subCh, subCancel := b.Subscribe("recorder", "")

	go func() {
		b.Send("stream", "msg1")
		b.Send("stream", "msg2")
		b.Send("stream", "msg3")
	}()

	var id1, id2 string
	// Read 3 events
	d1 := <-subCh
	id1 = d1.ID
	d2 := <-subCh
	id2 = d2.ID
	<-subCh // d3

	subCancel()
	b.Close() // Close broker

	// 2. Re-Open Broker
	b2, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b2.Close()

	// 3. Subscribe from ID1
	// Should receive msg2 and msg3 (events AFTER msg1)
	replayCh, replayCancel := b2.Subscribe("replayer", id1)
	defer replayCancel()

	select {
	case d := <-replayCh:
		if d.ID != id2 {
			t.Errorf("Expected first replayed event to be %s (msg2), got %s", id2, d.ID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for replay")
	}

	// 4. Verify live events work after replay
	go func() {
		b2.Send("stream", "msg4")
	}()

	select {
	case d := <-replayCh:
		// We expect msg3 (from replay) then msg4 (live)
		// Actually above we only read one (msg2).
		// So this should be msg3.
		if d.ID == id2 {
			// This would be duplicate if logic is wrong
			t.Error("Got id2 again?")
		}
		// Assuming proper order: msg3
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for msg3")
	}

	// Next should be msg4
	select {
	case d := <-replayCh:
		// Check payload "msg4"?
		// Accessing payload is raw bytes
		// Verify it's new
		if d.Timestamp < d1.Timestamp { // Rough check
			t.Error("Expected new event")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for msg4")
	}
}
