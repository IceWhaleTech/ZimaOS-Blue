//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestVectorStoreSearchKeywordFallsBackWithoutFTS(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.Store(ctx, "another memory", nil, map[string]string{"tag_0": "other"}, ""); err != nil {
		t.Fatalf("Store seed: %v", err)
	}

	chunk, err := store.Store(ctx, "remember exact phrase", nil, map[string]string{"tag_0": "smoke", "source_id": "daily/2026-03-08.md"}, "")
	if err != nil {
		t.Fatalf("Store source chunk: %v", err)
	}
	if chunk.ID == "daily/2026-03-08.md" {
		t.Fatalf("vector chunk id should stay unique internally, got %q", chunk.ID)
	}

	results, err := store.SearchKeyword(ctx, "exact phrase", 5)
	if err != nil {
		t.Fatalf("SearchKeyword content fallback: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected content fallback results")
	}
	if results[0].Chunk.Content != "remember exact phrase" {
		t.Fatalf("unexpected top content result: %+v", results[0])
	}
	if results[0].Chunk.ID != "daily/2026-03-08.md" {
		t.Fatalf("expected external source id, got %q", results[0].Chunk.ID)
	}

	results, err = store.SearchKeyword(ctx, "smoke", 5)
	if err != nil {
		t.Fatalf("SearchKeyword tag fallback: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected tag fallback results")
	}
	if results[0].Chunk.Content != "remember exact phrase" {
		t.Fatalf("unexpected top tag result: %+v", results[0])
	}
}

func TestVectorStoreStoresQuantizedVectorsForInt8Column(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	queryVec := []float32{0.1, -0.2, 0.3, -0.4}
	if _, err := store.Store(ctx, "vector memory", queryVec, map[string]string{"source_id": "daily/2026-03-08.md"}, "test-model"); err != nil {
		t.Fatalf("Store with embedding: %v", err)
	}

	results, err := store.SearchVector(ctx, queryVec, 5, 0)
	if err != nil {
		t.Fatalf("SearchVector: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected vector search results")
	}
	if results[0].Chunk.ID != "daily/2026-03-08.md" {
		t.Fatalf("expected external source id, got %q", results[0].Chunk.ID)
	}
}

func TestVectorStoreDeleteRemovesRowsBySourceID(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	for _, content := range []string{"first same file", "second same file"} {
		if _, err := store.Store(ctx, content, nil, map[string]string{"source_id": "daily/2026-03-08.md"}, ""); err != nil {
			t.Fatalf("Store %q: %v", content, err)
		}
	}

	if err := store.Delete(ctx, "daily/2026-03-08.md"); err != nil {
		t.Fatalf("Delete by source id: %v", err)
	}

	results, err := store.SearchKeyword(ctx, "same file", 5)
	if err != nil {
		t.Fatalf("Search after delete: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected rows to be deleted, got %d results", len(results))
	}
}

func TestNewVectorStoreDropsStaleFTSTriggersWhenFTSDisabled(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vector.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	if _, err := db.Exec(vectorStoreSchema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TRIGGER memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
			SELECT RAISE(FAIL, 'fts unavailable');
		END;
	`); err != nil {
		t.Fatalf("create stale trigger: %v", err)
	}
	_ = db.Close()

	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       dbPath,
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	if _, err := store.Store(context.Background(), "write after cleanup", nil, nil, ""); err != nil {
		t.Fatalf("Store after cleanup: %v", err)
	}
}

func TestNewVectorStoreCreatesMissingParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "path", "vector.db")
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       dbPath,
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("expected sqlite db file at %s: %v", dbPath, err)
	}
}
