package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	// Note: sqlite-vec requires CGO sqlite driver (mattn/go-sqlite3).
	// This store is currently unused. To use it, build with CGO and
	// change the import below to: _ "github.com/mattn/go-sqlite3"
	_ "modernc.org/sqlite"
)

// SqliteVecStore provides vector storage using sqlite-vec extension.
// sqlite-vec enables efficient HNSW-based vector similarity search.
type SqliteVecStore struct {
	db           *sql.DB
	embeddingDim int
	mu           sync.RWMutex
}

// SqliteVecConfig holds configuration for sqlite-vec store.
type SqliteVecConfig struct {
	DBPath       string
	EmbeddingDim int  // Default: 1536 (OpenAI)
	EnableHNSW   bool // Use HNSW index for faster search
}

// NewSqliteVecStore creates a new sqlite-vec based vector store.
func NewSqliteVecStore(cfg SqliteVecConfig) (*SqliteVecStore, error) {
	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 1536
	}

	// Open with CGO driver and load sqlite-vec extension
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Try to load sqlite-vec extension
	// The extension file should be in the system library path or specified explicitly
	_, err = db.Exec("SELECT load_extension('vec0')")
	if err != nil {
		// Try alternative extension names
		_, err = db.Exec("SELECT load_extension('sqlite-vec')")
		if err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to load sqlite-vec extension: %w (ensure vec0.so/dll is available)", err)
		}
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, err
	}

	store := &SqliteVecStore{
		db:           db,
		embeddingDim: cfg.EmbeddingDim,
	}

	if err := store.initSchema(cfg.EnableHNSW); err != nil {
		db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SqliteVecStore) initSchema(enableHNSW bool) error {
	// Create metadata table
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS memory_metadata (
			id TEXT PRIMARY KEY,
			content TEXT NOT NULL,
			metadata TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create metadata table: %w", err)
	}

	// Create FTS5 table for keyword search
	_, err = s.db.Exec(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
			id, content, tokenize='porter'
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create FTS table: %w", err)
	}

	// Create sqlite-vec virtual table for vector search
	// vec0 is the virtual table module provided by sqlite-vec
	vecTableSQL := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_vectors USING vec0(
			id TEXT PRIMARY KEY,
			embedding float[%d]
		)
	`, s.embeddingDim)

	_, err = s.db.Exec(vecTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create vector table: %w", err)
	}

	return nil
}

// Store stores a memory chunk with its embedding.
func (s *SqliteVecStore) Store(ctx context.Context, chunk *MemoryChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Store metadata
	metadataJSON, _ := json.Marshal(chunk.Metadata)
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO memory_metadata (id, content, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, chunk.ID, chunk.Content, string(metadataJSON), chunk.CreatedAt, chunk.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to store metadata: %w", err)
	}

	// Store in FTS
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO memory_fts (id, content) VALUES (?, ?)
	`, chunk.ID, chunk.Content)
	if err != nil {
		return fmt.Errorf("failed to store FTS: %w", err)
	}

	// Store vector if embedding exists
	if len(chunk.Embedding) > 0 {
		embJSON, _ := json.Marshal(chunk.Embedding)
		_, err = tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO memory_vectors (id, embedding) VALUES (?, ?)
		`, chunk.ID, string(embJSON))
		if err != nil {
			return fmt.Errorf("failed to store vector: %w", err)
		}
	}

	return tx.Commit()
}

// SearchVector performs KNN vector similarity search using sqlite-vec.
func (s *SqliteVecStore) SearchVector(ctx context.Context, queryEmb []float32, limit int) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	embJSON, _ := json.Marshal(queryEmb)

	// Use sqlite-vec's KNN search
	// The vec_distance_cosine function calculates cosine distance
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			v.id,
			m.content,
			m.metadata,
			m.created_at,
			m.updated_at,
			vec_distance_cosine(v.embedding, ?) as distance
		FROM memory_vectors v
		JOIN memory_metadata m ON v.id = m.id
		ORDER BY distance ASC
		LIMIT ?
	`, string(embJSON), limit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var chunk MemoryChunk
		var metadataStr string
		var distance float64

		if err := rows.Scan(&chunk.ID, &chunk.Content, &metadataStr, &chunk.CreatedAt, &chunk.UpdatedAt, &distance); err != nil {
			continue
		}

		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &chunk.Metadata)
		}

		// Convert distance to similarity score (1 - distance for cosine)
		score := float32(1.0 - distance)

		results = append(results, VectorSearchResult{
			Chunk:     chunk,
			Score:     score,
			MatchType: "vector",
		})
	}

	return results, nil
}

// SearchKeyword performs FTS5 keyword search.
func (s *SqliteVecStore) SearchKeyword(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			m.id, m.content, m.metadata, m.created_at, m.updated_at,
			bm25(memory_fts) as score
		FROM memory_fts f
		JOIN memory_metadata m ON f.id = m.id
		WHERE memory_fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var chunk MemoryChunk
		var metadataStr string
		var score float64

		if err := rows.Scan(&chunk.ID, &chunk.Content, &metadataStr, &chunk.CreatedAt, &chunk.UpdatedAt, &score); err != nil {
			continue
		}

		if metadataStr != "" {
			json.Unmarshal([]byte(metadataStr), &chunk.Metadata)
		}

		results = append(results, VectorSearchResult{
			Chunk:     chunk,
			Score:     float32(-score), // BM25 returns negative scores
			MatchType: "keyword",
		})
	}

	return results, nil
}

// Get retrieves a memory by ID.
func (s *SqliteVecStore) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chunk MemoryChunk
	var metadataStr string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, content, metadata, created_at, updated_at
		FROM memory_metadata WHERE id = ?
	`, id).Scan(&chunk.ID, &chunk.Content, &metadataStr, &chunk.CreatedAt, &chunk.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if metadataStr != "" {
		json.Unmarshal([]byte(metadataStr), &chunk.Metadata)
	}

	return &chunk, nil
}

// Delete removes a memory by ID.
func (s *SqliteVecStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, "DELETE FROM memory_metadata WHERE id = ?", id)
	tx.ExecContext(ctx, "DELETE FROM memory_fts WHERE id = ?", id)
	tx.ExecContext(ctx, "DELETE FROM memory_vectors WHERE id = ?", id)

	return tx.Commit()
}

// Close closes the database connection.
func (s *SqliteVecStore) Close() error {
	return s.db.Close()
}

// Stats returns store statistics.
func (s *SqliteVecStore) Stats(ctx context.Context) (*VectorStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats VectorStoreStats
	stats.EmbeddingDim = s.embeddingDim

	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_metadata").Scan(&stats.ChunkCount)
	s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_vectors").Scan(&stats.EmbeddedCount)

	var oldest, newest time.Time
	s.db.QueryRowContext(ctx, "SELECT MIN(created_at) FROM memory_metadata").Scan(&oldest)
	s.db.QueryRowContext(ctx, "SELECT MAX(created_at) FROM memory_metadata").Scan(&newest)
	stats.OldestChunk = oldest
	stats.NewestChunk = newest

	return &stats, nil
}
