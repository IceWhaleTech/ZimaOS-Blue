package providerpool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
		providerType := ProviderTypeBuiltin
		if name == "custom" || !isBuiltinProvider(providerID) {
			providerType = ProviderTypeCustom
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
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
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
				CreatedAt: time.Now(),
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
	builtinIDs := map[string]bool{
		"openai":       true,
		"anthropic":    true,
		"google":       true,
		"deepseek":     true,
		"moonshot":     true,
		"azure-openai": true,
		"openrouter":   true,
		"aihubmix":     true,
		"ollama":       true,
	}
	return builtinIDs[id]
}
