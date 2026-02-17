package cache

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// LFUCache implements Cache with least-frequently-used eviction.
type LFUCache struct {
	config Config

	mu       sync.RWMutex
	items    map[string]*lfuItem
	freqList map[int64]*list.List // freq -> doubly-linked list of keys
	minFreq  int64

	hits      atomic.Int64
	misses    atomic.Int64
	sets      atomic.Int64
	deletes   atomic.Int64
	evictions atomic.Int64

	stopCh  chan struct{}
	stopped bool
}

type lfuItem struct {
	key       string
	value     interface{}
	freq      int64
	expiresAt int64
	element   *list.Element
}

// NewLFUCache creates a new LFU cache.
func NewLFUCache(config Config) *LFUCache {
	if config.MaxSize <= 0 {
		config.MaxSize = 1000
	}
	c := &LFUCache{
		config:   config,
		items:    make(map[string]*lfuItem),
		freqList: make(map[int64]*list.List),
		stopCh:   make(chan struct{}),
	}
	if config.CleanupInterval > 0 {
		go c.cleanupLoop()
	}
	return c
}

func (c *LFUCache) Get(ctx context.Context, key string) (interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item, ok := c.items[key]
	if !ok {
		c.misses.Add(1)
		return nil, ErrKeyNotFound
	}
	if item.expiresAt != 0 && timeutil.NowNano() > item.expiresAt {
		c.removeLocked(item)
		c.misses.Add(1)
		return nil, ErrKeyExpired
	}
	c.incrementFreq(item)
	c.hits.Add(1)
	return item.value, nil
}

func (c *LFUCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var expiresAt int64
	if ttl > 0 {
		expiresAt = timeutil.NowNano() + int64(ttl)
	}

	if item, ok := c.items[key]; ok {
		item.value = value
		item.expiresAt = expiresAt
		c.incrementFreq(item)
		c.sets.Add(1)
		return nil
	}

	for len(c.items) >= c.config.MaxSize {
		c.evictLocked()
	}

	item := &lfuItem{key: key, value: value, freq: 1, expiresAt: expiresAt}
	c.addToFreqList(item, 1)
	c.items[key] = item
	c.minFreq = 1
	c.sets.Add(1)
	return nil
}

func (c *LFUCache) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[key]; ok {
		c.removeLocked(item)
		c.deletes.Add(1)
	}
	return nil
}

func (c *LFUCache) Exists(ctx context.Context, key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok {
		return false
	}
	return item.expiresAt == 0 || timeutil.NowNano() <= item.expiresAt
}

func (c *LFUCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*lfuItem)
	c.freqList = make(map[int64]*list.List)
	c.minFreq = 0
	return nil
}

func (c *LFUCache) Stats() CacheStats {
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

func (c *LFUCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}
	return nil
}

func (c *LFUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// GetFrequency returns the access frequency for a key.
func (c *LFUCache) GetFrequency(key string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	item, ok := c.items[key]
	if !ok {
		return 0, false
	}
	return item.freq, true
}

func (c *LFUCache) incrementFreq(item *lfuItem) {
	oldFreq := item.freq
	fl := c.freqList[oldFreq]
	if fl != nil {
		fl.Remove(item.element)
		if fl.Len() == 0 {
			delete(c.freqList, oldFreq)
			if c.minFreq == oldFreq {
				c.minFreq = oldFreq + 1
			}
		}
	}
	item.freq++
	c.addToFreqList(item, item.freq)
}

func (c *LFUCache) addToFreqList(item *lfuItem, freq int64) {
	fl, ok := c.freqList[freq]
	if !ok {
		fl = list.New()
		c.freqList[freq] = fl
	}
	item.element = fl.PushFront(item)
}

func (c *LFUCache) evictLocked() {
	fl := c.freqList[c.minFreq]
	if fl == nil || fl.Len() == 0 {
		return
	}
	el := fl.Back()
	item := el.Value.(*lfuItem)
	fl.Remove(el)
	if fl.Len() == 0 {
		delete(c.freqList, c.minFreq)
	}
	delete(c.items, item.key)
	c.evictions.Add(1)
	if c.config.OnEvict != nil {
		c.config.OnEvict(item.key, item.value)
	}
}

func (c *LFUCache) removeLocked(item *lfuItem) {
	fl := c.freqList[item.freq]
	if fl != nil {
		fl.Remove(item.element)
		if fl.Len() == 0 {
			delete(c.freqList, item.freq)
		}
	}
	delete(c.items, item.key)
}

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

func (c *LFUCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := timeutil.NowNano()
	var toRemove []*lfuItem
	for _, item := range c.items {
		if item.expiresAt != 0 && now > item.expiresAt {
			toRemove = append(toRemove, item)
		}
	}
	for _, item := range toRemove {
		c.removeLocked(item)
	}
}
