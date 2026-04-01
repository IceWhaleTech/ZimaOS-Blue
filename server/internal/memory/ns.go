package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
)

// NamespaceConfig holds per-namespace settings.
type NamespaceConfig struct {
	EmbeddingDim int      `json:"embedding_dim"`
	DefaultTTL   Duration `json:"default_ttl,omitempty"`
	MaxEntries   int      `json:"max_entries"`
	MaxSizeBytes int64    `json:"max_size_bytes"`
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
	db     *sql.DB
	readDB *sql.DB
	mu     sync.RWMutex
}

// NewNamespaceStore creates a NamespaceStore and initializes the schema.
func NewNamespaceStore(db *sql.DB) (*NamespaceStore, error) {
	return NewNamespaceStoreWithReadDB(db, db)
}

// NewNamespaceStoreWithReadDB creates a NamespaceStore with separate write and
// read database handles and initializes the schema.
func NewNamespaceStoreWithReadDB(writeDB, readDB *sql.DB) (*NamespaceStore, error) {
	if writeDB == nil {
		return nil, fmt.Errorf("namespace db is required")
	}
	if readDB == nil {
		readDB = writeDB
	}
	s := &NamespaceStore{db: writeDB, readDB: readDB}
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

func (s *NamespaceStore) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.db, "namespaces")
}

func (s *NamespaceStore) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), "namespaces")
}

func (s *NamespaceStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

type namespaceRow struct {
	ID        string `json:"id" zorm:"id"`
	Config    string `json:"config" zorm:"config"`
	CreatedAt string `json:"created_at" zorm:"created_at"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
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

	_, err = s.table(ctx).Insert(map[string]interface{}{
		"id":         ns.ID,
		"config":     string(cfgJSON),
		"created_at": ns.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": ns.UpdatedAt.UTC().Format(time.RFC3339Nano),
	},
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

	var rows []namespaceRow
	_, err := s.readTable(ctx).Select(&rows,
		z.Where(z.Eq("id", id)),
		z.Limit(1),
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("namespace %q not found", id)
	}
	return rowToNamespace(rows[0])
}

// List returns all namespaces.
func (s *NamespaceStore) List(ctx context.Context) ([]*Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var rows []namespaceRow
	_, err := s.readTable(ctx).Select(&rows, z.OrderBy("created_at"))
	if err != nil {
		return nil, err
	}

	result := make([]*Namespace, 0, len(rows))
	for i := range rows {
		ns, err := rowToNamespace(rows[i])
		if err != nil {
			return nil, err
		}
		result = append(result, ns)
	}
	return result, nil
}

// Delete removes a namespace.
func (s *NamespaceStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, err := s.table(ctx).Delete(z.Where(z.Eq("id", id)))
	if err != nil {
		return err
	}
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
	n, err := s.table(ctx).Update(
		z.V{
			"config":     string(cfgJSON),
			"updated_at": timeutil.NowTime().UTC().Format(time.RFC3339Nano),
		},
		z.Fields("config", "updated_at"),
		z.Where(z.Eq("id", id)),
	)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("namespace %q not found", id)
	}
	return nil
}

func rowToNamespace(row namespaceRow) (*Namespace, error) {
	var ns Namespace
	ns.ID = row.ID
	if err := json.Unmarshal([]byte(row.Config), &ns.Config); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	ns.CreatedAt = parseNamespaceTime(row.CreatedAt)
	ns.UpdatedAt = parseNamespaceTime(row.UpdatedAt)
	return &ns, nil
}

func parseNamespaceTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
