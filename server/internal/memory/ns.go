package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// NamespaceConfig holds per-namespace settings.
type NamespaceConfig struct {
	EmbeddingDim  int      `json:"embedding_dim"`
	DefaultTTL    Duration `json:"default_ttl,omitempty"`
	MaxEntries    int      `json:"max_entries"`
	MaxSizeBytes  int64    `json:"max_size_bytes"`
}

// DefaultNamespaceConfig returns sensible defaults.
func DefaultNamespaceConfig() NamespaceConfig {
	return NamespaceConfig{
		EmbeddingDim: 384,
		MaxEntries:   100000,
		MaxSizeBytes: 100 * 1024 * 1024, // 100MB
	}
}

// Namespace represents an isolated memory namespace.
type Namespace struct {
	ID        string          `json:"id"`
	Config    NamespaceConfig `json:"config"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// NamespaceStats holds usage statistics for a namespace.
type NamespaceStats struct {
	NamespaceID  string `json:"namespace_id"`
	EntryCount   int    `json:"entry_count"`
	TotalSize    int64  `json:"total_size_bytes"`
	ActiveCount  int    `json:"active_count"`
	ExpiredCount int    `json:"expired_count"`
	DeletedCount int    `json:"deleted_count"`
}

// NamespaceStore manages namespace CRUD backed by SQLite.
type NamespaceStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewNamespaceStore creates a NamespaceStore and initializes the schema.
func NewNamespaceStore(db *sql.DB) (*NamespaceStore, error) {
	s := &NamespaceStore{db: db}
	if err := s.initSchema(); err != nil {
		return nil, fmt.Errorf("namespace schema init: %w", err)
	}
	return s, nil
}

func (s *NamespaceStore) initSchema() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS namespaces (
			id         TEXT PRIMARY KEY,
			config     TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// Create creates a new namespace. Returns error if it already exists.
func (s *NamespaceStore) Create(ctx context.Context, ns *Namespace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfgJSON, err := json.Marshal(ns.Config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	now := timeutil.NowTime()
	ns.CreatedAt = now
	ns.UpdatedAt = now

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO namespaces (id, config, created_at, updated_at) VALUES (?, ?, ?, ?)`,
		ns.ID, string(cfgJSON), ns.CreatedAt, ns.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert namespace: %w", err)
	}
	return nil
}

// Get retrieves a namespace by ID.
func (s *NamespaceStore) Get(ctx context.Context, id string) (*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ns Namespace
	var cfgJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, config, created_at, updated_at FROM namespaces WHERE id = ?`, id,
	).Scan(&ns.ID, &cfgJSON, &ns.CreatedAt, &ns.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("namespace %q not found", id)
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(cfgJSON), &ns.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return &ns, nil
}

// List returns all namespaces.
func (s *NamespaceStore) List(ctx context.Context) ([]*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, config, created_at, updated_at FROM namespaces ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Namespace
	for rows.Next() {
		var ns Namespace
		var cfgJSON string
		if err := rows.Scan(&ns.ID, &cfgJSON, &ns.CreatedAt, &ns.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(cfgJSON), &ns.Config); err != nil {
			return nil, err
		}
		result = append(result, &ns)
	}
	return result, rows.Err()
}

// Delete removes a namespace.
func (s *NamespaceStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	res, err := s.db.ExecContext(ctx, `DELETE FROM namespaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("namespace %q not found", id)
	}
	return nil
}

// UpdateConfig updates the config for a namespace.
func (s *NamespaceStore) UpdateConfig(ctx context.Context, id string, cfg NamespaceConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE namespaces SET config = ?, updated_at = ? WHERE id = ?`,
		string(cfgJSON), timeutil.NowTime(), id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("namespace %q not found", id)
	}
	return nil
}
