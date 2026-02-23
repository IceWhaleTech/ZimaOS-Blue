package mediagen

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

// MigrateFromProviderPool migrates media provider configs from the provider pool's
// providers.json to the mediagen config store, then removes media entries from the
// provider pool file. This is idempotent.
func MigrateFromProviderPool(providerPoolDir, mediaDir string) {
	poolFile := filepath.Join(providerPoolDir, "providers.json")
	data, err := os.ReadFile(poolFile)
	if err != nil {
		return // No provider pool data
	}

	// Parse as generic JSON to preserve structure
	var storage map[string]json.RawMessage
	if err := json.Unmarshal(data, &storage); err != nil {
		return
	}

	providersRaw, ok := storage["providers"]
	if !ok {
		return
	}

	var providers map[string]json.RawMessage
	if err := json.Unmarshal(providersRaw, &providers); err != nil {
		return
	}

	// Identify media providers
	mediaIDs := map[string]bool{
		"dashscope-image": true, "gemini-image": true,
		"mulerouter": true, "minimax-audio": true,
	}

	type providerEntry struct {
		Provider *struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Enabled bool   `json:"enabled"`
			BaseURL string `json:"base_url"`
		} `json:"provider"`
		APIKeys []string `json:"api_keys,omitempty"`
	}

	// Migrate to media config (only if not already done)
	mediaConfigPath := filepath.Join(mediaDir, configStoreFile)
	if _, statErr := os.Stat(mediaConfigPath); statErr != nil {
		store := NewConfigStore(mediaDir)
		configs := make(map[string]*MediaProviderConfig)
		for _, b := range BuiltinMediaProviders("") {
			configs[b.ID] = b
		}

		migrated := 0
		for key, raw := range providers {
			var pd providerEntry
			if json.Unmarshal(raw, &pd) != nil || pd.Provider == nil {
				continue
			}
			if pd.Provider.Type != "media" && !mediaIDs[pd.Provider.ID] {
				continue
			}
			c, ok := configs[pd.Provider.ID]
			if !ok {
				continue
			}
			c.Enabled = pd.Provider.Enabled
			if pd.Provider.BaseURL != "" {
				c.BaseURL = pd.Provider.BaseURL
			}
			if len(pd.APIKeys) > 0 && pd.APIKeys[0] != "" {
				c.APIKey = pd.APIKeys[0]
				c.HasAPIKey = true
			}
			migrated++
			_ = key
		}

		if migrated > 0 {
			if err := store.Save(configs); err != nil {
				log.Printf("[mediagen] migration: failed to save: %v", err)
				return
			}
			log.Printf("[mediagen] migrated %d media providers from provider pool", migrated)
		}
	}

	// Remove media providers from provider pool file
	removed := 0
	for key, raw := range providers {
		if mediaIDs[key] {
			delete(providers, key)
			removed++
			continue
		}
		// Also check by provider.id or provider.type
		var pd providerEntry
		if json.Unmarshal(raw, &pd) == nil && pd.Provider != nil {
			if mediaIDs[pd.Provider.ID] || pd.Provider.Type == "media" {
				delete(providers, key)
				removed++
			}
		}
	}

	if removed > 0 {
		updatedProviders, _ := json.Marshal(providers)
		storage["providers"] = updatedProviders
		updatedData, err := json.MarshalIndent(storage, "", "  ")
		if err == nil {
			if err := os.WriteFile(poolFile, updatedData, 0600); err != nil {
				log.Printf("[mediagen] migration: failed to clean provider pool: %v", err)
			} else {
				log.Printf("[mediagen] removed %d media providers from provider pool", removed)
			}
		}
	}
}
