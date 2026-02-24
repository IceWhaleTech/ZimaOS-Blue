package config

import (
	"context"
	"encoding/json"
	"reflect"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

const configKeyPrefix = "config:app:"

// configSections maps section names to their Config field names.
var configSections = []string{
	"server", "log", "worker", "resources", "cgroup", "channels",
	"performance", "security", "llm", "session", "embedding", "memory",
	"grayscale", "companion", "claudecode", "claude_code_cli",
	"first_run", "cc_switch", "statistics", "tool_calling",
	"proxy", "pruner", "update", "heartbeat",
}

// ConfigStore persists Config sections in kvstore (SQLite-backed).
type ConfigStore struct {
	mu     sync.RWMutex
	kv     kvstore.Store
	config *Config
	loaded bool
}

// NewConfigStore creates a new config store backed by kvstore.
func NewConfigStore(kv kvstore.Store) *ConfigStore {
	return &ConfigStore{kv: kv}
}

// LoadOrImport loads config from DB if present, otherwise imports from the
// provided YAML-loaded config and persists it. Returns the effective config.
func (s *ConfigStore) LoadOrImport(yamlCfg *Config) (*Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()

	// Check if DB already has config (use "server" section as sentinel)
	var serverCfg ServerConfig
	err := s.kv.GetJSON(ctx, configKeyPrefix+"server", &serverCfg)
	if err == nil {
		// DB has config — load all sections
		cfg, loadErr := s.loadLocked(ctx, yamlCfg)
		if loadErr != nil {
			return nil, loadErr
		}
		s.config = cfg
		s.loaded = true
		return cfg, nil
	}

	// First run — import YAML config into DB
	if importErr := s.importLocked(ctx, yamlCfg); importErr != nil {
		return nil, importErr
	}
	s.config = yamlCfg
	s.loaded = true
	return yamlCfg, nil
}

// Config returns the current in-memory config.
func (s *ConfigStore) Config() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// GetSection returns a config section as JSON.
func (s *ConfigStore) GetSection(section string) (json.RawMessage, error) {
	ctx := context.Background()
	var raw json.RawMessage
	if err := s.kv.GetJSON(ctx, configKeyPrefix+section, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// SetSection updates a config section in DB and refreshes in-memory config.
func (s *ConfigStore) SetSection(section string, value json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()
	if err := s.kv.SetJSON(ctx, configKeyPrefix+section, value, 0); err != nil {
		return err
	}

	// Refresh in-memory config
	if s.config != nil {
		applySection(s.config, section, value)
	}
	return nil
}

// Reload re-reads all sections from DB into memory.
func (s *ConfigStore) Reload() (*Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := context.Background()
	cfg, err := s.loadLocked(ctx, s.config)
	if err != nil {
		return nil, err
	}
	s.config = cfg
	return cfg, nil
}

// Import persists the given config into DB (overwrites all sections).
func (s *ConfigStore) Import(cfg *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.importLocked(context.Background(), cfg)
}

// loadLocked reads all config sections from DB, falling back to fallback for missing sections.
func (s *ConfigStore) loadLocked(ctx context.Context, fallback *Config) (*Config, error) {
	cfg := defaults()
	if fallback != nil {
		cfg = *fallback
	}

	for _, section := range configSections {
		var raw json.RawMessage
		if err := s.kv.GetJSON(ctx, configKeyPrefix+section, &raw); err != nil {
			continue // Use default/fallback value
		}
		applySection(&cfg, section, raw)
	}

	return &cfg, nil
}

// importLocked writes all config sections to DB.
func (s *ConfigStore) importLocked(ctx context.Context, cfg *Config) error {
	sections := extractSections(cfg)
	for name, data := range sections {
		if err := s.kv.SetJSON(ctx, configKeyPrefix+name, data, 0); err != nil {
			return err
		}
	}
	// Mark import timestamp
	return s.kv.SetJSON(ctx, configKeyPrefix+"_imported_at", time.Now().Format(time.RFC3339), 0)
}

// extractSections serializes each Config field into a map of section name → JSON.
func extractSections(cfg *Config) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(configSections))
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		data, err := json.Marshal(v.Field(i).Interface())
		if err != nil {
			continue
		}
		result[yamlTag] = data
	}
	return result
}

// applySection deserializes a JSON blob into the corresponding Config field.
func applySection(cfg *Config, section string, data json.RawMessage) {
	v := reflect.ValueOf(cfg).Elem()
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		yamlTag := field.Tag.Get("yaml")
		if yamlTag == section {
			ptr := reflect.New(field.Type)
			if err := json.Unmarshal(data, ptr.Interface()); err == nil {
				v.Field(i).Set(ptr.Elem())
			}
			return
		}
	}
}
