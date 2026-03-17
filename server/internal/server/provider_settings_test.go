package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

func TestMigrateLegacyProviderSettingsImportsPrimaryFile(t *testing.T) {
	t.Parallel()

	kv := kvstore.NewMemoryStore()
	dataDir := t.TempDir()
	legacyPath := filepath.Join(dataDir, legacyProviderSettingsFilename)

	if err := os.WriteFile(legacyPath, []byte(`{"providers":{"openai":{"name":"openai","api_key":"sk-test","base_url":"https://api.openai.com","enabled":true}}}`), 0600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	result, err := MigrateLegacyProviderSettings(context.Background(), kv, dataDir)
	if err != nil {
		t.Fatalf("migrate legacy provider settings: %v", err)
	}
	if result == nil {
		t.Fatal("expected migration result")
	}
	if result.SourcePath != legacyPath {
		t.Fatalf("unexpected source path: %s", result.SourcePath)
	}

	var cfg ProvidersConfig
	if err := kv.GetJSON(context.Background(), providerSettingsKVKey, &cfg); err != nil {
		t.Fatalf("load migrated config from kvstore: %v", err)
	}
	got, ok := cfg.Providers["openai"]
	if !ok {
		t.Fatal("expected openai provider config in kvstore")
	}
	if got.APIKey != "sk-test" {
		t.Fatalf("unexpected api key: %q", got.APIKey)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected primary legacy file to be archived, stat err=%v", err)
	}
	if result.ArchivedPath == "" {
		t.Fatal("expected archived path to be recorded")
	}
	if _, err := os.Stat(result.ArchivedPath); err != nil {
		t.Fatalf("expected archived file to exist: %v", err)
	}
}

func TestMigrateLegacyProviderSettingsImportsArchivedFile(t *testing.T) {
	t.Parallel()

	kv := kvstore.NewMemoryStore()
	dataDir := t.TempDir()
	archivedPath := filepath.Join(dataDir, legacyProviderSettingsFilename+".migrated")

	if err := os.WriteFile(archivedPath, []byte(`{"providers":{"claude":{"name":"claude","api_key":"legacy-key","enabled":true}}}`), 0600); err != nil {
		t.Fatalf("write archived legacy file: %v", err)
	}

	result, err := MigrateLegacyProviderSettings(context.Background(), kv, dataDir)
	if err != nil {
		t.Fatalf("migrate archived legacy provider settings: %v", err)
	}
	if result == nil {
		t.Fatal("expected migration result")
	}
	if result.SourcePath != archivedPath {
		t.Fatalf("unexpected source path: %s", result.SourcePath)
	}
	if result.ArchivedPath != "" {
		t.Fatalf("expected no archive step for archived input, got %q", result.ArchivedPath)
	}

	var cfg ProvidersConfig
	if err := kv.GetJSON(context.Background(), providerSettingsKVKey, &cfg); err != nil {
		t.Fatalf("load migrated archived config from kvstore: %v", err)
	}
	got, ok := cfg.Providers["claude"]
	if !ok {
		t.Fatal("expected claude provider config in kvstore")
	}
	if got.APIKey != "legacy-key" {
		t.Fatalf("unexpected api key: %q", got.APIKey)
	}
}

func TestMigrateLegacyProviderSettingsSkipsWhenKVAlreadyPopulated(t *testing.T) {
	t.Parallel()

	kv := kvstore.NewMemoryStore()
	dataDir := t.TempDir()
	legacyPath := filepath.Join(dataDir, legacyProviderSettingsFilename)

	existing := ProvidersConfig{
		Providers: map[string]ProviderConfig{
			"openai": {
				Name:    "openai",
				APIKey:  "from-db",
				Enabled: true,
			},
		},
	}
	if err := kv.SetJSON(context.Background(), providerSettingsKVKey, &existing, 0); err != nil {
		t.Fatalf("seed kvstore config: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"providers":{"openai":{"name":"openai","api_key":"from-file","enabled":true}}}`), 0600); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	result, err := MigrateLegacyProviderSettings(context.Background(), kv, dataDir)
	if err != nil {
		t.Fatalf("migrate with existing kv config: %v", err)
	}
	if result != nil {
		t.Fatalf("expected no migration result when kvstore already populated, got %#v", result)
	}

	var cfg ProvidersConfig
	if err := kv.GetJSON(context.Background(), providerSettingsKVKey, &cfg); err != nil {
		t.Fatalf("load kvstore config: %v", err)
	}
	if cfg.Providers["openai"].APIKey != "from-db" {
		t.Fatalf("expected kvstore config to remain unchanged, got %q", cfg.Providers["openai"].APIKey)
	}
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("expected legacy file to remain untouched: %v", err)
	}
}
