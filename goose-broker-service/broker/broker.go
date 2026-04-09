package broker

import (
	"fmt"
	"sort"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/google/uuid"
)

// Config holds the configuration for the broker.
type Config struct {
	PersistencePath   string
	MaxLogSize        int64
	MaxRetainedLogs   int
	UnsafeBatchWrites bool
	SyncOnWrite       bool
}

// Event represents a single unit of data in the system.
type Event struct {
	ID        string         `json:"id"`
	Stream    string         `json:"stream"`
	Timestamp int64          `json:"timestamp"`
	Payload   jsontext.Value `json:"payload"`
}

// Delivery wraps an Event and provides an Ack method.
type Delivery struct {
	Event
	Ack func()
}

// Broker acts as the central event bus.
type Broker struct {
	config Config
	mutex  sync.Mutex
	log    *Log

	subsMutex   sync.RWMutex
	subscribers map[string]chan Delivery

	topicMu sync.RWMutex
	topics  map[string]Topic

	metricsMu sync.RWMutex
	metrics   BrokerMetrics
}

// TopicMetrics aggregates message activity per topic.
type TopicMetrics struct {
	Published          uint64 `json:"published"`
	LastEventTimestamp int64  `json:"last_event_timestamp,omitempty"`
}

// BrokerMetrics aggregates runtime broker activity since process start.
type BrokerMetrics struct {
	PublishedMessages uint64                  `json:"published_messages"`
	RequiredAcks      uint64                  `json:"required_acks"`
	ActiveSubscribers int                     `json:"active_subscribers"`
	TopicsCount       int                     `json:"topics_count"`
	LogSizeBytes      int64                   `json:"log_size_bytes"`
	ByTopic           map[string]TopicMetrics `json:"by_topic"`
}

// TopicUpdate carries editable fields for a topic.
type TopicUpdate struct {
	Description    *string `json:"description,omitempty"`
	RetentionHours *int    `json:"retention_hours,omitempty"`
	Disabled       *bool   `json:"disabled,omitempty"`
}

// ErrTopicNotFound indicates an operation was made on an unknown topic.
type ErrTopicNotFound struct {
	Topic string
}

func (e ErrTopicNotFound) Error() string {
	return fmt.Sprintf("topic %q not found", e.Topic)
}

// ErrTopicDisabled indicates publish is disabled for the topic.
type ErrTopicDisabled struct {
	Topic string
}

func (e ErrTopicDisabled) Error() string {
	return fmt.Sprintf("topic %q is disabled", e.Topic)
}

// ErrRequiredConsumerNotSubscribed indicates a required consumer is not connected.
type ErrRequiredConsumerNotSubscribed struct {
	Consumer string
}

func (e ErrRequiredConsumerNotSubscribed) Error() string {
	return fmt.Sprintf("required consumer %q is not subscribed", e.Consumer)
}

// New creates a new Broker instance.
func New(config Config) (*Broker, error) {
	if config.PersistencePath == "" {
		return nil, fmt.Errorf("persistence path is required")
	}
	if config.MaxLogSize == 0 {
		config.MaxLogSize = 10 * 1024 * 1024
	}
	if config.MaxRetainedLogs == 0 {
		config.MaxRetainedLogs = 5
	}

	l, err := openLog(config.PersistencePath, config.MaxLogSize, config.MaxRetainedLogs, config.UnsafeBatchWrites, config.SyncOnWrite)
	if err != nil {
		return nil, err
	}

	b := &Broker{
		config:      config,
		log:         l,
		subscribers: make(map[string]chan Delivery),
		metrics: BrokerMetrics{
			ByTopic: make(map[string]TopicMetrics),
		},
	}

	if err := b.loadTopics(); err != nil {
		_ = l.Close()
		return nil, err
	}

	return b, nil
}

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
	if stream == "" {
		return fmt.Errorf("stream is required")
	}

	b.topicMu.RLock()
	topic, exists := b.topics[stream]
	b.topicMu.RUnlock()
	if !exists {
		return ErrTopicNotFound{Topic: stream}
	}
	if topic.Disabled {
		return ErrTopicDisabled{Topic: stream}
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	b.subsMutex.RLock()
	for _, name := range requiredConsumers {
		if _, ok := b.subscribers[name]; !ok {
			b.subsMutex.RUnlock()
			return ErrRequiredConsumerNotSubscribed{Consumer: name}
		}
	}
	b.subsMutex.RUnlock()

	event := Event{
		ID:        uuid.New().String(),
		Stream:    stream,
		Timestamp: time.Now().UnixNano(),
		Payload:   payloadBytes,
	}

	if err := func() error {
		b.mutex.Lock()
		defer b.mutex.Unlock()
		return b.log.Append(event)
	}(); err != nil {
		return err
	}
	b.recordPublished(stream, event.Timestamp)

	b.subsMutex.RLock()
	defer b.subsMutex.RUnlock()

	var wg sync.WaitGroup
	requiredMap := make(map[string]bool, len(requiredConsumers))
	for _, name := range requiredConsumers {
		requiredMap[name] = true
	}

	for name, ch := range b.subscribers {
		isRequired := requiredMap[name]
		consumerName := name

		ack := func() {}
		if isRequired {
			wg.Add(1)
			var once sync.Once
			ack = func() {
				once.Do(func() {
					_ = b.persistAck(event.ID, consumerName)
					b.recordRequiredAck()
					wg.Done()
				})
			}
		}

		delivery := Delivery{Event: event, Ack: ack}
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

	wg.Wait()
	return nil
}

// Subscribe registers a named consumer and returns a channel of deliveries.
// If fromID is specified, it replays history starting after that ID.
func (b *Broker) Subscribe(name string, fromID string) (<-chan Delivery, func()) {
	liveCh := make(chan Delivery, 100)
	userCh := make(chan Delivery, 100)

	b.subsMutex.Lock()
	b.subscribers[name] = liveCh
	b.subsMutex.Unlock()

	done := make(chan struct{})

	go func() {
		defer close(userCh)

		if fromID != "" {
			currentSize := b.log.Size()
			replayCh := make(chan Event)
			go func() {
				defer close(replayCh)
				_ = b.log.Replay(fromID, replayCh, currentSize)
			}()

			for event := range replayCh {
				select {
				case userCh <- Delivery{Event: event, Ack: func() {}}:
				case <-done:
					return
				}
			}
		}

		for {
			select {
			case d, ok := <-liveCh:
				if !ok {
					return
				}
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
		close(done)
		b.subsMutex.Lock()
		if b.subscribers[name] == liveCh {
			delete(b.subscribers, name)
		}
		b.subsMutex.Unlock()
	}

	return userCh, unsubscribe
}

func (b *Broker) recordPublished(topic string, ts int64) {
	b.metricsMu.Lock()
	defer b.metricsMu.Unlock()
	b.metrics.PublishedMessages++
	m := b.metrics.ByTopic[topic]
	m.Published++
	m.LastEventTimestamp = ts
	b.metrics.ByTopic[topic] = m
}

func (b *Broker) recordRequiredAck() {
	b.metricsMu.Lock()
	b.metrics.RequiredAcks++
	b.metricsMu.Unlock()
}

// CreateTopic registers a new topic.
func (b *Broker) CreateTopic(name, description string, retentionHours int) (Topic, error) {
	if err := validateTopicName(name); err != nil {
		return Topic{}, err
	}
	if retentionHours < 0 {
		return Topic{}, fmt.Errorf("retention_hours must be >= 0")
	}

	b.topicMu.Lock()
	defer b.topicMu.Unlock()

	if _, exists := b.topics[name]; exists {
		return Topic{}, fmt.Errorf("topic %q already exists", name)
	}

	now := time.Now().UnixNano()
	topic := Topic{
		Name:           name,
		Description:    description,
		RetentionHours: retentionHours,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	b.topics[name] = topic

	if err := b.saveTopicsLocked(); err != nil {
		delete(b.topics, name)
		return Topic{}, err
	}

	return topic, nil
}

// UpdateTopic updates mutable topic fields.
func (b *Broker) UpdateTopic(name string, update TopicUpdate) (Topic, error) {
	if err := validateTopicName(name); err != nil {
		return Topic{}, err
	}

	b.topicMu.Lock()
	defer b.topicMu.Unlock()

	topic, exists := b.topics[name]
	if !exists {
		return Topic{}, ErrTopicNotFound{Topic: name}
	}

	if update.Description != nil {
		topic.Description = *update.Description
	}
	if update.RetentionHours != nil {
		if *update.RetentionHours < 0 {
			return Topic{}, fmt.Errorf("retention_hours must be >= 0")
		}
		topic.RetentionHours = *update.RetentionHours
	}
	if update.Disabled != nil {
		topic.Disabled = *update.Disabled
	}
	topic.UpdatedAt = time.Now().UnixNano()
	b.topics[name] = topic

	if err := b.saveTopicsLocked(); err != nil {
		return Topic{}, err
	}

	return topic, nil
}

// DeleteTopic removes a topic from registry.
func (b *Broker) DeleteTopic(name string) error {
	if err := validateTopicName(name); err != nil {
		return err
	}

	b.topicMu.Lock()
	defer b.topicMu.Unlock()

	if _, exists := b.topics[name]; !exists {
		return ErrTopicNotFound{Topic: name}
	}
	delete(b.topics, name)

	if len(b.topics) == 0 {
		now := time.Now().UnixNano()
		b.topics["events"] = Topic{Name: "events", Description: "default topic", RetentionHours: 168, CreatedAt: now, UpdatedAt: now}
	}

	return b.saveTopicsLocked()
}

// GetTopic returns topic metadata.
func (b *Broker) GetTopic(name string) (Topic, bool) {
	b.topicMu.RLock()
	defer b.topicMu.RUnlock()
	topic, ok := b.topics[name]
	return topic, ok
}

// ListTopics returns sorted topic metadata.
func (b *Broker) ListTopics() []Topic {
	b.topicMu.RLock()
	defer b.topicMu.RUnlock()

	topics := make([]Topic, 0, len(b.topics))
	for _, topic := range b.topics {
		topics = append(topics, topic)
	}
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Name < topics[j].Name
	})
	return topics
}

// Metrics returns a copy of runtime metrics.
func (b *Broker) Metrics() BrokerMetrics {
	b.metricsMu.RLock()
	metrics := BrokerMetrics{
		PublishedMessages: b.metrics.PublishedMessages,
		RequiredAcks:      b.metrics.RequiredAcks,
		ByTopic:           make(map[string]TopicMetrics, len(b.metrics.ByTopic)),
	}
	for topic, m := range b.metrics.ByTopic {
		metrics.ByTopic[topic] = m
	}
	b.metricsMu.RUnlock()

	b.subsMutex.RLock()
	metrics.ActiveSubscribers = len(b.subscribers)
	b.subsMutex.RUnlock()

	b.topicMu.RLock()
	metrics.TopicsCount = len(b.topics)
	b.topicMu.RUnlock()

	b.mutex.Lock()
	metrics.LogSizeBytes = b.log.Size()
	b.mutex.Unlock()

	return metrics
}

// Close handles graceful shutdown of the broker.
func (b *Broker) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.log.Close()
}
