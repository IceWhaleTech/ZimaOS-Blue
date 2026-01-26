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

	_ "modernc.org/sqlite"
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
	return time.Now().After(e.expiresAt)
}

// MemoryStore is an in-memory key-value store.
type MemoryStore struct {
	mu    sync.RWMutex
	data  map[string]*entry
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
		e.expiresAt = time.Now().Add(ttl)
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
	db *sql.DB
	mu sync.Mutex
}

// NewSQLiteStore creates a new SQLite-backed store.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Set busy timeout
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set busy timeout: %w", err)
	}

	store := &SQLiteStore{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate: %w", err)
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

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Get retrieves a value by key.
func (s *SQLiteStore) Get(ctx context.Context, key string) (interface{}, error) {
	var value string
	var expiresAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		"SELECT value, expires_at FROM kvstore WHERE key = ?",
		key,
	).Scan(&value, &expiresAt)

	if err == sql.ErrNoRows {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key: %w", err)
	}

	// Check expiration
	if expiresAt.Valid && time.Now().After(expiresAt.Time) {
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

	var expiresAt sql.NullTime
	if ttl > 0 {
		expiresAt = sql.NullTime{Time: time.Now().Add(ttl), Valid: true}
	}

	valueStr := fmt.Sprintf("%v", value)

	_, err := s.db.ExecContext(ctx,
		"INSERT OR REPLACE INTO kvstore (key, value, expires_at) VALUES (?, ?, ?)",
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
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM kvstore WHERE key = ? AND (expires_at IS NULL OR expires_at > ?)",
		key, time.Now(),
	).Scan(&count)

	if err != nil {
		return false, fmt.Errorf("failed to check key: %w", err)
	}

	return count > 0, nil
}

// Keys returns keys matching a pattern (SQL LIKE pattern: % matches any).
func (s *SQLiteStore) Keys(ctx context.Context, pattern string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT key FROM kvstore WHERE key LIKE ? AND (expires_at IS NULL OR expires_at > ?)",
		pattern, time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
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
