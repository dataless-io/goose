package broker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Client interacts with a remote Broker via HTTP
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewClient creates a new Broker Client
func NewClient(baseURL string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{},
	}
}

// Send publishes an event to the remote broker
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

	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+"/publish", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

// Subscribe listens to the remote broker stream.
// Returns a channel of Events (Delivery is implicit as Ack is handled by the server for now, or client logic).
// Note: The generic 'Delivery' struct in broker package has Ack(), but for remote client, the server does auto-ack
// when writing to stream (as per current http_transport implementation).
// So client just receives Events.
func (c *Client) Subscribe(consumerName string) (<-chan Event, func(), error) {
	url := fmt.Sprintf("%s/subscribe?id=%s", c.BaseURL, consumerName)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	ch := make(chan Event, 100)
	done := make(chan struct{})

	// We need to keep a reference to close body on cancel
	// to interrupt the scanner.

	go func() {
		defer close(ch)
		// We don't defer resp.Body.Close() here because we might close it manually in cancel.
		// Or we can do it, calling Close multiple times is safe? Yes usually.
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			select {
			case <-done:
				return
			default:
				var event Event
				if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
					// Logic: stop or continue?
					continue
				}
				ch <- event
			}
		}
	}()

	cancel := func() {
		close(done)
		resp.Body.Close() // Force read failure to unblock Scanner
	}

	return ch, cancel, nil
}
