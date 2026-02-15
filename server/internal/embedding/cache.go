package embedding

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Cache caches embeddings to avoid redundant API calls.
type Cache struct {
	db         *sql.DB
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
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cache database: %w", err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(1)

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create schema
	if _, err := db.Exec(cacheSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create cache schema: %w", err)
	}

	maxEntries := cfg.MaxEntries
	if maxEntries == 0 {
		maxEntries = 10000
	}

	return &Cache{
		db:         db,
		provider:   cfg.Provider,
		model:      cfg.Model,
		maxEntries: maxEntries,
	}, nil
}

// Get retrieves an embedding from the cache.
func (c *Cache) Get(ctx context.Context, textHash string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var embeddingJSON string
	err := c.db.QueryRowContext(ctx,
		"SELECT embedding FROM embedding_cache WHERE provider = ? AND model = ? AND text_hash = ?",
		c.provider, c.model, textHash,
	).Scan(&embeddingJSON)

	if err == sql.ErrNoRows {
		return nil, false
	}
	if err != nil {
		return nil, false
	}

	// Update accessed_at
	_, _ = c.db.ExecContext(ctx,
		"UPDATE embedding_cache SET accessed_at = ? WHERE provider = ? AND model = ? AND text_hash = ?",
		time.Now(), c.provider, c.model, textHash,
	)

	var embedding []float32
	if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
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

	now := time.Now()
	_, err = c.db.ExecContext(ctx, `
		INSERT INTO embedding_cache (provider, model, text_hash, embedding, dimensions, created_at, accessed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model, text_hash) DO UPDATE SET
			embedding = excluded.embedding,
			accessed_at = excluded.accessed_at
	`, c.provider, c.model, textHash, string(embeddingJSON), len(embedding), now, now)

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

	for _, hash := range textHashes {
		var embeddingJSON string
		err := c.db.QueryRowContext(ctx,
			"SELECT embedding FROM embedding_cache WHERE provider = ? AND model = ? AND text_hash = ?",
			c.provider, c.model, hash,
		).Scan(&embeddingJSON)

		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			continue
		}

		var embedding []float32
		if err := json.Unmarshal([]byte(embeddingJSON), &embedding); err != nil {
			continue
		}

		result[hash] = embedding
	}

	// Update accessed_at for found entries
	if len(result) > 0 {
		now := time.Now()
		for hash := range result {
			_, _ = c.db.ExecContext(ctx,
				"UPDATE embedding_cache SET accessed_at = ? WHERE provider = ? AND model = ? AND text_hash = ?",
				now, c.provider, c.model, hash,
			)
		}
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

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO embedding_cache (provider, model, text_hash, embedding, dimensions, created_at, accessed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider, model, text_hash) DO UPDATE SET
			embedding = excluded.embedding,
			accessed_at = excluded.accessed_at
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for hash, embedding := range embeddings {
		embeddingJSON, err := json.Marshal(embedding)
		if err != nil {
			continue
		}
		_, err = stmt.ExecContext(ctx, c.provider, c.model, hash, string(embeddingJSON), len(embedding), now, now)
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
	var count int
	err := c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM embedding_cache WHERE provider = ? AND model = ?",
		c.provider, c.model,
	).Scan(&count)

	if err != nil || count <= c.maxEntries {
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

	_, err := c.db.ExecContext(ctx, `
		DELETE FROM embedding_cache
		WHERE provider = ? AND model = ? AND id IN (
			SELECT id FROM embedding_cache
			WHERE provider = ? AND model = ?
			ORDER BY accessed_at ASC
			LIMIT (
				SELECT MAX(0, COUNT(*) - ?) FROM embedding_cache
				WHERE provider = ? AND model = ?
			)
		)
	`, c.provider, c.model, c.provider, c.model, targetCount, c.provider, c.model)

	return err
}

// Close closes the cache database.
func (c *Cache) Close() error {
	return c.db.Close()
}

// Stats returns cache statistics.
func (c *Cache) Stats(ctx context.Context) (CacheStats, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var stats CacheStats

	err := c.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM embedding_cache WHERE provider = ? AND model = ?",
		c.provider, c.model,
	).Scan(&stats.EntryCount)
	if err != nil {
		return stats, err
	}

	err = c.db.QueryRowContext(ctx,
		"SELECT MIN(created_at), MAX(accessed_at) FROM embedding_cache WHERE provider = ? AND model = ?",
		c.provider, c.model,
	).Scan(&stats.OldestEntry, &stats.NewestAccess)
	if err != nil && err != sql.ErrNoRows {
		return stats, err
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
