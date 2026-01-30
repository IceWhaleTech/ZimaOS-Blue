package proxy

import (
	"bytes"
	"testing"
	"time"
)

// BenchmarkBufferPool benchmarks buffer pool operations
func BenchmarkBufferPool(b *testing.B) {
	config := DefaultPerformanceConfig()
	pool := NewBufferPool(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf.WriteString("test data for buffer pool benchmark")
			pool.Put(buf)
		}
	})
}

// BenchmarkBufferPoolNoPool benchmarks without pool (for comparison)
func BenchmarkBufferPoolNoPool(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := bytes.NewBuffer(make([]byte, 0, 4096))
			buf.WriteString("test data for buffer pool benchmark")
			_ = buf
		}
	})
}

// BenchmarkResponseCache benchmarks cache operations
func BenchmarkResponseCache(b *testing.B) {
	config := DefaultPerformanceConfig()
	config.CacheEnabled = true
	cache := NewResponseCache(config)

	// Pre-populate cache
	testData := []byte("test response data for cache benchmark")
	for i := 0; i < 100; i++ {
		key := cache.GenerateCacheKey("provider", "model", string(rune(i)))
		cache.Set(key, testData, 200, nil)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := cache.GenerateCacheKey("provider", "model", string(rune(i%100)))
			cache.Get(key)
			i++
		}
	})
}

// BenchmarkCacheKeyGeneration benchmarks cache key generation
func BenchmarkCacheKeyGeneration(b *testing.B) {
	config := DefaultPerformanceConfig()
	cache := NewResponseCache(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GenerateCacheKey("openai", "gpt-4", "What is the meaning of life?")
	}
}

// BenchmarkConnectionMetrics benchmarks metrics recording
func BenchmarkConnectionMetrics(b *testing.B) {
	config := DefaultPerformanceConfig()
	metrics := NewConnectionMetrics(config)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			metrics.RecordRequest("openai")
			metrics.RecordRequestComplete("openai", true, 100*int64(time.Millisecond), true)
		}
	})
}

// TestBufferPoolReuse tests buffer pool reuse rate
func TestBufferPoolReuse(t *testing.T) {
	config := DefaultPerformanceConfig()
	pool := NewBufferPool(config)

	// Get and put buffers multiple times
	for i := 0; i < 1000; i++ {
		buf := pool.Get()
		buf.WriteString("test")
		pool.Put(buf)
	}

	stats := pool.Stats()
	reuseRate := stats["reuse_rate"].(float64)

	// Reuse rate should be high after warmup
	if reuseRate < 90 {
		t.Errorf("Expected reuse rate > 90%%, got %.2f%%", reuseRate)
	}
}

// TestResponseCacheHitRate tests cache hit rate
func TestResponseCacheHitRate(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.CacheEnabled = true
	config.CacheTTL = 1 * time.Hour
	cache := NewResponseCache(config)

	testData := []byte("cached response")

	// Set some entries
	for i := 0; i < 10; i++ {
		key := cache.GenerateCacheKey("provider", "model", string(rune(i)))
		cache.Set(key, testData, 200, nil)
	}

	// Get entries (should all hit)
	for i := 0; i < 10; i++ {
		key := cache.GenerateCacheKey("provider", "model", string(rune(i)))
		_, found := cache.Get(key)
		if !found {
			t.Errorf("Expected cache hit for key %d", i)
		}
	}

	// Get non-existent entries (should all miss)
	for i := 10; i < 20; i++ {
		key := cache.GenerateCacheKey("provider", "model", string(rune(i)))
		_, found := cache.Get(key)
		if found {
			t.Errorf("Expected cache miss for key %d", i)
		}
	}

	stats := cache.Stats()
	hitRate := stats["hit_rate"].(float64)

	// Should be 50% (10 hits, 10 misses)
	if hitRate < 49 || hitRate > 51 {
		t.Errorf("Expected hit rate ~50%%, got %.2f%%", hitRate)
	}
}

// TestCacheExpiration tests cache entry expiration
func TestCacheExpiration(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.CacheEnabled = true
	config.CacheTTL = 10 * time.Millisecond
	cache := NewResponseCache(config)

	key := cache.GenerateCacheKey("provider", "model", "prompt")
	cache.Set(key, []byte("data"), 200, nil)

	// Should hit immediately
	_, found := cache.Get(key)
	if !found {
		t.Error("Expected cache hit before expiration")
	}

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should miss after expiration
	_, found = cache.Get(key)
	if found {
		t.Error("Expected cache miss after expiration")
	}
}

// TestConnectionMetricsAccuracy tests metrics accuracy
func TestConnectionMetricsAccuracy(t *testing.T) {
	config := DefaultPerformanceConfig()
	metrics := NewConnectionMetrics(config)

	// Record 100 requests
	for i := 0; i < 100; i++ {
		metrics.RecordRequest("openai")
		success := i%10 != 0 // 90% success rate
		reused := i%2 == 0   // 50% reuse rate
		metrics.RecordRequestComplete("openai", success, int64(i)*int64(time.Millisecond), reused)
	}

	stats := metrics.Stats()

	if stats["total_requests"].(int64) != 100 {
		t.Errorf("Expected 100 total requests, got %d", stats["total_requests"])
	}

	if stats["completed_requests"].(int64) != 100 {
		t.Errorf("Expected 100 completed requests, got %d", stats["completed_requests"])
	}

	if stats["failed_requests"].(int64) != 10 {
		t.Errorf("Expected 10 failed requests, got %d", stats["failed_requests"])
	}

	reuseRate := stats["reuse_rate"].(float64)
	if reuseRate < 49 || reuseRate > 51 {
		t.Errorf("Expected reuse rate ~50%%, got %.2f%%", reuseRate)
	}
}

// TestPerformanceManager tests the performance manager
func TestPerformanceManager(t *testing.T) {
	pm := NewPerformanceManager(nil)

	// Test buffer pool
	buf := pm.GetBufferPool().Get()
	buf.WriteString("test")
	pm.GetBufferPool().Put(buf)

	// Test cache
	pm.GetCache().Set("key", []byte("value"), 200, nil)

	// Test metrics
	pm.GetConnectionMetrics().RecordRequest("test")
	pm.GetConnectionMetrics().RecordRequestComplete("test", true, 100, true)

	// Get all stats
	stats := pm.Stats()

	if stats["buffer_pool"] == nil {
		t.Error("Expected buffer_pool stats")
	}
	if stats["cache"] == nil {
		t.Error("Expected cache stats")
	}
	if stats["connection_metrics"] == nil {
		t.Error("Expected connection_metrics stats")
	}
}

// TestCacheEviction tests cache eviction when at capacity
func TestCacheEviction(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.CacheEnabled = true
	config.CacheMaxSize = 5
	config.CacheTTL = 1 * time.Hour
	cache := NewResponseCache(config)

	// Add 10 entries (should evict 5)
	for i := 0; i < 10; i++ {
		key := cache.GenerateCacheKey("provider", "model", string(rune(i)))
		cache.Set(key, []byte("data"), 200, nil)
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
	}

	stats := cache.Stats()
	if stats["entries"].(int) != 5 {
		t.Errorf("Expected 5 entries after eviction, got %d", stats["entries"])
	}

	if stats["evictions"].(int64) != 5 {
		t.Errorf("Expected 5 evictions, got %d", stats["evictions"])
	}
}

// TestBufferPoolOversized tests handling of oversized buffers
func TestBufferPoolOversized(t *testing.T) {
	config := DefaultPerformanceConfig()
	config.BufferMaxSize = 1024 // 1KB max
	pool := NewBufferPool(config)

	buf := pool.Get()
	// Write more than max size
	largeData := make([]byte, 2048)
	buf.Write(largeData)

	// Put should not return oversized buffer to pool
	pool.Put(buf)

	stats := pool.Stats()
	if stats["oversized"].(int64) != 1 {
		t.Errorf("Expected 1 oversized buffer, got %d", stats["oversized"])
	}
}
