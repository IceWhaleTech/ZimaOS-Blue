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
	maxBytes       uint64
	totalBytes     uint64
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
	sizeBytes uint64
}

// NewConversationCache creates a new conversation cache.
// The cleanup goroutine is lazily started on first write to reduce startup overhead.
func NewConversationCache(ttl time.Duration, maxSize int) *ConversationCache {
	return &ConversationCache{
		entries:  make(map[string]*cacheEntry),
		ttl:      ttl,
		maxSize:  maxSize,
		maxBytes: defaultConversationCacheMaxBytes,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
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

	// Return a deep lightweight copy to prevent external modification.
	messages, _ := cloneCacheableMessages(entry.messages)
	return messages, true
}

// Set stores messages in cache.
func (c *ConversationCache) Set(conversationID string, messages []memory.Message) {
	c.ensureCleanupStarted()
	messageCopy, sizeBytes := cloneCacheableMessages(messages)
	now := timeutil.NowTime()

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	c.cleanupExpiredLocked(now)
	if existing := c.entries[conversationID]; existing != nil {
		c.subtractBytesLocked(existing.sizeBytes)
	}

	c.entries[conversationID] = &cacheEntry{
		messages:  messageCopy,
		timestamp: now,
		sizeBytes: sizeBytes,
	}
	c.totalBytes += sizeBytes
	c.enforceBudgetsLocked()
}

// Invalidate removes a conversation from cache.
func (c *ConversationCache) Invalidate(conversationID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}

	if existing := c.entries[conversationID]; existing != nil {
		c.subtractBytesLocked(existing.sizeBytes)
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
	c.totalBytes = 0
}

// Stats returns cache statistics.
func (c *ConversationCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStats{
		Size:    len(c.entries),
		MaxSize: c.maxSize,
		TTL:     c.ttl,
		Bytes:   c.totalBytes,
	}
}

// evictOldest removes the oldest entry from cache (must be called with lock held).
func (c *ConversationCache) evictOldestLocked() {
	var oldestID string
	var oldestTime time.Time

	for id, entry := range c.entries {
		if entry == nil {
			oldestID = id
			break
		}
		if oldestID == "" || entry.timestamp.Before(oldestTime) {
			oldestID = id
			oldestTime = entry.timestamp
		}
	}

	if oldestID != "" {
		if entry := c.entries[oldestID]; entry != nil {
			c.subtractBytesLocked(entry.sizeBytes)
		}
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

	c.cleanupExpiredLocked(timeutil.NowTime())
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
		c.totalBytes = 0
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
	Bytes   uint64
}

func (c *ConversationCache) cleanupExpiredLocked(now time.Time) {
	for id, entry := range c.entries {
		if entry == nil || now.Sub(entry.timestamp) > c.ttl {
			if entry != nil {
				c.subtractBytesLocked(entry.sizeBytes)
			}
			delete(c.entries, id)
		}
	}
}

func (c *ConversationCache) enforceBudgetsLocked() {
	for {
		overSize := c.maxSize > 0 && len(c.entries) > c.maxSize
		overBytes := c.maxBytes > 0 && c.totalBytes > c.maxBytes
		if !overSize && !overBytes {
			return
		}
		if len(c.entries) == 0 {
			c.totalBytes = 0
			return
		}
		c.evictOldestLocked()
	}
}

func (c *ConversationCache) subtractBytesLocked(size uint64) {
	if c.totalBytes >= size {
		c.totalBytes -= size
		return
	}
	c.totalBytes = 0
}

func estimateMessagesBytes(messages []memory.Message) uint64 {
	var total uint64
	for _, msg := range messages {
		total += uint64(len(msg.Role) + len(msg.Content) + len(msg.ToolCallID) + len(msg.ToolName) + len(msg.Provider) + len(msg.Model))
		for _, call := range msg.ToolCalls {
			total += uint64(len(call.ID) + len(call.Name) + len(call.Arguments))
		}
		for _, att := range msg.Attachments {
			total += uint64(len(att.Type) + len(att.Name) + len(att.MimeType) + len(att.Data))
		}
	}
	return total
}
