package broker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	json "github.com/go-json-experiment/json"
)

var validTopicName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// Topic describes a publishable stream namespace.
type Topic struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	RetentionHours int    `json:"retention_hours,omitempty"`
	Disabled       bool   `json:"disabled,omitempty"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type topicStore struct {
	Topics []Topic `json:"topics"`
}

func validateTopicName(name string) error {
	if name == "" {
		return fmt.Errorf("topic name is required")
	}
	if !validTopicName.MatchString(name) {
		return fmt.Errorf("invalid topic name: only letters, numbers, dot, dash and underscore are allowed")
	}
	return nil
}

func (b *Broker) topicMetadataPath() string {
	return b.config.PersistencePath + ".topics.json"
}

func (b *Broker) loadTopics() error {
	b.topicMu.Lock()
	defer b.topicMu.Unlock()

	b.topics = make(map[string]Topic)
	metadataPath := b.topicMetadataPath()

	bytes, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			now := time.Now().UnixNano()
			b.topics["events"] = Topic{Name: "events", Description: "default topic", RetentionHours: 168, CreatedAt: now, UpdatedAt: now}
			return b.saveTopicsLocked()
		}
		return err
	}

	var store topicStore
	if err := json.Unmarshal(bytes, &store); err != nil {
		return fmt.Errorf("invalid topics metadata file: %w", err)
	}

	for _, topic := range store.Topics {
		if err := validateTopicName(topic.Name); err != nil {
			continue
		}
		if topic.CreatedAt == 0 {
			topic.CreatedAt = time.Now().UnixNano()
		}
		if topic.UpdatedAt == 0 {
			topic.UpdatedAt = topic.CreatedAt
		}
		if topic.RetentionHours < 0 {
			topic.RetentionHours = 0
		}
		b.topics[topic.Name] = topic
	}

	if len(b.topics) == 0 {
		now := time.Now().UnixNano()
		b.topics["events"] = Topic{Name: "events", Description: "default topic", RetentionHours: 168, CreatedAt: now, UpdatedAt: now}
		return b.saveTopicsLocked()
	}

	return nil
}

func (b *Broker) saveTopicsLocked() error {
	store := topicStore{Topics: make([]Topic, 0, len(b.topics))}
	for _, topic := range b.topics {
		store.Topics = append(store.Topics, topic)
	}
	sort.Slice(store.Topics, func(i, j int) bool {
		return store.Topics[i].Name < store.Topics[j].Name
	})

	bytes, err := json.Marshal(store)
	if err != nil {
		return err
	}

	path := b.topicMetadataPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
