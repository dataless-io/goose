package broker

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Config holds the configuration for the broker
type Config struct {
	PersistencePath string
}

// Event represents a single unit of data in the system
type Event struct {
	ID        string          `json:"id"`
	Stream    string          `json:"stream"`
	Timestamp int64           `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

// Broker acts as the central event bus
type Broker struct {
	config Config
	mutex  sync.Mutex // Protects concurrent writes to log
	log    *Log
}

// New creates a new Broker instance
func New(config Config) (*Broker, error) {
	log, err := openLog(config.PersistencePath)
	if err != nil {
		return nil, err
	}

	return &Broker{
		config: config,
		log:    log,
	}, nil
}

// Send publishes a new event to the specified stream.
// It persists the event to the append-only log before returning.
func (b *Broker) Send(stream string, payload interface{}) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	event := Event{
		ID:        uuid.New().String(),
		Stream:    stream,
		Timestamp: time.Now().UnixNano(),
		Payload:   payloadBytes,
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()

	return b.log.Append(event)
}

// Close handles the graceful shutdown of the broker
func (b *Broker) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.log.Close()
}
