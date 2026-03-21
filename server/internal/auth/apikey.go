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

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
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
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	Key         string     `json:"key,omitempty"`
	Scopes      []string   `json:"scopes"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	LastUsed    *time.Time `json:"last_used,omitempty"`
	Revoked     bool       `json:"revoked"`
	RotatedFrom *string    `json:"rotated_from,omitempty"`
	RotatedTo   *string    `json:"rotated_to,omitempty"`
	RotatedAt   *time.Time `json:"rotated_at,omitempty"`
	GracePeriod *time.Time `json:"grace_period,omitempty"`
}

// HasScope checks if the API key has the specified scope
func (k *APIKeyInfo) HasScope(scope string) bool {
	for _, s := range k.Scopes {
		if s == "*" || s == scope {
			return true
		}
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
	ownsDB    bool
}

// APIKeyServiceOption is a functional option for APIKeyService.
type APIKeyServiceOption func(*APIKeyService)

// WithEncryption enables encryption for API key storage.
func WithEncryption(enc *Encryptor) APIKeyServiceOption {
	return func(s *APIKeyService) {
		s.encryptor = enc
	}
}

// NewAPIKeyService creates a new API key service with its own SQLite database.
func NewAPIKeyService(dbPath string, opts ...APIKeyServiceOption) (*APIKeyService, error) {
	svc := &APIKeyService{ownsDB: true}
	for _, opt := range opts {
		opt(svc)
	}

	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)

		openSvc := &APIKeyService{db: db, encryptor: svc.encryptor}
		return openSvc.initDB()
	})
	if err != nil {
		return nil, err
	}

	svc.db = db
	return svc, nil
}

// NewAPIKeyServiceWithDB creates an API key service using an existing shared database connection.
func NewAPIKeyServiceWithDB(db *sql.DB, opts ...APIKeyServiceOption) (*APIKeyService, error) {
	svc := &APIKeyService{db: db, ownsDB: false}
	for _, opt := range opts {
		opt(svc)
	}

	if err := svc.initSchema(); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s *APIKeyService) initDB() error {
	_, err := s.db.Exec(`PRAGMA journal_mode=WAL`)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA synchronous=FULL`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`PRAGMA wal_autocheckpoint=1000`); err != nil {
		return err
	}
	s.db.Exec(`PRAGMA cache_size=-500`)

	if err := s.initSchema(); err != nil {
		return err
	}

	s.db.Exec(`PRAGMA shrink_memory`)
	return nil
}

func (s *APIKeyService) initSchema() error {
	_, err := s.db.Exec(`
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

	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_from TEXT`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_to TEXT`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN rotated_at DATETIME`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN grace_period DATETIME`)
	s.db.Exec(`ALTER TABLE api_keys ADD COLUMN encrypted_key TEXT`)

	return nil
}

// Close closes the database connection if this service owns it.
func (s *APIKeyService) Close() error {
	if s.ownsDB {
		return s.db.Close()
	}
	return nil
}

func (s *APIKeyService) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "api_keys")
}

// apiKeyRow is used for scanning from zorm
type apiKeyRow struct {
	ID           string  `json:"id" zorm:"id"`
	UserID       string  `json:"user_id" zorm:"user_id"`
	Name         string  `json:"name" zorm:"name"`
	Prefix       string  `json:"prefix" zorm:"prefix"`
	Scopes       string  `json:"scopes" zorm:"scopes"`
	CreatedAt    string  `json:"created_at" zorm:"created_at"`
	ExpiresAt    *string `json:"expires_at" zorm:"expires_at"`
	LastUsed     *string `json:"last_used" zorm:"last_used"`
	Revoked      int     `json:"revoked" zorm:"revoked"`
	RotatedFrom  *string `json:"rotated_from" zorm:"rotated_from"`
	RotatedTo    *string `json:"rotated_to" zorm:"rotated_to"`
	RotatedAt    *string `json:"rotated_at" zorm:"rotated_at"`
	GracePeriod  *string `json:"grace_period" zorm:"grace_period"`
	EncryptedKey *string `json:"encrypted_key" zorm:"encrypted_key"`
	KeyHash      *string `json:"key_hash" zorm:"key_hash"`
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, s)
	if t.IsZero() {
		t, _ = time.Parse(time.RFC3339, s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05-07:00", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", s)
	}
	return t
}

func parseTimePtr(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t := parseTime(*s)
	if t.IsZero() {
		return nil
	}
	return &t
}

func rowToAPIKeyInfo(row apiKeyRow) *APIKeyInfo {
	info := &APIKeyInfo{
		ID:          row.ID,
		UserID:      row.UserID,
		Name:        row.Name,
		Prefix:      row.Prefix,
		Scopes:      strings.Split(row.Scopes, ","),
		CreatedAt:   parseTime(row.CreatedAt),
		ExpiresAt:   parseTimePtr(row.ExpiresAt),
		LastUsed:    parseTimePtr(row.LastUsed),
		Revoked:     row.Revoked == 1,
		RotatedFrom: row.RotatedFrom,
		RotatedTo:   row.RotatedTo,
		RotatedAt:   parseTimePtr(row.RotatedAt),
		GracePeriod: parseTimePtr(row.GracePeriod),
	}
	return info
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
	now := timeutil.NowTime()

	var encryptedKey *string
	if s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(key)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key: %w", err)
		}
		encryptedKey = &encrypted
	}

	_, err := s.table(ctx).Insert(map[string]interface{}{
		"id":            id,
		"user_id":       req.UserID,
		"name":          req.Name,
		"prefix":        prefix,
		"key_hash":      keyHash,
		"encrypted_key": encryptedKey,
		"scopes":        scopes,
		"created_at":    now,
		"expires_at":    expiresAt,
	})
	if err != nil {
		return nil, err
	}

	return &APIKeyInfo{
		ID:        id,
		UserID:    req.UserID,
		Name:      req.Name,
		Prefix:    prefix,
		Key:       key,
		Scopes:    req.Scopes,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// ValidateKey validates an API key and returns its information
func (s *APIKeyService) ValidateKey(ctx context.Context, key string) (*APIKeyInfo, error) {
	keyHash := hashKey(key)

	var rows []apiKeyRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "user_id", "name", "prefix", "scopes", "created_at", "expires_at", "last_used", "revoked"),
		z.Where(z.Eq("key_hash", keyHash)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAPIKeyNotFound
	}

	info := rowToAPIKeyInfo(rows[0])

	if info.Revoked {
		return nil, ErrAPIKeyRevoked
	}

	if info.ExpiresAt != nil && info.ExpiresAt.Before(timeutil.NowTime()) {
		return nil, ErrAPIKeyExpired
	}

	// Update last used time asynchronously
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.table(bgCtx).Update(
			z.V{"last_used": timeutil.NowTime()},
			z.Where(z.Eq("id", info.ID)),
		)
	}()

	return info, nil
}

// ListKeys lists all API keys for a user
func (s *APIKeyService) ListKeys(ctx context.Context, userID string) ([]*APIKeyInfo, error) {
	var rows []apiKeyRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "user_id", "name", "prefix", "scopes", "created_at", "expires_at", "last_used", "revoked"),
		z.Where(z.Eq("user_id", userID), z.Eq("revoked", 0)),
		// created_at can collide under coarse clocks; rowid makes ordering deterministic.
		z.OrderBy("created_at DESC, rowid DESC"),
	)
	if err != nil {
		return nil, err
	}

	keys := make([]*APIKeyInfo, len(rows))
	for i, row := range rows {
		keys[i] = rowToAPIKeyInfo(row)
	}
	return keys, nil
}

// RevokeKey revokes an API key
func (s *APIKeyService) RevokeKey(ctx context.Context, id, userID string) error {
	n, err := s.table(ctx).Update(
		z.V{"revoked": 1},
		z.Where(z.Eq("id", id), z.Eq("user_id", userID)),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnauthorized
	}
	return nil
}

// RotateKeyRequest represents a request to rotate an API key
type RotateKeyRequest struct {
	KeyID       string
	UserID      string
	GracePeriod time.Duration
}

// RotateKeyResult contains the result of a key rotation
type RotateKeyResult struct {
	OldKey *APIKeyInfo `json:"old_key"`
	NewKey *APIKeyInfo `json:"new_key"`
}

// RotateKey rotates an API key
func (s *APIKeyService) RotateKey(ctx context.Context, req *RotateKeyRequest) (*RotateKeyResult, error) {
	var rows []apiKeyRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "user_id", "name", "prefix", "scopes", "created_at", "expires_at", "last_used", "revoked", "rotated_to"),
		z.Where(z.Eq("id", req.KeyID), z.Eq("user_id", req.UserID)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrAPIKeyNotFound
	}

	oldRow := rows[0]
	if oldRow.Revoked == 1 {
		return nil, ErrAPIKeyRevoked
	}
	if oldRow.RotatedTo != nil {
		return nil, ErrRotationPending
	}

	oldInfo := rowToAPIKeyInfo(oldRow)

	gracePeriod := req.GracePeriod
	if gracePeriod == 0 {
		gracePeriod = 24 * time.Hour
	}

	newID := uuid.New().String()
	newKey := generateAPIKey()
	newPrefix := newKey[:8]
	newKeyHash := hashKey(newKey)
	now := timeutil.NowTime()
	graceEnd := now.Add(gracePeriod)
	scopesStr := strings.Join(oldInfo.Scopes, ",")

	var encryptedKey *string
	if s.encryptor != nil {
		encrypted, err := s.encryptor.EncryptString(newKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt key: %w", err)
		}
		encryptedKey = &encrypted
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	t := z.TableContext(ctx, tx, "api_keys")

	_, err = t.Insert(map[string]interface{}{
		"id":            newID,
		"user_id":       oldInfo.UserID,
		"name":          oldInfo.Name,
		"prefix":        newPrefix,
		"key_hash":      newKeyHash,
		"encrypted_key": encryptedKey,
		"scopes":        scopesStr,
		"created_at":    now,
		"expires_at":    oldInfo.ExpiresAt,
		"rotated_from":  oldInfo.ID,
	})
	if err != nil {
		return nil, err
	}

	_, err = t.Update(
		z.V{
			"rotated_to":   newID,
			"rotated_at":   now,
			"grace_period": graceEnd,
		},
		z.Where(z.Eq("id", oldInfo.ID)),
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	oldInfo.RotatedTo = &newID
	rotatedAt := now
	oldInfo.RotatedAt = &rotatedAt
	oldInfo.GracePeriod = &graceEnd

	newInfo := &APIKeyInfo{
		ID:          newID,
		UserID:      oldInfo.UserID,
		Name:        oldInfo.Name,
		Prefix:      newPrefix,
		Key:         newKey,
		Scopes:      oldInfo.Scopes,
		CreatedAt:   now,
		ExpiresAt:   oldInfo.ExpiresAt,
		RotatedFrom: &oldInfo.ID,
	}

	return &RotateKeyResult{OldKey: oldInfo, NewKey: newInfo}, nil
}

// CompleteRotation completes a rotation by revoking the old key
func (s *APIKeyService) CompleteRotation(ctx context.Context, oldKeyID, userID string) error {
	n, err := s.table(ctx).Update(
		z.V{"revoked": 1},
		z.Where(
			z.Eq("id", oldKeyID),
			z.Eq("user_id", userID),
			z.IsNotNull("rotated_to"),
		),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUnauthorized
	}
	return nil
}

// CleanupExpiredRotations revokes old keys that have passed their grace period
func (s *APIKeyService) CleanupExpiredRotations(ctx context.Context) (int64, error) {
	now := time.Now()
	n, err := s.table(ctx).Update(
		z.V{"revoked": 1},
		z.Where(
			z.IsNotNull("rotated_to"),
			z.IsNotNull("grace_period"),
			z.Lte("grace_period", now),
			z.Eq("revoked", 0),
		),
	)
	return int64(n), err
}

// GetRotationHistory returns the rotation history for a key
func (s *APIKeyService) GetRotationHistory(ctx context.Context, keyID, userID string) ([]*APIKeyInfo, error) {
	// Find the root key
	rootID := keyID
	for {
		var rows []apiKeyRow
		_, err := s.table(ctx).Select(&rows,
			z.Fields("rotated_from"),
			z.Where(z.Eq("id", rootID), z.Eq("user_id", userID)),
			z.Limit(1),
		)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, ErrAPIKeyNotFound
		}
		if rows[0].RotatedFrom == nil {
			break
		}
		rootID = *rows[0].RotatedFrom
	}

	// Traverse forward
	var history []*APIKeyInfo
	currentID := rootID

	for currentID != "" {
		var rows []apiKeyRow
		_, err := s.table(ctx).Select(&rows,
			z.Where(z.Eq("id", currentID), z.Eq("user_id", userID)),
			z.Limit(1),
		)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}

		info := rowToAPIKeyInfo(rows[0])
		history = append(history, info)

		if info.RotatedTo != nil {
			currentID = *info.RotatedTo
		} else {
			break
		}
	}

	return history, nil
}

// GetDecryptedKey retrieves and decrypts an API key by its ID.
func (s *APIKeyService) GetDecryptedKey(ctx context.Context, keyID, userID string) (string, error) {
	if s.encryptor == nil {
		return "", ErrKeyNotConfigured
	}

	var rows []apiKeyRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("encrypted_key", "revoked"),
		z.Where(z.Eq("id", keyID), z.Eq("user_id", userID)),
		z.Limit(1),
	)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", ErrAPIKeyNotFound
	}

	if rows[0].Revoked == 1 {
		return "", ErrAPIKeyRevoked
	}

	if rows[0].EncryptedKey == nil || *rows[0].EncryptedKey == "" {
		return "", errors.New("key was not stored encrypted")
	}

	decrypted, err := s.encryptor.DecryptString(*rows[0].EncryptedKey)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt key: %w", err)
	}

	return decrypted, nil
}

// ReEncryptAllKeys re-encrypts all API keys with a new encryptor.
func (s *APIKeyService) ReEncryptAllKeys(ctx context.Context, oldEnc, newEnc *Encryptor) (int64, error) {
	if oldEnc == nil || newEnc == nil {
		return 0, ErrKeyNotConfigured
	}

	var rows []apiKeyRow
	_, err := s.table(ctx).Select(&rows,
		z.Fields("id", "encrypted_key"),
		z.Where(z.IsNotNull("encrypted_key"), z.Eq("revoked", 0)),
	)
	if err != nil {
		return 0, err
	}

	var count int64
	for _, row := range rows {
		if row.EncryptedKey == nil {
			continue
		}

		decrypted, err := oldEnc.DecryptString(*row.EncryptedKey)
		if err != nil {
			return count, fmt.Errorf("failed to decrypt key %s: %w", row.ID, err)
		}

		newEncrypted, err := newEnc.EncryptString(decrypted)
		if err != nil {
			return count, fmt.Errorf("failed to re-encrypt key %s: %w", row.ID, err)
		}

		_, err = s.table(ctx).Update(
			z.V{"encrypted_key": newEncrypted},
			z.Where(z.Eq("id", row.ID)),
		)
		if err != nil {
			return count, fmt.Errorf("failed to update key %s: %w", row.ID, err)
		}

		count++
	}

	return count, nil
}

func generateAPIKey() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	return "ek_" + hex.EncodeToString(bytes)
}

func hashKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
