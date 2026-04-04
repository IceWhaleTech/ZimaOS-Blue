//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	if _, err := store.Store(ctx, "another memory", nil, map[string]string{"tag_0": "other"}); err != nil {
		t.Fatalf("Store seed: %v", err)
	}

	chunk, err := store.Store(ctx, "remember exact phrase", nil, map[string]string{"tag_0": "smoke", "source_id": "daily/2026-03-08.md"})
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
	if _, err := store.Store(ctx, "vector memory", queryVec, map[string]string{"source_id": "daily/2026-03-08.md"}); err != nil {
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
		if _, err := store.Store(ctx, content, nil, map[string]string{"source_id": "daily/2026-03-08.md"}); err != nil {
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

func TestVectorStoreStoreRepairsStaleVecPrimaryKeyCollision(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector-stale-vec.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	emb := []float32{0.1, -0.2, 0.3, -0.4}
	serialized := serializeFloat32(emb)

	if _, err := store.db.Exec(
		`INSERT INTO memory_vec (rowid, embedding) VALUES (?, vec_quantize_int8(?, 'unit'))`,
		1,
		serialized,
	); err != nil {
		t.Fatalf("seed stale vec row: %v", err)
	}

	chunk, err := store.Store(ctx, "repair stale vec row", emb, map[string]string{"source_id": "daily/2026-03-08.md"})
	if err != nil {
		t.Fatalf("Store should recover from stale vec row collision: %v", err)
	}
	if chunk == nil {
		t.Fatal("expected stored chunk")
	}

	results, err := store.SearchVector(ctx, emb, 5, 0)
	if err != nil {
		t.Fatalf("SearchVector after repair: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected vector search results after repair")
	}
	if results[0].Chunk.Content != "repair stale vec row" {
		t.Fatalf("unexpected top vector result: %+v", results[0])
	}
}

func TestVectorStoreUsesReaderDBForReads(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector-reader.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	t.Cleanup(func() {
		if store.readDB != nil && store.readDB != store.db {
			_ = store.readDB.Close()
		}
		if store.db != nil {
			_ = store.db.Close()
		}
	})

	if store.readDB == nil {
		t.Fatal("expected vector store reader db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected vector store to use a separate read db")
	}

	ctx := context.Background()
	queryVec := []float32{0.1, -0.2, 0.3, -0.4}
	chunk, err := store.Store(ctx, "reader backed vector memory", queryVec, nil)
	if err != nil {
		t.Fatalf("Store with embedding: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := store.Get(ctx, chunk.ID)
	if err != nil {
		t.Fatalf("Get via reader: %v", err)
	}
	if got == nil || got.ID != chunk.ID {
		t.Fatalf("unexpected chunk via reader: %+v", got)
	}

	vectorResults, err := store.SearchVector(ctx, queryVec, 5, 0)
	if err != nil {
		t.Fatalf("SearchVector via reader: %v", err)
	}
	if len(vectorResults) == 0 || vectorResults[0].Chunk.ID != chunk.ID {
		t.Fatalf("unexpected vector results via reader: %+v", vectorResults)
	}

	keywordResults, err := store.SearchKeyword(ctx, "reader backed", 5)
	if err != nil {
		t.Fatalf("SearchKeyword via reader: %v", err)
	}
	if len(keywordResults) == 0 || keywordResults[0].Chunk.ID != chunk.ID {
		t.Fatalf("unexpected keyword results via reader: %+v", keywordResults)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats via reader: %v", err)
	}
	if stats.ChunkCount != 1 {
		t.Fatalf("stats chunk_count = %d, want 1", stats.ChunkCount)
	}
}

func TestVectorStoreUsesReaderDBForFTSKeywordSearch(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector-reader-fts.db"),
		EmbeddingDim: 4,
		MaxChunks:    10,
		EnableFTS:    true,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	t.Cleanup(func() {
		if store.readDB != nil && store.readDB != store.db {
			_ = store.readDB.Close()
		}
		if store.db != nil {
			_ = store.db.Close()
		}
	})

	if !store.enableFTS {
		t.Skip("fts is not available in this sqlite build")
	}

	ctx := context.Background()
	chunk, err := store.Store(ctx, "reader backed keyword memory", nil, nil)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	results, err := store.SearchKeyword(ctx, "reader backed", 5)
	if err != nil {
		t.Fatalf("SearchKeyword via reader FTS: %v", err)
	}
	if !store.enableFTS {
		t.Fatal("enableFTS disabled during reader-backed FTS search")
	}
	if len(results) == 0 || results[0].Chunk.ID != chunk.ID {
		t.Fatalf("unexpected FTS results via reader: %+v", results)
	}
}

func TestVectorStorePruneKeepsNewestChunks(t *testing.T) {
	store, err := NewVectorStore(VectorStoreConfig{
		DBPath:       filepath.Join(t.TempDir(), "vector-prune.db"),
		EmbeddingDim: 4,
		MaxChunks:    2,
		EnableFTS:    false,
	})
	if err != nil {
		t.Fatalf("NewVectorStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	oldest, err := store.Store(ctx, "oldest chunk", nil, nil)
	if err != nil {
		t.Fatalf("Store(oldest): %v", err)
	}
	middle, err := store.Store(ctx, "middle chunk", nil, nil)
	if err != nil {
		t.Fatalf("Store(middle): %v", err)
	}
	newest, err := store.Store(ctx, "newest chunk", nil, nil)
	if err != nil {
		t.Fatalf("Store(newest): %v", err)
	}

	base := time.Date(2026, time.March, 28, 10, 0, 0, 0, time.UTC)
	for _, item := range []struct {
		id string
		at time.Time
	}{
		{id: oldest.ID, at: base},
		{id: middle.ID, at: base.Add(time.Minute)},
		{id: newest.ID, at: base.Add(2 * time.Minute)},
	} {
		if _, err := store.db.Exec(
			`UPDATE memory_chunks SET created_at = ?, updated_at = ? WHERE chunk_id = ?`,
			item.at, item.at, item.id,
		); err != nil {
			t.Fatalf("update timestamps for %s: %v", item.id, err)
		}
	}

	pruned, err := store.Prune(ctx)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}

	if _, err := store.Get(ctx, oldest.ID); err != ErrNotFound {
		t.Fatalf("Get(oldest) error = %v, want %v", err, ErrNotFound)
	}
	if _, err := store.Get(ctx, middle.ID); err != nil {
		t.Fatalf("Get(middle): %v", err)
	}
	if _, err := store.Get(ctx, newest.ID); err != nil {
		t.Fatalf("Get(newest): %v", err)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats after prune: %v", err)
	}
	if stats.ChunkCount != 2 {
		t.Fatalf("stats chunk_count = %d, want 2", stats.ChunkCount)
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

	if _, err := store.Store(context.Background(), "write after cleanup", nil, nil); err != nil {
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
