package server

import (
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConversationCache caches conversation messages to reduce database queries.
type ConversationCache struct {
	entries  map[string]*cacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
	maxSize  int
	initOnce sync.Once
}

type cacheEntry struct {
	messages  []memory.Message
	timestamp time.Time
}

// NewConversationCache creates a new conversation cache.
// The cleanup goroutine is lazily started on first write to reduce startup overhead.
func NewConversationCache(ttl time.Duration, maxSize int) *ConversationCache {
	return &ConversationCache{
		ttl:     ttl,
		maxSize: maxSize,
	}
}

// ensureInit lazily initializes the entries map and cleanup goroutine.
func (c *ConversationCache) ensureInit() {
	c.initOnce.Do(func() {
		c.entries = make(map[string]*cacheEntry)
		go c.cleanupLoop()
	})
}

// Get retrieves messages from cache if available and not expired.
func (c *ConversationCache) Get(conversationID string) ([]memory.Message, bool) {
	c.ensureInit()
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[conversationID]
	if !exists {
		return nil, false
	}

	// Check if expired
	if timeutil.SinceTime(entry.timestamp) > c.ttl {
		return nil, false
	}

	// Return a copy to prevent external modification
	messages := make([]memory.Message, len(entry.messages))
	copy(messages, entry.messages)

	return messages, true
}

// Set stores messages in cache.
func (c *ConversationCache) Set(conversationID string, messages []memory.Message) {
	c.ensureInit()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check cache size limit
	if len(c.entries) >= c.maxSize {
		// Evict oldest entry (simple LRU)
		c.evictOldest()
	}

	// Store a copy to prevent external modification
	messageCopy := make([]memory.Message, len(messages))
	copy(messageCopy, messages)

	c.entries[conversationID] = &cacheEntry{
		messages:  messageCopy,
		timestamp: timeutil.NowTime(),
	}
}

// Invalidate removes a conversation from cache.
func (c *ConversationCache) Invalidate(conversationID string) {
	c.ensureInit()
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, conversationID)
}

// Clear removes all entries from cache.
func (c *ConversationCache) Clear() {
	c.ensureInit()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Stats returns cache statistics.
func (c *ConversationCache) Stats() CacheStats {
	c.ensureInit()
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size: len(c.entries),
		MaxSize: c.maxSize,
		TTL: c.ttl,
	}
}

// evictOldest removes the oldest entry from cache (must be called with lock held).
func (c *ConversationCache) evictOldest() {
	var oldestID string
	var oldestTime time.Time

	for id, entry := range c.entries {
		if oldestID == "" || entry.timestamp.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.timestamp
		}
	}

	if oldestID != "" {
		delete(c.entries, oldestID)
	}
}

// cleanupLoop periodically removes expired entries.
func (c *ConversationCache) cleanupLoop() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

// cleanup removes expired entries.
func (c *ConversationCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := timeutil.NowTime()
	for id, entry := range c.entries {
		if now.Sub(entry.timestamp) > c.ttl {
			delete(c.entries, id)
		}
	}
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Size    int
	MaxSize int
	TTL     time.Duration
}
