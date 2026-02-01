// Package cache provides caching implementations using ecache.
package cache

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/orca-zhang/ecache"
	ecache2 "github.com/orca-zhang/ecache2"
	"github.com/orca-zhang/ecache2/stats"
)

// ECache wraps ecache.Cache with the Cache interface.
type ECache struct {
	cache  *ecache.Cache
	config Config

	// Statistics
	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64
}

// NewECache creates a new ecache-based cache.
// bucketCount: number of buckets (shards) for concurrent access
// bucketSize: maximum items per bucket
func NewECache(config Config) *ECache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	// Calculate bucket parameters
	// Use 16 buckets by default for good concurrency
	bucketCount := 16
	bucketSize := config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}

	ttl := config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	c := &ECache{
		cache:  ecache.NewLRUCache(uint16(bucketCount), uint16(bucketSize), ttl),
		config: config,
	}

	return c
}

// Get retrieves a value from the cache.
func (c *ECache) Get(ctx context.Context, key string) (interface{}, error) {
	if v, ok := c.cache.Get(key); ok {
		c.hits.Add(1)
		return v, nil
	}
	c.misses.Add(1)
	return nil, ErrKeyNotFound
}

// Set stores a value in the cache.
// TTL is determined by the DefaultTTL in config at cache creation time.
func (c *ECache) Set(ctx context.Context, key string, value interface{}) error {
	c.cache.Put(key, value)
	c.sets.Add(1)
	return nil
}

// Delete removes a key from the cache.
func (c *ECache) Delete(ctx context.Context, key string) error {
	c.cache.Del(key)
	c.deletes.Add(1)
	return nil
}

// Exists checks if a key exists in the cache.
func (c *ECache) Exists(ctx context.Context, key string) bool {
	_, ok := c.cache.Get(key)
	return ok
}

// Clear removes all entries from the cache.
func (c *ECache) Clear(ctx context.Context) error {
	// ecache doesn't have a clear method, create a new instance
	bucketCount := 16
	bucketSize := c.config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}
	ttl := c.config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.cache = ecache.NewLRUCache(uint16(bucketCount), uint16(bucketSize), ttl)
	return nil
}

// Len returns the number of items in the cache.
func (c *ECache) Len() int {
	// ecache doesn't expose length, estimate from stats
	return int(c.sets.Load() - c.deletes.Load() - c.evictions.Load())
}

// Stats returns cache statistics.
func (c *ECache) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Sets:      c.sets.Load(),
		Deletes:   c.deletes.Load(),
		Evictions: c.evictions.Load(),
		Size:      int64(c.Len()),
		Capacity:  int64(c.config.MaxSize),
		HitRate:   hitRate,
	}
}

// Close closes the cache and releases resources.
func (c *ECache) Close() error {
	// ecache doesn't need explicit cleanup
	return nil
}

// GetOrSet retrieves a value or sets it if not present.
func (c *ECache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error)) (interface{}, error) {
	if v, ok := c.cache.Get(key); ok {
		c.hits.Add(1)
		return v, nil
	}

	c.misses.Add(1)

	value, err := fn()
	if err != nil {
		return nil, err
	}

	c.cache.Put(key, value)
	c.sets.Add(1)

	return value, nil
}

// SetNX sets a value only if the key doesn't exist.
func (c *ECache) SetNX(ctx context.Context, key string, value interface{}) bool {
	if _, ok := c.cache.Get(key); ok {
		return false
	}

	c.cache.Put(key, value)
	c.sets.Add(1)
	return true
}

// Touch updates the access time of a key.
func (c *ECache) Touch(ctx context.Context, key string) bool {
	if v, ok := c.cache.Get(key); ok {
		// Re-put to refresh TTL
		c.cache.Put(key, v)
		return true
	}
	return false
}

// ECache2 wraps ecache.Cache with LRU-2 mode for better hot data protection.
type ECache2 struct {
	cache  *ecache.Cache
	config Config

	// Statistics
	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64
}

// NewECache2 creates a new ecache-based cache with LRU-2 mode.
// LRU-2 mode protects frequently accessed data from being evicted by bulk operations.
func NewECache2(config Config) *ECache2 {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	bucketCount := 16
	bucketSize := config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}

	ttl := config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	c := &ECache2{
		cache:  ecache.NewLRUCache(uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4)),
		config: config,
	}

	return c
}

// Get retrieves a value from the cache.
func (c *ECache2) Get(ctx context.Context, key string) (interface{}, error) {
	if v, ok := c.cache.Get(key); ok {
		c.hits.Add(1)
		return v, nil
	}
	c.misses.Add(1)
	return nil, ErrKeyNotFound
}

// Set stores a value in the cache.
// TTL is determined by the DefaultTTL in config at cache creation time.
func (c *ECache2) Set(ctx context.Context, key string, value interface{}) error {
	c.cache.Put(key, value)
	c.sets.Add(1)
	return nil
}

// Delete removes a key from the cache.
func (c *ECache2) Delete(ctx context.Context, key string) error {
	c.cache.Del(key)
	c.deletes.Add(1)
	return nil
}

// Exists checks if a key exists in the cache.
func (c *ECache2) Exists(ctx context.Context, key string) bool {
	_, ok := c.cache.Get(key)
	return ok
}

// Clear removes all entries from the cache.
func (c *ECache2) Clear(ctx context.Context) error {
	bucketCount := 16
	bucketSize := c.config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}
	ttl := c.config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.cache = ecache.NewLRUCache(uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4))
	return nil
}

// Stats returns cache statistics.
func (c *ECache2) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Sets:      c.sets.Load(),
		Deletes:   c.deletes.Load(),
		Evictions: c.evictions.Load(),
		Capacity:  int64(c.config.MaxSize),
		HitRate:   hitRate,
	}
}

// Close closes the cache and releases resources.
func (c *ECache2) Close() error {
	return nil
}

// GenericCache is a generic cache using ecache2 with type-safe keys.
// K can be string, int, int64, uint64, etc.
type GenericCache[K ecache2.Hashable] struct {
	cache  *ecache2.Cache[K]
	config Config
	pool   string
}

// NewGenericCache creates a new generic cache with LRU-2 mode.
func NewGenericCache[K ecache2.Hashable](config Config) *GenericCache[K] {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	bucketCount := 16
	bucketSize := config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}

	ttl := config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	return &GenericCache[K]{
		cache:  ecache2.NewLRUCache[K](uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4)),
		config: config,
	}
}

// NewGenericCacheWithStats creates a new generic cache with stats tracking.
// Only works with string keys due to stats plugin limitation.
func NewGenericCacheWithStats(config Config, poolName string) *GenericCache[string] {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	bucketCount := 16
	bucketSize := config.MaxSize / bucketCount
	if bucketSize < 10 {
		bucketSize = 10
	}

	ttl := config.DefaultTTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	cache := ecache2.NewLRUCache[string](uint16(bucketCount), uint16(bucketSize), ttl).LRU2(uint16(bucketSize / 4))

	// Bind stats
	stats.Bind(poolName, cache)

	return &GenericCache[string]{
		cache:  cache,
		config: config,
		pool:   poolName,
	}
}

// Get retrieves a value from the cache.
func (c *GenericCache[K]) Get(key K) (interface{}, bool) {
	return c.cache.Get(key)
}

// GetInt64 retrieves an int64 value from the cache.
func (c *GenericCache[K]) GetInt64(key K) (int64, bool) {
	return c.cache.GetInt64(key)
}

// Put stores a value in the cache.
func (c *GenericCache[K]) Put(key K, value interface{}) {
	c.cache.Put(key, value)
}

// PutInt64 stores an int64 value in the cache.
func (c *GenericCache[K]) PutInt64(key K, value int64) {
	c.cache.PutInt64(key, value)
}

// Del removes a key from the cache.
func (c *GenericCache[K]) Del(key K) {
	c.cache.Del(key)
}

// Stats returns cache statistics from the stats plugin.
func (c *GenericCache[K]) Stats() CacheStats {
	if c.pool == "" {
		return CacheStats{Capacity: int64(c.config.MaxSize)}
	}

	v, ok := stats.Stats().Load(c.pool)
	if !ok {
		return CacheStats{Capacity: int64(c.config.MaxSize)}
	}

	node := v.(*stats.StatsNode)
	return CacheStats{
		Hits:     int64(node.GetHit),
		Misses:   int64(node.GetMiss),
		Sets:     int64(node.Added + node.Updated),
		Deletes:  int64(node.DelHit),
		Capacity: int64(c.config.MaxSize),
		HitRate:  node.HitRate() * 100,
	}
}
