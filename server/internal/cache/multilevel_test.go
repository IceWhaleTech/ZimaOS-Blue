package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

func setupMultiLevelCache(t *testing.T) (*MultiLevelCache, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "multilevel-cache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := DefaultMultiLevelConfig()
	config.L1.MaxSize = 10
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0

	cache, err := NewMultiLevelCache(config)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("NewMultiLevelCache() error = %v", err)
	}

	cleanup := func() {
		cache.Close()
		os.RemoveAll(tmpDir)
	}

	return cache, cleanup
}

func TestMultiLevelCache_GetSet(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

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

func TestMultiLevelCache_L1Hit(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Get(ctx, "key1")

	stats := cache.Stats()
	if stats.L1Hits != 1 {
		t.Errorf("L1Hits = %d, want 1", stats.L1Hits)
	}
}

func TestMultiLevelCache_L2Hit(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set only in L2
	cache.SetL2Only(ctx, "key1", "value1", 0)

	// Get should hit L2
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}

	stats := cache.Stats()
	if stats.L2Hits != 1 {
		t.Errorf("L2Hits = %d, want 1", stats.L2Hits)
	}
}

func TestMultiLevelCache_Promotion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "multilevel-promotion-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultMultiLevelConfig()
	config.L1.MaxSize = 10
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0
	config.PromoteOnHit = true

	cache, err := NewMultiLevelCache(config)
	if err != nil {
		t.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set only in L2
	cache.SetL2Only(ctx, "key1", "value1", 0)

	// First get should hit L2 and promote to L1
	cache.Get(ctx, "key1")

	stats := cache.Stats()
	if stats.Promotions != 1 {
		t.Errorf("Promotions = %d, want 1", stats.Promotions)
	}

	// Second get should hit L1
	cache.Get(ctx, "key1")

	stats = cache.Stats()
	if stats.L1Hits != 1 {
		t.Errorf("L1Hits after promotion = %d, want 1", stats.L1Hits)
	}
}

func TestMultiLevelCache_WriteThrough(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "multilevel-writethrough-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultMultiLevelConfig()
	config.L1.MaxSize = 10
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0
	config.WriteThrough = true

	cache, err := NewMultiLevelCache(config)
	if err != nil {
		t.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set should write to both L1 and L2
	cache.Set(ctx, "key1", "value1", 0)

	// Clear L1
	cache.L1().Clear(ctx)

	// Should still be in L2
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() after L1 clear error = %v", err)
	}

	if value != "value1" {
		t.Errorf("Get() = %v, want %v", value, "value1")
	}
}

func TestMultiLevelCache_Delete(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Delete(ctx, "key1")

	if cache.Exists(ctx, "key1") {
		t.Error("key1 should not exist after Delete()")
	}
}

func TestMultiLevelCache_Clear(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Clear(ctx)

	if cache.Exists(ctx, "key1") || cache.Exists(ctx, "key2") {
		t.Error("Cache should be empty after Clear()")
	}
}

func TestMultiLevelCache_Demote(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set in L1 only
	cache.SetL1Only(ctx, "key1", "value1", 0)

	// Demote to L2
	err := cache.Demote(ctx, "key1")
	if err != nil {
		t.Fatalf("Demote() error = %v", err)
	}

	// Should not be in L1
	if cache.L1().Exists(ctx, "key1") {
		t.Error("key1 should not be in L1 after Demote()")
	}

	// Should be in L2
	if !cache.L2().Exists(ctx, "key1") {
		t.Error("key1 should be in L2 after Demote()")
	}
}

func TestMultiLevelCache_GetOrSet(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

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
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should not call fn again)", callCount)
	}
}

func TestMultiLevelCache_Stats(t *testing.T) {
	cache, cleanup := setupMultiLevelCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Get(ctx, "key1")
	cache.Get(ctx, "nonexistent")
	cache.Delete(ctx, "key1")

	stats := cache.Stats()

	if stats.L1Hits != 1 {
		t.Errorf("L1Hits = %d, want 1", stats.L1Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
	if stats.Sets != 1 {
		t.Errorf("Sets = %d, want 1", stats.Sets)
	}
	if stats.Deletes != 1 {
		t.Errorf("Deletes = %d, want 1", stats.Deletes)
	}
}

func TestCacheManager(t *testing.T) {
	manager := NewCacheManager()
	defer manager.Close()

	ctx := context.Background()

	// Create and register caches
	cache1 := NewLRUCache(Config{MaxSize: 10, CleanupInterval: 0})
	cache2 := NewLRUCache(Config{MaxSize: 10, CleanupInterval: 0})

	manager.Register("cache1", cache1)
	manager.Register("cache2", cache2)

	// Get cache
	c, ok := manager.Get("cache1")
	if !ok {
		t.Fatal("Get(cache1) returned false")
	}

	c.Set(ctx, "key1", "value1", 0)

	// Stats
	stats := manager.Stats()
	if len(stats) != 2 {
		t.Errorf("Stats() returned %d caches, want 2", len(stats))
	}

	// Clear all
	manager.ClearAll(ctx)

	// Unregister
	manager.Unregister("cache1")

	_, ok = manager.Get("cache1")
	if ok {
		t.Error("Get(cache1) should return false after Unregister()")
	}
}

func BenchmarkMultiLevelCache_Get(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "multilevel-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultMultiLevelConfig()
	config.L1.MaxSize = 10000
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0

	cache, err := NewMultiLevelCache(config)
	if err != nil {
		b.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(ctx, string(rune(i)), "benchmark value", 0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ctx, string(rune(i%1000)))
	}
}

func BenchmarkMultiLevelCache_Set(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "multilevel-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultMultiLevelConfig()
	config.L1.MaxSize = 10000
	config.L1.CleanupInterval = 0
	config.L2.Path = tmpDir
	config.L2.CleanupInterval = 0
	config.WriteThrough = false // Disable write-through for fair comparison

	cache, err := NewMultiLevelCache(config)
	if err != nil {
		b.Fatalf("NewMultiLevelCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, string(rune(i%10000)), "benchmark value", time.Hour)
	}
}
