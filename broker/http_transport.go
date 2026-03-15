package broker

import (
	"context"
	"errors"
	"net/http"

	"github.com/fulldump/box"
	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
)

type CreateBrokerRequest struct {
	Name string `json:"name"`
}

type PublishRequest struct {
	Stream            string      `json:"stream"`
	Payload           interface{} `json:"payload"`
	RequiredConsumers []string    `json:"required_consumers"`
}

type BrokerResponse struct {
	Name string `json:"name"`
}

// NewHandler creates a new HTTP handler that serves the broker manager API.
func NewHandler(m *BrokerManager) http.Handler {
	b := box.NewBox()

	// 1. Broker Management APIs

	// GET /brokers - List brokers
	b.Handle("GET", "/brokers", func(ctx context.Context) ([]string, error) {
		return m.ListBrokers(), nil
	})

	// POST /brokers - Create new broker
	b.Handle("POST", "/brokers", func(ctx context.Context, req *CreateBrokerRequest) (*BrokerResponse, error) {
		if req.Name == "" {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, errors.New("name required")
		}

		if _, err := m.CreateBroker(req.Name); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict) // Assuming existing causes error
			return nil, err
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return &BrokerResponse{Name: req.Name}, nil
	})

	// GET /brokers/{name} - Get broker details
	b.Handle("GET", "/brokers/{name}", func(ctx context.Context) (*BrokerResponse, error) {
		name := box.GetUrlParameter(ctx, "name")
		if _, exists := m.GetBroker(name); !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		return &BrokerResponse{Name: name}, nil
	})

	// DELETE /brokers/{name}
	b.Handle("DELETE", "/brokers/{name}", func(ctx context.Context) error {
		name := box.GetUrlParameter(ctx, "name")
		if err := m.DeleteBroker(name); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return err
		}
		box.GetResponse(ctx).WriteHeader(http.StatusOK)
		return nil
	})

	// POST /brokers/{name}/publish
	b.Handle("POST", "/brokers/{name}/publish", func(ctx context.Context, req *PublishRequest) error {
		name := box.GetUrlParameter(ctx, "name")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return errors.New("broker not found")
		}

		if err := broker.Send(req.Stream, req.Payload, req.RequiredConsumers...); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusInternalServerError)
			return errors.New("failed to send event: " + err.Error())
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return nil
	})

	// GET /brokers/{name}/subscribe
	b.Handle("GET", "/brokers/{name}/subscribe", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		name := box.GetUrlParameter(ctx, "name")
		broker, exists := m.GetBroker(name)
		if !exists {
			http.Error(w, "Broker not found", http.StatusNotFound)
			return
		}
		handleSubscribe(w, r, broker)
	})

	return b
}

func handleSubscribe(w http.ResponseWriter, r *http.Request, b *Broker) {
	// 1. Set Headers for Streaming
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	// 2. Subscribe to Broker with unique name
	clientID := "http-client-" + r.RemoteAddr
	if queryID := r.URL.Query().Get("id"); queryID != "" {
		clientID = queryID
	}
	fromID := r.URL.Query().Get("from_id")
	if fromID == "" {
		fromID = r.Header.Get("Last-Event-ID")
	}

	events, unsubscribe := b.Subscribe(clientID, fromID)
	defer unsubscribe()

	// 3. Flush headers
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	encoder := jsontext.NewEncoder(w)

	// 4. Stream Loop
	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case delivery := <-events:
			if err := json.MarshalEncode(encoder, delivery.Event); err != nil {
				return
			}
			w.Write([]byte{'\n'})
			flusher.Flush()
			delivery.Ack()
		}
	}
}
