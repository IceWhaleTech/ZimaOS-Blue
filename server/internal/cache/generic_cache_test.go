package cache

import (
	"testing"
	"time"
)

func TestGenericCache_ExpiresEntries(t *testing.T) {
	cache := NewGenericCache[string](Config{
		MaxSize:    4,
		DefaultTTL: 40 * time.Millisecond,
	})

	cache.Put("a", "value")

	if got, ok := cache.Get("a"); !ok || got != "value" {
		t.Fatalf("Get(a) = (%v, %v), want (value, true)", got, ok)
	}

	time.Sleep(80 * time.Millisecond)

	if got, ok := cache.Get("a"); ok || got != nil {
		t.Fatalf("Get(a) after ttl = (%v, %v), want (nil, false)", got, ok)
	}
}

func TestGenericCache_EvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	cache := NewGenericCache[string](Config{
		MaxSize: 2,
	})

	cache.Put("first", "1")
	cache.Put("second", "2")

	if _, ok := cache.Get("first"); !ok {
		t.Fatal("expected first to be present before eviction")
	}

	cache.Put("third", "3")

	if got, ok := cache.Get("second"); ok || got != nil {
		t.Fatalf("Get(second) = (%v, %v), want (nil, false)", got, ok)
	}

	if got, ok := cache.Get("first"); !ok || got != "1" {
		t.Fatalf("Get(first) = (%v, %v), want (1, true)", got, ok)
	}

	if got, ok := cache.Get("third"); !ok || got != "3" {
		t.Fatalf("Get(third) = (%v, %v), want (3, true)", got, ok)
	}
}

func TestGenericCache_Int64RoundTrip(t *testing.T) {
	cache := NewGenericCache[uint64](Config{
		MaxSize: 4,
	})

	cache.PutInt64(42, 99)

	got, ok := cache.GetInt64(42)
	if !ok {
		t.Fatal("expected int64 cache hit")
	}
	if got != 99 {
		t.Fatalf("GetInt64(42) = %d, want 99", got)
	}
}

func TestGenericCache_StatsTrackLocalActivityWithoutPool(t *testing.T) {
	cache := NewGenericCache[string](Config{
		MaxSize: 3,
	})

	cache.Put("a", "value")
	if _, ok := cache.Get("a"); !ok {
		t.Fatal("expected cache hit for key a")
	}
	if _, ok := cache.Get("missing"); ok {
		t.Fatal("expected cache miss for missing key")
	}
	cache.Del("a")

	stats := cache.Stats()
	if stats.Hits != 1 {
		t.Fatalf("Stats().Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("Stats().Misses = %d, want 1", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Fatalf("Stats().Sets = %d, want 1", stats.Sets)
	}
	if stats.Deletes != 1 {
		t.Fatalf("Stats().Deletes = %d, want 1", stats.Deletes)
	}
	if stats.Capacity != 3 {
		t.Fatalf("Stats().Capacity = %d, want 3", stats.Capacity)
	}
	if stats.HitRate != 50 {
		t.Fatalf("Stats().HitRate = %v, want 50", stats.HitRate)
	}
}
