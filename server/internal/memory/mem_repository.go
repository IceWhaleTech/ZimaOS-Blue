package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MemoryRepository provides CRUD, search, versioning, and purge for MemoryEntry.
// It uses SQLite for metadata storage and delegates vector search to HybridSearcher.
type MemoryRepository struct {
	db        *sql.DB
	mu        sync.RWMutex
	encryptor *ContentEncryptor
}

// SetEncryptor sets the content encryptor for transparent encryption at rest.
func (r *MemoryRepository) SetEncryptor(enc *ContentEncryptor) {
	r.encryptor = enc
}

// NewMemoryRepository creates a repository and initializes the entries table.
func NewMemoryRepository(db *sql.DB) (*MemoryRepository, error) {
	r := &MemoryRepository{db: db}
	if err := r.initSchema(); err != nil {
		return nil, fmt.Errorf("memory repo schema init: %w", err)
	}
	return r, nil
}

func (r *MemoryRepository) initSchema() error {
	_, err := r.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_entries (
			id           TEXT PRIMARY KEY,
			namespace    TEXT NOT NULL,
			content      TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT 'text',
			category     TEXT DEFAULT '',
			tags         TEXT DEFAULT '[]',
			metadata     TEXT DEFAULT '{}',
			importance   REAL DEFAULT 0.5,
			source       TEXT DEFAULT '',
			version      INTEGER DEFAULT 1,
			parent_id    TEXT DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'active',
			ttl_ns       INTEGER DEFAULT 0,
			created_at   DATETIME NOT NULL,
			updated_at   DATETIME NOT NULL,
			expires_at   DATETIME,
			deleted_at   DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_entries_namespace ON memory_entries(namespace);
		CREATE INDEX IF NOT EXISTS idx_entries_status ON memory_entries(status);
		CREATE INDEX IF NOT EXISTS idx_entries_parent ON memory_entries(parent_id);
		CREATE INDEX IF NOT EXISTS idx_entries_expires ON memory_entries(expires_at);
	`)
	return err
}

// Create inserts a new memory entry.
func (r *MemoryRepository) Create(ctx context.Context, e *MemoryEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tagsJSON, _ := json.Marshal(e.Tags)
	metaJSON, _ := json.Marshal(e.Metadata)

	// Encrypt content if encryptor is available
	content := e.Content
	if r.encryptor != nil {
		encrypted, err := r.encryptor.Encrypt(content)
		if err != nil {
			return fmt.Errorf("encrypt content: %w", err)
		}
		content = encrypted
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO memory_entries
			(id, namespace, content, content_type, category, tags, metadata,
			 importance, source, version, parent_id, status, ttl_ns,
			 created_at, updated_at, expires_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Namespace, content, string(e.ContentType),
		e.Category, string(tagsJSON), string(metaJSON),
		e.Importance, e.Source, e.Version, e.ParentID, string(e.Status),
		int64(e.TTL), e.CreatedAt, e.UpdatedAt, e.ExpiresAt, e.DeletedAt,
	)
	return err
}

// Get retrieves a memory entry by ID.
func (r *MemoryRepository) Get(ctx context.Context, id string) (*MemoryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.scanEntry(r.db.QueryRowContext(ctx, `
		SELECT id, namespace, content, content_type, category, tags, metadata,
		       importance, source, version, parent_id, status, ttl_ns,
		       created_at, updated_at, expires_at, deleted_at
		FROM memory_entries WHERE id = ?`, id))
}

func (r *MemoryRepository) scanEntry(row *sql.Row) (*MemoryEntry, error) {
	var e MemoryEntry
	var contentType, status, tagsJSON, metaJSON string
	var ttlNs int64
	err := row.Scan(
		&e.ID, &e.Namespace, &e.Content, &contentType,
		&e.Category, &tagsJSON, &metaJSON,
		&e.Importance, &e.Source, &e.Version, &e.ParentID,
		&status, &ttlNs,
		&e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt, &e.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("entry not found")
	}
	if err != nil {
		return nil, err
	}
	e.ContentType = ContentType(contentType)
	e.Status = EntryStatus(status)
	e.TTL = Duration(ttlNs)
	_ = json.Unmarshal([]byte(tagsJSON), &e.Tags)
	_ = json.Unmarshal([]byte(metaJSON), &e.Metadata)

	// Decrypt content if encryptor is available
	if r.encryptor != nil {
		decrypted, err := r.encryptor.Decrypt(e.Content)
		if err != nil {
			return nil, fmt.Errorf("decrypt content: %w", err)
		}
		e.Content = decrypted
	}

	return &e, nil
}

// Update creates a new version of an existing entry.
func (r *MemoryRepository) Update(ctx context.Context, id, newContent string) (*MemoryEntry, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Content == newContent {
		return existing, nil // idempotent
	}
	newEntry := existing.NewVersion(newContent)
	newEntry.ComputeExpiresAt()
	if err := r.Create(ctx, newEntry); err != nil {
		return nil, err
	}
	return newEntry, nil
}

// Delete soft-deletes an entry.
func (r *MemoryRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	res, err := r.db.ExecContext(ctx, `
		UPDATE memory_entries SET status = 'deleted', deleted_at = ?, updated_at = ?
		WHERE id = ? AND status != 'deleted'`, now, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("entry %q not found or already deleted", id)
	}
	return nil
}

// ListParams controls listing behavior.
type ListParams struct {
	Namespace string
	Status    EntryStatus
	Category  string
	Cursor    string // ID to start after
	Limit     int
}

// List returns entries matching the given params.
func (r *MemoryRepository) List(ctx context.Context, p ListParams) ([]*MemoryEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	var conditions []string
	var args []any

	conditions = append(conditions, "namespace = ?")
	args = append(args, p.Namespace)

	if p.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(p.Status))
	}
	if p.Category != "" {
		conditions = append(conditions, "category = ?")
		args = append(args, p.Category)
	}
	if p.Cursor != "" {
		conditions = append(conditions, "id > ?")
		args = append(args, p.Cursor)
	}

	where := strings.Join(conditions, " AND ")
	args = append(args, p.Limit)

	query := fmt.Sprintf(`
		SELECT id, namespace, content, content_type, category, tags, metadata,
		       importance, source, version, parent_id, status, ttl_ns,
		       created_at, updated_at, expires_at, deleted_at
		FROM memory_entries WHERE %s ORDER BY created_at DESC LIMIT ?`, where)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		var contentType, status, tagsJSON, metaJSON string
		var ttlNs int64
		if err := rows.Scan(
			&e.ID, &e.Namespace, &e.Content, &contentType,
			&e.Category, &tagsJSON, &metaJSON,
			&e.Importance, &e.Source, &e.Version, &e.ParentID,
			&status, &ttlNs,
			&e.CreatedAt, &e.UpdatedAt, &e.ExpiresAt, &e.DeletedAt,
		); err != nil {
			return nil, err
		}
		e.ContentType = ContentType(contentType)
		e.Status = EntryStatus(status)
		e.TTL = Duration(ttlNs)
		_ = json.Unmarshal([]byte(tagsJSON), &e.Tags)
		_ = json.Unmarshal([]byte(metaJSON), &e.Metadata)
		// Decrypt content
		if r.encryptor != nil {
			if dec, err := r.encryptor.Decrypt(e.Content); err == nil {
				e.Content = dec
			}
		}
		result = append(result, &e)
	}
	return result, rows.Err()
}

// History returns the version chain for an entry (newest first).
func (r *MemoryRepository) History(ctx context.Context, id string) ([]*MemoryEntry, error) {
	var chain []*MemoryEntry
	currentID := id
	for currentID != "" {
		e, err := r.Get(ctx, currentID)
		if err != nil {
			break
		}
		chain = append(chain, e)
		currentID = e.ParentID
	}
	return chain, nil
}

// PurgeExpired removes entries whose expires_at has passed.
// Returns the number of entries purged.
func (r *MemoryRepository) PurgeExpired(ctx context.Context) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	res, err := r.db.ExecContext(ctx, `
		DELETE FROM memory_entries
		WHERE expires_at IS NOT NULL AND expires_at < ? AND status != 'deleted'`,
		time.Now())
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// Stats returns statistics for a namespace.
func (r *MemoryRepository) Stats(ctx context.Context, namespace string) (*NamespaceStats, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &NamespaceStats{NamespaceID: namespace}
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(LENGTH(content)), 0),
			COALESCE(SUM(CASE WHEN status = 'active' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'expired' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'deleted' THEN 1 ELSE 0 END), 0)
		FROM memory_entries WHERE namespace = ?`, namespace,
	).Scan(&stats.EntryCount, &stats.TotalSize, &stats.ActiveCount, &stats.ExpiredCount, &stats.DeletedCount)
	return stats, err
}

// CountEncrypted returns the count of encrypted and plaintext entries.
func (r *MemoryRepository) CountEncrypted(ctx context.Context) (encrypted, plaintext int, err error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	err = r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN content LIKE 'ENC:v1:%' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN content NOT LIKE 'ENC:v1:%' THEN 1 ELSE 0 END), 0)
		FROM memory_entries WHERE status != 'deleted'`).Scan(&encrypted, &plaintext)
	return
}

// CountAll returns total entry count (excluding deleted).
func (r *MemoryRepository) CountAll(ctx context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_entries WHERE status != 'deleted'`).Scan(&count)
	return count, err
}

// ListRaw returns raw id+content pairs for migration, bypassing decryption.
func (r *MemoryRepository) ListRaw(ctx context.Context, offset, limit int) ([]RawEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, content FROM memory_entries WHERE status != 'deleted'
		 ORDER BY id LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []RawEntry
	for rows.Next() {
		var e RawEntry
		if err := rows.Scan(&e.ID, &e.Content); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// UpdateContentRaw updates only the content field, bypassing encryption logic.
// Used by migration operations.
func (r *MemoryRepository) UpdateContentRaw(ctx context.Context, id, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, err := r.db.ExecContext(ctx,
		`UPDATE memory_entries SET content = ?, updated_at = ? WHERE id = ?`,
		content, time.Now(), id)
	return err
}
