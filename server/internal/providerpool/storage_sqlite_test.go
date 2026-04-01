package providerpool

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func testProviderSecretEncryptor(t *testing.T) SecretEncryptor {
	t.Helper()
	enc, err := auth.NewEncryptor(&auth.EncryptionConfig{Key: []byte("01234567890123456789012345678901")})
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}
	return enc
}

func TestSQLiteStorage_EncryptsProviderSecretsAtRest(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pp-sqlite-encrypted-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "providers.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	storage, err := NewSQLiteStorage(db, WithStorageEncryptor(testProviderSecretEncryptor(t)))
	if err != nil {
		t.Fatalf("new sqlite storage: %v", err)
	}

	provider := &Provider{
		ID:       "encrypted-provider",
		Name:     "Encrypted Provider",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		BaseURL:  "https://example.com/v1",
		APIKeys: []APIKey{{
			ID:      "k1",
			Key:     "sk-test-secret-12345",
			Enabled: true,
		}},
		OAuth: &OAuthConfig{
			AccessToken:  "oauth-access-secret",
			RefreshToken: "oauth-refresh-secret",
			ClientSecret: "oauth-client-secret",
		},
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	var rawKeys, rawOAuth string
	if err := db.QueryRow("SELECT api_keys, oauth_secrets FROM pp_providers WHERE id = ?", provider.ID).Scan(&rawKeys, &rawOAuth); err != nil {
		t.Fatalf("query raw secret columns: %v", err)
	}
	for _, secret := range []string{"sk-test-secret-12345", "oauth-access-secret", "oauth-refresh-secret", "oauth-client-secret"} {
		if strings.Contains(rawKeys, secret) || strings.Contains(rawOAuth, secret) {
			t.Fatalf("plaintext secret %q should not appear in stored columns", secret)
		}
	}
	if !strings.Contains(rawKeys, encryptedSecretPrefix) || !strings.Contains(rawOAuth, encryptedSecretPrefix) {
		t.Fatalf("expected encrypted secret prefix in stored columns, got api_keys=%q oauth=%q", rawKeys, rawOAuth)
	}

	loaded, err := storage.LoadProvider(provider.ID)
	if err != nil {
		t.Fatalf("load encrypted provider: %v", err)
	}
	if len(loaded.APIKeys) != 1 || loaded.APIKeys[0].Key != "sk-test-secret-12345" {
		t.Fatalf("api key restore failed: %+v", loaded.APIKeys)
	}
	if loaded.OAuth == nil || loaded.OAuth.AccessToken != "oauth-access-secret" || loaded.OAuth.RefreshToken != "oauth-refresh-secret" || loaded.OAuth.ClientSecret != "oauth-client-secret" {
		t.Fatalf("oauth secret restore failed: %+v", loaded.OAuth)
	}
}

func TestSQLiteStorage_LoadsLegacyPlaintextSecretsWithEncryptor(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pp-sqlite-legacy-secrets-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	db, err := sql.Open("sqlite3", filepath.Join(tmpDir, "providers.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	plainStorage, err := NewSQLiteStorage(db)
	if err != nil {
		t.Fatalf("new plaintext sqlite storage: %v", err)
	}
	provider := &Provider{
		ID:       "legacy-plaintext-provider",
		Name:     "Legacy Plaintext Provider",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		BaseURL:  "https://example.com/v1",
		APIKeys: []APIKey{{
			ID:      "k1",
			Key:     "sk-legacy-plaintext",
			Enabled: true,
		}},
	}
	if err := plainStorage.SaveProvider(provider); err != nil {
		t.Fatalf("save plaintext provider: %v", err)
	}

	encryptedStorage, err := NewSQLiteStorage(db, WithStorageEncryptor(testProviderSecretEncryptor(t)))
	if err != nil {
		t.Fatalf("new encrypted sqlite storage: %v", err)
	}
	loaded, err := encryptedStorage.LoadProvider(provider.ID)
	if err != nil {
		t.Fatalf("load legacy plaintext provider with encryptor: %v", err)
	}
	if len(loaded.APIKeys) != 1 || loaded.APIKeys[0].Key != "sk-legacy-plaintext" {
		t.Fatalf("expected legacy plaintext API key to load, got %+v", loaded.APIKeys)
	}
}

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

func TestSQLiteStorage_UsesReaderDBForLoads(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pp-sqlite-reader-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "providers.db")
	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open writer sqlite: %v", err)
	}
	defer writeDB.Close()

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open reader sqlite: %v", err)
	}
	defer readDB.Close()

	storage, err := NewSQLiteStorageWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("new sqlite storage with read db: %v", err)
	}
	if storage.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if storage.readDB == storage.db {
		t.Fatal("expected sqlite storage to use a separate read db")
	}

	provider := &Provider{
		ID:       "reader-provider",
		Name:     "Reader Provider",
		Type:     ProviderTypeCustom,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
		BaseURL:  "https://example.com/v1",
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("save provider: %v", err)
	}

	loaded, err := storage.LoadProvider(provider.ID)
	if err != nil {
		t.Fatalf("load provider through reader db: %v", err)
	}
	if loaded == nil || loaded.ID != provider.ID {
		t.Fatalf("expected provider %q via reader db, got %+v", provider.ID, loaded)
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
