package broker_test

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"goose/broker"
)

func TestParallelConsumer_Serialization(t *testing.T) {
	// Verify that events with SAME ID are processed sequentially

	processedOrder := make(chan string, 2)

	handler := func(d broker.Delivery) {
		var payload string
		json.Unmarshal(d.Payload, &payload)

		if payload == "first" {
			time.Sleep(100 * time.Millisecond) // Simulate slow processing
		}
		processedOrder <- payload
		d.Ack()
	}

	pc := broker.NewParallelConsumer(handler)
	ch := make(chan broker.Delivery)

	go pc.Consume(ch)

	// Send 2 events, same ID
	id := "same-id"

	ch <- broker.Delivery{
		Event: broker.Event{ID: id, Payload: json.RawMessage(`"first"`)},
		Ack:   func() {},
	}
	ch <- broker.Delivery{
		Event: broker.Event{ID: id, Payload: json.RawMessage(`"second"`)},
		Ack:   func() {},
	}
	close(ch)

	// Check order
	p1 := <-processedOrder
	p2 := <-processedOrder

	if p1 != "first" || p2 != "second" {
		t.Errorf("Expected sequential order [first, second], got [%s, %s]", p1, p2)
	}
}

func TestParallelConsumer_Parallelism(t *testing.T) {
	// Verify that events with DIFFERENT IDs runs in parallel

	var wg sync.WaitGroup
	wg.Add(2)

	startTimes := make(chan time.Time, 2)

	handler := func(d broker.Delivery) {
		startTimes <- time.Now()
		time.Sleep(100 * time.Millisecond) // Occupy worker
		wg.Done()
		d.Ack()
	}

	pc := broker.NewParallelConsumer(handler)
	ch := make(chan broker.Delivery)

	go pc.Consume(ch)

	// Send 2 events, different IDs
	ch <- broker.Delivery{
		Event: broker.Event{ID: "id-1", Payload: json.RawMessage(`"data"`)},
		Ack:   func() {},
	}
	ch <- broker.Delivery{
		Event: broker.Event{ID: "id-2", Payload: json.RawMessage(`"data"`)},
		Ack:   func() {},
	}

	wg.Wait()
	close(ch) // Stop consumer

	t1 := <-startTimes
	t2 := <-startTimes

	// The difference between start times should be negligible if they run in parallel
	// If serialized, t2 would start 100ms after t1.
	diff := t2.Sub(t1)
	if diff < 0 {
		diff = -diff
	}

	if diff > 50*time.Millisecond {
		t.Errorf("Expected parallel execution (start delay ~0), got delay %v", diff)
	}
}
