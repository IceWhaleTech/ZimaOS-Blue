package embedding

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	_ "github.com/mattn/go-sqlite3"
)

// Cache caches embeddings to avoid redundant API calls.
type Cache struct {
	db         *sql.DB
	readDB     *sql.DB
	provider   string
	model      string
	maxEntries int
	mu         sync.Mutex
}

const cacheSchema = `
CREATE TABLE IF NOT EXISTS embedding_cache (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    text_hash TEXT NOT NULL,
    embedding TEXT NOT NULL,
    dimensions INTEGER NOT NULL,
    created_at DATETIME NOT NULL,
    accessed_at DATETIME NOT NULL,
    UNIQUE(provider, model, text_hash)
);

CREATE INDEX IF NOT EXISTS idx_embedding_cache_lookup
    ON embedding_cache(provider, model, text_hash);
CREATE INDEX IF NOT EXISTS idx_embedding_cache_accessed
    ON embedding_cache(accessed_at);
`

// CacheConfig holds cache configuration.
type CacheConfig struct {
	DBPath     string
	Provider   string
	Model      string
	MaxEntries int
}

// NewCache creates a new embedding cache.
func NewCache(cfg CacheConfig) (*Cache, error) {
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(cfg.DBPath, cfg.DBPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(2)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			return fmt.Errorf("failed to enable WAL mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
			return fmt.Errorf("failed to set synchronous mode: %w", err)
		}
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}
		if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
			return fmt.Errorf("failed to set wal autocheckpoint: %w", err)
		}
		if _, err := db.Exec(cacheSchema); err != nil {
			return fmt.Errorf("failed to create cache schema: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}

	readDB, readErr := openEmbeddingCacheReaderDB(cfg.DBPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	maxEntries := cfg.MaxEntries
	if maxEntries == 0 {
		maxEntries = 10000
	}

	return &Cache{
		db:         db,
		readDB:     readDB,
		provider:   cfg.Provider,
		model:      cfg.Model,
		maxEntries: maxEntries,
	}, nil
}

func (c *Cache) table(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, c.db, "embedding_cache")
}

func (c *Cache) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, c.reader(), "embedding_cache")
}

func (c *Cache) reader() *sql.DB {
	if c != nil && c.readDB != nil {
		return c.readDB
	}
	if c == nil {
		return nil
	}
	return c.db
}

func openEmbeddingCacheReaderDB(dbPath string) (*sql.DB, error) {
	if dbPath == "" || dbPath == ":memory:" {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("failed to set embedding cache reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

type cacheRow struct {
	ID         int64  `json:"id" zorm:"id"`
	Provider   string `json:"provider" zorm:"provider"`
	Model      string `json:"model" zorm:"model"`
	TextHash   string `json:"text_hash" zorm:"text_hash"`
	Embedding  string `json:"embedding" zorm:"embedding"`
	Dimensions int    `json:"dimensions" zorm:"dimensions"`
	CreatedAt  string `json:"created_at" zorm:"created_at"`
	AccessedAt string `json:"accessed_at" zorm:"accessed_at"`
}

// Get retrieves an embedding from the cache.
func (c *Cache) Get(ctx context.Context, textHash string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var rows []cacheRow
	_, err := c.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
			z.Eq("text_hash", textHash),
		),
		z.Limit(1),
	)
	if err != nil {
		return nil, false
	}
	if len(rows) == 0 {
		return nil, false
	}

	// Update accessed_at
	_, _ = c.table(ctx).Update(
		z.V{"accessed_at": timeutil.NowTime().UTC().Format(time.RFC3339Nano)},
		z.Fields("accessed_at"),
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
			z.Eq("text_hash", textHash),
		),
	)

	var embedding []float32
	if err := json.Unmarshal([]byte(rows[0].Embedding), &embedding); err != nil {
		return nil, false
	}

	return embedding, true
}

// Set stores an embedding in the cache.
func (c *Cache) Set(ctx context.Context, textHash string, embedding []float32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	now := timeutil.NowTime()
	nowStr := now.UTC().Format(time.RFC3339Nano)
	_, err = c.table(ctx).Insert(
		map[string]interface{}{
			"provider":    c.provider,
			"model":       c.model,
			"text_hash":   textHash,
			"embedding":   string(embeddingJSON),
			"dimensions":  len(embedding),
			"created_at":  nowStr,
			"accessed_at": nowStr,
		},
		z.OnConflictDoUpdateSet(
			[]string{"provider", "model", "text_hash"},
			[]string{"embedding", "dimensions", "accessed_at"},
		),
	)

	if err != nil {
		return fmt.Errorf("failed to cache embedding: %w", err)
	}

	// Prune if needed
	go c.pruneIfNeeded(context.Background())

	return nil
}

// GetBatch retrieves multiple embeddings from the cache.
func (c *Cache) GetBatch(ctx context.Context, textHashes []string) (map[string][]float32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result := make(map[string][]float32)
	if len(textHashes) == 0 {
		return result, nil
	}

	var rows []cacheRow
	_, err := c.readTable(ctx).Select(&rows,
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
			z.In("text_hash", stringSliceToInterfaces(textHashes)...),
		),
	)
	if err != nil {
		return result, nil
	}
	for i := range rows {
		var embedding []float32
		if err := json.Unmarshal([]byte(rows[i].Embedding), &embedding); err != nil {
			continue
		}
		result[rows[i].TextHash] = embedding
	}

	// Update accessed_at for found entries
	if len(result) > 0 {
		_, _ = c.table(ctx).Update(
			z.V{"accessed_at": timeutil.NowTime().UTC().Format(time.RFC3339Nano)},
			z.Fields("accessed_at"),
			z.Where(
				z.Eq("provider", c.provider),
				z.Eq("model", c.model),
				z.In("text_hash", stringSliceToInterfaces(mapKeys(result))...),
			),
		)
	}

	return result, nil
}

// SetBatch stores multiple embeddings in the cache.
func (c *Cache) SetBatch(ctx context.Context, embeddings map[string][]float32) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := timeutil.NowTime()
	nowStr := now.UTC().Format(time.RFC3339Nano)
	table := z.TableContext(ctx, tx, "embedding_cache")
	for hash, embedding := range embeddings {
		embeddingJSON, err := json.Marshal(embedding)
		if err != nil {
			continue
		}
		_, err = table.Insert(
			map[string]interface{}{
				"provider":    c.provider,
				"model":       c.model,
				"text_hash":   hash,
				"embedding":   string(embeddingJSON),
				"dimensions":  len(embedding),
				"created_at":  nowStr,
				"accessed_at": nowStr,
			},
			z.OnConflictDoUpdateSet(
				[]string{"provider", "model", "text_hash"},
				[]string{"embedding", "dimensions", "accessed_at"},
			),
		)
		if err != nil {
			continue
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Prune if needed
	go c.pruneIfNeeded(context.Background())

	return nil
}

// Prune removes old entries to stay within maxEntries.
func (c *Cache) Prune(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.pruneUnsafe(ctx)
}

// pruneIfNeeded prunes if the cache exceeds maxEntries.
func (c *Cache) pruneIfNeeded(ctx context.Context) {
	var count int64
	_, err := c.readTable(ctx).Select(
		&count,
		z.Fields("count(1)"),
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
		),
	)

	if err != nil || count <= int64(c.maxEntries) {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.pruneUnsafe(ctx)
}

// pruneUnsafe prunes without locking (caller must hold lock).
func (c *Cache) pruneUnsafe(ctx context.Context) error {
	// Delete oldest entries to get back to 80% of maxEntries
	targetCount := int(float64(c.maxEntries) * 0.8)

	_, err := c.table(ctx).Delete(
		z.Where(z.Expr(`
			provider = ? AND model = ? AND id IN (
				SELECT id FROM embedding_cache
				WHERE provider = ? AND model = ?
				ORDER BY accessed_at ASC
				LIMIT (
					SELECT MAX(0, COUNT(*) - ?) FROM embedding_cache
					WHERE provider = ? AND model = ?
				)
			)
		`, c.provider, c.model, c.provider, c.model, targetCount, c.provider, c.model)),
	)

	return err
}

// Close closes the cache database.
func (c *Cache) Close() error {
	if c.readDB != nil && c.readDB != c.db {
		_ = c.readDB.Close()
	}
	return c.db.Close()
}

// Stats returns cache statistics.
func (c *Cache) Stats(ctx context.Context) (CacheStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var stats CacheStats
	var entryCount int64

	if _, err := c.readTable(ctx).Select(
		&entryCount,
		z.Fields("count(1)"),
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
		),
	); err != nil {
		return stats, err
	}
	stats.EntryCount = int(entryCount)

	var rows []z.V
	if _, err := c.readTable(ctx).Select(
		&rows,
		z.Fields("min(created_at)", "max(accessed_at)"),
		z.Where(
			z.Eq("provider", c.provider),
			z.Eq("model", c.model),
		),
		z.Limit(1),
	); err != nil {
		return stats, err
	}
	if len(rows) > 0 {
		stats.OldestEntry = parseCacheStatsTime(rows[0], "min(created_at)")
		stats.NewestAccess = parseCacheStatsTime(rows[0], "max(accessed_at)")
	}

	stats.MaxEntries = c.maxEntries
	stats.Provider = c.provider
	stats.Model = c.model

	return stats, nil
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	EntryCount   int       `json:"entry_count"`
	MaxEntries   int       `json:"max_entries"`
	OldestEntry  time.Time `json:"oldest_entry,omitempty"`
	NewestAccess time.Time `json:"newest_access,omitempty"`
}

// CachedProvider wraps a Provider with caching.
type CachedProvider struct {
	provider Provider
	cache    *Cache
}

// NewCachedProvider creates a new CachedProvider.
func NewCachedProvider(provider Provider, cache *Cache) *CachedProvider {
	return &CachedProvider{
		provider: provider,
		cache:    cache,
	}
}

// Name returns the provider name.
func (p *CachedProvider) Name() string {
	return p.provider.Name()
}

// Model returns the model name.
func (p *CachedProvider) Model() string {
	return p.provider.Model()
}

// Dimensions returns the embedding dimensions.
func (p *CachedProvider) Dimensions() int {
	return p.provider.Dimensions()
}

// Embed generates an embedding with caching.
func (p *CachedProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	hash := TextHash(text)

	// Check cache
	if embedding, ok := p.cache.Get(ctx, hash); ok {
		return embedding, nil
	}

	// Generate embedding
	embedding, err := p.provider.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	// Cache result
	_ = p.cache.Set(ctx, hash, embedding)

	return embedding, nil
}

// EmbedBatch generates embeddings with caching.
func (p *CachedProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	// Calculate hashes
	hashes := make([]string, len(texts))
	for i, text := range texts {
		hashes[i] = TextHash(text)
	}

	// Check cache
	cached, _ := p.cache.GetBatch(ctx, hashes)

	// Find texts that need embedding
	toEmbed := make([]string, 0)
	toEmbedIndices := make([]int, 0)
	for i, hash := range hashes {
		if _, ok := cached[hash]; !ok {
			toEmbed = append(toEmbed, texts[i])
			toEmbedIndices = append(toEmbedIndices, i)
		}
	}

	// Generate missing embeddings
	var newEmbeddings [][]float32
	if len(toEmbed) > 0 {
		var err error
		newEmbeddings, err = p.provider.EmbedBatch(ctx, toEmbed)
		if err != nil {
			return nil, err
		}

		// Cache new embeddings
		toCache := make(map[string][]float32)
		for i, idx := range toEmbedIndices {
			toCache[hashes[idx]] = newEmbeddings[i]
		}
		_ = p.cache.SetBatch(ctx, toCache)
	}

	// Build result
	result := make([][]float32, len(texts))
	newIdx := 0
	for i, hash := range hashes {
		if embedding, ok := cached[hash]; ok {
			result[i] = embedding
		} else {
			result[i] = newEmbeddings[newIdx]
			newIdx++
		}
	}

	return result, nil
}

func stringSliceToInterfaces(values []string) []interface{} {
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}

func mapKeys(values map[string][]float32) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func parseCacheStatsTime(values z.V, key string) time.Time {
	raw, ok := values[key]
	if !ok || raw == nil {
		return time.Time{}
	}
	switch typed := raw.(type) {
	case string:
		return parseEmbeddingCacheTime(typed)
	case []byte:
		return parseEmbeddingCacheTime(string(typed))
	default:
		return time.Time{}
	}
}

func parseEmbeddingCacheTime(raw string) time.Time {
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
