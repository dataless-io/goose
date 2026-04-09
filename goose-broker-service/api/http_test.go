package api_test

import (
	"bufio"
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dataless-io/goose-broker-service/api"
	"github.com/dataless-io/goose-broker-service/broker"
)

func setupServer(t *testing.T, cfg api.Config) (*broker.BrokerManager, *httptest.Server) {
	t.Helper()
	m, err := broker.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if _, err := m.CreateBroker("default"); err != nil {
		t.Fatalf("create default broker: %v", err)
	}

	srv := httptest.NewServer(api.NewHTTPHandler(m, cfg))
	t.Cleanup(func() {
		srv.Close()
		_ = m.Close()
	})

	return m, srv
}

func TestPutMessageBlocksUntilRequiredConsumerCommits(t *testing.T) {
	m, srv := setupServer(t, api.Config{})

	b, _ := m.GetBroker("default")
	ch, cancel := b.Subscribe("worker-a", "")
	defer cancel()

	go func() {
		d := <-ch
		time.Sleep(140 * time.Millisecond)
		d.Ack()
	}()

	body := []byte(`{"stream":"events","payload":{"msg":"hola"},"required_consumers":["worker-a"]}`)
	req, err := http.NewRequest(http.MethodPut, srv.URL+"/brokers/default/messages", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	if elapsed < 120*time.Millisecond {
		t.Fatalf("request returned too early, elapsed=%v", elapsed)
	}
}

func TestPutMessageFailsWhenRequiredConsumerMissing(t *testing.T) {
	_, srv := setupServer(t, api.Config{})
	body := []byte(`{"stream":"events","payload":{"msg":"hola"},"required_consumers":["ghost"]}`)
	resp, err := http.DefaultClient.Post(srv.URL+"/brokers/default/publish", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for publish endpoint, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/brokers/default/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for put endpoint, got %d", resp2.StatusCode)
	}
}

func TestWriteAuthAndReadAccess(t *testing.T) {
	_, srv := setupServer(t, api.Config{APIKey: "top-secret"})

	readResp, err := http.Get(srv.URL + "/brokers")
	if err != nil {
		t.Fatal(err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for reads, got %d", readResp.StatusCode)
	}

	writeBody := []byte(`{"name":"payments"}`)
	unauthReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/brokers", bytes.NewReader(writeBody))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthResp, err := http.DefaultClient.Do(unauthReq)
	if err != nil {
		t.Fatal(err)
	}
	defer unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without API key, got %d", unauthResp.StatusCode)
	}

	authReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/brokers", bytes.NewReader(writeBody))
	authReq.Header.Set("Content-Type", "application/json")
	authReq.Header.Set("X-API-Key", "top-secret")
	authResp, err := http.DefaultClient.Do(authReq)
	if err != nil {
		t.Fatal(err)
	}
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 with API key, got %d", authResp.StatusCode)
	}
}

func TestStaticUIIsServed(t *testing.T) {
	_, srv := setupServer(t, api.Config{})

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for index, got %d", resp.StatusCode)
	}

	assetResp, err := http.Get(srv.URL + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	defer assetResp.Body.Close()

	if assetResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for js asset, got %d", assetResp.StatusCode)
	}
}

func TestTopicEndpointsAndMetrics(t *testing.T) {
	_, srv := setupServer(t, api.Config{})

	createTopicBody := []byte(`{"name":"orders","description":"orders flow","retention_hours":48}`)
	resp, err := http.Post(srv.URL+"/brokers/default/topics", "application/json", bytes.NewReader(createTopicBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 creating topic, got %d", resp.StatusCode)
	}

	publishBody := []byte(`{"stream":"orders","payload":{"id":"o-1"}}`)
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/brokers/default/messages", bytes.NewReader(publishBody))
	req.Header.Set("Content-Type", "application/json")
	pubResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 publishing to topic, got %d", pubResp.StatusCode)
	}

	metricsResp, err := http.Get(srv.URL + "/brokers/default/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer metricsResp.Body.Close()
	if metricsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 metrics, got %d", metricsResp.StatusCode)
	}
}

func TestSubscribeSupportsTopicFilter(t *testing.T) {
	m, srv := setupServer(t, api.Config{})
	b, _ := m.GetBroker("default")
	if _, err := b.CreateTopic("alerts", "", 24); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(srv.URL + "/brokers/default/subscribe?stream=alerts&id=ui-consumer")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	go func() {
		_ = b.Send("events", map[string]any{"id": "e1"})
		_ = b.Send("alerts", map[string]any{"id": "a1"})
	}()

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		t.Fatalf("expected at least one filtered message")
	}
	line := scanner.Text()
	if !bytes.Contains([]byte(line), []byte(`"stream":"alerts"`)) {
		t.Fatalf("expected filtered alerts stream, got: %s", line)
	}
}

func TestBrokerSettingsEndpoints(t *testing.T) {
	_, srv := setupServer(t, api.Config{})

	body := []byte(`{"display_name":"Primary","description":"for production","disabled":true}`)
	req, _ := http.NewRequest(http.MethodPatch, srv.URL+"/brokers/default/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 patch settings, got %d", resp.StatusCode)
	}

	getResp, err := http.Get(srv.URL + "/brokers/default/settings")
	if err != nil {
		t.Fatal(err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 get settings, got %d", getResp.StatusCode)
	}

	publishBody := []byte(`{"stream":"events","payload":{"msg":"blocked"}}`)
	putReq, _ := http.NewRequest(http.MethodPut, srv.URL+"/brokers/default/messages", bytes.NewReader(publishBody))
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatal(err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 when broker disabled, got %d", putResp.StatusCode)
	}
}
