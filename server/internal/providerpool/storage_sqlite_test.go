package providerpool

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteStorage_LoadProvider_LegacyUpdatedAtFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pp-sqlite-legacy-time-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "providers.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	storage, err := NewSQLiteStorage(db)
	if err != nil {
		t.Fatalf("new sqlite storage: %v", err)
	}

	provider := &Provider{
		ID:       "legacy-time-provider",
		Name:     "Legacy Time Provider",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		BaseURL:  "https://example.com/v1",
		APIKeys: []APIKey{{
			ID:      "k1",
			Key:     "sk-test-legacy",
			KeyHash: HashAPIKey("sk-test-legacy"),
			Enabled: true,
		}},
		Priority:  10,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now(),
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	setRawProviderField(t, db, provider.ID, "updated_at", "2026-03-02 07:25:00")

	loaded, err := storage.LoadProvider(provider.ID)
	if err != nil {
		t.Fatalf("load provider with legacy timestamp: %v", err)
	}
	if loaded.ID != provider.ID {
		t.Fatalf("loaded wrong provider: %s", loaded.ID)
	}
	if len(loaded.APIKeys) != 1 || loaded.APIKeys[0].Key != "sk-test-legacy" {
		t.Fatalf("api key restore failed: %+v", loaded.APIKeys)
	}
	if loaded.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be parsed from legacy format")
	}

	all, err := storage.LoadAllProviders()
	if err != nil {
		t.Fatalf("load all providers: %v", err)
	}
	if len(all) != 1 || all[0].ID != provider.ID {
		t.Fatalf("expected 1 loaded provider, got %+v", all)
	}
}

func TestSQLiteStorage_LoadProvider_UnparseableTimestampStillLoads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pp-sqlite-bad-time-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "providers.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	storage, err := NewSQLiteStorage(db)
	if err != nil {
		t.Fatalf("new sqlite storage: %v", err)
	}

	provider := &Provider{
		ID:       "bad-time-provider",
		Name:     "Bad Time Provider",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		BaseURL:  "https://example.com/v1",
		APIKeys: []APIKey{{
			ID:      "k1",
			Key:     "sk-test-bad",
			KeyHash: HashAPIKey("sk-test-bad"),
			Enabled: true,
		}},
		Priority:  10,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now(),
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	setRawProviderField(t, db, provider.ID, "updated_at", "not-a-time")

	loaded, err := storage.LoadProvider(provider.ID)
	if err != nil {
		t.Fatalf("load provider with unparseable timestamp: %v", err)
	}
	if loaded.ID != provider.ID {
		t.Fatalf("loaded wrong provider: %s", loaded.ID)
	}
	if len(loaded.APIKeys) != 1 || loaded.APIKeys[0].Key != "sk-test-bad" {
		t.Fatalf("api key restore failed: %+v", loaded.APIKeys)
	}
}

func setRawProviderField(t *testing.T, db *sql.DB, providerID, field, value string) {
	t.Helper()

	var raw string
	if err := db.QueryRow("SELECT data FROM pp_providers WHERE id = ?", providerID).Scan(&raw); err != nil {
		t.Fatalf("query provider data: %v", err)
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("unmarshal provider data: %v", err)
	}
	obj[field] = value

	updated, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("marshal provider data: %v", err)
	}

	if _, err := db.Exec("UPDATE pp_providers SET data = ? WHERE id = ?", string(updated), providerID); err != nil {
		t.Fatalf("update provider data: %v", err)
	}
}
