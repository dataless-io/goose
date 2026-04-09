package broker

import (
	"context"
	"errors"
	"fmt"
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

type PutMessageRequest struct {
	Stream            string      `json:"stream"`
	Payload           interface{} `json:"payload"`
	RequiredConsumers []string    `json:"required_consumers"`
}

type BrokerResponse struct {
	Name string `json:"name"`
}

type CreateTopicRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	RetentionHours int    `json:"retention_hours"`
}

type UpdateTopicRequest struct {
	Description    *string `json:"description,omitempty"`
	RetentionHours *int    `json:"retention_hours,omitempty"`
	Disabled       *bool   `json:"disabled,omitempty"`
}

type BrokerDetailsResponse struct {
	Name         string `json:"name"`
	DisplayName  string `json:"display_name,omitempty"`
	Description  string `json:"description,omitempty"`
	Disabled     bool   `json:"disabled,omitempty"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	TopicsCount  int    `json:"topics_count"`
	Subscribers  int    `json:"subscribers"`
	Published    uint64 `json:"published_messages"`
	RequiredAcks uint64 `json:"required_acks"`
}

type UpdateBrokerRequest struct {
	DisplayName *string `json:"display_name,omitempty"`
	Description *string `json:"description,omitempty"`
	Disabled    *bool   `json:"disabled,omitempty"`
}

// NewHandler creates an HTTP handler exposing the broker manager API.
func NewHandler(m *BrokerManager) http.Handler {
	b := box.NewBox()

	b.Handle("GET", "/brokers", func(ctx context.Context) ([]string, error) {
		return m.ListBrokers(), nil
	})

	b.Handle("GET", "/brokers/details", func(ctx context.Context) ([]BrokerDetailsResponse, error) {
		names := m.ListBrokers()
		out := make([]BrokerDetailsResponse, 0, len(names))
		for _, name := range names {
			bk, ok := m.GetBroker(name)
			if !ok {
				continue
			}
			settings, _ := m.GetBrokerSettings(name)
			metrics := bk.Metrics()
			out = append(out, BrokerDetailsResponse{
				Name:         name,
				DisplayName:  settings.DisplayName,
				Description:  settings.Description,
				Disabled:     settings.Disabled,
				CreatedAt:    settings.CreatedAt,
				UpdatedAt:    settings.UpdatedAt,
				TopicsCount:  metrics.TopicsCount,
				Subscribers:  metrics.ActiveSubscribers,
				Published:    metrics.PublishedMessages,
				RequiredAcks: metrics.RequiredAcks,
			})
		}
		return out, nil
	})

	b.Handle("POST", "/brokers", func(ctx context.Context, req *CreateBrokerRequest) (*BrokerResponse, error) {
		if req.Name == "" {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, errors.New("name required")
		}

		if _, err := m.CreateBroker(req.Name); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict)
			return nil, err
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return &BrokerResponse{Name: req.Name}, nil
	})

	b.Handle("GET", "/brokers/{name}", func(ctx context.Context) (*BrokerResponse, error) {
		name := box.GetUrlParameter(ctx, "name")
		if _, exists := m.GetBroker(name); !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		return &BrokerResponse{Name: name}, nil
	})

	b.Handle("GET", "/brokers/{name}/settings", func(ctx context.Context) (*BrokerSettings, error) {
		name := box.GetUrlParameter(ctx, "name")
		if _, exists := m.GetBroker(name); !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		settings, _ := m.GetBrokerSettings(name)
		return &settings, nil
	})

	b.Handle("PUT", "/brokers/{name}/settings", func(ctx context.Context, req *UpdateBrokerRequest) (*BrokerSettings, error) {
		name := box.GetUrlParameter(ctx, "name")
		settings, err := m.UpdateBrokerSettings(name, BrokerSettingsUpdate{
			DisplayName: req.DisplayName,
			Description: req.Description,
			Disabled:    req.Disabled,
		})
		if err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, err
		}
		return &settings, nil
	})

	b.Handle("PATCH", "/brokers/{name}/settings", func(ctx context.Context, req *UpdateBrokerRequest) (*BrokerSettings, error) {
		name := box.GetUrlParameter(ctx, "name")
		settings, err := m.UpdateBrokerSettings(name, BrokerSettingsUpdate{
			DisplayName: req.DisplayName,
			Description: req.Description,
			Disabled:    req.Disabled,
		})
		if err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, err
		}
		return &settings, nil
	})

	b.Handle("GET", "/brokers/{name}/metrics", func(ctx context.Context) (*BrokerMetrics, error) {
		name := box.GetUrlParameter(ctx, "name")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		metrics := broker.Metrics()
		return &metrics, nil
	})

	b.Handle("GET", "/brokers/{name}/topics", func(ctx context.Context) ([]Topic, error) {
		name := box.GetUrlParameter(ctx, "name")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		return broker.ListTopics(), nil
	})

	b.Handle("POST", "/brokers/{name}/topics", func(ctx context.Context, req *CreateTopicRequest) (*Topic, error) {
		name := box.GetUrlParameter(ctx, "name")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}

		topic, err := broker.CreateTopic(req.Name, req.Description, req.RetentionHours)
		if err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, err
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return &topic, nil
	})

	b.Handle("GET", "/brokers/{name}/topics/{topic}", func(ctx context.Context) (*Topic, error) {
		name := box.GetUrlParameter(ctx, "name")
		topicName := box.GetUrlParameter(ctx, "topic")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		topic, ok := broker.GetTopic(topicName)
		if !ok {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("topic not found")
		}
		return &topic, nil
	})

	b.Handle("PUT", "/brokers/{name}/topics/{topic}", func(ctx context.Context, req *UpdateTopicRequest) (*Topic, error) {
		name := box.GetUrlParameter(ctx, "name")
		topicName := box.GetUrlParameter(ctx, "topic")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		topic, err := broker.UpdateTopic(topicName, TopicUpdate{
			Description:    req.Description,
			RetentionHours: req.RetentionHours,
			Disabled:       req.Disabled,
		})
		if err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, err
		}
		return &topic, nil
	})

	b.Handle("PATCH", "/brokers/{name}/topics/{topic}", func(ctx context.Context, req *UpdateTopicRequest) (*Topic, error) {
		name := box.GetUrlParameter(ctx, "name")
		topicName := box.GetUrlParameter(ctx, "topic")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return nil, errors.New("broker not found")
		}
		topic, err := broker.UpdateTopic(topicName, TopicUpdate{
			Description:    req.Description,
			RetentionHours: req.RetentionHours,
			Disabled:       req.Disabled,
		})
		if err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return nil, err
		}
		return &topic, nil
	})

	b.Handle("DELETE", "/brokers/{name}/topics/{topic}", func(ctx context.Context) error {
		name := box.GetUrlParameter(ctx, "name")
		topicName := box.GetUrlParameter(ctx, "topic")
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return errors.New("broker not found")
		}
		if err := broker.DeleteTopic(topicName); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return err
		}
		box.GetResponse(ctx).WriteHeader(http.StatusNoContent)
		return nil
	})

	b.Handle("DELETE", "/brokers/{name}", func(ctx context.Context) error {
		name := box.GetUrlParameter(ctx, "name")
		if err := m.DeleteBroker(name); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return err
		}
		box.GetResponse(ctx).WriteHeader(http.StatusOK)
		return nil
	})

	b.Handle("POST", "/brokers/{name}/publish", func(ctx context.Context, req *PublishRequest) error {
		name := box.GetUrlParameter(ctx, "name")
		if m.IsBrokerDisabled(name) {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict)
			return errors.New("broker is disabled")
		}
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return errors.New("broker not found")
		}
		if req.Stream == "" {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return errors.New("stream required")
		}

		if err := broker.Send(req.Stream, req.Payload, req.RequiredConsumers...); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict)
			return fmt.Errorf("failed to send event: %w", err)
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return nil
	})

	b.Handle("PUT", "/brokers/{name}/messages", func(ctx context.Context, req *PutMessageRequest) error {
		name := box.GetUrlParameter(ctx, "name")
		if m.IsBrokerDisabled(name) {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict)
			return errors.New("broker is disabled")
		}
		broker, exists := m.GetBroker(name)
		if !exists {
			box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
			return errors.New("broker not found")
		}
		if req.Stream == "" {
			box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
			return errors.New("stream required")
		}

		if err := broker.Send(req.Stream, req.Payload, req.RequiredConsumers...); err != nil {
			box.GetResponse(ctx).WriteHeader(http.StatusConflict)
			return fmt.Errorf("failed to commit message: %w", err)
		}

		box.GetResponse(ctx).WriteHeader(http.StatusCreated)
		return nil
	})

	b.Handle("GET", "/brokers/{name}/subscribe", func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		name := box.GetUrlParameter(ctx, "name")
		if m.IsBrokerDisabled(name) {
			http.Error(w, "broker is disabled", http.StatusConflict)
			return
		}
		broker, exists := m.GetBroker(name)
		if !exists {
			http.Error(w, "broker not found", http.StatusNotFound)
			return
		}
		streamFilter := r.URL.Query().Get("stream")
		handleSubscribe(w, r, broker, streamFilter)
	})

	return b
}

func handleSubscribe(w http.ResponseWriter, r *http.Request, b *Broker, streamFilter string) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

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

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}
	flusher.Flush()

	encoder := jsontext.NewEncoder(w)
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-events:
			if !ok {
				return
			}
			if streamFilter != "" && delivery.Event.Stream != streamFilter {
				delivery.Ack()
				continue
			}
			if err := json.MarshalEncode(encoder, delivery.Event); err != nil {
				return
			}
			_, _ = w.Write([]byte{'\n'})
			flusher.Flush()
			delivery.Ack()
		}
	}
}
