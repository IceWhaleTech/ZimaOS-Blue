// Package cache provides caching implementations using ecache.
package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/orca-zhang/ecache"
)

const (
	genericCacheMaxBucketCount = 16
	genericCacheTargetLoad     = 50
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

type genericCacheKey interface {
	comparable
}

type genericCacheEntry[K genericCacheKey] struct {
	key       K
	value     interface{}
	expiresAt int64
}

// GenericCache is a lightweight generic cache with type-safe keys.
// K can be string, int, int64, uint64, etc.
type GenericCache[K genericCacheKey] struct {
	config Config
	pool   string

	mu    sync.Mutex
	items map[K]*list.Element
	order *list.List

	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64

	stopCh chan struct{}
	closed bool
}

func computeGenericCacheLayout(maxSize int) (bucketCount, bucketSize, secondLevelSize uint16) {
	if maxSize <= 0 {
		maxSize = 1000
	}

	shards := (maxSize + genericCacheTargetLoad - 1) / genericCacheTargetLoad
	if shards < 1 {
		shards = 1
	}
	if shards > genericCacheMaxBucketCount {
		shards = genericCacheMaxBucketCount
	}

	perBucket := (maxSize + shards - 1) / shards
	if perBucket < 1 {
		perBucket = 1
	}

	secondLevel := perBucket / 4
	if secondLevel < 1 {
		secondLevel = 1
	}

	return uint16(shards), uint16(perBucket), uint16(secondLevel)
}

// NewGenericCache creates a new generic cache.
func NewGenericCache[K genericCacheKey](config Config) *GenericCache[K] {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	return &GenericCache[K]{
		config: config,
	}
}

// NewGenericCacheWithStats creates a new generic cache with local stats tracking.
func NewGenericCacheWithStats(config Config, poolName string) *GenericCache[string] {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	return &GenericCache[string]{
		config: config,
		pool:   poolName,
	}
}

// Get retrieves a value from the cache.
func (c *GenericCache[K]) Get(key K) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.items == nil {
		c.misses.Add(1)
		return nil, false
	}

	el, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, false
	}

	entry := el.Value.(*genericCacheEntry[K])
	if entry.expiresAt != 0 && timeutil.NowNano() > entry.expiresAt {
		c.removeLocked(el)
		c.misses.Add(1)
		return nil, false
	}

	c.order.MoveToFront(el)
	c.hits.Add(1)
	return entry.value, true
}

// GetInt64 retrieves an int64 value from the cache.
func (c *GenericCache[K]) GetInt64(key K) (int64, bool) {
	v, ok := c.Get(key)
	if !ok {
		return 0, false
	}
	i, ok := v.(int64)
	return i, ok
}

// Put stores a value in the cache.
func (c *GenericCache[K]) Put(key K, value interface{}) {
	c.putValue(key, value)
}

// PutInt64 stores an int64 value in the cache.
func (c *GenericCache[K]) PutInt64(key K, value int64) {
	c.putValue(key, value)
}

// Del removes a key from the cache.
func (c *GenericCache[K]) Del(key K) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.items == nil {
		return
	}
	el, ok := c.items[key]
	if !ok {
		return
	}
	c.removeLocked(el)
	c.deletes.Add(1)
}

// Stats returns local cache statistics.
func (c *GenericCache[K]) Stats() CacheStats {
	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	var hitRate float64
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	c.mu.Lock()
	size := int64(len(c.items))
	c.mu.Unlock()

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Sets:      c.sets.Load(),
		Deletes:   c.deletes.Load(),
		Evictions: c.evictions.Load(),
		Size:      size,
		Capacity: int64(c.config.MaxSize),
		HitRate:  hitRate,
	}
}

func (c *GenericCache[K]) putValue(key K, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.ensureStorageLocked()
	c.startCleanupLoopLocked()

	expiresAt := c.expiryForWrite()
	if el, ok := c.items[key]; ok {
		entry := el.Value.(*genericCacheEntry[K])
		entry.value = value
		entry.expiresAt = expiresAt
		c.order.MoveToFront(el)
		c.sets.Add(1)
		return
	}

	c.cleanupExpiredLocked(timeutil.NowNano())
	for len(c.items) >= c.config.MaxSize {
		c.evictLocked()
	}

	el := c.order.PushFront(&genericCacheEntry[K]{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	})
	c.items[key] = el
	c.sets.Add(1)
}

func (c *GenericCache[K]) ensureStorageLocked() {
	if c.items == nil {
		c.items = make(map[K]*list.Element)
	}
	if c.order == nil {
		c.order = list.New()
	}
}

func (c *GenericCache[K]) startCleanupLoopLocked() {
	if c.config.CleanupInterval <= 0 || c.stopCh != nil || c.closed {
		return
	}
	c.stopCh = make(chan struct{})
	go c.cleanupLoop()
}

func (c *GenericCache[K]) expiryForWrite() int64 {
	ttl := c.config.DefaultTTL
	if ttl <= 0 {
		return 0
	}
	return timeutil.NowNano() + int64(ttl)
}

func (c *GenericCache[K]) evictLocked() {
	if c.order == nil {
		return
	}
	el := c.order.Back()
	if el == nil {
		return
	}
	c.removeLocked(el)
	c.evictions.Add(1)
}

func (c *GenericCache[K]) removeLocked(el *list.Element) {
	entry := el.Value.(*genericCacheEntry[K])
	delete(c.items, entry.key)
	c.order.Remove(el)
}

func (c *GenericCache[K]) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			c.cleanupExpiredLocked(timeutil.NowNano())
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

func (c *GenericCache[K]) cleanupExpiredLocked(now int64) {
	if c.order == nil {
		return
	}
	for el := c.order.Back(); el != nil; {
		prev := el.Prev()
		entry := el.Value.(*genericCacheEntry[K])
		if entry.expiresAt != 0 && now > entry.expiresAt {
			c.removeLocked(el)
		}
		el = prev
	}
}
