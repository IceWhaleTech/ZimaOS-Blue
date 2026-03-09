package providerpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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

	// Pricing configuration
	SavePricingConfig(config *PricingConfig) error
	LoadPricingConfig() (*PricingConfig, error)

	// Pool configuration
	SaveConfig(config *PoolConfig) error
	LoadConfig() (*PoolConfig, error)
}

// FileStorage implements Storage using JSON files
type FileStorage struct {
	basePath  string
	encryptor SecretEncryptor
	mu        sync.RWMutex
}

// NewFileStorage creates a new FileStorage
func NewFileStorage(basePath string, opts ...StorageOption) (*FileStorage, error) {
	storageOpts := applyStorageOptions(opts...)
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
		encryptor: storageOpts.encryptor,
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

// providerStorage is the internal structure for storing providers
type providerStorage struct {
	Providers map[string]*providerData `json:"providers"`
	UpdatedAt time.Time                `json:"updated_at"`
}

// providerData stores provider with API keys (for storage only)
type providerData struct {
	Provider     *Provider     `json:"provider"`
	APIKeys      []string      `json:"api_keys,omitempty"`      // Store actual keys separately
	OAuthSecrets *oauthSecrets `json:"oauth_secrets,omitempty"` // Store OAuth tokens separately
}

// oauthSecrets stores OAuth tokens that have json:"-" tags on OAuthConfig
type oauthSecrets struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ClientSecret string `json:"client_secret,omitempty"`
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

	apiKeys, err := prepareStoredAPIKeys(provider, s.encryptor)
	if err != nil {
		return err
	}
	secrets, err := prepareStoredOAuthSecrets(provider, s.encryptor)
	if err != nil {
		return err
	}

	// Store provider with keys separately
	storage.Providers[provider.ID] = &providerData{
		Provider:     provider,
		APIKeys:      apiKeys,
		OAuthSecrets: secrets,
	}
	storage.UpdatedAt = timeutil.NowTime()

	return s.saveProvidersInternal(storage)
}

// LoadProvider loads a provider from storage
func (s *FileStorage) LoadProvider(id string) (*Provider, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	storage, err := s.loadProvidersInternal()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrProviderNotFound
		}
		return nil, err
	}

	data, ok := storage.Providers[id]
	if !ok {
		return nil, ErrProviderNotFound
	}

	provider := data.Provider
	if err := restoreStoredAPIKeys(provider, data.APIKeys, s.encryptor); err != nil {
		return nil, err
	}

	// Restore OAuth secrets
	if err := restoreStoredOAuthSecrets(provider, data.OAuthSecrets, s.encryptor); err != nil {
		return nil, err
	}

	return provider, nil
}

// LoadAllProviders loads all providers from storage
func (s *FileStorage) LoadAllProviders() ([]*Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	storage, err := s.loadProvidersInternal()
	if err != nil {
		if os.IsNotExist(err) {
			return []*Provider{}, nil
		}
		return nil, err
	}

	providers := make([]*Provider, 0, len(storage.Providers))
	needsSave := false
	for _, data := range storage.Providers {
		provider := data.Provider
		if storedAPIKeysNeedEncryption(data.APIKeys, s.encryptor) || storedOAuthSecretsNeedEncryption(data.OAuthSecrets, s.encryptor) {
			needsSave = true
		}
		if err := restoreStoredAPIKeys(provider, data.APIKeys, s.encryptor); err != nil {
			return nil, err
		}
		// Backfill missing key IDs (pre-existing providers saved before key ID system)
		for i := range provider.APIKeys {
			if provider.APIKeys[i].ID == "" {
				provider.APIKeys[i].ID = GenerateID("key")
				needsSave = true
			}
			if normalizeAPIKeyMetadata(&provider.APIKeys[i]) {
				needsSave = true
			}
		}
		if err := restoreStoredOAuthSecrets(provider, data.OAuthSecrets, s.encryptor); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}

	// Persist backfilled IDs so they're stable across restarts
	if needsSave {
		storage.UpdatedAt = timeutil.NowTime()
		_ = s.saveProvidersInternal(storage)
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
	storage.UpdatedAt = timeutil.NowTime()

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
		return nil, fmt.Errorf("failed to unmarshal providers: %w", err)
	}

	// Restore API keys from separate storage to Provider.APIKeys
	for _, data := range storage.Providers {
		if data.Provider != nil {
			if err := restoreStoredAPIKeys(data.Provider, data.APIKeys, s.encryptor); err != nil {
				return nil, fmt.Errorf("failed to restore provider API keys: %w", err)
			}
			if err := restoreStoredOAuthSecrets(data.Provider, data.OAuthSecrets, s.encryptor); err != nil {
				return nil, fmt.Errorf("failed to restore provider oauth secrets: %w", err)
			}
		}
	}

	return &storage, nil
}

// saveProvidersInternal saves the providers file (must hold lock)
func (s *FileStorage) saveProvidersInternal(storage *providerStorage) error {
	data, err := json.MarshalIndent(storage, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.providersFile(), data, 0600)
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

	// Iterate through each day in the range (truncate to midnight to cover all days)
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())
	for d := startDay; !d.After(endDay); d = d.AddDate(0, 0, 1) {
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

// configFile returns the path to the pool configuration file
func (s *FileStorage) configFile() string {
	return filepath.Join(s.basePath, "config.json")
}

// SaveConfig saves the pool configuration
func (s *FileStorage) SaveConfig(config *PoolConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configFile(), data, 0644)
}

// LoadConfig loads the pool configuration
func (s *FileStorage) LoadConfig() (*PoolConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.configFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}
		return nil, err
	}

	var config PoolConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// extractOAuthSecrets extracts OAuth secrets from a provider for storage.
func extractOAuthSecrets(provider *Provider) *oauthSecrets {
	if provider.OAuth == nil {
		return nil
	}
	if provider.OAuth.AccessToken == "" && provider.OAuth.RefreshToken == "" && provider.OAuth.ClientSecret == "" {
		return nil
	}
	return &oauthSecrets{
		AccessToken:  provider.OAuth.AccessToken,
		RefreshToken: provider.OAuth.RefreshToken,
		ClientSecret: provider.OAuth.ClientSecret,
	}
}

// restoreOAuthSecrets restores OAuth secrets from storage into a provider.
func restoreOAuthSecrets(provider *Provider, secrets *oauthSecrets) {
	if secrets == nil || provider.OAuth == nil {
		return
	}
	provider.OAuth.AccessToken = secrets.AccessToken
	provider.OAuth.RefreshToken = secrets.RefreshToken
	provider.OAuth.ClientSecret = secrets.ClientSecret
}
