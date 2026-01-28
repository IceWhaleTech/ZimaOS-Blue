package claudecode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// CacheKeyStrategy defines how cache keys are generated.
type CacheKeyStrategy string

const (
	// KeyByPromptHash generates key from prompt hash only.
	KeyByPromptHash CacheKeyStrategy = "prompt_hash"
	// KeyByPromptAndModel generates key from prompt + model.
	KeyByPromptAndModel CacheKeyStrategy = "prompt_and_model"
	// KeyByFullRequest generates key from full request parameters.
	KeyByFullRequest CacheKeyStrategy = "full_request"
)

// CacheConfig contains configuration for response caching.
type CacheConfig struct {
	// Enabled indicates whether caching is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// MaxSize is the maximum number of cached entries.
	MaxSize int `json:"max_size" yaml:"max_size"`
	// TTL is the time-to-live for cached entries.
	TTL time.Duration `json:"ttl" yaml:"ttl"`
	// KeyStrategy is the strategy for generating cache keys.
	KeyStrategy CacheKeyStrategy `json:"key_strategy" yaml:"key_strategy"`
}

// DefaultCacheConfig returns the default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled:     false,
		MaxSize:     1000,
		TTL:         1 * time.Hour,
		KeyStrategy: KeyByPromptAndModel,
	}
}

// CacheEntry represents a cached response.
type CacheEntry struct {
	// Key is the cache key.
	Key string `json:"key"`
	// Result is the cached run result.
	Result *RunResult `json:"result"`
	// CreatedAt is when the entry was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the entry expires.
	ExpiresAt time.Time `json:"expires_at"`
	// HitCount is the number of times this entry was accessed.
	HitCount int64 `json:"hit_count"`
}

// IsExpired returns true if the entry has expired.
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

// CacheStats contains cache statistics.
type CacheStats struct {
	// Size is the current number of entries.
	Size int `json:"size"`
	// MaxSize is the maximum number of entries.
	MaxSize int `json:"max_size"`
	// Hits is the total number of cache hits.
	Hits int64 `json:"hits"`
	// Misses is the total number of cache misses.
	Misses int64 `json:"misses"`
	// HitRate is the cache hit rate (0-1).
	HitRate float64 `json:"hit_rate"`
	// Evictions is the total number of evictions.
	Evictions int64 `json:"evictions"`
	// Expirations is the total number of expirations.
	Expirations int64 `json:"expirations"`
}

// Cache defines the interface for response caching.
type Cache interface {
	// Get retrieves a cached result by key.
	Get(key string) (*RunResult, bool)
	// Set stores a result in the cache.
	Set(key string, result *RunResult)
	// Delete removes an entry from the cache.
	Delete(key string)
	// Clear removes all entries from the cache.
	Clear()
	// Stats returns cache statistics.
	Stats() CacheStats
}

// MemoryCache implements an in-memory LRU cache.
type MemoryCache struct {
	config  CacheConfig
	entries map[string]*CacheEntry
	order   []string // LRU order (oldest first)
	stats   CacheStats
	mu      sync.RWMutex
}

// NewMemoryCache creates a new in-memory cache.
func NewMemoryCache(config CacheConfig) *MemoryCache {
	return &MemoryCache{
		config:  config,
		entries: make(map[string]*CacheEntry),
		order:   make([]string, 0, config.MaxSize),
		stats:   CacheStats{MaxSize: config.MaxSize},
	}
}

// Get retrieves a cached result by key.
func (c *MemoryCache) Get(key string) (*RunResult, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		c.stats.Misses++
		return nil, false
	}

	// Check expiration
	if entry.IsExpired() {
		c.deleteEntry(key)
		c.stats.Misses++
		c.stats.Expirations++
		return nil, false
	}

	// Update hit count and LRU order
	entry.HitCount++
	c.stats.Hits++
	c.moveToEnd(key)

	return entry.Result, true
}

// Set stores a result in the cache.
func (c *MemoryCache) Set(key string, result *RunResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if entry already exists
	if _, exists := c.entries[key]; exists {
		c.deleteEntry(key)
	}

	// Evict oldest entries if at capacity
	for len(c.entries) >= c.config.MaxSize && len(c.order) > 0 {
		oldestKey := c.order[0]
		c.deleteEntry(oldestKey)
		c.stats.Evictions++
	}

	// Create new entry
	now := time.Now()
	entry := &CacheEntry{
		Key:       key,
		Result:    result,
		CreatedAt: now,
		ExpiresAt: now.Add(c.config.TTL),
		HitCount:  0,
	}

	c.entries[key] = entry
	c.order = append(c.order, key)
	c.stats.Size = len(c.entries)
}

// Delete removes an entry from the cache.
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deleteEntry(key)
}

// Clear removes all entries from the cache.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.order = make([]string, 0, c.config.MaxSize)
	c.stats.Size = 0
}

// Stats returns cache statistics.
func (c *MemoryCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := c.stats
	total := stats.Hits + stats.Misses
	if total > 0 {
		stats.HitRate = float64(stats.Hits) / float64(total)
	}
	return stats
}

// deleteEntry removes an entry without locking.
func (c *MemoryCache) deleteEntry(key string) {
	delete(c.entries, key)
	c.removeFromOrder(key)
	c.stats.Size = len(c.entries)
}

// removeFromOrder removes a key from the LRU order.
func (c *MemoryCache) removeFromOrder(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			return
		}
	}
}

// moveToEnd moves a key to the end of the LRU order.
func (c *MemoryCache) moveToEnd(key string) {
	c.removeFromOrder(key)
	c.order = append(c.order, key)
}

// GenerateCacheKey generates a cache key for the given parameters.
func GenerateCacheKey(params *RunParams, strategy CacheKeyStrategy) string {
	var data string

	switch strategy {
	case KeyByPromptHash:
		data = params.Prompt
	case KeyByPromptAndModel:
		data = params.Prompt + "|" + params.Model
	case KeyByFullRequest:
		// Serialize relevant parameters
		keyData := struct {
			Prompt       string `json:"prompt"`
			Model        string `json:"model"`
			SystemPrompt string `json:"system_prompt,omitempty"`
		}{
			Prompt:       params.Prompt,
			Model:        params.Model,
			SystemPrompt: params.SystemPrompt,
		}
		jsonData, _ := json.Marshal(keyData)
		data = string(jsonData)
	default:
		data = params.Prompt + "|" + params.Model
	}

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes (32 hex chars)
}

// CachedRunner wraps a Runner with caching support.
type CachedRunner struct {
	runner *Runner
	cache  Cache
	config CacheConfig
}

// NewCachedRunner creates a new CachedRunner.
func NewCachedRunner(runner *Runner, config CacheConfig) *CachedRunner {
	return &CachedRunner{
		runner: runner,
		cache:  NewMemoryCache(config),
		config: config,
	}
}

// Run executes a CLI command with caching.
func (r *CachedRunner) Run(ctx context.Context, params *RunParams) (*RunResult, error) {
	if !r.config.Enabled {
		return r.runner.Run(ctx, params)
	}

	// Generate cache key
	key := GenerateCacheKey(params, r.config.KeyStrategy)

	// Check cache
	if result, found := r.cache.Get(key); found {
		return result, nil
	}

	// Execute and cache result
	result, err := r.runner.Run(ctx, params)
	if err != nil {
		return nil, err
	}

	r.cache.Set(key, result)
	return result, nil
}

// ClearCache clears the cache.
func (r *CachedRunner) ClearCache() {
	r.cache.Clear()
}

// CacheStats returns cache statistics.
func (r *CachedRunner) CacheStats() CacheStats {
	return r.cache.Stats()
}
