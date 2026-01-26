package cache

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLRUCache_GetSet(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0, // Disable cleanup for testing
	})
	defer cache.Close()

	ctx := context.Background()

	// Set a value
	err := cache.Set(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get the value
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestLRUCache_GetNotFound(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	_, err := cache.Get(ctx, "nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("Get() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestLRUCache_Expiration(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// Set with short TTL
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should exist immediately
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = cache.Get(ctx, "key1")
	if err != ErrKeyExpired {
		t.Errorf("Get() after expiration error = %v, want %v", err, ErrKeyExpired)
	}
}

func TestLRUCache_Eviction(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         3,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// Fill the cache
	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Access key1 to make it recently used
	cache.Get(ctx, "key1")

	// Add another key, should evict key2 (least recently used)
	cache.Set(ctx, "key4", "value4", 0)

	// key2 should be evicted
	_, err := cache.Get(ctx, "key2")
	if err != ErrKeyNotFound {
		t.Errorf("Get(key2) error = %v, want %v", err, ErrKeyNotFound)
	}

	// key1 should still exist
	_, err = cache.Get(ctx, "key1")
	if err != nil {
		t.Errorf("Get(key1) error = %v", err)
	}
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(Config{
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

func TestLRUCache_Clear(t *testing.T) {
	cache := NewLRUCache(Config{
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

func TestLRUCache_Exists(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)

	if !cache.Exists(ctx, "key1") {
		t.Error("Exists(key1) = false, want true")
	}

	if cache.Exists(ctx, "nonexistent") {
		t.Error("Exists(nonexistent) = true, want false")
	}
}

func TestLRUCache_Stats(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// Generate some activity
	cache.Set(ctx, "key1", "value1", 0)
	cache.Get(ctx, "key1")           // Hit
	cache.Get(ctx, "nonexistent")    // Miss
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

func TestLRUCache_Keys(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	keys := cache.Keys()
	if len(keys) != 3 {
		t.Errorf("Keys() length = %d, want 3", len(keys))
	}
}

func TestLRUCache_GetOrSet(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()
	callCount := 0

	fn := func() (interface{}, error) {
		callCount++
		return "generated", nil
	}

	// First call should generate
	value, err := cache.GetOrSet(ctx, "key1", fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if value != "generated" {
		t.Errorf("GetOrSet() = %v, want %v", value, "generated")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1", callCount)
	}

	// Second call should use cache
	value, err = cache.GetOrSet(ctx, "key1", fn, 0)
	if err != nil {
		t.Fatalf("GetOrSet() error = %v", err)
	}
	if value != "generated" {
		t.Errorf("GetOrSet() = %v, want %v", value, "generated")
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not call fn again)", callCount)
	}
}

func TestLRUCache_SetNX(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// First SetNX should succeed
	ok, err := cache.SetNX(ctx, "key1", "value1", 0)
	if err != nil {
		t.Fatalf("SetNX() error = %v", err)
	}
	if !ok {
		t.Error("SetNX() = false, want true")
	}

	// Second SetNX should fail
	ok, err = cache.SetNX(ctx, "key1", "value2", 0)
	if err != nil {
		t.Fatalf("SetNX() error = %v", err)
	}
	if ok {
		t.Error("SetNX() = true, want false")
	}

	// Value should be original
	value, _ := cache.Get(ctx, "key1")
	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestLRUCache_Touch(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         3,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0)

	// Touch key1 to make it recently used
	if !cache.Touch(ctx, "key1") {
		t.Error("Touch(key1) = false, want true")
	}

	// Add another key, should evict key2
	cache.Set(ctx, "key4", "value4", 0)

	// key1 should still exist
	if !cache.Exists(ctx, "key1") {
		t.Error("key1 should exist after Touch")
	}

	// key2 should be evicted
	if cache.Exists(ctx, "key2") {
		t.Error("key2 should be evicted")
	}
}

func TestLRUCache_OnEvict(t *testing.T) {
	evicted := make(map[string]interface{})
	var mu sync.Mutex

	cache := NewLRUCache(Config{
		MaxSize:         2,
		CleanupInterval: 0,
		OnEvict: func(key string, value interface{}) {
			mu.Lock()
			evicted[key] = value
			mu.Unlock()
		},
	})
	defer cache.Close()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Set(ctx, "key3", "value3", 0) // Should evict key1

	mu.Lock()
	defer mu.Unlock()

	if _, ok := evicted["key1"]; !ok {
		t.Error("OnEvict was not called for key1")
	}
	if evicted["key1"] != "value1" {
		t.Errorf("OnEvict value = %v, want %v", evicted["key1"], "value1")
	}
}

func TestLRUCache_Concurrent(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         100,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			cache.Set(ctx, key, i, 0)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := string(rune('a' + i%26))
			cache.Get(ctx, key)
		}(i)
	}

	wg.Wait()

	// Should not panic and cache should be consistent
	if cache.Len() > 100 {
		t.Errorf("Len() = %d, want <= 100", cache.Len())
	}
}

func TestLRUCache_Cleanup(t *testing.T) {
	cache := NewLRUCache(Config{
		MaxSize:         10,
		CleanupInterval: 50 * time.Millisecond,
	})
	defer cache.Close()

	ctx := context.Background()

	// Set with short TTL
	cache.Set(ctx, "key1", "value1", 30*time.Millisecond)

	// Should exist initially
	if !cache.Exists(ctx, "key1") {
		t.Error("key1 should exist initially")
	}

	// Wait for cleanup
	time.Sleep(150 * time.Millisecond)

	// Should be cleaned up
	if cache.Len() != 0 {
		t.Errorf("Len() after cleanup = %d, want 0", cache.Len())
	}
}

func BenchmarkLRUCache_Get(b *testing.B) {
	cache := NewLRUCache(Config{
		MaxSize:         10000,
		CleanupInterval: 0,
	})
	defer cache.Close()

	ctx := context.Background()

	// Pre-populate
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

func BenchmarkLRUCache_Set(b *testing.B) {
	cache := NewLRUCache(Config{
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
