package proxy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestDiskCache(t *testing.T) (*DiskCache, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test_cache.db")
	dc, err := NewDiskCache(path)
	if err != nil {
		t.Fatalf("NewDiskCache: %v", err)
	}
	return dc, func() { dc.Close() }
}

func TestDiskCache_SetAndGet(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	entry := &CCCacheEntry{
		Key:         "test-key-1",
		Body:        []byte(`{"result":"ok"}`),
		StatusCode:  200,
		Headers:     map[string]string{"Content-Type": "application/json"},
		Provider:    "anthropic",
		Model:       "claude-3-opus",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		ContentHash: "abc123",
	}

	dc.Set(entry)

	got, ok := dc.Get("test-key-1")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if string(got.Body) != `{"result":"ok"}` {
		t.Errorf("body mismatch: %s", got.Body)
	}
	if got.StatusCode != 200 {
		t.Errorf("status code mismatch: %d", got.StatusCode)
	}
	if got.Provider != "anthropic" {
		t.Errorf("provider mismatch: %s", got.Provider)
	}
}

func TestDiskCache_GetExpired(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	entry := &CCCacheEntry{
		Key:       "expired-key",
		Body:      []byte("old"),
		ExpiresAt: time.Now().Add(-1 * time.Minute), // Already expired
		CreatedAt: time.Now().Add(-2 * time.Minute),
	}
	dc.Set(entry)

	_, ok := dc.Get("expired-key")
	if ok {
		t.Error("expired entry should not be returned")
	}
}

func TestDiskCache_GetMissing(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	_, ok := dc.Get("nonexistent")
	if ok {
		t.Error("missing key should return false")
	}
}

func TestDiskCache_Delete(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	entry := &CCCacheEntry{
		Key:       "del-key",
		Body:      []byte("data"),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	}
	dc.Set(entry)

	dc.Delete("del-key")

	_, ok := dc.Get("del-key")
	if ok {
		t.Error("deleted entry should not be found")
	}
}

func TestDiskCache_Clear(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	for i := 0; i < 5; i++ {
		dc.Set(&CCCacheEntry{
			Key:       "clear-" + string(rune('a'+i)),
			Body:      []byte("data"),
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now(),
		})
	}

	if dc.Count() != 5 {
		t.Errorf("expected 5 entries, got %d", dc.Count())
	}

	dc.Clear()

	if dc.Count() != 0 {
		t.Errorf("expected 0 entries after clear, got %d", dc.Count())
	}
}

func TestDiskCache_Cleanup(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	// Add expired and valid entries
	dc.Set(&CCCacheEntry{
		Key:       "valid",
		Body:      []byte("ok"),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	})
	dc.Set(&CCCacheEntry{
		Key:       "expired",
		Body:      []byte("old"),
		ExpiresAt: time.Now().Add(-1 * time.Minute),
		CreatedAt: time.Now().Add(-2 * time.Minute),
	})

	removed := dc.Cleanup()
	if removed != 1 {
		t.Errorf("expected 1 expired entry removed, got %d", removed)
	}
	if dc.Count() != 1 {
		t.Errorf("expected 1 remaining entry, got %d", dc.Count())
	}
}

func TestDiskCache_TopN(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	// Insert entries with different hit counts
	for i := 0; i < 5; i++ {
		dc.Set(&CCCacheEntry{
			Key:       "top-" + string(rune('a'+i)),
			Body:      []byte("data"),
			ExpiresAt: time.Now().Add(30 * time.Minute),
			CreatedAt: time.Now(),
			HitCount:  int64(i * 10),
		})
	}

	top := dc.TopN(3)
	if len(top) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(top))
	}

	// Should be ordered by hit_count DESC
	if top[0].HitCount < top[1].HitCount || top[1].HitCount < top[2].HitCount {
		t.Error("TopN should return entries ordered by hit_count DESC")
	}
}

func TestDiskCache_Count(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	if dc.Count() != 0 {
		t.Error("empty cache should have count 0")
	}

	dc.Set(&CCCacheEntry{
		Key:       "count-1",
		Body:      []byte("a"),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	})

	if dc.Count() != 1 {
		t.Errorf("expected count 1, got %d", dc.Count())
	}
}

func TestDiskCache_Upsert(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	entry := &CCCacheEntry{
		Key:       "upsert-key",
		Body:      []byte("v1"),
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	}
	dc.Set(entry)

	// Update same key
	entry.Body = []byte("v2")
	dc.Set(entry)

	got, ok := dc.Get("upsert-key")
	if !ok {
		t.Fatal("expected to find entry")
	}
	if string(got.Body) != "v2" {
		t.Errorf("expected updated body 'v2', got %s", got.Body)
	}
	if dc.Count() != 1 {
		t.Errorf("upsert should not create duplicate, count=%d", dc.Count())
	}
}

func TestDiskCache_InvalidPath(t *testing.T) {
	_, err := NewDiskCache("/nonexistent/path/to/cache.db")
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestDiskCache_HeadersPersistence(t *testing.T) {
	dc, cleanup := newTestDiskCache(t)
	defer cleanup()

	headers := map[string]string{
		"Content-Type":  "application/json",
		"X-Custom":      "value",
		"Cache-Control": "no-store",
	}

	dc.Set(&CCCacheEntry{
		Key:       "headers-key",
		Body:      []byte("body"),
		Headers:   headers,
		ExpiresAt: time.Now().Add(30 * time.Minute),
		CreatedAt: time.Now(),
	})

	got, ok := dc.Get("headers-key")
	if !ok {
		t.Fatal("expected to find entry")
	}
	for k, v := range headers {
		if got.Headers[k] != v {
			t.Errorf("header %s: expected %q, got %q", k, v, got.Headers[k])
		}
	}
}

// Ensure unused import doesn't cause issues
var _ = os.TempDir
