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

// Client is a HTTP client for the Goose Broker
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a new broker client
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
			Timeout:   10 * time.Second, // Timeout for Send commands
		},
	}
}

// Send publishes an event to the stream
func (c *Client) Send(stream string, payload interface{}, requiredConsumers ...string) error {
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

	resp, err := c.httpClient.Post(c.baseURL+"/publish", "application/json", bytes.NewReader(data))
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
// It returns a channel of Deliveries and a CancelFunc to stop the subscription.
func (c *Client) Subscribe(stream, consumerID, fromID string) (<-chan Delivery, func()) {
	ch := make(chan Delivery, 100)
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		defer close(ch)

		// Build URL with params
		u, _ := url.Parse(c.baseURL + "/subscribe")
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

		// Use a custom long-timeout client/transport implies streaming?
		// NOTE: default httpClient has 10s timeout which is BAD for streaming.
		// We need a separate client or transport for default timeout.
		streamClient := &http.Client{
			Timeout: 0, // No timeout for streaming
		}

		resp, err := streamClient.Do(req)
		if err != nil {
			// Context canceled is normal
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
					// Check if likely connection closed perfectly
					fmt.Printf("Client stream decode error: %v\n", err)
				}
				return
			}

			// If user filtered by stream?
			// The /subscribe endpoint currently subscribes to ALL streams (global).
			// The broker.Subscribe method in http_transport calls b.Subscribe(clientID, fromID) which subscribes to everything?
			// Wait, broker.Subscribe(name, fromID) -> returns sub channel.
			// Is broker.Subscribe global or filtered?
			// In broker.go: Subscribe(name, fromID) subscribes to ALL events.
			// Filtering by 'stream' is usually done client-side or we need to improve broker?
			// The user asked "Subscribe(stream, consumerID)".
			// If implementation is global, we should filter here.
			if stream != "" && event.Stream != stream {
				continue
			}

			ch <- Delivery{
				Event: event,
				Ack:   func() {}, // No-op for now as HTTP transport auto-acks
			}
		}
	}()

	return ch, cancel
}
