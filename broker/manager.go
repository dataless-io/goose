package broker

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// BrokerManager manages multiple Broker instances.
type BrokerManager struct {
	basePath string
	brokers  map[string]*Broker
	mutex    sync.RWMutex
}

// NewManager creates a new BrokerManager using the given base path for persistence.
func NewManager(basePath string) (*BrokerManager, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &BrokerManager{
		basePath: basePath,
		brokers:  make(map[string]*Broker),
	}, nil
}

// CreateBroker creates a new named broker.
// Returns error if broker already exists.
func (m *BrokerManager) CreateBroker(name string) (*Broker, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.brokers[name]; exists {
		return nil, fmt.Errorf("broker '%s' already exists", name)
	}

	// Define persistence path for this broker
	// We use a directory per broker to keep things clean? or just a log file?
	// Plan said "basePath/{name}.log". Simple enough.
	logPath := filepath.Join(m.basePath, fmt.Sprintf("%s.log", name))

	b, err := New(Config{
		PersistencePath: logPath,
		// We could Expose global config for rotation/batching here if needed.
		// For now using defaults.
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create broker: %w", err)
	}

	m.brokers[name] = b
	return b, nil
}

// GetBroker retrieves an existing broker by name.
func (m *BrokerManager) GetBroker(name string) (*Broker, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	b, exists := m.brokers[name]
	return b, exists
}

// ListBrokers returns a list of all broker names.
func (m *BrokerManager) ListBrokers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.brokers))
	for name := range m.brokers {
		names = append(names, name)
	}
	return names
}

// DeleteBroker closes and removes a broker from management.
// It does NOT delete the persistence file (to be safe/explicit).
func (m *BrokerManager) DeleteBroker(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	b, exists := m.brokers[name]
	if !exists {
		return fmt.Errorf("broker '%s' not found", name)
	}

	if err := b.Close(); err != nil {
		// Log error but proceed?
		fmt.Printf("Error closing broker %s: %v\n", name, err)
	}

	delete(m.brokers, name)
	return nil
}

// Close closes all managed brokers.
func (m *BrokerManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var firstErr error
	for name, b := range m.brokers {
		if err := b.Close(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			fmt.Printf("Error closing broker %s: %v\n", name, err)
		}
	}
	return firstErr
}
