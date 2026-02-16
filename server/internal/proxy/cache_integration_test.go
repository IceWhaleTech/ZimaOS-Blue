package proxy

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func newTestCCCache(t *testing.T) *CCCache {
	t.Helper()
	dir := t.TempDir()
	config := DefaultCacheConfig()
	config.Enabled = true
	config.StoragePath = filepath.Join(dir, "test_cache.db")
	config.StorageType = "multilevel"
	return NewCCCache(config)
}

func TestCCCache_L1L2_SetAndGet(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	headers := http.Header{"Content-Type": []string{"application/json"}}
	cache.Set("key1", []byte(`{"ok":true}`), 200, headers, "anthropic", "claude-3")

	// Should hit L1
	entry, ok := cache.Get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if string(entry.Body) != `{"ok":true}` {
		t.Errorf("body mismatch: %s", entry.Body)
	}
}

func TestCCCache_L2Promotion(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	headers := http.Header{"Content-Type": []string{"application/json"}}
	cache.Set("promo-key", []byte("data"), 200, headers, "p", "m")

	// Wait for async L2 write
	time.Sleep(100 * time.Millisecond)

	// Delete from L1 only
	cache.l1.Del("promo-key")

	// Should hit L2 and promote to L1
	entry, ok := cache.Get("promo-key")
	if !ok {
		t.Fatal("expected L2 hit after L1 eviction")
	}
	if string(entry.Body) != "data" {
		t.Errorf("body mismatch after promotion: %s", entry.Body)
	}
}

func TestCCCache_Stats_L1L2Breakdown(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	headers := http.Header{}
	cache.Set("s1", []byte("a"), 200, headers, "p", "m")

	// L1 hit
	cache.Get("s1")

	// Wait for L2 write, then evict from L1
	time.Sleep(100 * time.Millisecond)
	cache.l1.Del("s1")

	// L2 hit
	cache.Get("s1")

	// Miss
	cache.Get("nonexistent")

	stats := cache.Stats()
	if stats["l1_hits"].(int64) != 1 {
		t.Errorf("expected 1 L1 hit, got %v", stats["l1_hits"])
	}
	if stats["disk_hits"].(int64) != 1 {
		t.Errorf("expected 1 disk hit, got %v", stats["disk_hits"])
	}
	if stats["misses"].(int64) != 1 {
		t.Errorf("expected 1 miss, got %v", stats["misses"])
	}
}

func TestCCCache_CanonicalKey(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	body := []byte(`{"model":"claude-3","messages":[{"role":"user","content":"hi"}]}`)
	key := cache.GenerateCanonicalKey(body)

	if len(key) != 16 {
		t.Errorf("expected 16 char key (FNV-1a), got %d", len(key))
	}

	// Same body should produce same key
	key2 := cache.GenerateCanonicalKey(body)
	if key != key2 {
		t.Error("same body should produce same canonical key")
	}
}

func TestCCCache_Warmup(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	headers := http.Header{}
	for i := 0; i < 5; i++ {
		key := "warm-" + string(rune('a'+i))
		cache.Set(key, []byte("data"), 200, headers, "p", "m")
	}

	// Wait for async L2 writes
	time.Sleep(200 * time.Millisecond)

	// Clear L1 only (delete known keys)
	for i := 0; i < 5; i++ {
		key := "warm-" + string(rune('a'+i))
		cache.l1.Del(key)
	}

	loaded := cache.Warmup(10)
	if loaded != 5 {
		t.Errorf("expected 5 entries loaded from warmup, got %d", loaded)
	}
}

func TestCCCache_Clear(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	headers := http.Header{}
	cache.Set("c1", []byte("a"), 200, headers, "p", "m")
	cache.Set("c2", []byte("b"), 200, headers, "p", "m")

	cache.Clear()

	_, ok := cache.Get("c1")
	if ok {
		t.Error("cache should be empty after clear")
	}
}

func TestCCCache_RecordLatencySaved(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	cache.RecordLatencySaved(100)
	cache.RecordLatencySaved(200)

	stats := cache.Stats()
	if stats["latency_saved_ms"].(int64) != 300 {
		t.Errorf("expected 300ms saved, got %v", stats["latency_saved_ms"])
	}
}

func TestCCCache_DisabledCache(t *testing.T) {
	config := DefaultCacheConfig()
	config.Enabled = false
	cache := NewCCCache(config)
	defer cache.Stop()

	headers := http.Header{}
	cache.Set("disabled", []byte("data"), 200, headers, "p", "m")

	_, ok := cache.Get("disabled")
	if ok {
		t.Error("disabled cache should not return entries")
	}
}

func TestCCCache_MemoryOnly(t *testing.T) {
	config := DefaultCacheConfig()
	config.StorageType = "memory"
	cache := NewCCCache(config)
	defer cache.Stop()

	if cache.GetDisk() != nil {
		t.Error("memory-only cache should not have disk")
	}

	headers := http.Header{}
	cache.Set("mem", []byte("data"), 200, headers, "p", "m")

	entry, ok := cache.Get("mem")
	if !ok || string(entry.Body) != "data" {
		t.Error("memory-only cache should work for L1")
	}
}

func TestCCCache_Singleflight(t *testing.T) {
	cache := newTestCCCache(t)
	defer cache.Stop()

	sf := cache.GetSingleflight()
	if sf == nil {
		t.Error("singleflight should be initialized")
	}
}
