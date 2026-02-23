package providerpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
	Migrated       int      `json:"migrated"`
	Skipped        int      `json:"skipped"`
	Errors         []string `json:"errors,omitempty"`
	MigratedNames  []string `json:"migrated_names,omitempty"`
	SkippedNames   []string `json:"skipped_names,omitempty"`
	BackupPath     string   `json:"backup_path,omitempty"`
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

		// Determine provider type: custom providers should be ProviderTypeCustom
		providerType := ProviderTypeCustom
		if isBuiltinProvider(providerID) {
			providerType = getBuiltinProviderType(providerID)
		}

		// Create new provider
		provider := &Provider{
			ID:        providerID,
			Name:      getProviderDisplayName(providerID),
			Type:      providerType,
			Enabled:   legacyProvider.Enabled,
			Status:    ProviderStatusInactive,
			BaseURL:   legacyProvider.BaseURL,
			Priority:  getDefaultPriority(providerID),
			CreatedAt: timeutil.NowTime(),
			UpdatedAt: timeutil.NowTime(),
		}

		// Set default base URL if not specified
		if provider.BaseURL == "" {
			provider.BaseURL = getDefaultBaseURL(providerID)
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
	mapping := map[string]string{
		"claude":  "anthropic",
		"openai":  "openai",
		"ollama":  "ollama",
		"custom":  "custom",
	}

	if id, ok := mapping[name]; ok {
		return id
	}
	return name
}

// getProviderDisplayName returns the display name for a provider ID
func getProviderDisplayName(id string) string {
	names := map[string]string{
		"anthropic": "Anthropic",
		"openai":    "OpenAI",
		"ollama":    "Ollama",
		"custom":    "Custom Provider",
		"deepseek":  "DeepSeek",
		"google":    "Google AI",
		"moonshot":  "Moonshot",
	}

	if name, ok := names[id]; ok {
		return name
	}
	return id
}

// getDefaultBaseURL returns the default base URL for a provider
func getDefaultBaseURL(id string) string {
	urls := map[string]string{
		"anthropic": "https://api.anthropic.com",
		"openai":    "https://api.openai.com/v1",
		"ollama":    "http://localhost:11434",
		"deepseek":  "https://api.deepseek.com",
		"google":    "https://generativelanguage.googleapis.com",
		"moonshot":  "https://api.moonshot.cn/v1",
	}

	if url, ok := urls[id]; ok {
		return url
	}
	return ""
}

// getDefaultPriority returns the default priority for a provider
func getDefaultPriority(id string) int {
	priorities := map[string]int{
		"anthropic": 100,
		"openai":    90,
		"deepseek":  80,
		"google":    70,
		"moonshot":  60,
		"ollama":    50,
		"custom":    40,
	}

	if priority, ok := priorities[id]; ok {
		return priority
	}
	return 50
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

// isBuiltinProvider checks if a provider ID is a built-in provider
func isBuiltinProvider(id string) bool {
	for _, p := range BuiltinProviders() {
		if p.ID == id {
			return true
		}
	}
	return false
}

// getBuiltinProviderType returns the type for a known builtin/platform provider.
func getBuiltinProviderType(id string) ProviderType {
	for _, p := range BuiltinProviders() {
		if p.ID == id {
			return p.Type
		}
	}
	return ProviderTypeCustom
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

	// Build a map of API key hashes to providers
	keyHashToProviders := make(map[string][]*Provider)

	for _, p := range providers {
		for _, key := range p.APIKeys {
			if key.KeyHash != "" && key.Enabled {
				keyHashToProviders[key.KeyHash] = append(keyHashToProviders[key.KeyHash], p)
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
		if key.KeyHash != "" {
			existingHashes[key.KeyHash] = true
		}
	}

	// Add unique keys from sources
	for _, source := range sources {
		for _, key := range source.APIKeys {
			if key.KeyHash != "" && !existingHashes[key.KeyHash] {
				// Update label to indicate merge
				key.Label = fmt.Sprintf("Merged from %s", source.ID)
				target.APIKeys = append(target.APIKeys, key)
				existingHashes[key.KeyHash] = true
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
