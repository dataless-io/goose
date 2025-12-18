package broker_test

import (
	"os"
	"testing"
	"time"

	"goose/broker"
)

func TestBroker_BlockingSend(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "broker_blocking_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b.Close()

	// Consumidor Lento (Bloqueante)
	slowConsumerName := "slow-processor"
	events, _ := b.Subscribe(slowConsumerName)

	// Lanza rutina que procesa lee el evento pero tarde en hacer Ack
	consumerAcked := make(chan bool)
	go func() {
		msg := <-events
		time.Sleep(200 * time.Millisecond) // Simula proceso lento
		msg.Ack()
		consumerAcked <- true
	}()

	// Productor
	start := time.Now()
	// Enviamos mensaje requiriendo Ack del slowConsumer
	err = b.Send("test-stream", "data", slowConsumerName)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verificaciones
	if elapsed < 200*time.Millisecond {
		t.Errorf("Send returned too fast (%v), expected > 200ms duration due to slow consumer blocking", elapsed)
	}

	select {
	case <-consumerAcked:
		// OK
	default:
		t.Error("Consumer should have processed the message")
	}
}

func TestBroker_NonBlocking_IfConsumerNotRequired(t *testing.T) {
	// Setup
	tmpFile, err := os.CreateTemp("", "broker_nonblocking_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b.Close()

	// Consumidor Lento
	slowConsumerName := "slow-processor-ignored"
	events, _ := b.Subscribe(slowConsumerName)

	go func() {
		<-events
		// No hacemos Ack nunca o muy tarde
		time.Sleep(1 * time.Second)
	}()

	// Productor
	start := time.Now()
	// NO requerimos confirmación de este consumidor
	err = b.Send("test-stream", "data")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Send blocked too long (%v), expected immediate return", elapsed)
	}
}
