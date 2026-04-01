package oauth

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func openOAuthTestDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestSQLiteTokenStore_SaveLoadAndDeleteToken(t *testing.T) {
	db := openOAuthTestDB(t, ":memory:")

	store, err := NewSQLiteTokenStore(db)
	if err != nil {
		t.Fatalf("create sqlite token store: %v", err)
	}

	now := time.Now().UTC()
	token := &Token{
		ProviderType: "copilot",
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenExpiry:  now.Add(time.Hour),
		Scopes:       []string{"scope:a", "scope:b"},
		Email:        "dev@example.com",
		ProjectID:    "project-1",
		Endpoint:     "https://example.com",
		CreatedAt:    now,
	}
	if err := store.SaveToken("provider-a", token); err != nil {
		t.Fatalf("save token: %v", err)
	}
	if token.ID == "" {
		t.Fatal("expected SaveToken to assign an ID")
	}

	loaded, err := store.LoadToken("provider-a", token.ID)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if loaded.Email != token.Email {
		t.Fatalf("expected email %q, got %q", token.Email, loaded.Email)
	}
	if len(loaded.Scopes) != 2 {
		t.Fatalf("expected 2 scopes, got %d", len(loaded.Scopes))
	}

	byEmail, err := store.LoadTokenByEmail("provider-a", token.Email)
	if err != nil {
		t.Fatalf("load token by email: %v", err)
	}
	if byEmail == nil || byEmail.ID != token.ID {
		t.Fatalf("expected token %q from LoadTokenByEmail, got %+v", token.ID, byEmail)
	}

	if err := store.DeleteToken("provider-a", token.ID); err != nil {
		t.Fatalf("delete token: %v", err)
	}
	if _, err := store.LoadToken("provider-a", token.ID); err == nil {
		t.Fatal("expected deleted token lookup to fail")
	}
}

func TestSQLiteTokenStore_LoadTokensMigratesEmptyProviderID(t *testing.T) {
	db := openOAuthTestDB(t, ":memory:")

	store, err := NewSQLiteTokenStore(db)
	if err != nil {
		t.Fatalf("create sqlite token store: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO oauth_tokens
		(id, provider_id, provider_type, access_token, refresh_token, token_expiry, email, created_at, updated_at)
		VALUES (?, '', ?, ?, ?, ?, ?, ?, ?)`,
		"tok-empty-provider", "copilot", "access", "refresh", now, "dev@example.com", now, now); err != nil {
		t.Fatalf("insert legacy token row: %v", err)
	}

	tokens, err := store.LoadTokens("provider-a")
	if err != nil {
		t.Fatalf("load tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(tokens))
	}
	if tokens[0].ProviderID != "provider-a" {
		t.Fatalf("expected provider_id to be migrated to provider-a, got %q", tokens[0].ProviderID)
	}

	var persistedProviderID string
	if err := db.QueryRow(`SELECT provider_id FROM oauth_tokens WHERE id = ?`, "tok-empty-provider").Scan(&persistedProviderID); err != nil {
		t.Fatalf("query migrated provider_id: %v", err)
	}
	if persistedProviderID != "provider-a" {
		t.Fatalf("expected persisted provider_id provider-a, got %q", persistedProviderID)
	}
}

func TestSQLiteTokenStore_MigratesLegacySchemaWithMissingIDColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "oauth.db")
	db := openOAuthTestDB(t, dbPath)

	if _, err := db.Exec(`CREATE TABLE oauth_tokens (
		provider_id   TEXT PRIMARY KEY,
		provider_type TEXT NOT NULL DEFAULT '',
		access_token  TEXT NOT NULL DEFAULT '',
		refresh_token TEXT NOT NULL DEFAULT '',
		token_expiry  TEXT NOT NULL DEFAULT '',
		scopes        TEXT,
		email         TEXT,
		project_id    TEXT,
		endpoint      TEXT,
		created_at    TEXT NOT NULL DEFAULT '',
		updated_at    TEXT NOT NULL DEFAULT ''
	)`); err != nil {
		t.Fatalf("create legacy oauth_tokens table: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := db.Exec(`INSERT INTO oauth_tokens
		(provider_id, provider_type, access_token, refresh_token, token_expiry, email, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"provider-a", "copilot", "access", "refresh", now, "dev@example.com", now, now); err != nil {
		t.Fatalf("insert legacy oauth token: %v", err)
	}

	store, err := NewSQLiteTokenStoreWithReadDB(db, db)
	if err != nil {
		t.Fatalf("create sqlite token store with migration: %v", err)
	}

	var idColumnCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('oauth_tokens') WHERE name = 'id'`).Scan(&idColumnCount); err != nil {
		t.Fatalf("query oauth_tokens columns: %v", err)
	}
	if idColumnCount != 1 {
		t.Fatalf("expected migrated oauth_tokens schema to include id column, got count=%d", idColumnCount)
	}

	tokens, err := store.LoadTokens("provider-a")
	if err != nil {
		t.Fatalf("load migrated tokens: %v", err)
	}
	if len(tokens) != 1 {
		t.Fatalf("expected 1 migrated token, got %d", len(tokens))
	}
	if tokens[0].ID == "" {
		t.Fatal("expected migrated token to receive generated id")
	}
	if tokens[0].ProviderID != "provider-a" {
		t.Fatalf("expected provider_id provider-a, got %q", tokens[0].ProviderID)
	}
}

func TestSQLiteTokenStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "oauth-reader.db")

	writeDB := openOAuthTestDB(t, dbPath)

	bootstrapStore, err := NewSQLiteTokenStore(writeDB)
	if err != nil {
		t.Fatalf("create sqlite token store: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	token := &Token{
		ID:           "tok-reader",
		ProviderType: "copilot",
		AccessToken:  "access-reader",
		RefreshToken: "refresh-reader",
		TokenExpiry:  now.Add(time.Hour),
		Scopes:       []string{"scope:a", "scope:b"},
		Email:        "reader@example.com",
		ProjectID:    "project-reader",
		Endpoint:     "https://example.com/reader",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := bootstrapStore.SaveToken("provider-reader", token); err != nil {
		t.Fatalf("save token: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open reader db: %v", err)
	}
	defer func() {
		_ = readDB.Close()
	}()

	store, err := NewSQLiteTokenStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("create sqlite token store with reader: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate read db")
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	loaded, err := store.LoadToken("provider-reader", token.ID)
	if err != nil {
		t.Fatalf("load token via reader: %v", err)
	}
	if loaded == nil || loaded.Email != token.Email || loaded.AccessToken != token.AccessToken {
		t.Fatalf("unexpected token via reader: %+v", loaded)
	}

	byEmail, err := store.LoadTokenByEmail("provider-reader", token.Email)
	if err != nil {
		t.Fatalf("load token by email via reader: %v", err)
	}
	if byEmail == nil || byEmail.ID != token.ID {
		t.Fatalf("unexpected token by email via reader: %+v", byEmail)
	}

	tokens, err := store.LoadTokens("provider-reader")
	if err != nil {
		t.Fatalf("load tokens via reader: %v", err)
	}
	if len(tokens) != 1 || tokens[0].ID != token.ID {
		t.Fatalf("unexpected token list via reader: %+v", tokens)
	}
}

func TestSQLiteTokenStore_DeduplicateKeepsLatestByEmail(t *testing.T) {
	db := openOAuthTestDB(t, ":memory:")

	store, err := NewSQLiteTokenStore(db)
	if err != nil {
		t.Fatalf("create sqlite token store: %v", err)
	}

	base := time.Now().UTC().Truncate(time.Second)
	for _, tc := range []struct {
		providerID string
		token      *Token
	}{
		{
			providerID: "provider-a",
			token: &Token{
				ID:           "tok-old",
				ProviderType: "copilot",
				AccessToken:  "access-old",
				RefreshToken: "refresh-old",
				TokenExpiry:  base.Add(1 * time.Hour),
				Email:        "same@example.com",
				CreatedAt:    base,
				UpdatedAt:    base,
			},
		},
		{
			providerID: "provider-a",
			token: &Token{
				ID:           "tok-new",
				ProviderType: "copilot",
				AccessToken:  "access-new",
				RefreshToken: "refresh-new",
				TokenExpiry:  base.Add(2 * time.Hour),
				Email:        "same@example.com",
				CreatedAt:    base.Add(1 * time.Minute),
				UpdatedAt:    base.Add(1 * time.Minute),
			},
		},
		{
			providerID: "provider-a",
			token: &Token{
				ID:           "tok-other-email",
				ProviderType: "copilot",
				AccessToken:  "access-other",
				RefreshToken: "refresh-other",
				TokenExpiry:  base.Add(90 * time.Minute),
				Email:        "other@example.com",
				CreatedAt:    base.Add(2 * time.Minute),
				UpdatedAt:    base.Add(2 * time.Minute),
			},
		},
		{
			providerID: "provider-b",
			token: &Token{
				ID:           "tok-other-provider",
				ProviderType: "copilot",
				AccessToken:  "access-provider-b",
				RefreshToken: "refresh-provider-b",
				TokenExpiry:  base.Add(3 * time.Hour),
				Email:        "same@example.com",
				CreatedAt:    base.Add(3 * time.Minute),
				UpdatedAt:    base.Add(3 * time.Minute),
			},
		},
	} {
		if err := store.SaveToken(tc.providerID, tc.token); err != nil {
			t.Fatalf("save token %s:%s: %v", tc.providerID, tc.token.ID, err)
		}
	}

	if err := store.Deduplicate(); err != nil {
		t.Fatalf("Deduplicate() error: %v", err)
	}

	providerATokens, err := store.LoadTokens("provider-a")
	if err != nil {
		t.Fatalf("LoadTokens(provider-a): %v", err)
	}
	if len(providerATokens) != 2 {
		t.Fatalf("expected 2 provider-a tokens after deduplicate, got %d", len(providerATokens))
	}
	providerAIDs := make(map[string]struct{}, len(providerATokens))
	for _, token := range providerATokens {
		providerAIDs[token.ID] = struct{}{}
	}
	if _, ok := providerAIDs["tok-new"]; !ok {
		t.Fatalf("expected tok-new to be kept, got ids=%v", providerAIDs)
	}
	if _, ok := providerAIDs["tok-other-email"]; !ok {
		t.Fatalf("expected tok-other-email to remain, got ids=%v", providerAIDs)
	}
	if _, ok := providerAIDs["tok-old"]; ok {
		t.Fatalf("expected tok-old to be removed, got ids=%v", providerAIDs)
	}

	providerBTokens, err := store.LoadTokens("provider-b")
	if err != nil {
		t.Fatalf("LoadTokens(provider-b): %v", err)
	}
	if len(providerBTokens) != 1 || providerBTokens[0].ID != "tok-other-provider" {
		t.Fatalf("expected provider-b token to remain untouched, got %+v", providerBTokens)
	}
}
