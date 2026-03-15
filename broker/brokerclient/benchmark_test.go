package brokerclient_test

import (
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"goose/broker"
	"goose/broker/brokerclient"
)

func BenchmarkClient_Send(b *testing.B) {
	// 1. Setup Broker
	tmpDir, err := os.MkdirTemp("", "bench_client_send")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Use minimal extensive config for performance (UnsafeBatchWrites=true if we want to test network more than disk)
	// But let's test realistic persistence: UnsafeBatchWrites=false by default, but maybe true for max throughput test.
	// Let's stick to default for now to be fair.
	// 2. Setup Manager
	manager, err := broker.NewManager(tmpDir)
	if err != nil {
		b.Fatal(err)
	}
	defer manager.Close()

	if _, err := manager.CreateBroker("default"); err != nil {
		b.Fatal(err)
	}

	// 2. Setup HTTP Server
	handler := broker.NewHandler(manager)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 3. Setup Client
	client := brokerclient.New(server.URL)
	payload := map[string]string{"data": "some-payload-data"}

	b.ResetTimer()
	b.SetParallelism(10) // 10 parallel producers

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := client.Send("bench-stream", payload); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkClient_EndToEnd(b *testing.B) {
	// Producer -> Broker -> Consumer
	// We want to measure throughput of full loop?
	// Benchmarks usually measure ops/sec of the loop body.
	// We can use a separate goroutine for consuming and count received events.

	// 1. Setup Manager
	tmpDir, err := os.MkdirTemp("", "bench_client_e2e")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	var manager *broker.BrokerManager
	if manager, err = broker.NewManager(tmpDir); err != nil {
		b.Fatal(err)
	}
	defer manager.Close()

	if _, err := manager.CreateBroker("default"); err != nil {
		b.Fatal(err)
	}

	// 2. Setup HTTP Server
	handler := broker.NewHandler(manager)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 3. Setup Client
	client := brokerclient.New(server.URL)
	payload := map[string]string{"data": "some-payload-data"}
	streamName := "e2e-stream"

	// 4. Start Consumer
	var receivedOps int64
	subClient := brokerclient.New(server.URL)
	subCh, cancel := subClient.Subscribe(streamName, "bench-consumer", "")
	defer cancel()

	// Consume in background
	go func() {
		for range subCh {
			atomic.AddInt64(&receivedOps, 1)
		}
	}()

	// Wait for subscription to be ready (approx)
	time.Sleep(200 * time.Millisecond)

	b.ResetTimer()
	b.SetParallelism(10)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := client.Send(streamName, payload); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.StopTimer()
	// Validation (Optional): check if receivedOps matches roughly b.N
	// Due to async nature, it might lag.
	// fmt.Printf("Sent: %d, Received: %d\n", b.N, atomic.LoadInt64(&receivedOps))
}
