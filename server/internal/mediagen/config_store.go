package mediagen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// configStoreFile is the filename for persisted media provider configs.
const configStoreFile = "providers.json"

// configOnDisk is the serialization format that includes the API key.
type configOnDisk struct {
	ID       string `json:"id"`
	Enabled  bool   `json:"enabled"`
	BaseURL  string `json:"base_url,omitempty"`
	APIKey   string `json:"api_key,omitempty"`
	Priority *int   `json:"priority,omitempty"`
}

// MediaConfigStore is the interface for media provider config persistence.
type MediaConfigStore interface {
	Load() (map[string]*configOnDisk, error)
	Save(configs map[string]*MediaProviderConfig) error
}

// ConfigStore persists media provider configurations to a JSON file.
type ConfigStore struct {
	dir string
	mu  sync.Mutex
}

// NewConfigStore creates a config store backed by dir/providers.json.
func NewConfigStore(dir string) *ConfigStore {
	_ = os.MkdirAll(dir, 0750)
	return &ConfigStore{dir: dir}
}

// Load reads saved configs from disk. Returns nil map if file doesn't exist.
func (s *ConfigStore) Load() (map[string]*configOnDisk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dir, configStoreFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var items []configOnDisk
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}

	result := make(map[string]*configOnDisk, len(items))
	for i := range items {
		result[items[i].ID] = &items[i]
	}
	return result, nil
}

// Save writes configs to disk.
func (s *ConfigStore) Save(configs map[string]*MediaProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []configOnDisk
	for _, c := range configs {
		priority := c.Priority
		items = append(items, configOnDisk{
			ID:       c.ID,
			Enabled:  c.Enabled,
			BaseURL:  c.BaseURL,
			APIKey:   c.APIKey,
			Priority: &priority,
		})
	}

	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.dir, configStoreFile)
	return os.WriteFile(path, data, 0600)
}

// Dir returns the store's directory path.
func (s *ConfigStore) Dir() string {
	return s.dir
}
