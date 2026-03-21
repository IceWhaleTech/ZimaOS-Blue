package server

import (
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConversationCache caches conversation messages to reduce database queries.
type ConversationCache struct {
	entries        map[string]*cacheEntry
	mu             sync.RWMutex
	ttl            time.Duration
	maxSize        int
	initOnce       sync.Once
	closeOnce      sync.Once
	stopCh         chan struct{}
	doneCh         chan struct{}
	cleanupStarted bool
	closed         bool
}

type cacheEntry struct {
	messages  []memory.Message
	timestamp time.Time
}

// NewConversationCache creates a new conversation cache.
// The cleanup goroutine is lazily started on first write to reduce startup overhead.
func NewConversationCache(ttl time.Duration, maxSize int) *ConversationCache {
	return &ConversationCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
		maxSize: maxSize,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
}

// ensureCleanupStarted lazily starts the cleanup goroutine on first write.
func (c *ConversationCache) ensureCleanupStarted() {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return
	}
	c.mu.RUnlock()
	c.initOnce.Do(func() {
		c.mu.Lock()
		if c.closed {
			c.mu.Unlock()
			return
		}
		c.cleanupStarted = true
		c.mu.Unlock()
		go c.cleanupLoop()
	})
}

// Get retrieves messages from cache if available and not expired.
func (c *ConversationCache) Get(conversationID string) ([]memory.Message, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return nil, false
	}

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
	c.ensureCleanupStarted()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	delete(c.entries, conversationID)
}

// Clear removes all entries from cache.
func (c *ConversationCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	c.entries = make(map[string]*cacheEntry)
}

// Stats returns cache statistics.
func (c *ConversationCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		TTL:     c.ttl,
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
	defer close(c.doneCh)

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

// Close stops the background cleanup loop and clears cached entries.
func (c *ConversationCache) Close() {
	if c == nil {
		return
	}
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.closed = true
		c.entries = make(map[string]*cacheEntry)
		started := c.cleanupStarted
		c.mu.Unlock()

		if started {
			close(c.stopCh)
			<-c.doneCh
			return
		}
		close(c.doneCh)
	})
}

// CacheStats contains cache statistics.
type CacheStats struct {
	Size    int
	MaxSize int
	TTL     time.Duration
}
