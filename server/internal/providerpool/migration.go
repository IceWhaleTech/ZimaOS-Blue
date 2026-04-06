package providerpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// LegacyProviderConfig represents the old provider configuration format
type LegacyProviderConfig struct {
	Name    string `json:"name"`
	APIKey  string `json:"api_key,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
	Enabled bool   `json:"enabled"`
}

// LegacyProvidersConfig represents the old providers configuration file
type LegacyProvidersConfig struct {
	Providers map[string]LegacyProviderConfig `json:"providers"`
}

// MigrationResult contains the result of a migration operation
type MigrationResult struct {
	Migrated      int      `json:"migrated"`
	Skipped       int      `json:"skipped"`
	Errors        []string `json:"errors,omitempty"`
	MigratedNames []string `json:"migrated_names,omitempty"`
	SkippedNames  []string `json:"skipped_names,omitempty"`
	BackupPath    string   `json:"backup_path,omitempty"`
}

type legacyProviderDefaults struct {
	DisplayName string
	BaseURL     string
	Priority    int
	Type        ProviderType
}

// MigrateFromLegacy migrates provider settings from the old format to the new Provider Pool format
func MigrateFromLegacy(dataDir string, pool *Pool) (*MigrationResult, error) {
	result := &MigrationResult{}

	// Path to old provider settings
	legacyPath := filepath.Join(dataDir, "provider_settings.json")

	// Check if legacy file exists
	if _, err := os.Stat(legacyPath); os.IsNotExist(err) {
		// No legacy config to migrate
		return result, nil
	}

	// Read legacy config
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read legacy config: %w", err)
	}

	var legacyConfig LegacyProvidersConfig
	if err := json.Unmarshal(data, &legacyConfig); err != nil {
		return nil, fmt.Errorf("failed to parse legacy config: %w", err)
	}

	// Check if there are any providers to migrate
	if len(legacyConfig.Providers) == 0 {
		return result, nil
	}

	// Get existing providers to avoid duplicates
	existingProviders := pool.Registry.List()
	existingIDs := make(map[string]bool)
	for _, p := range existingProviders {
		existingIDs[p.ID] = true
	}

	// Migrate each provider
	for name, legacyProvider := range legacyConfig.Providers {
		// Map legacy provider names to new IDs
		providerID := mapLegacyProviderName(name)

		// Skip if already exists in new system
		if existingIDs[providerID] {
			result.Skipped++
			result.SkippedNames = append(result.SkippedNames, name)
			continue
		}

		defaults, hasDefaults := lookupLegacyProviderDefaults(providerID)

		providerType := ProviderTypeCustom
		if hasDefaults {
			providerType = defaults.Type
		} else if builtinType, ok := lookupBuiltinProviderType(providerID); ok {
			providerType = builtinType
		}

		displayName := providerID
		baseURL := legacyProvider.BaseURL
		priority := 50
		if hasDefaults {
			displayName = defaults.DisplayName
			priority = defaults.Priority
			if baseURL == "" {
				baseURL = defaults.BaseURL
			}
		}

		// Create new provider
		provider := &Provider{
			ID:        providerID,
			Name:      displayName,
			Type:      providerType,
			Enabled:   legacyProvider.Enabled,
			Status:    ProviderStatusInactive,
			BaseURL:   baseURL,
			Priority:  priority,
			CreatedAt: timeutil.NowTime(),
			UpdatedAt: timeutil.NowTime(),
		}

		// Add API key if present
		if legacyProvider.APIKey != "" {
			apiKey := APIKey{
				ID:        uuid.New().String(),
				Key:       legacyProvider.APIKey,
				KeyHash:   hashAPIKey(legacyProvider.APIKey),
				Label:     "Migrated from legacy",
				CreatedAt: timeutil.NowTime(),
				Enabled:   true,
			}
			provider.APIKeys = []APIKey{apiKey}
			provider.Status = ProviderStatusActive
		}

		// Register the provider
		if err := pool.Registry.Register(provider); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to migrate %s: %v", name, err))
			continue
		}

		result.Migrated++
		result.MigratedNames = append(result.MigratedNames, name)
	}

	// Backup the legacy file
	if result.Migrated > 0 {
		backupPath := legacyPath + ".migrated"
		if err := os.Rename(legacyPath, backupPath); err != nil {
			// If rename fails, try copy
			if err := copyFile(legacyPath, backupPath); err == nil {
				os.Remove(legacyPath)
			}
		}
		result.BackupPath = backupPath
	}

	return result, nil
}

// mapLegacyProviderName maps old provider names to new provider IDs
func mapLegacyProviderName(name string) string {
	switch name {
	case "claude":
		return "anthropic"
	case "openai", "ollama", "custom":
		return name
	default:
		return name
	}
}

func lookupLegacyProviderDefaults(id string) (legacyProviderDefaults, bool) {
	switch id {
	case "anthropic":
		return legacyProviderDefaults{
			DisplayName: "Anthropic",
			BaseURL:     "https://api.anthropic.com",
			Priority:    100,
			Type:        ProviderTypeBuiltin,
		}, true
	case "openai":
		return legacyProviderDefaults{
			DisplayName: "OpenAI",
			BaseURL:     "https://api.openai.com/v1",
			Priority:    90,
			Type:        ProviderTypeBuiltin,
		}, true
	case "ollama":
		return legacyProviderDefaults{
			DisplayName: "Ollama",
			BaseURL:     "http://localhost:11434",
			Priority:    50,
			Type:        ProviderTypeBuiltin,
		}, true
	case "custom":
		return legacyProviderDefaults{
			DisplayName: "Custom Provider",
			Priority:    40,
			Type:        ProviderTypeCustom,
		}, true
	case "deepseek":
		return legacyProviderDefaults{
			DisplayName: "DeepSeek",
			BaseURL:     "https://api.deepseek.com",
			Priority:    80,
			Type:        ProviderTypeBuiltin,
		}, true
	case "google":
		return legacyProviderDefaults{
			DisplayName: "Google AI",
			BaseURL:     "https://generativelanguage.googleapis.com",
			Priority:    70,
			Type:        ProviderTypeBuiltin,
		}, true
	case "moonshot":
		return legacyProviderDefaults{
			DisplayName: "Moonshot",
			BaseURL:     "https://api.moonshot.cn/v1",
			Priority:    60,
			Type:        ProviderTypeBuiltin,
		}, true
	default:
		return legacyProviderDefaults{Type: ProviderTypeCustom}, false
	}
}

func lookupBuiltinProviderType(id string) (ProviderType, bool) {
	for _, p := range BuiltinProviders() {
		if p.ID == id {
			return p.Type, true
		}
	}
	return "", false
}

// hashAPIKey creates a hash representation of an API key for display
func hashAPIKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

// CheckMigrationNeeded checks if there's a legacy config that needs migration
func CheckMigrationNeeded(dataDir string) bool {
	legacyPath := filepath.Join(dataDir, "provider_settings.json")
	_, err := os.Stat(legacyPath)
	return err == nil
}

// DeduplicateResult contains the result of a deduplication operation
type DeduplicateResult struct {
	Merged       int      `json:"merged"`
	Deleted      []string `json:"deleted,omitempty"`
	KeptProvider string   `json:"kept_provider,omitempty"`
}

// DeduplicateProviders finds and merges duplicate custom providers based on base_url and api_key.
// This handles historical data where providers like "claude-code" and "custom" may have been
// created with the same configuration.
func DeduplicateProviders(pool *Pool) (*DeduplicateResult, error) {
	result := &DeduplicateResult{}

	providers := pool.Registry.List()
	if len(providers) < 2 {
		return result, nil
	}

	// Group custom providers by their base_url (normalized)
	type providerGroup struct {
		providers []*Provider
	}
	groups := make(map[string]*providerGroup)

	for _, p := range providers {
		// Only consider custom providers for deduplication
		if p.Type != ProviderTypeCustom {
			continue
		}

		// Normalize base URL (remove trailing slash)
		baseURL := normalizeURL(p.BaseURL)
		if baseURL == "" {
			continue
		}

		if groups[baseURL] == nil {
			groups[baseURL] = &providerGroup{}
		}
		groups[baseURL].providers = append(groups[baseURL].providers, p)
	}

	// Process each group with duplicates
	for _, group := range groups {
		if len(group.providers) < 2 {
			continue
		}

		// Check if providers have the same API key (by hash)
		duplicates := findDuplicatesByAPIKey(group.providers)
		if len(duplicates) < 2 {
			continue
		}

		// Merge duplicates: keep the one with higher priority or "custom" ID
		kept, toDelete := selectProviderToKeep(duplicates)
		if kept == nil || len(toDelete) == 0 {
			continue
		}

		// Merge API keys from deleted providers into the kept one
		mergeAPIKeys(kept, toDelete)

		// Update the kept provider
		if err := pool.Registry.Update(kept); err != nil {
			continue
		}

		// Delete the duplicate providers
		for _, p := range toDelete {
			if err := pool.Registry.Unregister(p.ID); err != nil {
				continue
			}
			result.Deleted = append(result.Deleted, p.ID)
			result.Merged++
		}

		if result.Merged > 0 {
			result.KeptProvider = kept.ID
		}
	}

	return result, nil
}

// normalizeURL normalizes a URL by removing trailing slashes
func normalizeURL(url string) string {
	for len(url) > 0 && url[len(url)-1] == '/' {
		url = url[:len(url)-1]
	}
	return url
}

// findDuplicatesByAPIKey finds providers that share the same API key
func findDuplicatesByAPIKey(providers []*Provider) []*Provider {
	if len(providers) < 2 {
		return nil
	}

	// Build a map of API key fingerprints to providers.
	keyHashToProviders := make(map[string][]*Provider)

	for _, p := range providers {
		for _, key := range p.APIKeys {
			if fp := apiKeyFingerprint(&key); fp != "" && key.Enabled {
				keyHashToProviders[fp] = append(keyHashToProviders[fp], p)
				break // Only consider the first enabled key
			}
		}
	}

	// Find the largest group of duplicates
	var largest []*Provider
	for _, group := range keyHashToProviders {
		if len(group) > len(largest) {
			largest = group
		}
	}

	return largest
}

// selectProviderToKeep selects which provider to keep and which to delete
// Priority: "custom" ID > higher priority > earlier creation time
func selectProviderToKeep(providers []*Provider) (*Provider, []*Provider) {
	if len(providers) < 2 {
		return nil, nil
	}

	var kept *Provider
	var toDelete []*Provider

	// First, prefer "custom" as the canonical ID
	for _, p := range providers {
		if p.ID == "custom" {
			kept = p
			break
		}
	}

	// If no "custom" found, select by priority then creation time
	if kept == nil {
		kept = providers[0]
		for _, p := range providers[1:] {
			if p.Priority > kept.Priority {
				kept = p
			} else if p.Priority == kept.Priority && p.CreatedAt.Before(kept.CreatedAt) {
				kept = p
			}
		}
	}

	// Build the list of providers to delete
	for _, p := range providers {
		if p.ID != kept.ID {
			toDelete = append(toDelete, p)
		}
	}

	return kept, toDelete
}

// mergeAPIKeys merges API keys from source providers into the target provider
func mergeAPIKeys(target *Provider, sources []*Provider) {
	// Build a set of existing key hashes
	existingHashes := make(map[string]bool)
	for _, key := range target.APIKeys {
		if fp := apiKeyFingerprint(&key); fp != "" {
			existingHashes[fp] = true
		}
	}

	// Add unique keys from sources
	for _, source := range sources {
		for _, key := range source.APIKeys {
			if fp := apiKeyFingerprint(&key); fp != "" && !existingHashes[fp] {
				// Update label to indicate merge
				key.Label = fmt.Sprintf("Merged from %s", source.ID)
				target.APIKeys = append(target.APIKeys, key)
				existingHashes[fp] = true
			}
		}
	}

	// Update the target's enabled status if any source was enabled
	for _, source := range sources {
		if source.Enabled {
			target.Enabled = true
			if target.Status == ProviderStatusInactive {
				target.Status = ProviderStatusActive
			}
			break
		}
	}
}
