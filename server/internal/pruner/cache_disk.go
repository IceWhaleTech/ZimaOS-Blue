package pruner

import (
	"bytes"
	"database/sql"
	"encoding/gob"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// DiskCache provides SQLite-backed cache persistence with TTL expiration.
// Reuses the same SQLite driver (modernc.org/sqlite) as the proxy's CCCache.
type DiskCache struct {
	db  *sql.DB
	ttl time.Duration
	mu  sync.RWMutex
}

// NewDiskCache creates a SQLite-backed disk cache at the given path.
// If dbPath is empty or the database cannot be opened, returns a no-op cache.
func NewDiskCache(dbPath string, ttl time.Duration) *DiskCache {
	if dbPath == "" {
		return &DiskCache{ttl: ttl}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return &DiskCache{ttl: ttl}
	}

	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")

	dc := &DiskCache{db: db, ttl: ttl}
	if err := dc.initSchema(); err != nil {
		db.Close()
		return &DiskCache{ttl: ttl}
	}

	return dc
}

func (d *DiskCache) initSchema() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS pruner_cache (
			key TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			created_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_pruner_cache_created ON pruner_cache(created_at);
	`)
	return err
}

// Get retrieves scored segments from disk cache.
func (d *DiskCache) Get(key string) ([]ScoredSegment, bool) {
	if d.db == nil {
		return nil, false
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	var data []byte
	var createdAt string

	err := d.db.QueryRow(
		"SELECT data, created_at FROM pruner_cache WHERE key = ?", key,
	).Scan(&data, &createdAt)
	if err != nil {
		return nil, false
	}

	// Check TTL
	ca, err := time.Parse(time.RFC3339, createdAt)
	if err != nil || time.Since(ca) > d.ttl {
		go func() {
			d.mu.Lock()
			defer d.mu.Unlock()
			d.db.Exec("DELETE FROM pruner_cache WHERE key = ?", key)
		}()
		return nil, false
	}

	var segments []ScoredSegment
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&segments); err != nil {
		return nil, false
	}

	return segments, true
}

// Put stores scored segments to disk cache.
func (d *DiskCache) Put(key string, segments []ScoredSegment) {
	if d.db == nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(segments); err != nil {
		return
	}

	d.db.Exec(`
		INSERT OR REPLACE INTO pruner_cache (key, data, created_at)
		VALUES (?, ?, ?)
	`, key, buf.Bytes(), time.Now().UTC().Format(time.RFC3339))
}

// Cleanup removes expired entries.
func (d *DiskCache) Cleanup() int64 {
	if d.db == nil {
		return 0
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-d.ttl).UTC().Format(time.RFC3339)
	result, err := d.db.Exec("DELETE FROM pruner_cache WHERE created_at <= ?", cutoff)
	if err != nil {
		return 0
	}
	n, _ := result.RowsAffected()
	return n
}

// Close closes the database connection.
func (d *DiskCache) Close() error {
	if d.db == nil {
		return nil
	}
	return d.db.Close()
}
