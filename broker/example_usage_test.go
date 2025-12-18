package broker_test

import (
	"fmt"
	"os"
	"time"

	"goose/broker"
)

// Example demonstrates a producer waiting for a critical consumer 'a',
// while consumer 'b' processes events asynchronously.
func Example() {
	// 1. Setup Broker
	tmpFile, _ := os.CreateTemp("", "broker_example_*.log")
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	b, _ := broker.New(broker.Config{PersistencePath: tmpPath})
	defer b.Close()

	// 2. Setup Critical Consumer 'a'
	// This consumer MUST acknowledge events for the producer to continue.
	streamA, cancelA := b.Subscribe("a")
	defer cancelA()

	go func() {
		for delivery := range streamA {
			fmt.Printf("[Consumer A] Received: %s\n", delivery.Event.Payload)
			time.Sleep(100 * time.Millisecond) // Simulate checking/processing
			delivery.Ack()
			fmt.Println("[Consumer A] Ack sent")
		}
	}()

	// 3. Setup Non-Critical Consumer 'b'
	// This consumer receives events but the producer DOES NOT wait for it.
	streamB, cancelB := b.Subscribe("b")
	defer cancelB()

	go func() {
		for delivery := range streamB {
			// Simulate slow processing (slower than A)
			time.Sleep(300 * time.Millisecond)
			fmt.Printf("[Consumer B] Received: %s (late)\n", delivery.Event.Payload)
			delivery.Ack()
		}
	}()

	// 4. Producer
	fmt.Println("[Producer] Sending event...")

	// We ONLY require 'a' to Ack. 'b' will receive it too, but we won't wait for 'b'.
	err := b.Send("example-stream", "Hello World", "a")
	if err != nil {
		panic(err)
	}

	fmt.Println("[Producer] Send completed (Consumer A acked)")

	// Give B some time to print before exiting test
	time.Sleep(400 * time.Millisecond)

	// Output:
	// [Producer] Sending event...
	// [Consumer A] Received: "Hello World"
	// [Consumer A] Ack sent
	// [Producer] Send completed (Consumer A acked)
	// [Consumer B] Received: "Hello World" (late)
}
