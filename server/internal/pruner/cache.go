package pruner

import (
	"hash/fnv"
	"time"
	"unsafe"

	ecache2 "github.com/orca-zhang/ecache2"
)

// LRUScoreCache wraps ecache2 for scored segment caching.
// Uses uint64 keys with FNV hash — ecache2 does direct value sharding (no rehash).
type LRUScoreCache struct {
	cache *ecache2.Cache[uint64]
}

// NewScoreCache creates an LRU cache with the given capacity.
func NewScoreCache(capacity int) *LRUScoreCache {
	if capacity <= 0 {
		capacity = 256
	}
	buckets := uint16(16)
	perBucket := uint16(capacity / int(buckets))
	if perBucket < 4 {
		perBucket = 4
	}
	return &LRUScoreCache{
		cache: ecache2.NewLRUCache[uint64](buckets, perBucket, 10*time.Minute),
	}
}

// Get retrieves scored segments from the cache.
func (c *LRUScoreCache) Get(key uint64) ([]ScoredSegment, bool) {
	v, ok := c.cache.Get(key)
	if !ok {
		return nil, false
	}
	segments, _ := v.([]ScoredSegment)
	return segments, segments != nil
}

// Put stores scored segments in the cache.
func (c *LRUScoreCache) Put(key uint64, segments []ScoredSegment) {
	c.cache.Put(key, segments)
}

// CacheKey computes a uint64 hash from content and query using FNV-1a.
// Uses unsafe string→bytes to avoid allocation for large content strings.
func CacheKey(code, query string) uint64 {
	h := fnv.New64a()
	h.Write(unsafeBytes(code))
	h.Write([]byte{0})
	h.Write(unsafeBytes(query))
	return h.Sum64()
}

// unsafeBytes converts a string to []byte without copying.
// Safe as long as the returned slice is not modified.
func unsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
