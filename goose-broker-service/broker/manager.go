package broker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	json "github.com/go-json-experiment/json"
)

var validBrokerName = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// ManagerConfig configures broker manager behavior.
type ManagerConfig struct {
	BasePath      string
	BrokerConfig  Config
	AutoLoadFiles bool
}

// BrokerSettings stores editable metadata per broker.
type BrokerSettings struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name,omitempty"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

// BrokerSettingsUpdate defines mutable fields.
type BrokerSettingsUpdate struct {
	DisplayName *string `json:"display_name,omitempty"`
	Description *string `json:"description,omitempty"`
	Disabled    *bool   `json:"disabled,omitempty"`
}

type brokerSettingsStore struct {
	Brokers []BrokerSettings `json:"brokers"`
}

// BrokerManager manages multiple broker instances.
type BrokerManager struct {
	basePath         string
	defaultBrokerCfg Config
	brokers          map[string]*Broker
	settings         map[string]BrokerSettings
	mutex            sync.RWMutex
}

// NewManager creates a manager using a base path for persistence.
func NewManager(basePath string) (*BrokerManager, error) {
	return NewManagerWithConfig(ManagerConfig{BasePath: basePath, AutoLoadFiles: true})
}

// NewManagerWithConfig creates a manager with explicit options.
func NewManagerWithConfig(cfg ManagerConfig) (*BrokerManager, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("base path is required")
	}
	if err := os.MkdirAll(cfg.BasePath, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	mgr := &BrokerManager{
		basePath:         cfg.BasePath,
		defaultBrokerCfg: cfg.BrokerConfig,
		brokers:          make(map[string]*Broker),
		settings:         make(map[string]BrokerSettings),
	}

	if cfg.AutoLoadFiles {
		if err := mgr.loadFromDisk(); err != nil {
			_ = mgr.Close()
			return nil, err
		}
	}

	return mgr, nil
}

func validateBrokerName(name string) error {
	if name == "" {
		return fmt.Errorf("broker name is required")
	}
	if strings.Contains(name, string(filepath.Separator)) {
		return fmt.Errorf("broker name contains invalid path separator")
	}
	if !validBrokerName.MatchString(name) {
		return fmt.Errorf("invalid broker name: only letters, numbers, dot, dash and underscore are allowed")
	}
	return nil
}

func (m *BrokerManager) brokerLogPath(name string) string {
	return filepath.Join(m.basePath, fmt.Sprintf("%s.log", name))
}

func (m *BrokerManager) settingsPath() string {
	return filepath.Join(m.basePath, "brokers.settings.json")
}

func (m *BrokerManager) loadSettingsLocked() error {
	path := m.settingsPath()
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var store brokerSettingsStore
	if err := json.Unmarshal(bytes, &store); err != nil {
		return err
	}

	for _, st := range store.Brokers {
		if err := validateBrokerName(st.Name); err != nil {
			continue
		}
		if st.CreatedAt == 0 {
			st.CreatedAt = time.Now().UnixNano()
		}
		if st.UpdatedAt == 0 {
			st.UpdatedAt = st.CreatedAt
		}
		m.settings[st.Name] = st
	}

	return nil
}

func (m *BrokerManager) saveSettingsLocked() error {
	store := brokerSettingsStore{Brokers: make([]BrokerSettings, 0, len(m.settings))}
	for _, st := range m.settings {
		store.Brokers = append(store.Brokers, st)
	}
	sort.Slice(store.Brokers, func(i, j int) bool {
		return store.Brokers[i].Name < store.Brokers[j].Name
	})

	bytes, err := json.Marshal(store)
	if err != nil {
		return err
	}
	tmp := m.settingsPath() + ".tmp"
	if err := os.WriteFile(tmp, bytes, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, m.settingsPath())
}

func (m *BrokerManager) ensureDefaultSettingsLocked(name string) {
	if _, exists := m.settings[name]; exists {
		return
	}
	now := time.Now().UnixNano()
	m.settings[name] = BrokerSettings{
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (m *BrokerManager) loadFromDisk() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if err := m.loadSettingsLocked(); err != nil {
		return fmt.Errorf("load broker settings: %w", err)
	}

	files, err := filepath.Glob(filepath.Join(m.basePath, "*.log"))
	if err != nil {
		return fmt.Errorf("failed to list persisted brokers: %w", err)
	}

	for _, path := range files {
		name := strings.TrimSuffix(filepath.Base(path), ".log")
		if err := validateBrokerName(name); err != nil {
			continue
		}

		cfg := m.defaultBrokerCfg
		cfg.PersistencePath = path

		b, err := New(cfg)
		if err != nil {
			return fmt.Errorf("failed to load broker %q: %w", name, err)
		}
		m.brokers[name] = b
		m.ensureDefaultSettingsLocked(name)
	}

	return m.saveSettingsLocked()
}

// CreateBroker creates a new named broker.
func (m *BrokerManager) CreateBroker(name string) (*Broker, error) {
	if err := validateBrokerName(name); err != nil {
		return nil, err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, exists := m.brokers[name]; exists {
		return nil, fmt.Errorf("broker %q already exists", name)
	}

	cfg := m.defaultBrokerCfg
	cfg.PersistencePath = m.brokerLogPath(name)

	b, err := New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create broker: %w", err)
	}

	m.brokers[name] = b
	m.ensureDefaultSettingsLocked(name)
	if err := m.saveSettingsLocked(); err != nil {
		_ = b.Close()
		delete(m.brokers, name)
		delete(m.settings, name)
		return nil, err
	}

	return b, nil
}

// GetBroker retrieves an existing broker by name.
func (m *BrokerManager) GetBroker(name string) (*Broker, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	b, exists := m.brokers[name]
	return b, exists
}

// ListBrokers returns all broker names.
func (m *BrokerManager) ListBrokers() []string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	names := make([]string, 0, len(m.brokers))
	for name := range m.brokers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetBrokerSettings returns editable broker settings.
func (m *BrokerManager) GetBrokerSettings(name string) (BrokerSettings, bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	st, ok := m.settings[name]
	if !ok {
		return BrokerSettings{}, false
	}
	return st, true
}

// ListBrokerSettings returns settings for all brokers.
func (m *BrokerManager) ListBrokerSettings() []BrokerSettings {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	out := make([]BrokerSettings, 0, len(m.settings))
	for name, st := range m.settings {
		if _, ok := m.brokers[name]; ok {
			out = append(out, st)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

// UpdateBrokerSettings updates mutable broker settings.
func (m *BrokerManager) UpdateBrokerSettings(name string, update BrokerSettingsUpdate) (BrokerSettings, error) {
	if err := validateBrokerName(name); err != nil {
		return BrokerSettings{}, err
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if _, ok := m.brokers[name]; !ok {
		return BrokerSettings{}, fmt.Errorf("broker %q not found", name)
	}

	st, ok := m.settings[name]
	if !ok {
		now := time.Now().UnixNano()
		st = BrokerSettings{Name: name, CreatedAt: now, UpdatedAt: now}
	}

	if update.DisplayName != nil {
		st.DisplayName = strings.TrimSpace(*update.DisplayName)
	}
	if update.Description != nil {
		st.Description = strings.TrimSpace(*update.Description)
	}
	if update.Disabled != nil {
		st.Disabled = *update.Disabled
	}
	st.UpdatedAt = time.Now().UnixNano()
	m.settings[name] = st

	if err := m.saveSettingsLocked(); err != nil {
		return BrokerSettings{}, err
	}

	return st, nil
}

// IsBrokerDisabled returns true if broker is disabled in settings.
func (m *BrokerManager) IsBrokerDisabled(name string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	st, ok := m.settings[name]
	return ok && st.Disabled
}

// DeleteBroker closes and removes a broker from management.
func (m *BrokerManager) DeleteBroker(name string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	b, exists := m.brokers[name]
	if !exists {
		return fmt.Errorf("broker %q not found", name)
	}

	if err := b.Close(); err != nil {
		return err
	}

	delete(m.brokers, name)
	delete(m.settings, name)
	if err := m.saveSettingsLocked(); err != nil {
		return err
	}
	return nil
}

// Close closes all managed brokers.
func (m *BrokerManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var firstErr error
	for name, b := range m.brokers {
		if err := b.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close broker %q: %w", name, err)
		}
	}

	return firstErr
}
