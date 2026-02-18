package ngrok

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConfigStore provides JSON file-based configuration storage with lazy initialization.
// The config file is only created when configuration is first saved.
type ConfigStore struct {
	mu       sync.RWMutex
	filePath string
	config   *RemoteAccessConfig
	loaded   bool
}

// NewConfigStore creates a new config store. The file is not created until Save is called.
func NewConfigStore(dataDir string) *ConfigStore {
	return &ConfigStore{
		filePath: filepath.Join(dataDir, "tunnel_config.json"),
	}
}

// GetConfig returns the configuration, loading from file if needed.
func (s *ConfigStore) GetConfig(ctx context.Context) (*RemoteAccessConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		if err := s.loadLocked(); err != nil {
			return nil, err
		}
	}

	// Return a copy to prevent external modification
	cfg := *s.config
	return &cfg, nil
}

// SaveConfig saves the configuration to file, creating it if needed.
func (s *ConfigStore) SaveConfig(ctx context.Context, config *RemoteAccessConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	config.UpdatedAt = timeutil.NowTime()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = config.UpdatedAt
	}

	s.config = config
	s.loaded = true

	return s.saveLocked()
}

// EnsureTunnelSubdomain returns the tunnel subdomain, generating one if needed.
func (s *ConfigStore) EnsureTunnelSubdomain(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.loaded {
		if err := s.loadLocked(); err != nil {
			return "", err
		}
	}

	if s.config.TunnelSubdomain != "" {
		return s.config.TunnelSubdomain, nil
	}

	// Generate new subdomain
	s.config.TunnelSubdomain = generateTunnelSubdomain()
	s.config.UpdatedAt = timeutil.NowTime()

	// Save to persist the subdomain
	if err := s.saveLocked(); err != nil {
		return "", err
	}

	return s.config.TunnelSubdomain, nil
}

// AddLog is a no-op for ConfigStore (logs are not persisted in JSON mode).
// This maintains API compatibility with Repository.
func (s *ConfigStore) AddLog(ctx context.Context, sessionID, eventType, message string, metadata map[string]interface{}) error {
	// Logs are not persisted in simplified JSON mode
	return nil
}

// GetLogs returns empty logs (not supported in JSON mode).
func (s *ConfigStore) GetLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	return []*RemoteAccessLog{}, nil
}

// GetErrorLogs returns empty logs (not supported in JSON mode).
func (s *ConfigStore) GetErrorLogs(ctx context.Context, limit, offset int) ([]*RemoteAccessLog, error) {
	return []*RemoteAccessLog{}, nil
}

// Close is a no-op for ConfigStore (no resources to release).
func (s *ConfigStore) Close() error {
	return nil
}

// loadLocked loads config from file. Must be called with lock held.
func (s *ConfigStore) loadLocked() error {
	s.config = &RemoteAccessConfig{
		ID:                    "default",
		Enabled:               false,
		DefaultProvider:       "auto",
		NotifyOnURLChange:     true,
		NotifyOnExpiryWarning: false,
		NotifyOnError:         true,
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet, use defaults
			s.loaded = true
			return nil
		}
		return err
	}

	if err := json.Unmarshal(data, s.config); err != nil {
		return err
	}

	s.loaded = true
	return nil
}

// saveLocked saves config to file. Must be called with lock held.
func (s *ConfigStore) saveLocked() error {
	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// generateTunnelSubdomain generates a random subdomain like "echo-" + 8 base58 chars.
func generateTunnelSubdomain() string {
	const base58Alphabet = "180789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	const n = 8
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = byte((int(timeutil.NowNano()) + i) % 256)
		}
	}
	for i := range b {
		b[i] = base58Alphabet[int(b[i])%len(base58Alphabet)]
	}
	return "echo-" + string(b)
}
