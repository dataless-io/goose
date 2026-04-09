package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type checkoutRequest struct {
	OrderID string  `json:"order_id"`
	Amount  float64 `json:"amount"`
}

type brokerEvent struct {
	ID      string          `json:"id"`
	Stream  string          `json:"stream"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	brokerURL := env("BROKER_URL", "http://127.0.0.1:8081")
	brokerName := env("BROKER_NAME", "default")
	topic := env("BROKER_TOPIC", "checkout-orders")
	apiKey := strings.TrimSpace(os.Getenv("BROKER_API_KEY"))
	consumerID := env("BROKER_CONSUMER_ID", "checkout-worker")
	addr := env("DEMO_HTTP_ADDR", ":8091")

	client := &http.Client{Timeout: 15 * time.Second}

	if err := ensureBrokerAndTopic(client, brokerURL, brokerName, topic, apiKey); err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runCheckoutConsumer(ctx, brokerURL, brokerName, topic, consumerID)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /checkout", func(w http.ResponseWriter, r *http.Request) {
		var req checkoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(req.OrderID) == "" {
			http.Error(w, "order_id is required", http.StatusBadRequest)
			return
		}

		msg := map[string]any{
			"order_id":   req.OrderID,
			"amount":     req.Amount,
			"created_at": time.Now().UTC().Format(time.RFC3339Nano),
		}

		started := time.Now()
		if err := publishWithCommit(client, brokerURL, brokerName, topic, apiKey, msg, []string{consumerID}); err != nil {
			http.Error(w, "broker publish failed: "+err.Error(), http.StatusConflict)
			return
		}

		resp := map[string]any{
			"status":         "accepted",
			"order_id":       req.OrderID,
			"commit_latency": time.Since(started).String(),
			"topic":          topic,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	server := &http.Server{Addr: addr, Handler: mux}
	log.Printf("checkout demo listening on %s", addr)
	log.Printf("checkout demo using broker=%s brokerName=%s topic=%s consumer=%s", brokerURL, brokerName, topic, consumerID)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("demo server failed: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	cancel()
	ctxShutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	_ = server.Shutdown(ctxShutdown)
}

func ensureBrokerAndTopic(client *http.Client, brokerURL, brokerName, topic, apiKey string) error {
	createBrokerBody := []byte(`{"name":"` + brokerName + `"}`)
	req, _ := http.NewRequest(http.MethodPost, strings.TrimRight(brokerURL, "/")+"/brokers", bytes.NewReader(createBrokerBody))
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}

	topicBody := []byte(`{"name":"` + topic + `","description":"checkout orders","retention_hours":168}`)
	reqTopic, _ := http.NewRequest(http.MethodPost, strings.TrimRight(brokerURL, "/")+"/brokers/"+brokerName+"/topics", bytes.NewReader(topicBody))
	reqTopic.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		reqTopic.Header.Set("X-API-Key", apiKey)
	}
	respTopic, err := client.Do(reqTopic)
	if err != nil {
		return err
	}
	defer respTopic.Body.Close()
	if respTopic.StatusCode != http.StatusCreated && respTopic.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(respTopic.Body)
		return fmt.Errorf("create topic failed: %d %s", respTopic.StatusCode, string(body))
	}

	return nil
}

func publishWithCommit(client *http.Client, brokerURL, brokerName, topic, apiKey string, payload map[string]any, required []string) error {
	bodyBytes, err := json.Marshal(map[string]any{
		"stream":             topic,
		"payload":            payload,
		"required_consumers": required,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, strings.TrimRight(brokerURL, "/")+"/brokers/"+brokerName+"/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("publish failed: %d %s", resp.StatusCode, string(body))
	}
	return nil
}

func runCheckoutConsumer(ctx context.Context, brokerURL, brokerName, topic, consumerID string) {
	url := fmt.Sprintf("%s/brokers/%s/subscribe?id=%s&stream=%s", strings.TrimRight(brokerURL, "/"), brokerName, consumerID, topic)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("consumer connect failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
		for scanner.Scan() {
			var evt brokerEvent
			if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
				continue
			}
			log.Printf("consumer received topic=%s id=%s payload=%s", evt.Stream, evt.ID, string(evt.Payload))
			time.Sleep(120 * time.Millisecond)
		}
		_ = resp.Body.Close()
		time.Sleep(1 * time.Second)
	}
}

func env(name, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
