package providerpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Storage defines the interface for provider data persistence
type Storage interface {
	// Provider operations
	SaveProvider(provider *Provider) error
	LoadProvider(id string) (*Provider, error)
	LoadAllProviders() ([]*Provider, error)
	DeleteProvider(id string) error

	// Model operations
	SaveModels(providerID string, models []*Model) error
	LoadModels(providerID string) ([]*Model, error)

	// Usage operations
	AppendUsage(record *UsageRecord) error
	LoadUsage(providerID string, start, end time.Time) ([]*UsageRecord, error)

	// Health status
	SaveHealthStatus(results map[string]*HealthCheckResult) error
	LoadHealthStatus() (map[string]*HealthCheckResult, error)

	// Pricing configuration
	SavePricingConfig(config *PricingConfig) error
	LoadPricingConfig() (*PricingConfig, error)
}

// FileStorage implements Storage using JSON files
type FileStorage struct {
	basePath  string
	encryptor *Encryptor
	mu        sync.RWMutex
}

// NewFileStorage creates a new FileStorage
func NewFileStorage(basePath string, encryptor *Encryptor) (*FileStorage, error) {
	// Create directory structure
	dirs := []string{
		basePath,
		filepath.Join(basePath, "models"),
		filepath.Join(basePath, "usage"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return &FileStorage{
		basePath:  basePath,
		encryptor: encryptor,
	}, nil
}

// providersFile returns the path to the providers file
func (s *FileStorage) providersFile() string {
	return filepath.Join(s.basePath, "providers.json")
}

// modelsFile returns the path to a provider's models file
func (s *FileStorage) modelsFile(providerID string) string {
	return filepath.Join(s.basePath, "models", providerID+".json")
}

// usageFile returns the path to a daily usage file
func (s *FileStorage) usageFile(date time.Time) string {
	return filepath.Join(s.basePath, "usage", date.Format("2006-01-02")+".jsonl")
}

// healthFile returns the path to the health status file
func (s *FileStorage) healthFile() string {
	return filepath.Join(s.basePath, "health.json")
}

// providerStorage is the internal structure for storing providers
type providerStorage struct {
	Providers map[string]*providerData `json:"providers"`
	UpdatedAt time.Time                `json:"updated_at"`
}

// providerData stores provider with encrypted API keys
type providerData struct {
	Provider         *Provider `json:"provider"`
	EncryptedAPIKeys []string  `json:"encrypted_api_keys,omitempty"`
}

// SaveProvider saves a provider to storage
func (s *FileStorage) SaveProvider(provider *Provider) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load existing providers
	storage, err := s.loadProvidersInternal()
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if storage == nil {
		storage = &providerStorage{
			Providers: make(map[string]*providerData),
		}
	}

	// Encrypt API keys
	encryptedKeys := make([]string, 0, len(provider.APIKeys))
	for _, key := range provider.APIKeys {
		if key.Key != "" && s.encryptor != nil {
			encrypted, err := s.encryptor.Encrypt(key.Key)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key: %w", err)
			}
			encryptedKeys = append(encryptedKeys, encrypted)
		} else if key.Key != "" {
			// No encryptor, store empty string as placeholder
			encryptedKeys = append(encryptedKeys, "")
		}
	}

	// Create a deep copy without the actual keys for storage
	providerCopy := *provider
	providerCopy.APIKeys = make([]APIKey, len(provider.APIKeys))
	for i, key := range provider.APIKeys {
		keyCopy := key
		keyCopy.Key = "" // Clear the actual key in the copy only
		providerCopy.APIKeys[i] = keyCopy
	}

	storage.Providers[provider.ID] = &providerData{
		Provider:         &providerCopy,
		EncryptedAPIKeys: encryptedKeys,
	}
	storage.UpdatedAt = time.Now()

	return s.saveProvidersInternal(storage)
}

// LoadProvider loads a provider from storage
func (s *FileStorage) LoadProvider(id string) (*Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storage, err := s.loadProvidersInternal()
	if err != nil {
		return nil, err
	}

	data, ok := storage.Providers[id]
	if !ok {
		return nil, ErrProviderNotFound
	}

	return s.decryptProvider(data)
}

// LoadAllProviders loads all providers from storage
func (s *FileStorage) LoadAllProviders() ([]*Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storage, err := s.loadProvidersInternal()
	if err != nil {
		if os.IsNotExist(err) {
			return []*Provider{}, nil
		}
		return nil, err
	}

	providers := make([]*Provider, 0, len(storage.Providers))
	for _, data := range storage.Providers {
		provider, err := s.decryptProvider(data)
		if err != nil {
			// Log error but continue loading other providers
			continue
		}
		providers = append(providers, provider)
	}

	return providers, nil
}

// DeleteProvider removes a provider from storage
func (s *FileStorage) DeleteProvider(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	storage, err := s.loadProvidersInternal()
	if err != nil {
		return err
	}

	if _, ok := storage.Providers[id]; !ok {
		return ErrProviderNotFound
	}

	delete(storage.Providers, id)
	storage.UpdatedAt = time.Now()

	// Also delete models file
	modelsPath := s.modelsFile(id)
	os.Remove(modelsPath) // Ignore error if file doesn't exist

	return s.saveProvidersInternal(storage)
}

// loadProvidersInternal loads the providers file (must hold lock)
func (s *FileStorage) loadProvidersInternal() (*providerStorage, error) {
	data, err := os.ReadFile(s.providersFile())
	if err != nil {
		return nil, err
	}

	var storage providerStorage
	if err := json.Unmarshal(data, &storage); err != nil {
		return nil, err
	}

	return &storage, nil
}

// saveProvidersInternal saves the providers file (must hold lock)
func (s *FileStorage) saveProvidersInternal(storage *providerStorage) error {
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.providersFile(), data, 0644)
}

// decryptProvider decrypts API keys in a provider
func (s *FileStorage) decryptProvider(data *providerData) (*Provider, error) {
	provider := *data.Provider

	// Decrypt API keys
	if s.encryptor != nil && len(data.EncryptedAPIKeys) > 0 {
		for i, encrypted := range data.EncryptedAPIKeys {
			if i < len(provider.APIKeys) {
				decrypted, err := s.encryptor.Decrypt(encrypted)
				if err != nil {
					return nil, fmt.Errorf("failed to decrypt API key: %w", err)
				}
				provider.APIKeys[i].Key = decrypted
			}
		}
	}

	return &provider, nil
}

// SaveModels saves models for a provider
func (s *FileStorage) SaveModels(providerID string, models []*Model) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.modelsFile(providerID), data, 0644)
}

// LoadModels loads models for a provider
func (s *FileStorage) LoadModels(providerID string) ([]*Model, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.modelsFile(providerID))
	if err != nil {
		if os.IsNotExist(err) {
			return []*Model{}, nil
		}
		return nil, err
	}

	var models []*Model
	if err := json.Unmarshal(data, &models); err != nil {
		return nil, err
	}

	return models, nil
}

// AppendUsage appends a usage record to the daily log
func (s *FileStorage) AppendUsage(record *UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := s.usageFile(record.Timestamp)

	// Open file in append mode
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return err
	}

	_, err = f.Write(append(data, '\n'))
	return err
}

// LoadUsage loads usage records for a time range
func (s *FileStorage) LoadUsage(providerID string, start, end time.Time) ([]*UsageRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var records []*UsageRecord

	// Iterate through each day in the range
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		filePath := s.usageFile(d)

		data, err := os.ReadFile(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		// Parse JSONL format
		lines := splitLines(data)
		for _, line := range lines {
			if len(line) == 0 {
				continue
			}

			var record UsageRecord
			if err := json.Unmarshal(line, &record); err != nil {
				continue // Skip malformed lines
			}

			// Filter by provider if specified
			if providerID != "" && record.ProviderID != providerID {
				continue
			}

			// Filter by time range
			if record.Timestamp.Before(start) || record.Timestamp.After(end) {
				continue
			}

			records = append(records, &record)
		}
	}

	return records, nil
}

// SaveHealthStatus saves health check results
func (s *FileStorage) SaveHealthStatus(results map[string]*HealthCheckResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.healthFile(), data, 0644)
}

// LoadHealthStatus loads health check results
func (s *FileStorage) LoadHealthStatus() (map[string]*HealthCheckResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.healthFile())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]*HealthCheckResult), nil
		}
		return nil, err
	}

	var results map[string]*HealthCheckResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}

	return results, nil
}

// splitLines splits data by newlines
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// pricingFile returns the path to the pricing configuration file
func (s *FileStorage) pricingFile() string {
	return filepath.Join(s.basePath, "pricing.json")
}

// SavePricingConfig saves the pricing configuration
func (s *FileStorage) SavePricingConfig(config *PricingConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.pricingFile(), data, 0644)
}

// LoadPricingConfig loads the pricing configuration
func (s *FileStorage) LoadPricingConfig() (*PricingConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.pricingFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, err
	}

	var config PricingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Ensure CustomPricing map is initialized
	if config.CustomPricing == nil {
		config.CustomPricing = make(map[string]*ModelPricing)
	}

	return &config, nil
}
