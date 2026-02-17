package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MultiLevelStats holds stats for a multi-level cache.
type MultiLevelStats struct {
	L1Hits     int64
	L2Hits     int64
	Misses     int64
	Sets       int64
	Deletes    int64
	Promotions int64
}

// MultiLevelCache combines L1 (memory) and L2 (disk) caches.
type MultiLevelCache struct {
	config MultiLevelConfig
	l1     *LRUCache
	l2     *DiskCache

	l1Hits     atomic.Int64
	l2Hits     atomic.Int64
	misses     atomic.Int64
	sets       atomic.Int64
	deletes    atomic.Int64
	promotions atomic.Int64
}

// NewMultiLevelCache creates a new multi-level cache.
func NewMultiLevelCache(config MultiLevelConfig) (*MultiLevelCache, error) {
	l1 := NewLRUCache(config.L1.Config)
	l2, err := NewDiskCache(config.L2)
	if err != nil {
		l1.Close()
		return nil, err
	}
	return &MultiLevelCache{config: config, l1: l1, l2: l2}, nil
}

func (c *MultiLevelCache) Get(ctx context.Context, key string) (interface{}, error) {
	v, err := c.l1.Get(ctx, key)
	if err == nil {
		c.l1Hits.Add(1)
		return v, nil
	}

	v, err = c.l2.Get(ctx, key)
	if err == nil {
		c.l2Hits.Add(1)
		if c.config.PromoteOnHit {
			c.l1.Set(ctx, key, v, 0)
			c.promotions.Add(1)
		}
		return v, nil
	}

	c.misses.Add(1)
	return nil, err
}

func (c *MultiLevelCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.sets.Add(1)
	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	if c.config.WriteThrough {
		return c.l2.Set(ctx, key, value, ttl)
	}
	return nil
}

func (c *MultiLevelCache) Delete(ctx context.Context, key string) error {
	c.deletes.Add(1)
	c.l1.Delete(ctx, key)
	c.l2.Delete(ctx, key)
	return nil
}

func (c *MultiLevelCache) Exists(ctx context.Context, key string) bool {
	return c.l1.Exists(ctx, key) || c.l2.Exists(ctx, key)
}

func (c *MultiLevelCache) Clear(ctx context.Context) error {
	c.l1.Clear(ctx)
	c.l2.Clear(ctx)
	return nil
}

func (c *MultiLevelCache) Stats() MultiLevelStats {
	return MultiLevelStats{
		L1Hits:     c.l1Hits.Load(),
		L2Hits:     c.l2Hits.Load(),
		Misses:     c.misses.Load(),
		Sets:       c.sets.Load(),
		Deletes:    c.deletes.Load(),
		Promotions: c.promotions.Load(),
	}
}

func (c *MultiLevelCache) Close() error {
	c.l1.Close()
	c.l2.Close()
	return nil
}

// SetL1Only sets a value only in L1.
func (c *MultiLevelCache) SetL1Only(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.l1.Set(ctx, key, value, ttl)
}

// SetL2Only sets a value only in L2.
func (c *MultiLevelCache) SetL2Only(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return c.l2.Set(ctx, key, value, ttl)
}

// L1 returns the L1 cache.
func (c *MultiLevelCache) L1() *LRUCache { return c.l1 }

// L2 returns the L2 cache.
func (c *MultiLevelCache) L2() *DiskCache { return c.l2 }

// Demote moves a key from L1 to L2.
func (c *MultiLevelCache) Demote(ctx context.Context, key string) error {
	v, err := c.l1.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := c.l2.Set(ctx, key, v, 0); err != nil {
		return err
	}
	c.l1.Delete(ctx, key)
	return nil
}

// GetOrSet retrieves a value or generates and stores it.
func (c *MultiLevelCache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	v, err := c.Get(ctx, key)
	if err == nil {
		return v, nil
	}
	v, err = fn()
	if err != nil {
		return nil, err
	}
	c.Set(ctx, key, v, ttl)
	return v, nil
}

// CacheManager manages named cache instances.
type CacheManager struct {
	mu     sync.RWMutex
	caches map[string]Cache
}

// NewCacheManager creates a new cache manager.
func NewCacheManager() *CacheManager {
	return &CacheManager{caches: make(map[string]Cache)}
}

func (m *CacheManager) Register(name string, c Cache) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.caches[name] = c
}

func (m *CacheManager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.caches, name)
}

func (m *CacheManager) Get(name string) (Cache, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.caches[name]
	return c, ok
}

func (m *CacheManager) Stats() map[string]CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := make(map[string]CacheStats, len(m.caches))
	for name, c := range m.caches {
		stats[name] = c.Stats()
	}
	return stats
}

func (m *CacheManager) ClearAll(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, c := range m.caches {
		c.Clear(ctx)
	}
}

func (m *CacheManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.caches {
		c.Close()
	}
}
