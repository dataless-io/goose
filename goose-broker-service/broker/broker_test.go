package broker_test

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	json "github.com/go-json-experiment/json"

	"github.com/dataless-io/goose-broker-service/broker"
)

func newTempBroker(t *testing.T) (*broker.Broker, string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "events.log")
	b, err := broker.New(broker.Config{PersistencePath: path})
	if err != nil {
		t.Fatalf("new broker: %v", err)
	}
	return b, path
}

func TestSendBlocksUntilRequiredConsumerCommits(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	ch, cancel := b.Subscribe("critical", "")
	defer cancel()

	go func() {
		d := <-ch
		time.Sleep(150 * time.Millisecond)
		d.Ack()
	}()

	start := time.Now()
	err := b.Send("events", map[string]any{"msg": "hola"}, "critical")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if elapsed < 120*time.Millisecond {
		t.Fatalf("send returned too fast: %v", elapsed)
	}
}

func TestSendBlocksUntilAllRequiredConsumersCommit(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	c1, cancel1 := b.Subscribe("consumer-a", "")
	defer cancel1()
	c2, cancel2 := b.Subscribe("consumer-b", "")
	defer cancel2()

	go func() {
		d := <-c1
		time.Sleep(80 * time.Millisecond)
		d.Ack()
	}()
	go func() {
		d := <-c2
		time.Sleep(170 * time.Millisecond)
		d.Ack()
	}()

	start := time.Now()
	err := b.Send("events", "payload", "consumer-a", "consumer-b")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if elapsed < 150*time.Millisecond {
		t.Fatalf("send should wait for both acks, elapsed=%v", elapsed)
	}
}

func TestSendReturnsErrorWhenRequiredConsumerIsNotSubscribed(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	err := b.Send("events", "payload", "missing-consumer")
	if err == nil {
		t.Fatalf("expected error")
	}

	var missingErr broker.ErrRequiredConsumerNotSubscribed
	if !errors.As(err, &missingErr) {
		t.Fatalf("expected ErrRequiredConsumerNotSubscribed, got: %T (%v)", err, err)
	}
	if missingErr.Consumer != "missing-consumer" {
		t.Fatalf("unexpected consumer in error: %q", missingErr.Consumer)
	}
}

func TestSendWithoutRequiredConsumersIsNonBlocking(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	ch, cancel := b.Subscribe("slow", "")
	defer cancel()

	go func() {
		<-ch
		time.Sleep(600 * time.Millisecond)
	}()

	start := time.Now()
	err := b.Send("events", "payload")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("send should not block when no required consumer, elapsed=%v", elapsed)
	}
}

func TestAckIsPersistedInLog(t *testing.T) {
	b, path := newTempBroker(t)
	defer b.Close()

	ch, cancel := b.Subscribe("audit", "")
	defer cancel()

	go func() {
		d := <-ch
		d.Ack()
	}()

	if err := b.Send("events", map[string]any{"a": 1}, "audit"); err != nil {
		t.Fatalf("send failed: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	if !s.Scan() {
		t.Fatal("expected first event")
	}
	var event broker.Event
	if err := json.Unmarshal(s.Bytes(), &event); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}

	if !s.Scan() {
		t.Fatal("expected ack record")
	}
	line := s.Text()
	if !strings.Contains(line, `"type":"ack"`) {
		t.Fatalf("expected ack line, got: %s", line)
	}
	if !strings.Contains(line, `"target_id":"`+event.ID+`"`) {
		t.Fatalf("ack target does not match event id")
	}
}

func TestReplayFromIDReturnsEventsAfterMarker(t *testing.T) {
	b, path := newTempBroker(t)

	recorder, cancel := b.Subscribe("recorder", "")

	if err := b.Send("events", "one"); err != nil {
		t.Fatal(err)
	}
	if err := b.Send("events", "two"); err != nil {
		t.Fatal(err)
	}
	if err := b.Send("events", "three"); err != nil {
		t.Fatal(err)
	}

	first := <-recorder
	second := <-recorder
	third := <-recorder
	_ = second
	_ = third

	cancel()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}

	b2, err := broker.New(broker.Config{PersistencePath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer b2.Close()

	replay, replayCancel := b2.Subscribe("replay", first.ID)
	defer replayCancel()

	got := make([]string, 0, 2)
	timeout := time.After(2 * time.Second)
	for len(got) < 2 {
		select {
		case d := <-replay:
			got = append(got, d.ID)
		case <-timeout:
			t.Fatalf("timeout waiting replay events, got=%v", got)
		}
	}

	if got[0] == first.ID || got[1] == first.ID {
		t.Fatalf("replay includes marker event, got=%v", got)
	}
}

func TestTopicCRUDAndPublishValidation(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	if _, err := b.CreateTopic("orders", "orders pipeline", 72); err != nil {
		t.Fatalf("create topic: %v", err)
	}

	updatedDesc := "orders and billing"
	retention := 96
	disabled := false
	if _, err := b.UpdateTopic("orders", broker.TopicUpdate{
		Description:    &updatedDesc,
		RetentionHours: &retention,
		Disabled:       &disabled,
	}); err != nil {
		t.Fatalf("update topic: %v", err)
	}

	if err := b.Send("orders", map[string]any{"id": "o-1"}); err != nil {
		t.Fatalf("send on existing topic: %v", err)
	}

	if err := b.Send("missing-topic", "x"); err == nil {
		t.Fatalf("expected missing topic to fail")
	}

	disabled = true
	if _, err := b.UpdateTopic("orders", broker.TopicUpdate{Disabled: &disabled}); err != nil {
		t.Fatalf("disable topic: %v", err)
	}
	if err := b.Send("orders", "x"); err == nil {
		t.Fatalf("expected disabled topic to fail")
	}

	if err := b.DeleteTopic("orders"); err != nil {
		t.Fatalf("delete topic: %v", err)
	}
}

func TestMetricsTrackPublishedAndRequiredAcks(t *testing.T) {
	b, _ := newTempBroker(t)
	defer b.Close()

	c, cancel := b.Subscribe("critical", "")
	defer cancel()

	go func() {
		d := <-c
		d.Ack()
	}()

	if err := b.Send("events", map[string]any{"id": 1}, "critical"); err != nil {
		t.Fatalf("send: %v", err)
	}

	metrics := b.Metrics()
	if metrics.PublishedMessages == 0 {
		t.Fatalf("expected published metrics")
	}
	if metrics.RequiredAcks == 0 {
		t.Fatalf("expected required ack metrics")
	}
	if metrics.ByTopic["events"].Published == 0 {
		t.Fatalf("expected per-topic metrics")
	}
}
