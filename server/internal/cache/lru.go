package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// LRUCache implements an LRU (Least Recently Used) cache.
type LRUCache struct {
	config Config

	mu       sync.RWMutex
	items    map[string]*list.Element
	evictList *list.List

	// Statistics
	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64

	// Cleanup
	stopCh  chan struct{}
	stopped bool
}

// lruEntry is the internal entry type for the LRU list.
type lruEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
	size      int64
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache(config Config) *LRUCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	c := &LRUCache{
		config:    config,
		items:     make(map[string]*list.Element),
		evictList: list.New(),
		stopCh:    make(chan struct{}),
	}

	// Start cleanup goroutine if interval is set
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}

	return c
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, ErrKeyNotFound
	}

	entry := elem.Value.(*lruEntry)

	// Check expiration
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		c.misses.Add(1)
		return nil, ErrKeyExpired
	}

	// Move to front (most recently used)
	c.evictList.MoveToFront(elem)
	c.hits.Add(1)

	return entry.value, nil
}

// Set stores a value in the cache.
func (c *LRUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sets.Add(1)

	// Calculate expiration
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	} else if c.config.DefaultTTL > 0 {
		expiresAt = time.Now().Add(c.config.DefaultTTL)
	}

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		c.evictList.MoveToFront(elem)
		entry := elem.Value.(*lruEntry)
		entry.value = value
		entry.expiresAt = expiresAt
		return nil
	}

	// Evict if necessary
	for c.evictList.Len() >= c.config.MaxSize {
		c.evictOldest()
	}

	// Add new entry
	entry := &lruEntry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem

	return nil
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
		c.deletes.Add(1)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *LRUCache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	entry := elem.Value.(*lruEntry)
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		return false
	}

	return true
}

// Clear removes all entries from the cache.
func (c *LRUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*list.Element)
	c.evictList.Init()

	return nil
}

// Stats returns cache statistics.
func (c *LRUCache) Stats() CacheStats {
	c.mu.RLock()
	size := int64(c.evictList.Len())
	c.mu.RUnlock()

	hits := c.hits.Load()
	misses := c.misses.Load()
	total := hits + misses

	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	return CacheStats{
		Hits:      hits,
		Misses:    misses,
		Sets:      c.sets.Load(),
		Deletes:   c.deletes.Load(),
		Evictions: c.evictions.Load(),
		Size:      size,
		Capacity:  int64(c.config.MaxSize),
		HitRate:   hitRate,
	}
}

// Close closes the cache and stops the cleanup goroutine.
func (c *LRUCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}

	return nil
}

// Len returns the number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.evictList.Len()
}

// Keys returns all keys in the cache.
func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	keys := make([]string, 0, c.evictList.Len())
	for key := range c.items {
		keys = append(keys, key)
	}
	return keys
}

// evictOldest removes the oldest entry from the cache.
func (c *LRUCache) evictOldest() {
	elem := c.evictList.Back()
	if elem != nil {
		c.removeElement(elem)
		c.evictions.Add(1)
	}
}

// removeElement removes an element from the cache.
func (c *LRUCache) removeElement(elem *list.Element) {
	c.evictList.Remove(elem)
	entry := elem.Value.(*lruEntry)
	delete(c.items, entry.key)

	if c.config.OnEvict != nil {
		c.config.OnEvict(entry.key, entry.value)
	}
}

// cleanupLoop periodically removes expired entries.
func (c *LRUCache) cleanupLoop() {
	ticker := time.NewTicker(c.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup removes expired entries.
func (c *LRUCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	var next *list.Element

	for elem := c.evictList.Back(); elem != nil; elem = next {
		next = elem.Prev()
		entry := elem.Value.(*lruEntry)

		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			c.removeElement(elem)
			c.evictions.Add(1)
		}
	}
}

// GetOrSet retrieves a value or sets it if not found.
func (c *LRUCache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
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

// SetNX sets a value only if the key does not exist.
func (c *LRUCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.items[key]; ok {
		return false, nil
	}

	c.sets.Add(1)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	} else if c.config.DefaultTTL > 0 {
		expiresAt = time.Now().Add(c.config.DefaultTTL)
	}

	for c.evictList.Len() >= c.config.MaxSize {
		c.evictOldest()
	}

	entry := &lruEntry{
		key:       key,
		value:     value,
		expiresAt: expiresAt,
	}
	elem := c.evictList.PushFront(entry)
	c.items[key] = elem

	return true, nil
}

// Touch updates the access time of a key without retrieving the value.
func (c *LRUCache) Touch(ctx context.Context, key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return false
	}

	entry := elem.Value.(*lruEntry)
	if !entry.expiresAt.IsZero() && time.Now().After(entry.expiresAt) {
		c.removeElement(elem)
		return false
	}

	c.evictList.MoveToFront(elem)
	return true
}
