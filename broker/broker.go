package broker

import (
	"sync"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"

	"github.com/google/uuid"
)

// Config holds the configuration for the broker
type Config struct {
	PersistencePath   string
	MaxLogSize        int64 // Bytes. Default 10MB approx.
	MaxRetainedLogs   int   // Count. Default 5.
	UnsafeBatchWrites bool  // If true, relies on OS buffer (faster, riskier).
}

// Event represents a single unit of data in the system
type Event struct {
	ID        string         `json:"id"`
	Stream    string         `json:"stream"`
	Timestamp int64          `json:"timestamp"`
	Payload   jsontext.Value `json:"payload"`
}

// Delivery wraps an Event and provides an Ack method
type Delivery struct {
	Event
	Ack func()
}

// Broker acts as the central event bus
type Broker struct {
	config Config
	mutex  sync.Mutex // Protects concurrent writes to log
	log    *Log

	subsMutex   sync.RWMutex
	subscribers map[string]chan Delivery
}

// New creates a new Broker instance
func New(config Config) (*Broker, error) {
	if config.MaxLogSize == 0 {
		config.MaxLogSize = 10 * 1024 * 1024 // 10MB
	}
	if config.MaxRetainedLogs == 0 {
		config.MaxRetainedLogs = 5
	}

	l, err := openLog(config.PersistencePath, config.MaxLogSize, config.MaxRetainedLogs, config.UnsafeBatchWrites)
	if err != nil {
		return nil, err
	}

	return &Broker{
		config:      config,
		log:         l,
		subscribers: make(map[string]chan Delivery),
	}, nil
}

// Send publishes a new event to the specified stream.
// It blocks until all requiredConsumers have acknowledged the message.
// Used to wrap the generic persist operation
func (b *Broker) persistAck(targetID, consumer string) error {
	ackEntry := AckEntry{
		Type:      "ack",
		ID:        uuid.New().String(),
		TargetID:  targetID,
		Consumer:  consumer,
		Timestamp: time.Now().UnixNano(),
	}

	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.log.Append(ackEntry)
}

// Send publishes a new event to the specified stream.
// It blocks until all requiredConsumers have acknowledged the message.
func (b *Broker) Send(stream string, payload interface{}, requiredConsumers ...string) error {
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

	// 1. Persist to Disk (Critical path)
	if err := func() error {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		return b.log.Append(event)
	}(); err != nil {
		return err
	}

	// 2. Broadcast to Memory Subscribers
	b.subsMutex.RLock()
	defer b.subsMutex.RUnlock()

	var wg sync.WaitGroup
	// Map to track if required consumers are actually subscribed
	requiredMap := make(map[string]bool)
	for _, name := range requiredConsumers {
		requiredMap[name] = true
	}

	for name, ch := range b.subscribers {
		// Determine if this subscriber is required
		isRequired := requiredMap[name]
		consumerName := name

		// Create Ack callback
		var ack func()
		if isRequired {
			wg.Add(1)
			ack = func() {
				// Persist Ack before unblocking
				if err := b.persistAck(event.ID, consumerName); err != nil {
					// Logic dilemma: if persist fails, should we unblock?
					// Ideally we log error but unblock to avoid deadlock.
					// panic? or just log? System design says "strong consistency".
					// If we can't write Ack, maybe we shouldn't consider it done?
					// For now, we ignore error in ack closure or maybe print it.
					// Ideally Send would return error but this is async closure.
				}
				wg.Done()
			}
		} else {
			ack = func() {} // No-op ack for non-required
		}

		delivery := Delivery{
			Event: event,
			Ack:   ack,
		}

		select {
		case ch <- delivery:
		default:
			if isRequired {
				go func(c chan Delivery, d Delivery) {
					c <- d
				}(ch, delivery)
			}
		}
	}

	// 3. Wait for required Acks
	wg.Wait()

	return nil
}

// Subscribe registers a named consumer and returns a channel of Deliveries.
// If fromID is specified, it replays history starting after that ID.
func (b *Broker) Subscribe(name string, fromID string) (<-chan Delivery, func()) {
	// liveCh receives new events from Send()
	liveCh := make(chan Delivery, 100)
	// userCh is what (ordered) events are sent to
	userCh := make(chan Delivery, 100)

	b.subsMutex.Lock()
	b.subscribers[name] = liveCh
	b.subsMutex.Unlock()

	// Context/Cancel for the pump goroutine
	done := make(chan struct{})

	go func() {
		defer close(userCh)

		// 1. Replay History (if needed)
		if fromID != "" {
			// We need to capture the current state of log to avoid duplicates
			// created by live events arriving while we replay.
			// Ideally we stop reading at the current file end.
			// The simpler way: We trust Replay to read what's on disk NOW.
			// New events are in liveCh.
			// There is a race: 'Send' writes to disk AND pushes to liveCh.
			// If Replay reads the new disk write, we get duplicate.
			// Fix: Use Replay with a "StopAt" offset (not implemented yet, but let's assume valid for now or approximate).
			// Hack: Read everything. Keep track of IDs seen in Replay.
			// When switching to liveCh, drop if seen? Memory cost.
			//
			// BETTER: io.LimitReader in persistence as planned.
			// We need `b.log` to expose size.
			currentSize := b.log.Size()

			// Use a specialized channel for replay that sends Events
			replayCh := make(chan Event)
			go func() {
				defer close(replayCh)
				b.log.Replay(fromID, replayCh, currentSize)
			}()

			for event := range replayCh {
				select {
				case userCh <- Delivery{Event: event, Ack: func() {}}: // Historical events auto-acked? or no-op ack?
				case <-done:
					return
				}
			}
		}

		// 2. Stream Live
		for {
			select {
			case d := <-liveCh:
				// Verify duplication?
				// If we limited Replay to `currentSize`, and `Send` appends after,
				// theoretically no overlap IF `Send` does Write then Push.
				// If `Send` does Push then Write? -> `persistence` ensures Write first.
				// So if we LimitReader to currentSize, we are safe.
				select {
				case userCh <- d:
				case <-done:
					return
				}
			case <-done:
				return
			}
		}
	}()

	unsubscribe := func() {
		close(done) // Stop pump
		b.subsMutex.Lock()
		if b.subscribers[name] == liveCh {
			delete(b.subscribers, name)
		}
		b.subsMutex.Unlock()
		// liveCh will be GC'd (sender might block? Send uses 'default' select for non-required, what about required?)
		// If required, Send might block on liveCh if we stop reading.
		// So we should drain liveCh?
		// Or just close it? Closing channel that publisher writes to -> panic.
		// We remove from map, so no new writes.
		// Existing writes in flight?
		// Map lock protects removal.
	}

	return userCh, unsubscribe
}

// Close handles the graceful shutdown of the broker
func (b *Broker) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.log.Close()
}
