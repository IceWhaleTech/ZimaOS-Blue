package pruner

import (
	"path/filepath"
	"testing"
	"time"
)

func TestDiskCache_PutGet(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 5*time.Minute)
	defer cache.Close()

	segments := []ScoredSegment{
		{Segment: Segment{StartLine: 0, EndLine: 5, Kind: SegmentParagraph, Content: "hello"}, Score: 0.9},
		{Segment: Segment{StartLine: 6, EndLine: 10, Kind: SegmentHeading, Content: "world"}, Score: 0.5},
	}

	cache.Put("testkey", segments)

	got, ok := cache.Get("testkey")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(got))
	}
	if got[0].Score != 0.9 {
		t.Errorf("expected score 0.9, got %f", got[0].Score)
	}
	if got[1].Segment.Kind != SegmentHeading {
		t.Errorf("expected SegmentHeading, got %v", got[1].Segment.Kind)
	}
}

func TestDiskCache_Miss(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 5*time.Minute)
	defer cache.Close()

	_, ok := cache.Get("nonexistent")
	if ok {
		t.Error("expected cache miss")
	}
}

func TestDiskCache_TTLExpiration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 1*time.Millisecond)
	defer cache.Close()

	segments := []ScoredSegment{
		{Segment: Segment{Content: "test"}, Score: 1.0},
	}
	cache.Put("expiring", segments)

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("expiring")
	if ok {
		t.Error("expected cache miss after TTL expiration")
	}
}

func TestDiskCache_Overwrite(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 5*time.Minute)
	defer cache.Close()

	seg1 := []ScoredSegment{{Segment: Segment{Content: "v1"}, Score: 0.5}}
	seg2 := []ScoredSegment{{Segment: Segment{Content: "v2"}, Score: 0.9}}

	cache.Put("key", seg1)
	cache.Put("key", seg2)

	got, ok := cache.Get("key")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got[0].Score != 0.9 {
		t.Errorf("expected updated score 0.9, got %f", got[0].Score)
	}
}

func TestDiskCache_EmptyPath(t *testing.T) {
	cache := NewDiskCache("", 5*time.Minute)

	// Put should not panic
	cache.Put("key", []ScoredSegment{{Score: 1.0}})

	// Get should return miss (no-op cache)
	_, ok := cache.Get("key")
	if ok {
		t.Error("expected cache miss for empty path")
	}
}

func TestDiskCache_ConcurrentAccess(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 5*time.Minute)
	defer cache.Close()

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			key := "key"
			segs := []ScoredSegment{{Score: float64(n)}}
			cache.Put(key, segs)
			cache.Get(key)
			done <- true
		}(i)
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify we can still read
	_, ok := cache.Get("key")
	if !ok {
		t.Error("expected cache hit after concurrent writes")
	}
}

func TestDiskCache_Cleanup(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 1*time.Millisecond)
	defer cache.Close()

	cache.Put("old", []ScoredSegment{{Score: 1.0}})
	time.Sleep(5 * time.Millisecond)

	n := cache.Cleanup()
	if n == 0 {
		t.Error("expected at least 1 expired entry cleaned up")
	}
}

func TestDiskCache_UsesReaderPoolForFileDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pruner_cache.db")
	cache := NewDiskCache(dbPath, 5*time.Minute)
	defer cache.Close()

	if cache.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if cache.readDB == cache.db {
		t.Fatal("expected file-backed disk cache to use a separate read db")
	}

	cache.Put("reader-key", []ScoredSegment{{Segment: Segment{Content: "reader"}, Score: 1.0}})

	if err := cache.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, ok := cache.Get("reader-key")
	if !ok {
		t.Fatal("expected cache hit via reader pool")
	}
	if len(got) != 1 || got[0].Segment.Content != "reader" {
		t.Fatalf("unexpected cached segments via reader pool: %+v", got)
	}
}
