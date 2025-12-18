package broker

import (
	"encoding/json"
	"net/http"
)

// NewHandler creates a new HTTP handler that serves the broker's event stream.
func NewHandler(b *Broker) http.Handler {
	mux := http.NewServeMux()

	// GET /subscribe - Listen to events
	mux.HandleFunc("/subscribe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 1. Set Headers for Streaming
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		// 2. Subscribe to Broker with unique name
		// For anonymous HTTP streams, we generate a unique ID.
		clientID := "http-client-" + r.RemoteAddr
		// Allow overriding clientID via query param for identifying consumers
		if queryID := r.URL.Query().Get("id"); queryID != "" {
			clientID = queryID
		}
		fromID := r.URL.Query().Get("from_id")
		if fromID == "" {
			fromID = r.Header.Get("Last-Event-ID")
		}

		events, unsubscribe := b.Subscribe(clientID, fromID)
		defer unsubscribe()

		// 3. Flush headers immediately to establish stream
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		flusher.Flush()

		encoder := json.NewEncoder(w)

		// 4. Stream Loop
		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				// Client disconnected
				return
			case delivery := <-events:
				// Write JSON Line
				if err := encoder.Encode(delivery.Event); err != nil {
					return
				}
				flusher.Flush()

				// Ack the delivery (we consider it delivered once written to stream)
				delivery.Ack()
			}
		}
	})

	// POST /publish - Publish a new event
	mux.HandleFunc("/publish", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Stream            string      `json:"stream"`
			Payload           interface{} `json:"payload"`
			RequiredConsumers []string    `json:"required_consumers"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if err := b.Send(req.Stream, req.Payload, req.RequiredConsumers...); err != nil {
			http.Error(w, "Failed to send event: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	})

	return mux
}
