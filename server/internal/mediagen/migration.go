package mediagen

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
)

func providerPoolLegacyCandidates(providerPoolDir string) []string {
	return []string{
		filepath.Join(providerPoolDir, "providers.json"),
		filepath.Join(providerPoolDir+".bak", "providers.json"),
	}
}

// MigrateFromProviderPool migrates media provider configs from legacy provider pool
// storage into the active media config store. When the source is the live provider
// pool file, migrated media entries are removed from that file. Backup files are
// treated as read-only migration sources.
func MigrateFromProviderPool(providerPoolDir string, store MediaConfigStore) {
	if store == nil {
		return
	}

	poolFile, data, err := readFirstProviderPoolLegacyFile(providerPoolDir)
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
		"mulerouter": true, "minimax-audio": true, "minimax-media": true,
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

	configs, err := loadMediaConfigsForMigration(store)
	if err != nil {
		log.Printf("[mediagen] migration: failed to load media configs: %v", err)
		return
	}

	migrated := 0
	for _, raw := range providers {
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
	}

	if migrated > 0 {
		if err := store.Save(configs); err != nil {
			log.Printf("[mediagen] migration: failed to save: %v", err)
			return
		}
		log.Printf("[mediagen] migrated %d media providers from provider pool", migrated)
	}

	if filepath.Dir(poolFile) == providerPoolDir+".bak" {
		return
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

func readFirstProviderPoolLegacyFile(providerPoolDir string) (string, []byte, error) {
	for _, candidate := range providerPoolLegacyCandidates(providerPoolDir) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return candidate, data, nil
		}
		if os.IsNotExist(err) {
			continue
		}
		return "", nil, err
	}
	return "", nil, os.ErrNotExist
}

func loadMediaConfigsForMigration(store MediaConfigStore) (map[string]*MediaProviderConfig, error) {
	configs := make(map[string]*MediaProviderConfig)
	for _, b := range BuiltinMediaProviders("") {
		configs[b.ID] = b
	}

	existing, err := store.Load()
	if err != nil {
		return nil, err
	}
	for id, cfg := range existing {
		current, ok := configs[id]
		if !ok {
			continue
		}
		current.Enabled = cfg.Enabled
		if cfg.BaseURL != "" {
			current.BaseURL = cfg.BaseURL
		}
		if cfg.APIKey != "" {
			current.APIKey = cfg.APIKey
			current.HasAPIKey = true
		}
	}

	return configs, nil
}
