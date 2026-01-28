package cache

import (
	"container/heap"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/timeutil"
)

// LFUCache implements an LFU (Least Frequently Used) cache.
type LFUCache struct {
	config Config

	mu       sync.RWMutex
	items    map[string]*lfuEntry
	freqHeap *lfuHeap

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

// lfuEntry is the internal entry type for the LFU cache.
type lfuEntry struct {
	key       string
	value     interface{}
	freq      int64
	expiresAt time.Time
	index     int // Index in the heap
}

// lfuHeap implements heap.Interface for LFU eviction.
type lfuHeap []*lfuEntry

func (h lfuHeap) Len() int           { return len(h) }
func (h lfuHeap) Less(i, j int) bool { return h[i].freq < h[j].freq }
func (h lfuHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *lfuHeap) Push(x interface{}) {
	n := len(*h)
	entry := x.(*lfuEntry)
	entry.index = n
	*h = append(*h, entry)
}

func (h *lfuHeap) Pop() interface{} {
	old := *h
	n := len(old)
	entry := old[n-1]
	old[n-1] = nil
	entry.index = -1
	*h = old[0 : n-1]
	return entry
}

// NewLFUCache creates a new LFU cache.
func NewLFUCache(config Config) *LFUCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}

	h := make(lfuHeap, 0, config.MaxSize)

	c := &LFUCache{
		config:   config,
		items:    make(map[string]*lfuEntry),
		freqHeap: &h,
		stopCh:   make(chan struct{}),
	}

	heap.Init(c.freqHeap)

	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}

	return c
}

// Get retrieves a value from the cache.
func (c *LFUCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, ErrKeyNotFound
	}

	// Check expiration
	if !entry.expiresAt.IsZero() && timeutil.NowTime().After(entry.expiresAt) {
		c.removeEntry(entry)
		c.misses.Add(1)
		return nil, ErrKeyExpired
	}

	// Increment frequency
	entry.freq++
	heap.Fix(c.freqHeap, entry.index)
	c.hits.Add(1)

	return entry.value, nil
}

// Set stores a value in the cache.
func (c *LFUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.sets.Add(1)

	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = timeutil.NowTime().Add(ttl)
	} else if c.config.DefaultTTL > 0 {
		expiresAt = timeutil.NowTime().Add(c.config.DefaultTTL)
	}

	// Check if key already exists
	if entry, ok := c.items[key]; ok {
		entry.value = value
		entry.expiresAt = expiresAt
		entry.freq++
		heap.Fix(c.freqHeap, entry.index)
		return nil
	}

	// Evict if necessary
	for c.freqHeap.Len() >= c.config.MaxSize {
		c.evictLeastFrequent()
	}

	// Add new entry
	entry := &lfuEntry{
		key:       key,
		value:     value,
		freq:      1,
		expiresAt: expiresAt,
	}
	heap.Push(c.freqHeap, entry)
	c.items[key] = entry

	return nil
}

// Delete removes a key from the cache.
func (c *LFUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.items[key]; ok {
		c.removeEntry(entry)
		c.deletes.Add(1)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *LFUCache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return false
	}

	if !entry.expiresAt.IsZero() && timeutil.NowTime().After(entry.expiresAt) {
		return false
	}

	return true
}

// Clear removes all entries from the cache.
func (c *LFUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*lfuEntry)
	h := make(lfuHeap, 0, c.config.MaxSize)
	c.freqHeap = &h
	heap.Init(c.freqHeap)

	return nil
}

// Stats returns cache statistics.
func (c *LFUCache) Stats() CacheStats {
	c.mu.RLock()
	size := int64(c.freqHeap.Len())
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

// Close closes the cache.
func (c *LFUCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}

	return nil
}

// Len returns the number of items in the cache.
func (c *LFUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.freqHeap.Len()
}

// evictLeastFrequent removes the least frequently used entry.
func (c *LFUCache) evictLeastFrequent() {
	if c.freqHeap.Len() == 0 {
		return
	}

	entry := heap.Pop(c.freqHeap).(*lfuEntry)
	delete(c.items, entry.key)
	c.evictions.Add(1)

	if c.config.OnEvict != nil {
		c.config.OnEvict(entry.key, entry.value)
	}
}

// removeEntry removes an entry from the cache.
func (c *LFUCache) removeEntry(entry *lfuEntry) {
	heap.Remove(c.freqHeap, entry.index)
	delete(c.items, entry.key)

	if c.config.OnEvict != nil {
		c.config.OnEvict(entry.key, entry.value)
	}
}

// cleanupLoop periodically removes expired entries.
func (c *LFUCache) cleanupLoop() {
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
func (c *LFUCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := timeutil.NowTime()
	var toRemove []*lfuEntry

	for _, entry := range c.items {
		if !entry.expiresAt.IsZero() && now.After(entry.expiresAt) {
			toRemove = append(toRemove, entry)
		}
	}

	for _, entry := range toRemove {
		c.removeEntry(entry)
		c.evictions.Add(1)
	}
}

// GetFrequency returns the access frequency of a key.
func (c *LFUCache) GetFrequency(key string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.items[key]
	if !ok {
		return 0, false
	}

	return entry.freq, true
}
