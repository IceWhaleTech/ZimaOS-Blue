package mediagen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type migrationMemoryConfigStore struct {
	configs map[string]*configOnDisk
}

func (m *migrationMemoryConfigStore) Load() (map[string]*configOnDisk, error) {
	if m.configs == nil {
		return nil, nil
	}
	result := make(map[string]*configOnDisk, len(m.configs))
	for k, v := range m.configs {
		copy := *v
		result[k] = &copy
	}
	return result, nil
}

func (m *migrationMemoryConfigStore) Save(configs map[string]*MediaProviderConfig) error {
	m.configs = make(map[string]*configOnDisk, len(configs))
	for k, v := range configs {
		m.configs[k] = &configOnDisk{
			ID:      v.ID,
			Enabled: v.Enabled,
			BaseURL: v.BaseURL,
			APIKey:  v.APIKey,
		}
	}
	return nil
}

func TestMigrateFromProviderPoolMigratesIntoActiveStore(t *testing.T) {
	t.Parallel()

	providerPoolDir := t.TempDir()
	store := &migrationMemoryConfigStore{}

	payload := map[string]any{
		"providers": map[string]any{
			"dashscope-image": map[string]any{
				"provider": map[string]any{
					"id":       "dashscope-image",
					"type":     "media",
					"enabled":  true,
					"base_url": "https://legacy.example.com",
				},
				"api_keys": []string{"legacy-key"},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal provider pool payload: %v", err)
	}
	if err := os.WriteFile(filepath.Join(providerPoolDir, "providers.json"), data, 0600); err != nil {
		t.Fatalf("write providers.json: %v", err)
	}

	MigrateFromProviderPool(providerPoolDir, store)

	cfg := store.configs["dashscope-image"]
	if cfg == nil {
		t.Fatal("expected dashscope-image config to be migrated")
	}
	if !cfg.Enabled {
		t.Fatal("expected migrated config to be enabled")
	}
	if cfg.BaseURL != "https://legacy.example.com" {
		t.Fatalf("unexpected base URL: %q", cfg.BaseURL)
	}
	if cfg.APIKey != "legacy-key" {
		t.Fatalf("unexpected api key: %q", cfg.APIKey)
	}

	updated, err := os.ReadFile(filepath.Join(providerPoolDir, "providers.json"))
	if err != nil {
		t.Fatalf("read updated providers.json: %v", err)
	}
	var cleaned map[string]map[string]json.RawMessage
	if err := json.Unmarshal(updated, &cleaned); err != nil {
		t.Fatalf("unmarshal cleaned providers.json: %v", err)
	}
	if providers := cleaned["providers"]; len(providers) != 0 {
		t.Fatalf("expected migrated media providers to be removed from live provider pool file, got %d entries", len(providers))
	}
}

func TestMigrateFromProviderPoolReadsBackupDirectory(t *testing.T) {
	t.Parallel()

	providerPoolDir := filepath.Join(t.TempDir(), "providerpool")
	backupDir := providerPoolDir + ".bak"
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatalf("mkdir backup dir: %v", err)
	}
	store := &migrationMemoryConfigStore{}

	payload := map[string]any{
		"providers": map[string]any{
			"gemini-image": map[string]any{
				"provider": map[string]any{
					"id":       "gemini-image",
					"type":     "media",
					"enabled":  true,
					"base_url": "https://backup.example.com",
				},
				"api_keys": []string{"backup-key"},
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal provider pool payload: %v", err)
	}
	backupFile := filepath.Join(backupDir, "providers.json")
	if err := os.WriteFile(backupFile, data, 0600); err != nil {
		t.Fatalf("write backup providers.json: %v", err)
	}

	MigrateFromProviderPool(providerPoolDir, store)

	cfg := store.configs["gemini-image"]
	if cfg == nil {
		t.Fatal("expected gemini-image config to be migrated from backup file")
	}
	if cfg.BaseURL != "https://backup.example.com" {
		t.Fatalf("unexpected base URL: %q", cfg.BaseURL)
	}
	if cfg.APIKey != "backup-key" {
		t.Fatalf("unexpected api key: %q", cfg.APIKey)
	}

	unchanged, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("read backup providers.json: %v", err)
	}
	if string(unchanged) != string(data) {
		t.Fatal("expected backup providers.json to remain unchanged")
	}
}
