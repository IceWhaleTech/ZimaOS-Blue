package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MultiLevelCache implements a multi-level cache with L1 (memory) and L2 (disk).
type MultiLevelCache struct {
	config MultiLevelConfig
	l1     *LRUCache
	l2     *DiskCache

	// Statistics
	l1Hits    atomic.Int64
	l2Hits    atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	promotions atomic.Int64

	mu sync.RWMutex
}

// NewMultiLevelCache creates a new multi-level cache.
func NewMultiLevelCache(config MultiLevelConfig) (*MultiLevelCache, error) {
	// Create L1 cache
	l1Config := Config{
		MaxSize:         config.L1.MaxSize,
		MaxMemory:       config.L1.MaxMemory,
		DefaultTTL:      config.L1.DefaultTTL,
		EvictionPolicy:  config.L1.EvictionPolicy,
		CleanupInterval: config.L1.CleanupInterval,
	}
	l1 := NewLRUCache(l1Config)

	// Create L2 cache
	l2, err := NewDiskCache(config.L2)
	if err != nil {
		l1.Close()
		return nil, err
	}

	return &MultiLevelCache{
		config: config,
		l1:     l1,
		l2:     l2,
	}, nil
}

// Get retrieves a value from the cache, checking L1 first, then L2.
func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, error) {
	// Try L1 first
	value, err := c.l1.Get(ctx, key)
	if err == nil {
		c.l1Hits.Add(1)
		return value, nil
	}

	// Try L2
	value, err = c.l2.Get(ctx, key)
	if err == nil {
		c.l2Hits.Add(1)

		// Promote to L1 if configured
		if c.config.PromoteOnHit {
			c.promote(ctx, key, value)
		}

		return value, nil
	}

	c.misses.Add(1)
	return nil, err
}

// Set stores a value in the cache.
func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.sets.Add(1)

	// Always write to L1
	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	// Write to L2 if write-through is enabled
	if c.config.WriteThrough {
		if err := c.l2.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}

	return nil
}

// SetL1Only stores a value only in L1 cache.
func (c *MultiLevelCache) SetL1Only(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.l1.Set(ctx, key, value, ttl)
}

// SetL2Only stores a value only in L2 cache.
func (c *MultiLevelCache) SetL2Only(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.l2.Set(ctx, key, value, ttl)
}

// Delete removes a key from both cache levels.
func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	c.deletes.Add(1)

	c.l1.Delete(ctx, key)
	c.l2.Delete(ctx, key)

	return nil
}

// Exists checks if a key exists in either cache level.
func (c *MultiLevelCache) Exists(ctx context.Context, key string) bool {
	if c.l1.Exists(ctx, key) {
		return true
	}
	return c.l2.Exists(ctx, key)
}

// Clear removes all entries from both cache levels.
func (c *MultiLevelCache) Clear(ctx context.Context) error {
	c.l1.Clear(ctx)
	c.l2.Clear(ctx)
	return nil
}

// Stats returns combined cache statistics.
func (c *MultiLevelCache) Stats() MultiLevelCacheStats {
	l1Stats := c.l1.Stats()
	l2Stats := c.l2.Stats()

	l1Hits := c.l1Hits.Load()
	l2Hits := c.l2Hits.Load()
	misses := c.misses.Load()
	total := l1Hits + l2Hits + misses

	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(l1Hits+l2Hits) / float64(total) * 100
	}

	return MultiLevelCacheStats{
		L1Stats:    l1Stats,
		L2Stats:    l2Stats,
		L1Hits:     l1Hits,
		L2Hits:     l2Hits,
		Misses:     misses,
		Sets:       c.sets.Load(),
		Deletes:    c.deletes.Load(),
		Promotions: c.promotions.Load(),
		HitRate:    hitRate,
	}
}

// Close closes both cache levels.
func (c *MultiLevelCache) Close() error {
	c.l1.Close()
	c.l2.Close()
	return nil
}

// promote moves a value from L2 to L1.
func (c *MultiLevelCache) promote(ctx context.Context, key string, value interface{}) {
	c.promotions.Add(1)
	c.l1.Set(ctx, key, value, 0) // Use default TTL
}

// Demote moves a value from L1 to L2.
func (c *MultiLevelCache) Demote(ctx context.Context, key string) error {
	value, err := c.l1.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := c.l2.Set(ctx, key, value, 0); err != nil {
		return err
	}

	c.l1.Delete(ctx, key)
	return nil
}

// GetOrSet retrieves a value or sets it if not found.
func (c *MultiLevelCache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get first
	if value, err := c.Get(ctx, key); err == nil {
		return value, nil
	}

	// Generate value
	value, err := fn()
	if err != nil {
		return nil, err
	}

	// Set the value
	if err := c.Set(ctx, key, value, ttl); err != nil {
		return nil, err
	}

	return value, nil
}

// L1 returns the L1 cache for direct access.
func (c *MultiLevelCache) L1() *LRUCache {
	return c.l1
}

// L2 returns the L2 cache for direct access.
func (c *MultiLevelCache) L2() *DiskCache {
	return c.l2
}

// MultiLevelCacheStats holds statistics for the multi-level cache.
type MultiLevelCacheStats struct {
	L1Stats    CacheStats `json:"l1_stats"`
	L2Stats    CacheStats `json:"l2_stats"`
	L1Hits     int64      `json:"l1_hits"`
	L2Hits     int64      `json:"l2_hits"`
	Misses     int64      `json:"misses"`
	Sets       int64      `json:"sets"`
	Deletes    int64      `json:"deletes"`
	Promotions int64      `json:"promotions"`
	HitRate    float64    `json:"hit_rate"`
}

// CacheManager provides a unified interface for cache management.
type CacheManager struct {
	caches map[string]Cache
	mu     sync.RWMutex
}

// NewCacheManager creates a new cache manager.
func NewCacheManager() *CacheManager {
	return &CacheManager{
		caches: make(map[string]Cache),
	}
}

// Register registers a cache with a name.
func (m *CacheManager) Register(name string, cache Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caches[name] = cache
}

// Get retrieves a cache by name.
func (m *CacheManager) Get(name string) (Cache, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cache, ok := m.caches[name]
	return cache, ok
}

// Unregister removes a cache by name.
func (m *CacheManager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cache, ok := m.caches[name]; ok {
		cache.Close()
		delete(m.caches, name)
	}
}

// ClearAll clears all registered caches.
func (m *CacheManager) ClearAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, cache := range m.caches {
		cache.Clear(ctx)
	}
}

// Stats returns statistics for all registered caches.
func (m *CacheManager) Stats() map[string]CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[string]CacheStats)
	for name, cache := range m.caches {
		stats[name] = cache.Stats()
	}
	return stats
}

// Close closes all registered caches.
func (m *CacheManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cache := range m.caches {
		cache.Close()
	}
	m.caches = make(map[string]Cache)
}
