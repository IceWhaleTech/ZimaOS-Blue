// Package kvstore provides key-value storage with TTL support.
package kvstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// Common errors
var (
	ErrKeyNotFound = errors.New("key not found")
)

// Store is the interface for key-value storage.
type Store interface {
	// Get retrieves a value by key.
	Get(ctx context.Context, key string) (interface{}, error)

	// Set stores a value with optional TTL (0 = no expiration).
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// Delete removes a key.
	Delete(ctx context.Context, key string) error

	// Exists checks if a key exists.
	Exists(ctx context.Context, key string) (bool, error)

	// Keys returns keys matching a pattern.
	Keys(ctx context.Context, pattern string) ([]string, error)

	// Clear removes all keys.
	Clear(ctx context.Context) error

	// SetJSON stores a JSON-serializable value.
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error

	// GetJSON retrieves and unmarshals a JSON value.
	GetJSON(ctx context.Context, key string, dest interface{}) error
}

// entry represents a stored value with expiration.
type entry struct {
	value     interface{}
	expiresAt time.Time
}

// isExpired checks if the entry has expired.
func (e *entry) isExpired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	// Treat exact boundary as expired to avoid 100ms cached-clock edge cases.
	return !timeutil.NowTime().Before(e.expiresAt)
}

// MemoryStore is an in-memory key-value store.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]*entry
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]*entry),
	}
}

// Get retrieves a value by key.
func (s *MemoryStore) Get(ctx context.Context, key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	if !ok || e.isExpired() {
		return nil, ErrKeyNotFound
	}

	return e.value, nil
}

// Set stores a value with optional TTL.
func (s *MemoryStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e := &entry{value: value}
	if ttl > 0 {
		e.expiresAt = timeutil.NowTime().Add(ttl)
	}

	s.data[key] = e
	return nil
}

// Delete removes a key.
func (s *MemoryStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.data, key)
	return nil
}

// Exists checks if a key exists.
func (s *MemoryStore) Exists(ctx context.Context, key string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	e, ok := s.data[key]
	if !ok || e.isExpired() {
		return false, nil
	}

	return true, nil
}

// Keys returns keys matching a pattern (glob-style: * matches any).
func (s *MemoryStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var keys []string
	for key, e := range s.data {
		if e.isExpired() {
			continue
		}
		if matchGlob(pattern, key) {
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Clear removes all keys.
func (s *MemoryStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]*entry)
	return nil
}

// SetJSON stores a JSON-serializable value.
func (s *MemoryStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return s.Set(ctx, key, string(data), ttl)
}

// GetJSON retrieves and unmarshals a JSON value.
func (s *MemoryStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	value, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}

	return json.Unmarshal([]byte(str), dest)
}

// matchGlob performs simple glob matching (* matches any characters).
func matchGlob(pattern, str string) bool {
	if pattern == "*" {
		return true
	}

	// Simple implementation: convert * to regex-like matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == str
	}

	// Check prefix
	if parts[0] != "" && !strings.HasPrefix(str, parts[0]) {
		return false
	}

	// Check suffix
	if parts[len(parts)-1] != "" && !strings.HasSuffix(str, parts[len(parts)-1]) {
		return false
	}

	return true
}

// SQLiteStore is a SQLite-backed key-value store.
type SQLiteStore struct {
	db     *sql.DB
	mu     sync.Mutex
	ownsDB bool // true if this store opened the DB and should close it
}

func (s *SQLiteStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "kvstore")
}

type kvRow struct {
	Key       string  `json:"key" zorm:"key"`
	Value     string  `json:"value" zorm:"value"`
	ExpiresAt *string `json:"expires_at" zorm:"expires_at"`
}

func parseKVTime(s *string) *time.Time {
	if s == nil {
		return nil
	}
	t, _ := time.Parse(time.RFC3339Nano, *s)
	if t.IsZero() {
		t, _ = time.Parse(time.RFC3339, *s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", *s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05-07:00", *s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05.999999999-07:00", *s)
	}
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02T15:04:05Z", *s)
	}
	if t.IsZero() {
		return nil
	}
	return &t
}

// NewSQLiteStore creates a new SQLite-backed store with its own database file.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(dbPath, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return fmt.Errorf("failed to set synchronous mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return fmt.Errorf("failed to set wal autocheckpoint: %w", err)
		}

		store := &SQLiteStore{db: db}
		if err := store.migrate(); err != nil {
			return fmt.Errorf("failed to migrate: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &SQLiteStore{db: db, ownsDB: true}, nil
}

// NewSQLiteStoreWithDB creates a kvstore backed by an existing *sql.DB connection.
// The caller retains ownership of the DB — Close() on this store is a no-op.
func NewSQLiteStoreWithDB(db *sql.DB) (*SQLiteStore, error) {
	store := &SQLiteStore{db: db, ownsDB: false}
	if err := store.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate kvstore table: %w", err)
	}
	return store, nil
}

// migrate creates the database schema.
func (s *SQLiteStore) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS kvstore (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		expires_at DATETIME
	);
	CREATE INDEX IF NOT EXISTS idx_kvstore_expires_at ON kvstore(expires_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection if this store owns it.
func (s *SQLiteStore) Close() error {
	if s.ownsDB {
		return s.db.Close()
	}
	return nil
}

// Get retrieves a value by key.
func (s *SQLiteStore) Get(ctx context.Context, key string) (interface{}, error) {
	var value string
	var expiresAtRaw sql.NullString
	err := s.db.QueryRowContext(ctx,
		"SELECT value, expires_at FROM kvstore WHERE key = ? LIMIT 1",
		key,
	).Scan(&value, &expiresAtRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Check expiration
	var expiresAt *time.Time
	if expiresAtRaw.Valid {
		expiresAt = parseKVTime(&expiresAtRaw.String)
	}
	if expiresAt != nil && !time.Now().Before(*expiresAt) {
		// Delete expired key
		s.Delete(ctx, key)
		return nil, ErrKeyNotFound
	}

	return value, nil
}

// Set stores a value with optional TTL.
func (s *SQLiteStore) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expiresAt interface{}
	if ttl > 0 {
		// Persist as RFC3339Nano for stable round-tripping and sub-second TTL support.
		expiresAt = time.Now().UTC().Add(ttl).Format(time.RFC3339Nano)
	}

	valueStr := fmt.Sprintf("%v", value)

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO kvstore (key, value, expires_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at`,
		key, valueStr, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("failed to set key: %w", err)
	}

	return nil
}

// Delete removes a key.
func (s *SQLiteStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM kvstore WHERE key = ?", key)
	if err != nil {
		return fmt.Errorf("failed to delete key: %w", err)
	}
	return nil
}

// Exists checks if a key exists.
func (s *SQLiteStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.Get(ctx, key)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrKeyNotFound) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check key: %w", err)
}

// Keys returns keys matching a pattern (SQL LIKE pattern: % matches any).
func (s *SQLiteStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT key, expires_at FROM kvstore WHERE key LIKE ?",
		pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}
	defer rows.Close()

	now := time.Now()
	keys := make([]string, 0)
	for rows.Next() {
		var key string
		var expiresAtRaw sql.NullString
		if err := rows.Scan(&key, &expiresAtRaw); err != nil {
			return nil, fmt.Errorf("failed to scan key row: %w", err)
		}

		var expiresAt *time.Time
		if expiresAtRaw.Valid {
			expiresAt = parseKVTime(&expiresAtRaw.String)
		}
		if expiresAt != nil && !now.Before(*expiresAt) {
			continue
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate key rows: %w", err)
	}

	return keys, nil
}

// Clear removes all keys.
func (s *SQLiteStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM kvstore")
	if err != nil {
		return fmt.Errorf("failed to clear: %w", err)
	}
	return nil
}

// SetJSON stores a JSON-serializable value.
func (s *SQLiteStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return s.Set(ctx, key, string(data), ttl)
}

// GetJSON retrieves and unmarshals a JSON value.
func (s *SQLiteStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	value, err := s.Get(ctx, key)
	if err != nil {
		return err
	}

	str, ok := value.(string)
	if !ok {
		return fmt.Errorf("value is not a string")
	}

	return json.Unmarshal([]byte(str), dest)
}
