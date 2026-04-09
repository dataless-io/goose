package broker_test

import (
	"sync"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/dataless-io/goose-broker-service/broker"
)

func TestParallelConsumerSerializesByID(t *testing.T) {
	processed := make(chan string, 2)
	handler := func(d broker.Delivery) {
		var payload string
		_ = json.Unmarshal(d.Payload, &payload)
		if payload == "first" {
			time.Sleep(100 * time.Millisecond)
		}
		processed <- payload
		d.Ack()
	}

	pc := broker.NewParallelConsumer(handler)
	ch := make(chan broker.Delivery)
	go pc.Consume(ch)

	id := "same-id"
	ch <- broker.Delivery{Event: broker.Event{ID: id, Payload: jsontext.Value(`"first"`)}, Ack: func() {}}
	ch <- broker.Delivery{Event: broker.Event{ID: id, Payload: jsontext.Value(`"second"`)}, Ack: func() {}}
	close(ch)

	a := <-processed
	b := <-processed
	if a != "first" || b != "second" {
		t.Fatalf("expected [first second], got [%s %s]", a, b)
	}
}

func TestParallelConsumerRunsDifferentIDsInParallel(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	starts := make(chan time.Time, 2)

	handler := func(d broker.Delivery) {
		starts <- time.Now()
		time.Sleep(100 * time.Millisecond)
		wg.Done()
		d.Ack()
	}

	pc := broker.NewParallelConsumer(handler)
	ch := make(chan broker.Delivery)
	go pc.Consume(ch)

	ch <- broker.Delivery{Event: broker.Event{ID: "id-1", Payload: jsontext.Value(`"a"`)}, Ack: func() {}}
	ch <- broker.Delivery{Event: broker.Event{ID: "id-2", Payload: jsontext.Value(`"b"`)}, Ack: func() {}}

	wg.Wait()
	close(ch)

	t1 := <-starts
	t2 := <-starts
	diff := t1.Sub(t2)
	if diff < 0 {
		diff = -diff
	}
	if diff > 50*time.Millisecond {
		t.Fatalf("expected near-parallel starts, diff=%v", diff)
	}
}
