package cache

import (
	"context"
	"testing"
	"time"
)

func TestLFUCache_GetSet(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestLFUCache_Eviction(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         3,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// Fill the cache
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Access key1 multiple times to increase frequency
	cache.Get(ctx, "key1")
	cache.Get(ctx, "key1")
	cache.Get(ctx, "key1")

	// Access key3 once
	cache.Get(ctx, "key3")

	// Add another key, should evict key2 (least frequently used)
	cache.Set(ctx, "key4", "value4", 0)

	// key2 should be evicted (freq=1)
	_, err := cache.Get(ctx, "key2")
	if err != ErrKeyNotFound {
		t.Errorf("Get(key2) error = %v, want %v", err, ErrKeyNotFound)
	}

	// key1 should still exist (freq=4)
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get(key1) error = %v", err)
	}
}

func TestLFUCache_Frequency(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)

	// Initial frequency should be 1
	freq, ok := cache.GetFrequency("key1")
	if !ok {
		t.Fatal("GetFrequency() returned false")
	}
	if freq != 1 {
		t.Errorf("Initial frequency = %d, want 1", freq)
	}

	// Access multiple times
	cache.Get(ctx, "key1")
	cache.Get(ctx, "key1")
	cache.Get(ctx, "key1")

	freq, _ = cache.GetFrequency("key1")
	if freq != 4 {
		t.Errorf("Frequency after 3 gets = %d, want 4", freq)
	}
}

func TestLFUCache_Expiration(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 50*time.Millisecond)

	// Should exist initially
	if !cache.Exists(ctx, "key1") {
		t.Error("key1 should exist initially")
	}

	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err := cache.Get(ctx, "key1")
	if err != ErrKeyExpired {
		t.Errorf("Get() after expiration error = %v, want %v", err, ErrKeyExpired)
	}
}

func TestLFUCache_Delete(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Delete(ctx, "key1")

	_, err := cache.Get(ctx, "key1")
	if err != ErrKeyNotFound {
		t.Errorf("Get() after Delete() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestLFUCache_Clear(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Clear(ctx)

	if cache.Len() != 0 {
		t.Errorf("Len() after Clear() = %d, want 0", cache.Len())
	}
}

func TestLFUCache_Stats(t *testing.T) {
	cache := NewLFUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Get(ctx, "key1")
	cache.Get(ctx, "nonexistent")
	cache.Delete(ctx, "key1")

	stats := cache.Stats()

	if stats.Hits != 1 {
		t.Errorf("Stats.Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Stats.Misses = %d, want 1", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Errorf("Stats.Sets = %d, want 1", stats.Sets)
	}
	if stats.Deletes != 1 {
		t.Errorf("Stats.Deletes = %d, want 1", stats.Deletes)
	}
}

func BenchmarkLFUCache_Get(b *testing.B) {
	cache := NewLFUCache(Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	for i := 0; i < 1000; i++ {
		cache.Set(ctx, string(rune(i)), i, 0)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Get(ctx, string(rune(i%1000)))
			i++
		}
	})
}

func BenchmarkLFUCache_Set(b *testing.B) {
	cache := NewLFUCache(Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			cache.Set(ctx, string(rune(i%10000)), i, 0)
			i++
		}
	})
}
