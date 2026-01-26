package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var (
	ErrAPIKeyNotFound  = errors.New("api key not found")
	ErrAPIKeyExpired   = errors.New("api key has expired")
	ErrAPIKeyRevoked   = errors.New("api key has been revoked")
	ErrUnauthorized    = errors.New("unauthorized")
)

// APIKeyInfo represents the information about an API key
type APIKeyInfo struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	Prefix    string     `json:"prefix"`
	Key       string     `json:"key,omitempty"` // Only populated on creation
	Scopes    []string   `json:"scopes"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	Revoked   bool       `json:"revoked"`
}

// HasScope checks if the API key has the specified scope
func (k *APIKeyInfo) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == "*" || s == scope {
			return true
		}
		// Check prefix match (e.g., "read:*" matches "read:users")
		if strings.HasSuffix(s, ":*") {
			prefix := strings.TrimSuffix(s, "*")
			if strings.HasPrefix(scope, prefix) {
				return true
			}
		}
	}
	return false
}

// CreateKeyRequest represents a request to create an API key
type CreateKeyRequest struct {
	UserID    string
	Name      string
	Scopes    []string
	ExpiresAt time.Time
}

// APIKeyService handles API key operations
type APIKeyService struct {
	db *sql.DB
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(dbPath string) (*APIKeyService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	svc := &APIKeyService{db: db}
	if err := svc.initDB(); err != nil {
		db.Close()
		return nil, err
	}

	return svc, nil
}

// initDB initializes the database schema
func (s *APIKeyService) initDB() error {
	// Enable WAL mode for better concurrency
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			prefix TEXT NOT NULL,
			key_hash TEXT NOT NULL UNIQUE,
			scopes TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME,
			last_used DATETIME,
			revoked INTEGER NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
		CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
	`)
	return err
}

// Close closes the database connection
func (s *APIKeyService) Close() error {
	return s.db.Close()
}

// CreateKey creates a new API key
func (s *APIKeyService) CreateKey(ctx context.Context, req *CreateKeyRequest) (*APIKeyInfo, error) {
	id := uuid.New().String()
	key := generateAPIKey()
	prefix := key[:8]
	keyHash := hashKey(key)

	var expiresAt *time.Time
	if !req.ExpiresAt.IsZero() {
		expiresAt = &req.ExpiresAt
	}

	scopes := strings.Join(req.Scopes, ",")
	now := time.Now()

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, user_id, name, prefix, key_hash, scopes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.UserID, req.Name, prefix, keyHash, scopes, now, expiresAt)
	if err != nil {
		return nil, err
	}

	return &APIKeyInfo{
		ID:        id,
		UserID:    req.UserID,
		Name:      req.Name,
		Prefix:    prefix,
		Key:       key, // Only returned on creation
		Scopes:    req.Scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateKey validates an API key and returns its information
func (s *APIKeyService) ValidateKey(ctx context.Context, key string) (*APIKeyInfo, error) {
	keyHash := hashKey(key)

	var info APIKeyInfo
	var scopesStr string
	var expiresAt, lastUsed sql.NullTime
	var revoked int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, prefix, scopes, created_at, expires_at, last_used, revoked
		FROM api_keys WHERE key_hash = ?
	`, keyHash).Scan(
		&info.ID, &info.UserID, &info.Name, &info.Prefix,
		&scopesStr, &info.CreatedAt, &expiresAt, &lastUsed, &revoked,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAPIKeyNotFound
		}
		return nil, err
	}

	if revoked == 1 {
		return nil, ErrAPIKeyRevoked
	}

	if expiresAt.Valid {
		info.ExpiresAt = &expiresAt.Time
		if expiresAt.Time.Before(time.Now()) {
			return nil, ErrAPIKeyExpired
		}
	}

	if lastUsed.Valid {
		info.LastUsed = &lastUsed.Time
	}

	info.Scopes = strings.Split(scopesStr, ",")
	info.Revoked = revoked == 1

	// Update last used time asynchronously
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(ctx, `UPDATE api_keys SET last_used = ? WHERE id = ?`, time.Now(), info.ID)
	}()

	return &info, nil
}

// ListKeys lists all API keys for a user
func (s *APIKeyService) ListKeys(ctx context.Context, userID string) ([]*APIKeyInfo, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, name, prefix, scopes, created_at, expires_at, last_used, revoked
		FROM api_keys WHERE user_id = ? AND revoked = 0
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*APIKeyInfo
	for rows.Next() {
		var info APIKeyInfo
		var scopesStr string
		var expiresAt, lastUsed sql.NullTime
		var revoked int

		err := rows.Scan(
			&info.ID, &info.UserID, &info.Name, &info.Prefix,
			&scopesStr, &info.CreatedAt, &expiresAt, &lastUsed, &revoked,
		)
		if err != nil {
			return nil, err
		}

		if expiresAt.Valid {
			info.ExpiresAt = &expiresAt.Time
		}
		if lastUsed.Valid {
			info.LastUsed = &lastUsed.Time
		}

		info.Scopes = strings.Split(scopesStr, ",")
		info.Revoked = revoked == 1
		keys = append(keys, &info)
	}

	return keys, rows.Err()
}

// RevokeKey revokes an API key
func (s *APIKeyService) RevokeKey(ctx context.Context, id, userID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked = 1 WHERE id = ? AND user_id = ?
	`, id, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrUnauthorized
	}

	return nil
}

// generateAPIKey generates a random API key
func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return "ek_" + hex.EncodeToString(bytes)
}

// hashKey hashes an API key using SHA-256
func hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
