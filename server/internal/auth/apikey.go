package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
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
	ErrRotationPending = errors.New("rotation already pending")
)

// APIKeyInfo represents the information about an API key
type APIKeyInfo struct {
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	Name         string     `json:"name"`
	Prefix       string     `json:"prefix"`
	Key          string     `json:"key,omitempty"` // Only populated on creation
	Scopes       []string   `json:"scopes"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	LastUsed     *time.Time `json:"last_used,omitempty"`
	Revoked      bool       `json:"revoked"`
	RotatedFrom  *string    `json:"rotated_from,omitempty"`  // ID of the key this was rotated from
	RotatedTo    *string    `json:"rotated_to,omitempty"`    // ID of the key this was rotated to
	RotatedAt    *time.Time `json:"rotated_at,omitempty"`    // When the rotation occurred
	GracePeriod  *time.Time `json:"grace_period,omitempty"`  // When the old key will be invalidated
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
	db        *sql.DB
	encryptor *Encryptor
}

// APIKeyServiceOption is a functional option for APIKeyService.
type APIKeyServiceOption func(*APIKeyService)

// WithEncryption enables encryption for API key storage.
func WithEncryption(enc *Encryptor) APIKeyServiceOption {
	return func(s *APIKeyService) {
		s.encryptor = enc
	}
}

// NewAPIKeyService creates a new API key service
func NewAPIKeyService(dbPath string, opts ...APIKeyServiceOption) (*APIKeyService, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	svc := &APIKeyService{db: db}

	// Apply options
	for _, opt := range opts {
		opt(svc)
	}

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
			encrypted_key TEXT,
			scopes TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			expires_at DATETIME,
			last_used DATETIME,
			revoked INTEGER NOT NULL DEFAULT 0,
			rotated_from TEXT,
			rotated_to TEXT,
			rotated_at DATETIME,
			grace_period DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id);
		CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
	`)
	if err != nil {
		return err
	}

	// Add new columns if they don't exist (for migration)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_from TEXT`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_to TEXT`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_at DATETIME`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN grace_period DATETIME`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN encrypted_key TEXT`)

	return nil
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

	// Encrypt the key if encryptor is configured
	var encryptedKey *string
	if s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key: %w", err)
		}
		encryptedKey = &encrypted
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, user_id, name, prefix, key_hash, encrypted_key, scopes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, req.UserID, req.Name, prefix, keyHash, encryptedKey, scopes, now, expiresAt)
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

// RotateKeyRequest represents a request to rotate an API key
type RotateKeyRequest struct {
	KeyID       string        // ID of the key to rotate
	UserID      string        // User ID for authorization
	GracePeriod time.Duration // How long the old key remains valid (default: 24h)
}

// RotateKeyResult contains the result of a key rotation
type RotateKeyResult struct {
	OldKey *APIKeyInfo `json:"old_key"`
	NewKey *APIKeyInfo `json:"new_key"`
}

// RotateKey rotates an API key, creating a new key and optionally keeping the old one valid for a grace period
func (s *APIKeyService) RotateKey(ctx context.Context, req *RotateKeyRequest) (*RotateKeyResult, error) {
	// Get the existing key
	var oldInfo APIKeyInfo
	var scopesStr string
	var expiresAt, lastUsed sql.NullTime
	var rotatedToID sql.NullString
	var revoked int

	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, prefix, scopes, created_at, expires_at, last_used, revoked, rotated_to
		FROM api_keys WHERE id = ? AND user_id = ?
	`, req.KeyID, req.UserID).Scan(
		&oldInfo.ID, &oldInfo.UserID, &oldInfo.Name, &oldInfo.Prefix,
		&scopesStr, &oldInfo.CreatedAt, &expiresAt, &lastUsed, &revoked, &rotatedToID,
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

	// Check if already rotated
	if rotatedToID.Valid {
		return nil, ErrRotationPending
	}

	oldInfo.Scopes = strings.Split(scopesStr, ",")
	if expiresAt.Valid {
		oldInfo.ExpiresAt = &expiresAt.Time
	}
	if lastUsed.Valid {
		oldInfo.LastUsed = &lastUsed.Time
	}

	// Set default grace period
	gracePeriod := req.GracePeriod
	if gracePeriod == 0 {
		gracePeriod = 24 * time.Hour
	}

	// Create new key
	newID := uuid.New().String()
	newKey := generateAPIKey()
	newPrefix := newKey[:8]
	newKeyHash := hashKey(newKey)
	now := time.Now()
	graceEnd := now.Add(gracePeriod)

	// Encrypt the new key if encryptor is configured
	var encryptedKey *string
	if s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(newKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key: %w", err)
		}
		encryptedKey = &encrypted
	}

	// Start transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Insert new key
	_, err = tx.ExecContext(ctx, `
		INSERT INTO api_keys (id, user_id, name, prefix, key_hash, encrypted_key, scopes, created_at, expires_at, rotated_from)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, newID, oldInfo.UserID, oldInfo.Name, newPrefix, newKeyHash, encryptedKey, scopesStr, now, oldInfo.ExpiresAt, oldInfo.ID)
	if err != nil {
		return nil, err
	}

	// Update old key with rotation info
	_, err = tx.ExecContext(ctx, `
		UPDATE api_keys SET rotated_to = ?, rotated_at = ?, grace_period = ? WHERE id = ?
	`, newID, now, graceEnd, oldInfo.ID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Update old key info
	oldInfo.RotatedTo = &newID
	rotatedAt := now
	oldInfo.RotatedAt = &rotatedAt
	oldInfo.GracePeriod = &graceEnd

	// Create new key info
	newInfo := &APIKeyInfo{
		ID:          newID,
		UserID:      oldInfo.UserID,
		Name:        oldInfo.Name,
		Prefix:      newPrefix,
		Key:         newKey, // Only returned on creation
		Scopes:      oldInfo.Scopes,
		CreatedAt:   now,
		ExpiresAt:   oldInfo.ExpiresAt,
		RotatedFrom: &oldInfo.ID,
	}

	return &RotateKeyResult{
		OldKey: &oldInfo,
		NewKey: newInfo,
	}, nil
}

// CompleteRotation completes a rotation by revoking the old key
func (s *APIKeyService) CompleteRotation(ctx context.Context, oldKeyID, userID string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked = 1
		WHERE id = ? AND user_id = ? AND rotated_to IS NOT NULL
	`, oldKeyID, userID)
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

// CleanupExpiredRotations revokes old keys that have passed their grace period
func (s *APIKeyService) CleanupExpiredRotations(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		UPDATE api_keys SET revoked = 1
		WHERE rotated_to IS NOT NULL
		AND grace_period IS NOT NULL
		AND grace_period < ?
		AND revoked = 0
	`, time.Now())
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetRotationHistory returns the rotation history for a key
func (s *APIKeyService) GetRotationHistory(ctx context.Context, keyID, userID string) ([]*APIKeyInfo, error) {
	// Find the root key (the original key in the rotation chain)
	rootID := keyID
	for {
		var rotatedFrom sql.NullString
		err := s.db.QueryRowContext(ctx, `
			SELECT rotated_from FROM api_keys WHERE id = ? AND user_id = ?
		`, rootID, userID).Scan(&rotatedFrom)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, ErrAPIKeyNotFound
			}
			return nil, err
		}
		if !rotatedFrom.Valid {
			break
		}
		rootID = rotatedFrom.String
	}

	// Now traverse forward to get all keys in the chain
	var history []*APIKeyInfo
	currentID := rootID

	for currentID != "" {
		var info APIKeyInfo
		var scopesStr string
		var expiresAt, lastUsed, rotatedAt, gracePeriod sql.NullTime
		var rotatedFrom, rotatedTo sql.NullString
		var revoked int

		err := s.db.QueryRowContext(ctx, `
			SELECT id, user_id, name, prefix, scopes, created_at, expires_at, last_used, revoked,
			       rotated_from, rotated_to, rotated_at, grace_period
			FROM api_keys WHERE id = ? AND user_id = ?
		`, currentID, userID).Scan(
			&info.ID, &info.UserID, &info.Name, &info.Prefix,
			&scopesStr, &info.CreatedAt, &expiresAt, &lastUsed, &revoked,
			&rotatedFrom, &rotatedTo, &rotatedAt, &gracePeriod,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return nil, err
		}

		info.Scopes = strings.Split(scopesStr, ",")
		info.Revoked = revoked == 1

		if expiresAt.Valid {
			info.ExpiresAt = &expiresAt.Time
		}
		if lastUsed.Valid {
			info.LastUsed = &lastUsed.Time
		}
		if rotatedFrom.Valid {
			info.RotatedFrom = &rotatedFrom.String
		}
		if rotatedTo.Valid {
			info.RotatedTo = &rotatedTo.String
		}
		if rotatedAt.Valid {
			info.RotatedAt = &rotatedAt.Time
		}
		if gracePeriod.Valid {
			info.GracePeriod = &gracePeriod.Time
		}

		history = append(history, &info)

		if rotatedTo.Valid {
			currentID = rotatedTo.String
		} else {
			break
		}
	}

	return history, nil
}

// GetDecryptedKey retrieves and decrypts an API key by its ID.
// This is useful for scenarios where the original key needs to be recovered.
// Returns an error if encryption is not configured or the key is not found.
func (s *APIKeyService) GetDecryptedKey(ctx context.Context, keyID, userID string) (string, error) {
	if s.encryptor == nil {
		return "", ErrKeyNotConfigured
	}

	var encryptedKey sql.NullString
	var revoked int

	err := s.db.QueryRowContext(ctx, `
		SELECT encrypted_key, revoked FROM api_keys WHERE id = ? AND user_id = ?
	`, keyID, userID).Scan(&encryptedKey, &revoked)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrAPIKeyNotFound
		}
		return "", err
	}

	if revoked == 1 {
		return "", ErrAPIKeyRevoked
	}

	if !encryptedKey.Valid || encryptedKey.String == "" {
		return "", errors.New("key was not stored encrypted")
	}

	decrypted, err := s.encryptor.DecryptString(encryptedKey.String)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt key: %w", err)
	}

	return decrypted, nil
}

// ReEncryptAllKeys re-encrypts all API keys with a new encryptor.
// This is useful when rotating the master encryption key.
func (s *APIKeyService) ReEncryptAllKeys(ctx context.Context, oldEnc, newEnc *Encryptor) (int64, error) {
	if oldEnc == nil || newEnc == nil {
		return 0, ErrKeyNotConfigured
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, encrypted_key FROM api_keys WHERE encrypted_key IS NOT NULL AND revoked = 0
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var count int64
	for rows.Next() {
		var id, encryptedKey string
		if err := rows.Scan(&id, &encryptedKey); err != nil {
			return count, err
		}

		// Decrypt with old key
		decrypted, err := oldEnc.DecryptString(encryptedKey)
		if err != nil {
			return count, fmt.Errorf("failed to decrypt key %s: %w", id, err)
		}

		// Re-encrypt with new key
		newEncrypted, err := newEnc.EncryptString(decrypted)
		if err != nil {
			return count, fmt.Errorf("failed to re-encrypt key %s: %w", id, err)
		}

		// Update in database
		_, err = s.db.ExecContext(ctx, `
			UPDATE api_keys SET encrypted_key = ? WHERE id = ?
		`, newEncrypted, id)
		if err != nil {
			return count, fmt.Errorf("failed to update key %s: %w", id, err)
		}

		count++
	}

	return count, rows.Err()
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
