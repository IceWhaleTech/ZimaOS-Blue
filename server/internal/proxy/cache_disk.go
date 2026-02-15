package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // Pure-Go SQLite driver (no CGO)
)

// DiskCache implements L2 disk-based cache using SQLite.
// Chosen over BadgerDB/BoltDB to avoid adding new dependencies —
// SQLite is already used elsewhere in the project.
type DiskCache struct {
	db       *sql.DB
	mu       sync.RWMutex
	path     string
	hitChan  chan string    // Buffered channel for batched hit count updates
	stopHits chan struct{}  // Signal to stop the hit batcher
}

// NewDiskCache creates a new SQLite-backed disk cache.
func NewDiskCache(dbPath string) (*DiskCache, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("disk cache: open db: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	// Set pragmas after opening (mattn/go-sqlite3 doesn't support DSN pragmas)
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA busy_timeout=5000")
	db.Exec("PRAGMA cache_size=-500") // ~512KB page cache for lower idle memory

	dc := &DiskCache{db: db, path: dbPath, hitChan: make(chan string, 256), stopHits: make(chan struct{})}
	if err := dc.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("disk cache: init schema: %w", err)
	}

	// Release unused memory after schema init
	db.Exec("PRAGMA shrink_memory")

	go dc.hitBatchLoop()

	return dc, nil
}

func (dc *DiskCache) initSchema() error {
	_, err := dc.db.Exec(`
		CREATE TABLE IF NOT EXISTS cache_entries (
			key TEXT PRIMARY KEY,
			body BLOB NOT NULL,
			status_code INTEGER NOT NULL,
			headers TEXT NOT NULL,
			provider TEXT,
			model TEXT,
			created_at DATETIME NOT NULL,
			expires_at DATETIME NOT NULL,
			hit_count INTEGER DEFAULT 0,
			content_hash TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_cache_expires ON cache_entries(expires_at);
		CREATE INDEX IF NOT EXISTS idx_cache_hits ON cache_entries(hit_count DESC);
	`)
	return err
}

// Get retrieves an entry from disk cache.
func (dc *DiskCache) Get(key string) (*CCCacheEntry, bool) {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	var (
		body        []byte
		statusCode  int
		headersJSON string
		provider    string
		model       string
		createdAt   string
		expiresAt   string
		hitCount    int64
		contentHash string
	)

	err := dc.db.QueryRow(`
		SELECT body, status_code, headers, provider, model, created_at, expires_at, hit_count, content_hash
		FROM cache_entries WHERE key = ? AND expires_at > ?
	`, key, time.Now().UTC().Format(time.RFC3339)).Scan(&body, &statusCode, &headersJSON, &provider, &model, &createdAt, &expiresAt, &hitCount, &contentHash)

	if err != nil {
		return nil, false
	}

	// Batch hit count update via channel (non-blocking)
	select {
	case dc.hitChan <- key:
	default:
		// Channel full, skip this hit count update
	}

	var headers map[string]string
	json.Unmarshal([]byte(headersJSON), &headers)

	ca, _ := time.Parse(time.RFC3339, createdAt)
	ea, _ := time.Parse(time.RFC3339, expiresAt)

	return &CCCacheEntry{
		Key:         key,
		Body:        body,
		StatusCode:  statusCode,
		Headers:     headers,
		Provider:    provider,
		Model:       model,
		CreatedAt:   ca,
		ExpiresAt:   ea,
		HitCount:    hitCount,
		ContentHash: contentHash,
	}, true
}

// Set stores an entry in disk cache.
func (dc *DiskCache) Set(entry *CCCacheEntry) {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	headersJSON, _ := json.Marshal(entry.Headers)

	dc.db.Exec(`
		INSERT OR REPLACE INTO cache_entries (key, body, status_code, headers, provider, model, created_at, expires_at, hit_count, content_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.Key, entry.Body, entry.StatusCode, string(headersJSON),
		entry.Provider, entry.Model,
		entry.CreatedAt.UTC().Format(time.RFC3339), entry.ExpiresAt.UTC().Format(time.RFC3339),
		entry.HitCount, entry.ContentHash)
}

// Delete removes an entry from disk cache.
func (dc *DiskCache) Delete(key string) {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.db.Exec("DELETE FROM cache_entries WHERE key = ?", key)
}

// Clear removes all entries from disk cache.
func (dc *DiskCache) Clear() {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	dc.db.Exec("DELETE FROM cache_entries")
}

// Cleanup removes expired entries.
func (dc *DiskCache) Cleanup() int64 {
	dc.mu.Lock()
	defer dc.mu.Unlock()
	result, err := dc.db.Exec("DELETE FROM cache_entries WHERE expires_at <= ?", time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0
	}
	n, _ := result.RowsAffected()
	return n
}

// TopN returns the top N entries by hit count (for warmup).
func (dc *DiskCache) TopN(n int) []*CCCacheEntry {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := dc.db.Query(`
		SELECT key, body, status_code, headers, provider, model, created_at, expires_at, hit_count, content_hash
		FROM cache_entries
		WHERE expires_at > ?
		ORDER BY hit_count DESC
		LIMIT ?
	`, now, n)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var entries []*CCCacheEntry
	for rows.Next() {
		var (
			body        []byte
			statusCode  int
			headersJSON string
			provider    string
			model       string
			createdAt   string
			expiresAt   string
			hitCount    int64
			contentHash string
			key         string
		)
		if err := rows.Scan(&key, &body, &statusCode, &headersJSON, &provider, &model, &createdAt, &expiresAt, &hitCount, &contentHash); err != nil {
			continue
		}

		var headers map[string]string
		json.Unmarshal([]byte(headersJSON), &headers)
		ca, _ := time.Parse(time.RFC3339, createdAt)
		ea, _ := time.Parse(time.RFC3339, expiresAt)

		entries = append(entries, &CCCacheEntry{
			Key:         key,
			Body:        body,
			StatusCode:  statusCode,
			Headers:     headers,
			Provider:    provider,
			Model:       model,
			CreatedAt:   ca,
			ExpiresAt:   ea,
			HitCount:    hitCount,
			ContentHash: contentHash,
		})
	}
	return entries
}

// Count returns the number of entries in disk cache.
func (dc *DiskCache) Count() int64 {
	var count int64
	dc.db.QueryRow("SELECT COUNT(*) FROM cache_entries WHERE expires_at > ?", time.Now().UTC().Format(time.RFC3339)).Scan(&count)
	return count
}

// Close closes the disk cache database.
func (dc *DiskCache) Close() error {
	close(dc.stopHits)
	return dc.db.Close()
}

// hitBatchLoop collects hit count updates and flushes them in batch every 5 seconds.
func (dc *DiskCache) hitBatchLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	pending := make(map[string]int64)

	for {
		select {
		case key := <-dc.hitChan:
			pending[key]++
		case <-ticker.C:
			dc.flushHits(pending)
			pending = make(map[string]int64)
		case <-dc.stopHits:
			// Drain remaining
			for {
				select {
				case key := <-dc.hitChan:
					pending[key]++
				default:
					dc.flushHits(pending)
					return
				}
			}
		}
	}
}

// flushHits writes accumulated hit counts to SQLite in a single transaction.
func (dc *DiskCache) flushHits(pending map[string]int64) {
	if len(pending) == 0 {
		return
	}
	dc.mu.Lock()
	defer dc.mu.Unlock()

	tx, err := dc.db.Begin()
	if err != nil {
		return
	}
	stmt, err := tx.Prepare("UPDATE cache_entries SET hit_count = hit_count + ? WHERE key = ?")
	if err != nil {
		tx.Rollback()
		return
	}
	defer stmt.Close()

	for key, count := range pending {
		stmt.Exec(count, key)
	}
	tx.Commit()
}
