package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// LRUCache implements Cache with least-recently-used eviction.
type LRUCache struct {
	config Config

	mu    sync.RWMutex
	items map[string]*list.Element
	order *list.List // front = most recent

	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64

	stopCh  chan struct{}
	stopped bool
}

type lruEntry struct {
	key        string
	value      interface{}
	expiresAt  int64
	accessedAt int64
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache(config Config) *LRUCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	c := &LRUCache{
		config: config,
		items:  make(map[string]*list.Element),
		order:  list.New(),
		stopCh: make(chan struct{}),
	}
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}
	return c
}

func (c *LRUCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	el, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, ErrKeyNotFound
	}
	e := el.Value.(*lruEntry)
	now := time.Now().UnixNano()
	if e.expiresAt != 0 && now >= e.expiresAt {
		c.removeLocked(el)
		c.misses.Add(1)
		return nil, ErrKeyExpired
	}
	e.accessedAt = now
	c.order.MoveToFront(el)
	c.hits.Add(1)
	return e.value, nil
}

func (c *LRUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	var expiresAt int64
	if ttl > 0 {
		expiresAt = now + int64(ttl)
	}

	if el, ok := c.items[key]; ok {
		e := el.Value.(*lruEntry)
		e.value = value
		e.expiresAt = expiresAt
		e.accessedAt = now
		c.order.MoveToFront(el)
		c.sets.Add(1)
		return nil
	}

	// Evict if full
	for c.order.Len() >= c.config.MaxSize {
		c.evictLocked()
	}

	e := &lruEntry{key: key, value: value, expiresAt: expiresAt, accessedAt: now}
	el := c.order.PushFront(e)
	c.items[key] = el
	c.sets.Add(1)
	return nil
}

func (c *LRUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		c.removeLocked(el)
		c.deletes.Add(1)
	}
	return nil
}

func (c *LRUCache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	e := el.Value.(*lruEntry)
	return e.expiresAt == 0 || time.Now().UnixNano() < e.expiresAt
}

func (c *LRUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
	return nil
}

func (c *LRUCache) Stats() CacheStats {
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

func (c *LRUCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}
	return nil
}

func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

func (c *LRUCache) Keys() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	keys := make([]string, 0, c.order.Len())
	for el := c.order.Front(); el != nil; el = el.Next() {
		keys = append(keys, el.Value.(*lruEntry).key)
	}
	return keys
}

func (c *LRUCache) GetOrSet(ctx context.Context, key string, fn func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	if el, ok := c.items[key]; ok {
		e := el.Value.(*lruEntry)
		if e.expiresAt == 0 || now < e.expiresAt {
			e.accessedAt = now
			c.order.MoveToFront(el)
			c.hits.Add(1)
			return e.value, nil
		}
		c.removeLocked(el)
	}
	c.misses.Add(1)

	value, err := fn()
	if err != nil {
		return nil, err
	}

	var expiresAt int64
	if ttl > 0 {
		expiresAt = now + int64(ttl)
	}
	for c.order.Len() >= c.config.MaxSize {
		c.evictLocked()
	}
	e := &lruEntry{key: key, value: value, expiresAt: expiresAt, accessedAt: now}
	el := c.order.PushFront(e)
	c.items[key] = el
	c.sets.Add(1)
	return value, nil
}

func (c *LRUCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()
	if el, ok := c.items[key]; ok {
		e := el.Value.(*lruEntry)
		if e.expiresAt == 0 || now < e.expiresAt {
			return false, nil
		}
		c.removeLocked(el)
	}

	var expiresAt int64
	if ttl > 0 {
		expiresAt = now + int64(ttl)
	}
	for c.order.Len() >= c.config.MaxSize {
		c.evictLocked()
	}
	e := &lruEntry{key: key, value: value, expiresAt: expiresAt, accessedAt: now}
	el := c.order.PushFront(e)
	c.items[key] = el
	c.sets.Add(1)
	return true, nil
}

func (c *LRUCache) Touch(ctx context.Context, key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return false
	}
	e := el.Value.(*lruEntry)
	now := time.Now().UnixNano()
	if e.expiresAt != 0 && now >= e.expiresAt {
		return false
	}
	e.accessedAt = now
	c.order.MoveToFront(el)
	return true
}

func (c *LRUCache) evictLocked() {
	el := c.order.Back()
	if el == nil {
		return
	}
	c.removeLocked(el)
	c.evictions.Add(1)
	e := el.Value.(*lruEntry)
	if c.config.OnEvict != nil {
		c.config.OnEvict(e.key, e.value)
	}
}

func (c *LRUCache) removeLocked(el *list.Element) {
	e := el.Value.(*lruEntry)
	delete(c.items, e.key)
	c.order.Remove(el)
}

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

func (c *LRUCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UnixNano()
	var toRemove []*list.Element
	for el := c.order.Front(); el != nil; el = el.Next() {
		e := el.Value.(*lruEntry)
		if e.expiresAt != 0 && now >= e.expiresAt {
			toRemove = append(toRemove, el)
		}
	}
	for _, el := range toRemove {
		c.removeLocked(el)
	}
}
