package oauth

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	z "github.com/IceWhaleTech/zorm"
)

// SQLiteTokenStore persists OAuth tokens in a SQLite database.
type SQLiteTokenStore struct {
	db *sql.DB
}

type oauthTokenRow struct {
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
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS oauth_tokens (
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
	)`)
	if err != nil {
		return nil, fmt.Errorf("create oauth_tokens table: %w", err)
	}
	return &SQLiteTokenStore{db: db}, nil
}

func (s *SQLiteTokenStore) SaveToken(providerID string, token *Token) error {
	token.ProviderID = providerID
	token.UpdatedAt = time.Now()
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now()
	}

	var scopesVal interface{}
	if len(token.Scopes) > 0 {
		scopesVal = strings.Join(token.Scopes, ",")
	}

	ctx := context.Background()
	_, err := s.table(ctx).Insert(map[string]interface{}{
		"provider_id":   providerID,
		"provider_type": token.ProviderType,
		"access_token":  token.AccessToken,
		"refresh_token": token.RefreshToken,
		"token_expiry":  token.TokenExpiry.Format(time.RFC3339),
		"scopes":        scopesVal,
		"email":         nilIfEmpty(token.Email),
		"project_id":    nilIfEmpty(token.ProjectID),
		"endpoint":      nilIfEmpty(token.Endpoint),
		"created_at":    token.CreatedAt.Format(time.RFC3339),
		"updated_at":    token.UpdatedAt.Format(time.RFC3339),
	}, z.OnConflictDoUpdateSet(
		[]string{"provider_id"},
		[]string{"provider_type", "access_token", "refresh_token", "token_expiry", "scopes", "email", "project_id", "endpoint", "updated_at"},
	))
	return err
}

func (s *SQLiteTokenStore) LoadToken(providerID string) (*Token, error) {
	ctx := context.Background()
	var rows []oauthTokenRow
	_, err := s.table(ctx).Select(&rows,
		z.Where(z.Eq("provider_id", providerID)),
		z.Limit(1),
	)
	if err != nil {
		return nil, fmt.Errorf("no oauth token for provider %s", providerID)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no oauth token for provider %s", providerID)
	}
	return rowToToken(&rows[0]), nil
}

func (s *SQLiteTokenStore) DeleteToken(providerID string) error {
	ctx := context.Background()
	_, err := s.table(ctx).Delete(z.Where(z.Eq("provider_id", providerID)))
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
		result[t.ProviderID] = t
	}
	return result, nil
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
	for pid, token := range sd.Tokens {
		if err := s.SaveToken(pid, token); err != nil {
			return fmt.Errorf("migrate token %s: %w", pid, err)
		}
	}
	return nil
}

func rowToToken(r *oauthTokenRow) *Token {
	t := &Token{
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
