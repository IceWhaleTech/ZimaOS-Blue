package embedding

import (
	"context"
	"path/filepath"
	"testing"
)

func TestCache_SetGetAndStats(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		DBPath:     filepath.Join(t.TempDir(), "embedding-cache.db"),
		Provider:   "test-provider",
		Model:      "test-model",
		MaxEntries: 10,
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	defer cache.Close()

	ctx := context.Background()
	vector := []float32{0.1, 0.2, 0.3}
	if err := cache.Set(ctx, "hash-1", vector); err != nil {
		t.Fatalf("set cache entry: %v", err)
	}

	got, ok := cache.Get(ctx, "hash-1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if len(got) != len(vector) || got[2] != vector[2] {
		t.Fatalf("unexpected embedding: %+v", got)
	}

	stats, err := cache.Stats(ctx)
	if err != nil {
		t.Fatalf("cache stats: %v", err)
	}
	if stats.EntryCount != 1 {
		t.Fatalf("stats EntryCount = %d, want 1", stats.EntryCount)
	}
	if stats.Provider != "test-provider" || stats.Model != "test-model" {
		t.Fatalf("unexpected stats metadata: %+v", stats)
	}
	if stats.OldestEntry.IsZero() || stats.NewestAccess.IsZero() {
		t.Fatalf("expected stats timestamps, got %+v", stats)
	}
}

func TestCache_GetBatchUsesReaderPool(t *testing.T) {
	cache, err := NewCache(CacheConfig{
		DBPath:     filepath.Join(t.TempDir(), "embedding-cache.db"),
		Provider:   "batch-provider",
		Model:      "batch-model",
		MaxEntries: 10,
	})
	if err != nil {
		t.Fatalf("new cache: %v", err)
	}
	defer cache.Close()

	if cache.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if cache.readDB == cache.db {
		t.Fatal("expected file-backed cache to use separate read db")
	}

	ctx := context.Background()
	if err := cache.SetBatch(ctx, map[string][]float32{
		"hash-a": {1, 2, 3},
		"hash-b": {4, 5, 6},
	}); err != nil {
		t.Fatalf("set batch: %v", err)
	}

	if err := cache.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := cache.GetBatch(ctx, []string{"hash-a", "hash-b", "missing"})
	if err != nil {
		t.Fatalf("get batch: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 cached embeddings, got %d", len(got))
	}
	if got["hash-a"][0] != 1 || got["hash-b"][2] != 6 {
		t.Fatalf("unexpected batch embeddings: %+v", got)
	}

	stats, err := cache.Stats(ctx)
	if err != nil {
		t.Fatalf("stats via reader: %v", err)
	}
	if stats.EntryCount != 2 {
		t.Fatalf("stats EntryCount = %d, want 2", stats.EntryCount)
	}
}
