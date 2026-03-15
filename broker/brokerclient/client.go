package brokerclient

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"goose/broker"
)

// Client is a HTTP client for the Goose Broker Service
type Client struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

// New creates a new broker service client
func New(baseURL string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		headers: make(map[string]string),
	}
}

// WithHeader adds a default header to all requests
func (c *Client) WithHeader(key, value string) *Client {
	c.headers[key] = value
	return c
}

// CreateBroker creates a new broker with the given name
func (c *Client) CreateBroker(name string) error {
	reqBody := struct {
		Name string `json:"name"`
	}{Name: name}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.baseURL+"/brokers", bytes.NewReader(data))
	if err != nil {
		return err
	}
	c.injectHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create broker failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// DeleteBroker deletes a broker
func (c *Client) DeleteBroker(name string) error {
	req, err := http.NewRequest("DELETE", c.baseURL+"/brokers/"+name, nil)
	if err != nil {
		return err
	}
	c.injectHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete broker failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// Broker returns a client scoped to a specific broker
func (c *Client) Broker(name string) *BrokerClient {
	return &BrokerClient{
		client:     c,
		brokerName: name,
	}
}

// injectHeaders adds configured headers to the request
func (c *Client) injectHeaders(req *http.Request) {
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
}

// Send is a convenience method for the "default" broker (backward compatibility)
func (c *Client) Send(stream string, payload interface{}, requiredConsumers ...string) error {
	return c.Broker("default").Send(stream, payload, requiredConsumers...)
}

// Subscribe is a convenience method for the "default" broker
func (c *Client) Subscribe(stream, consumerID, fromID string) (<-chan Delivery, func()) {
	return c.Broker("default").Subscribe(stream, consumerID, fromID)
}

// BrokerClient handles operations on a specific broker
type BrokerClient struct {
	client     *Client
	brokerName string
}

// Send publishes an event to the stream on this broker
func (b *BrokerClient) Send(stream string, payload interface{}, requiredConsumers ...string) error {
	reqBody := struct {
		Stream            string      `json:"stream"`
		Payload           interface{} `json:"payload"`
		RequiredConsumers []string    `json:"required_consumers"`
	}{
		Stream:            stream,
		Payload:           payload,
		RequiredConsumers: requiredConsumers,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/brokers/%s/publish", b.client.baseURL, b.brokerName)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	b.client.injectHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to post event: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("broker error (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Delivery mimics broker.Delivery but for client-side usage
type Delivery struct {
	broker.Event
	Ack func()
}

// Subscribe connects to the broker and streams events.
func (b *BrokerClient) Subscribe(stream, consumerID, fromID string) (<-chan Delivery, func()) {
	ch := make(chan Delivery, 100)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(ch)

		// Build URL with params
		u, _ := url.Parse(fmt.Sprintf("%s/brokers/%s/subscribe", b.client.baseURL, b.brokerName))
		q := u.Query()
		if consumerID != "" {
			q.Set("id", consumerID)
		}
		if fromID != "" {
			q.Set("from_id", fromID)
		}
		u.RawQuery = q.Encode()

		// Create request with context for cancellation
		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			fmt.Printf("Client subscribe error: %v\n", err)
			return
		}
		b.client.injectHeaders(req)

		// Use a custom long-timeout client/transport implies streaming?
		streamClient := &http.Client{
			Timeout: 0, // No timeout for streaming
		}

		resp, err := streamClient.Do(req)
		if err != nil {
			if ctx.Err() == nil {
				fmt.Printf("Client request failed: %v\n", err)
			}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Printf("Client subscribe failed (%d): %s\n", resp.StatusCode, string(body))
			return
		}

		// Stream Processor
		decoder := jsontext.NewDecoder(resp.Body)
		for {
			if ctx.Err() != nil {
				return
			}

			var event broker.Event
			if err := json.UnmarshalDecode(decoder, &event); err != nil {
				if err != io.EOF && ctx.Err() == nil {
					fmt.Printf("Client stream decode error: %v\n", err)
				}
				return
			}

			if stream != "" && event.Stream != stream {
				continue
			}

			ch <- Delivery{
				Event: event,
				Ack:   func() {},
			}
		}
	}()

	return ch, cancel
}
