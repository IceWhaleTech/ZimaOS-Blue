package oauth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SQLiteTokenStore persists OAuth tokens in a SQLite database.
type SQLiteTokenStore struct {
	db *sql.DB
}

type oauthTokenRow struct {
	ID           string  `json:"id" zorm:"id"`
	ProviderID   string  `json:"provider_id" zorm:"provider_id"`
	ProviderType string  `json:"provider_type" zorm:"provider_type"`
	AccessToken  string  `json:"access_token" zorm:"access_token"`
	RefreshToken string  `json:"refresh_token" zorm:"refresh_token"`
	TokenExpiry  string  `json:"token_expiry" zorm:"token_expiry"`
	Scopes       *string `json:"scopes" zorm:"scopes"`
	Email        *string `json:"email" zorm:"email"`
	ProjectID    *string `json:"project_id" zorm:"project_id"`
	Endpoint     *string `json:"endpoint" zorm:"endpoint"`
	CreatedAt    string  `json:"created_at" zorm:"created_at"`
	UpdatedAt    string  `json:"updated_at" zorm:"updated_at"`
}

func (s *SQLiteTokenStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "oauth_tokens")
}

// NewSQLiteTokenStore creates a new SQLite-backed OAuth token store.
func NewSQLiteTokenStore(db *sql.DB) (*SQLiteTokenStore, error) {
	// Create table with composite primary key (provider_id, id)
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS oauth_tokens (
		id            TEXT NOT NULL DEFAULT '',
		provider_id   TEXT NOT NULL DEFAULT '',
		provider_type TEXT NOT NULL DEFAULT '',
		access_token  TEXT NOT NULL DEFAULT '',
		refresh_token TEXT NOT NULL DEFAULT '',
		token_expiry  TEXT NOT NULL DEFAULT '',
		scopes        TEXT,
		email         TEXT,
		project_id    TEXT,
		endpoint      TEXT,
		created_at    TEXT NOT NULL DEFAULT '',
		updated_at    TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (provider_id, id)
	)`)
	if err != nil {
		return nil, fmt.Errorf("create oauth_tokens table: %w", err)
	}

	store := &SQLiteTokenStore{db: db}
	store.migrateSchema()
	return store, nil
}

// migrateSchema handles schema migration from old single-PK to new composite-PK.
func (s *SQLiteTokenStore) migrateSchema() {
	// Check if 'id' column exists
	var hasID bool
	rows, err := s.db.Query("PRAGMA table_info(oauth_tokens)")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == "id" {
			hasID = true
		}
	}

	if hasID {
		return // Already migrated
	}

	// Old schema: provider_id is sole PK, no id column.
	// Recreate table with new schema and migrate data.
	tx, err := s.db.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()

	tx.Exec("ALTER TABLE oauth_tokens RENAME TO oauth_tokens_old")
	tx.Exec(`CREATE TABLE oauth_tokens (
		id            TEXT NOT NULL DEFAULT '',
		provider_id   TEXT NOT NULL DEFAULT '',
		provider_type TEXT NOT NULL DEFAULT '',
		access_token  TEXT NOT NULL DEFAULT '',
		refresh_token TEXT NOT NULL DEFAULT '',
		token_expiry  TEXT NOT NULL DEFAULT '',
		scopes        TEXT,
		email         TEXT,
		project_id    TEXT,
		endpoint      TEXT,
		created_at    TEXT NOT NULL DEFAULT '',
		updated_at    TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (provider_id, id)
	)`)
	// Migrate existing rows — assign generated IDs
	oldRows, err := tx.Query("SELECT provider_id, provider_type, access_token, refresh_token, token_expiry, scopes, email, project_id, endpoint, created_at, updated_at FROM oauth_tokens_old")
	if err == nil {
		defer oldRows.Close()
		for oldRows.Next() {
			var pid, ptype, at, rt, te, ca, ua string
			var scopes, email, projectID, endpoint *string
			if err := oldRows.Scan(&pid, &ptype, &at, &rt, &te, &scopes, &email, &projectID, &endpoint, &ca, &ua); err != nil {
				continue
			}
			id := generateTokenID()
			tx.Exec(`INSERT INTO oauth_tokens (id, provider_id, provider_type, access_token, refresh_token, token_expiry, scopes, email, project_id, endpoint, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				id, pid, ptype, at, rt, te, scopes, email, projectID, endpoint, ca, ua)
		}
	}
	tx.Exec("DROP TABLE IF EXISTS oauth_tokens_old")
	tx.Commit()
}

func (s *SQLiteTokenStore) SaveToken(providerID string, token *Token) error {
	token.ProviderID = providerID
	token.UpdatedAt = timeutil.NowTime()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = timeutil.NowTime()
	}

	// If no ID provided, check if a token with the same email already exists for this provider
	// If so, update that token instead of creating a duplicate
	if token.ID == "" && token.Email != "" {
		existingToken, err := s.LoadTokenByEmail(providerID, token.Email)
		if err == nil && existingToken != nil {
			token.ID = existingToken.ID
			token.CreatedAt = existingToken.CreatedAt
		} else {
			token.ID = generateTokenID()
		}
	} else if token.ID == "" {
		token.ID = generateTokenID()
	}

	var scopesVal interface{}
	if len(token.Scopes) > 0 {
		scopesVal = strings.Join(token.Scopes, ",")
	}

	ctx := context.Background()
	_, err := s.table(ctx).Insert(map[string]interface{}{
		"id":            token.ID,
		"provider_id":   providerID,
		"provider_type": token.ProviderType,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"token_expiry":  token.TokenExpiry.Format(time.RFC3339),
		"scopes":        scopesVal,
		"email":         nilIfEmpty(token.Email),
		"project_id":    nilIfEmpty(token.ProjectID),
		"endpoint":      nilIfEmpty(token.Endpoint),
		"created_at":   token.CreatedAt.Format(time.RFC3339),
		"updated_at":   token.UpdatedAt.Format(time.RFC3339),
	}, z.OnConflictDoUpdateSet(
		[]string{"provider_id", "id"},
		[]string{"provider_type", "access_token", "refresh_token", "token_expiry", "scopes", "email", "project_id", "endpoint", "updated_at"},
	))
	return err
}

func (s *SQLiteTokenStore) LoadToken(providerID, tokenID string) (*Token, error) {
	ctx := context.Background()
	var rows []oauthTokenRow
	_, err := s.table(ctx).Select(&rows,
		z.Where(z.Eq("provider_id", providerID), z.Eq("id", tokenID)),
		z.Limit(1),
	)
	if err != nil || len(rows) == 0 {
		return nil, fmt.Errorf("no oauth token for provider %s id %s", providerID, tokenID)
	}
	return rowToToken(&rows[0]), nil
}

// LoadTokenByEmail loads an OAuth token by email address for a given provider.
// This is used to find existing tokens when re-authenticating to avoid duplicates.
func (s *SQLiteTokenStore) LoadTokenByEmail(providerID, email string) (*Token, error) {
	ctx := context.Background()
	var rows []oauthTokenRow
	_, err := s.table(ctx).Select(&rows,
		z.Where(z.Eq("provider_id", providerID), z.Eq("email", email)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rowToToken(&rows[0]), nil
}

func (s *SQLiteTokenStore) LoadTokens(providerID string) ([]*Token, error) {
	ctx := context.Background()
	var rows []oauthTokenRow

	// Also load tokens with empty provider_id that might need migration
	// This handles tokens imported before the provider_id fix
	_, err := s.table(ctx).Select(&rows,
		z.Where(z.Or(z.Eq("provider_id", providerID), z.Eq("provider_id", ""))),
	)
	if err != nil {
		return nil, err
	}

	// Migrate tokens with empty provider_id to correct provider_id
	for i := range rows {
		if rows[i].ProviderID == "" {
			// Update the provider_id
			_, err := s.db.Exec("UPDATE oauth_tokens SET provider_id = ? WHERE id = ? AND provider_id = ''",
				providerID, rows[i].ID)
			if err != nil {
				slog.Warn("[oauth] failed to migrate token provider_id", "token_id", rows[i].ID, "error", err)
			}
			rows[i].ProviderID = providerID
		}
	}

	tokens := make([]*Token, 0, len(rows))
	for i := range rows {
		tokens = append(tokens, rowToToken(&rows[i]))
	}
	return tokens, nil
}

func (s *SQLiteTokenStore) DeleteToken(providerID, tokenID string) error {
	ctx := context.Background()
	_, err := s.table(ctx).Delete(z.Where(z.Eq("provider_id", providerID), z.Eq("id", tokenID)))
	return err
}

func (s *SQLiteTokenStore) ListTokens() (map[string]*Token, error) {
	ctx := context.Background()
	var rows []oauthTokenRow
	_, err := s.table(ctx).Select(&rows)
	if err != nil {
		return nil, err
	}
	result := make(map[string]*Token, len(rows))
	for i := range rows {
		t := rowToToken(&rows[i])
		// Key by provider_id:id for uniqueness
		result[t.ProviderID+":"+t.ID] = t
	}
	return result, nil
}

// Deduplicate removes duplicate tokens for each provider, keeping only the most recent one per email.
// This can be called to clean up any duplicates that may have been created due to bugs.
func (s *SQLiteTokenStore) Deduplicate() error {
	ctx := context.Background()

	// Find duplicates: same provider_id and email (where email is not empty)
	// Keep the one with the latest token_expiry
	_, err := s.table(ctx).Exec(`
		DELETE FROM oauth_tokens WHERE id NOT IN (
			SELECT ot1.id FROM oauth_tokens ot1
			WHERE ot1.email IS NOT NULL AND ot1.email != ''
			AND ot1.token_expiry = (
				SELECT MAX(ot2.token_expiry)
				FROM oauth_tokens ot2
				WHERE ot2.provider_id = ot1.provider_id
				AND (ot2.email = ot1.email OR (ot2.email IS NULL AND ot1.email IS NULL))
			)
			AND EXISTS (
				SELECT 1 FROM oauth_tokens ot3
				WHERE ot3.provider_id = ot1.provider_id
				AND (ot3.email = ot1.email OR (ot3.email IS NULL AND ot1.email IS NULL))
				GROUP BY ot3.provider_id, ot3.email
				HAVING COUNT(*) > 1
			)
		)
		AND email IS NOT NULL AND email != ''
	`)
	return err
}

// MigrateFromJSON imports tokens from a legacy JSON file into this store.
func (s *SQLiteTokenStore) MigrateFromJSON(jsonPath string) error {
	data, err := readFileIfExists(jsonPath)
	if err != nil || data == nil {
		return err
	}
	var sd struct {
		Tokens map[string]*Token `json:"tokens"`
	}
	if err := json.Unmarshal(data, &sd); err != nil {
		return fmt.Errorf("unmarshal oauth_tokens.json: %w", err)
	}
	// Deduplicate by email during migration
	seen := make(map[string]*Token) // email -> token
	for key, token := range sd.Tokens {
		// key is in format "providerID:tokenID", extract providerID
		providerID := key
		if idx := strings.Index(key, ":"); idx != -1 {
			providerID = key[:idx]
		}

		// Use token's ProviderID if available, otherwise use extracted providerID
		if token.ProviderID == "" {
			token.ProviderID = providerID
		}

		if token.ID == "" {
			token.ID = generateTokenID()
		}

		// Deduplicate by email: keep only the first token per email per provider
		if token.Email != "" {
			dedupKey := providerID + ":" + token.Email
			if existing, ok := seen[dedupKey]; ok {
				// Skip duplicate, but update if this one has more recent expiry
				if token.TokenExpiry.After(existing.TokenExpiry) {
					seen[dedupKey] = token
				}
				continue
			}
			seen[dedupKey] = token
		}
	}

	// Save deduplicated tokens
	for _, token := range seen {
		if err := s.SaveToken(token.ProviderID, token); err != nil {
			return fmt.Errorf("migrate token %s: %w", token.ProviderID+":"+token.ID, err)
		}
	}

	return nil
}

func rowToToken(r *oauthTokenRow) *Token {
	t := &Token{
		ID:           r.ID,
		ProviderID:   r.ProviderID,
		ProviderType: r.ProviderType,
		AccessToken:  r.AccessToken,
		RefreshToken: r.RefreshToken,
	}
	t.TokenExpiry, _ = time.Parse(time.RFC3339, r.TokenExpiry)
	t.CreatedAt, _ = time.Parse(time.RFC3339, r.CreatedAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339, r.UpdatedAt)
	if r.Scopes != nil && *r.Scopes != "" {
		t.Scopes = strings.Split(*r.Scopes, ",")
	}
	if r.Email != nil {
		t.Email = *r.Email
	}
	if r.ProjectID != nil {
		t.ProjectID = *r.ProjectID
	}
	if r.Endpoint != nil {
		t.Endpoint = *r.Endpoint
	}
	return t
}

func generateTokenID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "oat_" + hex.EncodeToString(b)
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func readFileIfExists(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}
