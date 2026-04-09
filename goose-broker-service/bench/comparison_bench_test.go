package bench

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/IBM/sarama"
	"github.com/nats-io/nats.go"

	"github.com/dataless-io/goose-broker-service/broker"
)

func BenchmarkGooseHTTP_Publish(b *testing.B) {
	manager, err := broker.NewManager(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer manager.Close()

	bk, err := manager.CreateBroker("default")
	if err != nil {
		b.Fatal(err)
	}
	if _, err := bk.CreateTopic("bench", "", 24); err != nil {
		b.Fatal(err)
	}

	server := httptest.NewServer(broker.NewHandler(manager))
	defer server.Close()

	body := []byte(`{"stream":"bench","payload":{"source":"goose"}}`)
	client := &http.Client{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest(http.MethodPut, server.URL+"/brokers/default/messages", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				b.Error(err)
				continue
			}
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				b.Errorf("unexpected status: %d", resp.StatusCode)
			}
		}
	})
}

func BenchmarkNATS_Publish(b *testing.B) {
	natsURL := strings.TrimSpace(os.Getenv("NATS_URL"))
	if natsURL == "" {
		b.Skip("NATS_URL not set")
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		b.Fatalf("nats connect: %v", err)
	}
	defer nc.Close()

	payload := []byte(`{"source":"nats"}`)
	subject := "bench.goose.compare"

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if err := nc.Publish(subject, payload); err != nil {
				b.Error(err)
			}
		}
	})
	_ = nc.Flush()
}

func BenchmarkKafka_Publish(b *testing.B) {
	brokersRaw := strings.TrimSpace(os.Getenv("KAFKA_BROKERS"))
	if brokersRaw == "" {
		b.Skip("KAFKA_BROKERS not set")
	}
	topic := strings.TrimSpace(os.Getenv("KAFKA_TOPIC"))
	if topic == "" {
		topic = "goose-bench"
	}

	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.RequiredAcks = sarama.WaitForLocal

	producer, err := sarama.NewSyncProducer(strings.Split(brokersRaw, ","), cfg)
	if err != nil {
		b.Fatalf("kafka connect: %v", err)
	}
	defer producer.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := &sarama.ProducerMessage{Topic: topic, Value: sarama.StringEncoder(`{"source":"kafka"}`)}
			_, _, err := producer.SendMessage(msg)
			if err != nil {
				b.Error(err)
			}
		}
	})
}

func BenchmarkComparisonSummary(b *testing.B) {
	b.Skip(fmt.Sprintf("Run explicit targets: %s", "BenchmarkGooseHTTP_Publish|BenchmarkNATS_Publish|BenchmarkKafka_Publish"))
}
