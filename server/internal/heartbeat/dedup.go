package heartbeat

import (
	"hash/fnv"
	"sync"
	"time"
)

// DedupCache prevents delivering identical heartbeat alerts within a time window.
type DedupCache struct {
	mu      sync.Mutex
	entries map[uint64]time.Time
	ttl     time.Duration
}

// NewDedupCache creates a dedup cache with the given TTL (typically 24h).
func NewDedupCache(ttl time.Duration) *DedupCache {
	return &DedupCache{
		entries: make(map[uint64]time.Time),
		ttl:     ttl,
	}
}

// IsDuplicate returns true if the same text was recorded within the TTL window.
// Also performs lazy eviction of expired entries.
func (d *DedupCache) IsDuplicate(text string) bool {
	h := hashText(text)
	now := time.Now()

	d.mu.Lock()
	defer d.mu.Unlock()

	// Lazy eviction
	for k, t := range d.entries {
		if now.Sub(t) > d.ttl {
			delete(d.entries, k)
		}
	}

	if lastSent, ok := d.entries[h]; ok {
		return now.Sub(lastSent) < d.ttl
	}
	return false
}

// Record stores the text hash with the current timestamp.
func (d *DedupCache) Record(text string) {
	h := hashText(text)
	d.mu.Lock()
	defer d.mu.Unlock()
	d.entries[h] = time.Now()
}

func hashText(text string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(text))
	return h.Sum64()
}
