package broker

import (
	"encoding/json"
	"net/http"
)

// NewHandler creates a new HTTP handler that serves the broker's event stream.
func NewHandler(b *Broker) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Set Headers for Streaming
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Transfer-Encoding", "chunked")

		// 2. Subscribe to Broker with unique name
		// For anonymous HTTP streams, we generate a unique ID.
		clientID := "http-client-" + r.RemoteAddr
		events, unsubscribe := b.Subscribe(clientID)
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
}
