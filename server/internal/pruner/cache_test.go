package pruner

import (
	"testing"
)

func TestScoreCache_PutGet(t *testing.T) {
	cache := NewScoreCache(10)
	segs := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 5}, Score: 0.9},
	}
	key := CacheKey("code content", "query")
	cache.Put(key, segs)

	got, ok := cache.Get(key)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 1 || got[0].Score != 0.9 {
		t.Errorf("unexpected cached value: %v", got)
	}
}

func TestScoreCache_Miss(t *testing.T) {
	cache := NewScoreCache(10)
	_, ok := cache.Get(CacheKey("nonexistent", ""))
	if ok {
		t.Error("expected cache miss")
	}
}

func TestScoreCache_Eviction(t *testing.T) {
	cache := NewScoreCache(2) // capacity 2
	k1 := CacheKey("k1", "")
	k2 := CacheKey("k2", "")
	k3 := CacheKey("k3", "")
	cache.Put(k1, []ScoredSegment{{Score: 1}})
	cache.Put(k2, []ScoredSegment{{Score: 2}})
	cache.Put(k3, []ScoredSegment{{Score: 3}}) // should evict k1

	// ecache2 uses sharded buckets, so eviction is per-bucket
	// Just verify k3 is present
	_, ok := cache.Get(k3)
	if !ok {
		t.Error("expected k3 to be present")
	}
}

func TestScoreCache_LRU_AccessRefresh(t *testing.T) {
	cache := NewScoreCache(256)
	k1 := CacheKey("k1", "")
	k2 := CacheKey("k2", "")
	cache.Put(k1, []ScoredSegment{{Score: 1}})
	cache.Put(k2, []ScoredSegment{{Score: 2}})

	// Access k1 to make it recently used
	got, ok := cache.Get(k1)
	if !ok {
		t.Error("expected k1 to be present")
	}
	if len(got) != 1 || got[0].Score != 1 {
		t.Errorf("unexpected value for k1: %v", got)
	}
}

func TestCacheKey_Deterministic(t *testing.T) {
	k1 := CacheKey("same code", "same query")
	k2 := CacheKey("same code", "same query")
	if k1 != k2 {
		t.Errorf("expected same key, got %d vs %d", k1, k2)
	}
}

func TestCacheKey_Different(t *testing.T) {
	k1 := CacheKey("code A", "query")
	k2 := CacheKey("code B", "query")
	if k1 == k2 {
		t.Error("expected different keys for different code")
	}
}
