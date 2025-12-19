package broker_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"goose/broker"
)

// BenchmarkBroker_HighConcurrency measures the core throughput of the Broker
// with highly concurrent producers and consumers.
func BenchmarkBroker_HighConcurrency(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "bench_broker_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Use large enough log size to avoid excessive rotation overhead dominating the benchmark
	// unless we want to test rotation impact. Let's use 100MB.
	brokerInst, err := broker.New(broker.Config{
		PersistencePath: filepath.Join(tmpDir, "events.log"),
		MaxLogSize:      100 * 1024 * 1024,
		MaxRetainedLogs: 5,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer brokerInst.Close()

	// 1. Setup Consumers
	// Real-world: Consumers are async but might fan-out.
	// We add one "Logging" consumer that just acknowledges everything fast.
	subCh, subCancel := brokerInst.Subscribe("logger", "")
	defer subCancel()

	// Consumer Goroutine (Drain the channel)
	go func() {
		for msg := range subCh {
			msg.Ack()
		}
	}()

	// Payload (1KB typical event)
	payload := make([]byte, 1024)
	streamName := "benchmark-stream"

	// Reset Timer to ignore setup
	b.ResetTimer()

	// 2. Parallel Producers
	// RunParallel runs the function in multiple goroutines.
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := brokerInst.Send(streamName, payload)
			if err != nil {
				b.Error(err)
			}
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "events/s")
}

// BenchmarkBroker_WithRotation measures performance when rotation occurs frequently
func BenchmarkBroker_WithRotation(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "bench_broker_rot_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Small log size to force constant rotation
	brokerInst, err := broker.New(broker.Config{
		PersistencePath: filepath.Join(tmpDir, "events.log"),
		MaxLogSize:      20 * 1024 * 1024, // 1MB
		MaxRetainedLogs: 5_000_000,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer brokerInst.Close()

	subCh, subCancel := brokerInst.Subscribe("rot-logger", "")
	defer subCancel()
	go func() {
		for msg := range subCh {
			msg.Ack()
		}
	}()

	payload := make([]byte, 1024)
	streamName := "rot-stream"

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			brokerInst.Send(streamName, payload)
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "events/s")
}

// BenchmarkBroker_FanOut_10x10 simulates 10 concurrent consumers and parallel producers
func BenchmarkBroker_FanOut_10x10(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "bench_broker_fanout_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brokerInst, err := broker.New(broker.Config{
		PersistencePath: filepath.Join(tmpDir, "events.log"),
		MaxLogSize:      100 * 1024 * 1024,
		MaxRetainedLogs: 5,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer brokerInst.Close()

	// 1. Setup 10 Consumers
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("consumer-%d", i)
		subCh, _ := brokerInst.Subscribe(name, "")

		// Async drain
		go func() {
			for msg := range subCh {
				msg.Ack()
			}
		}()
	}

	payload := make([]byte, 1024)
	streamName := "fanout-stream"

	b.ResetTimer()

	// 2. Producers (High Concurrency)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := brokerInst.Send(streamName, payload); err != nil {
				b.Error(err)
			}
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "events/s")
}

// BenchmarkBroker_FanOut_Unsafe simulates 10 concurrent consumers and parallel producers
// WITH UnsafeBatchWrites enabled.
func BenchmarkBroker_FanOut_Unsafe(b *testing.B) {
	// Setup
	tmpDir, err := os.MkdirTemp("", "bench_broker_unsafe_*")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	brokerInst, err := broker.New(broker.Config{
		PersistencePath:   filepath.Join(tmpDir, "events.log"),
		MaxLogSize:        100 * 1024 * 1024,
		MaxRetainedLogs:   5,
		UnsafeBatchWrites: true,
	})
	if err != nil {
		b.Fatal(err)
	}
	defer brokerInst.Close()

	// 1. Setup 10 Consumers
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("consumer-%d", i)
		subCh, _ := brokerInst.Subscribe(name, "")

		go func() {
			for msg := range subCh {
				msg.Ack()
			}
		}()
	}

	payload := make([]byte, 1024)
	streamName := "unsafe-stream"

	b.ResetTimer()

	// 2. Producers (High Concurrency)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := brokerInst.Send(streamName, payload); err != nil {
				b.Error(err)
			}
		}
	})

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "events/s")
}
