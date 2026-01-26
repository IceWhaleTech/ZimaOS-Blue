package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

func setupDiskCache(t *testing.T) (*DiskCache, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "disk-cache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	config := DefaultL2Config()
	config.Path = tmpDir
	config.CleanupInterval = 0 // Disable cleanup for testing

	cache, err := NewDiskCache(config)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("NewDiskCache() error = %v", err)
	}

	cleanup := func() {
		cache.Close()
		os.RemoveAll(tmpDir)
	}

	return cache, cleanup
}

func TestDiskCache_GetSet(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set a string value
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

func TestDiskCache_GetNotFound(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	_, err := cache.Get(ctx, "nonexistent")
	if err != ErrKeyNotFound {
		t.Errorf("Get() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestDiskCache_Expiration(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	// Set with short TTL
	err := cache.Set(ctx, "key1", "value1", 50*time.Millisecond)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Should exist initially
	if !cache.Exists(ctx, "key1") {
		t.Error("key1 should exist initially")
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, err = cache.Get(ctx, "key1")
	if err != ErrKeyExpired {
		t.Errorf("Get() after expiration error = %v, want %v", err, ErrKeyExpired)
	}
}

func TestDiskCache_Delete(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Delete(ctx, "key1")

	_, err := cache.Get(ctx, "key1")
	if err != ErrKeyNotFound {
		t.Errorf("Get() after Delete() error = %v, want %v", err, ErrKeyNotFound)
	}
}

func TestDiskCache_Clear(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)
	cache.Set(ctx, "key2", "value2", 0)
	cache.Clear(ctx)

	if cache.Len() != 0 {
		t.Errorf("Len() after Clear() = %d, want 0", cache.Len())
	}
}

func TestDiskCache_Exists(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	cache.Set(ctx, "key1", "value1", 0)

	if !cache.Exists(ctx, "key1") {
		t.Error("Exists(key1) = false, want true")
	}

	if cache.Exists(ctx, "nonexistent") {
		t.Error("Exists(nonexistent) = true, want false")
	}
}

func TestDiskCache_Stats(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

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

func TestDiskCache_DiskSize(t *testing.T) {
	cache, cleanup := setupDiskCache(t)
	defer cleanup()

	ctx := context.Background()

	initialSize := cache.DiskSize()
	if initialSize != 0 {
		t.Errorf("Initial DiskSize() = %d, want 0", initialSize)
	}

	cache.Set(ctx, "key1", "value1", 0)

	if cache.DiskSize() <= initialSize {
		t.Error("DiskSize() should increase after Set()")
	}
}

func TestDiskCache_Compression(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-compression-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultL2Config()
	config.Path = tmpDir
	config.Compression = true
	config.CompressionLevel = 9
	config.CleanupInterval = 0

	cache, err := NewDiskCache(config)
	if err != nil {
		t.Fatalf("NewDiskCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Set a large value that compresses well
	largeValue := make([]byte, 10000)
	for i := range largeValue {
		largeValue[i] = 'a'
	}

	err = cache.Set(ctx, "key1", string(largeValue), 0)
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Get the value back
	value, err := cache.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if value != string(largeValue) {
		t.Error("Get() returned different value than Set()")
	}

	// Disk size should be less than uncompressed size
	if cache.DiskSize() >= int64(len(largeValue)) {
		t.Errorf("DiskSize() = %d, should be less than %d (compressed)", cache.DiskSize(), len(largeValue))
	}
}

func TestDiskCache_SizeLimit(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-limit-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultL2Config()
	config.Path = tmpDir
	config.MaxDiskSize = 1000 // Very small limit
	config.Compression = false
	config.CleanupInterval = 0

	cache, err := NewDiskCache(config)
	if err != nil {
		t.Fatalf("NewDiskCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	// Add entries until we exceed the limit
	for i := 0; i < 100; i++ {
		cache.Set(ctx, string(rune('a'+i)), "some value that takes space", 0)
	}

	// Disk size should be around the limit
	if cache.DiskSize() > config.MaxDiskSize*2 {
		t.Errorf("DiskSize() = %d, should be around %d", cache.DiskSize(), config.MaxDiskSize)
	}

	// Some entries should have been evicted
	stats := cache.Stats()
	if stats.Evictions == 0 {
		t.Error("Expected some evictions")
	}
}

func BenchmarkDiskCache_Set(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultL2Config()
	config.Path = tmpDir
	config.CleanupInterval = 0

	cache, err := NewDiskCache(config)
	if err != nil {
		b.Fatalf("NewDiskCache() error = %v", err)
	}
	defer cache.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(ctx, string(rune(i%10000)), "benchmark value", 0)
	}
}

func BenchmarkDiskCache_Get(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "disk-cache-bench-*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := DefaultL2Config()
	config.Path = tmpDir
	config.CleanupInterval = 0

	cache, err := NewDiskCache(config)
	if err != nil {
		b.Fatalf("NewDiskCache() error = %v", err)
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
