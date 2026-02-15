package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	// Use pure-Go sqlite driver; vec0 extension won't load but falls back gracefully
	_ "modernc.org/sqlite"
)

// MemoryChunk represents a chunk of memory with embedding.
type MemoryChunk struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Embedding []float32         `json:"embedding,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// VectorSearchResult represents a search result with similarity score.
type VectorSearchResult struct {
	Chunk      MemoryChunk `json:"chunk"`
	Score      float32     `json:"score"`
	MatchType  string      `json:"match_type"` // "vector", "keyword", "hybrid"
}

// VectorStoreConfig holds vector store configuration.
type VectorStoreConfig struct {
	DBPath         string
	EmbeddingDim   int
	MaxChunks      int
	EnableFTS      bool
	EnableVec      bool   // Enable sqlite-vec for fast vector search
	SimilarityFunc string // "cosine", "dot", "euclidean" (only used when sqlite-vec unavailable)
}

// VectorStore provides vector-based memory storage.
type VectorStore struct {
	db           *sql.DB
	embeddingDim int
	maxChunks    int
	enableFTS    bool
	enableVec    bool // sqlite-vec extension loaded
	mu           sync.RWMutex
	encryptor    *ContentEncryptor
}

// SetEncryptor sets the content encryptor for transparent encryption at rest.
func (vs *VectorStore) SetEncryptor(enc *ContentEncryptor) {
	vs.encryptor = enc
}

// decryptChunkContent decrypts a chunk's content if an encryptor is set.
func (s *VectorStore) decryptChunkContent(chunk *MemoryChunk) {
	if s.encryptor != nil {
		if dec, err := s.encryptor.Decrypt(chunk.Content); err == nil {
			chunk.Content = dec
		}
	}
}

const vectorStoreSchema = `
CREATE TABLE IF NOT EXISTS memory_chunks (
    id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    embedding TEXT,
    metadata TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_chunks_created_at ON memory_chunks(created_at);
CREATE INDEX IF NOT EXISTS idx_memory_chunks_updated_at ON memory_chunks(updated_at);
`

const ftsSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts USING fts5(
    id,
    content,
    content='memory_chunks',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(rowid, id, content) VALUES (NEW.rowid, NEW.id, NEW.content);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, id, content) VALUES('delete', OLD.rowid, OLD.id, OLD.content);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
    INSERT INTO memory_chunks_fts(memory_chunks_fts, rowid, id, content) VALUES('delete', OLD.rowid, OLD.id, OLD.content);
    INSERT INTO memory_chunks_fts(rowid, id, content) VALUES (NEW.rowid, NEW.id, NEW.content);
END;
`

// NewVectorStore creates a new vector store.
func NewVectorStore(cfg VectorStoreConfig) (*VectorStore, error) {
	db, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable WAL mode
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Create schema
	if _, err := db.Exec(vectorStoreSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create schema: %w", err)
	}

	// Create FTS table if enabled
	if cfg.EnableFTS {
		if _, err := db.Exec(ftsSchema); err != nil {
			// FTS might already exist, ignore error
			_ = err
		}
	}

	embeddingDim := cfg.EmbeddingDim
	if embeddingDim == 0 {
		embeddingDim = 1536 // Default OpenAI dimension
	}

	maxChunks := cfg.MaxChunks
	if maxChunks == 0 {
		maxChunks = 10000
	}

	// Try to load sqlite-vec extension for fast vector search
	enableVec := false
	if cfg.EnableVec {
		// Try different extension names
		for _, extName := range []string{"vec0", "sqlite-vec", "vec"} {
			_, err := db.Exec(fmt.Sprintf("SELECT load_extension('%s')", extName))
			if err == nil {
				enableVec = true
				break
			}
		}

		if enableVec {
			// Create vec0 virtual table for vector search
			vecTableSQL := fmt.Sprintf(`
				CREATE VIRTUAL TABLE IF NOT EXISTS memory_vectors USING vec0(
					id TEXT PRIMARY KEY,
					embedding float[%d]
				)
			`, embeddingDim)
			if _, err := db.Exec(vecTableSQL); err != nil {
				// Log but don't fail - fall back to in-memory search
				enableVec = false
			}
		}
	}

	return &VectorStore{
		db:           db,
		embeddingDim: embeddingDim,
		maxChunks:    maxChunks,
		enableFTS:    cfg.EnableFTS,
		enableVec:    enableVec,
	}, nil
}

// Store stores a memory chunk.
func (s *VectorStore) Store(ctx context.Context, content string, emb []float32, metadata map[string]string) (*MemoryChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chunk := &MemoryChunk{
		ID:        uuid.New().String(),
		Content:   content,
		Embedding: emb,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Encrypt content for storage
	storeContent := content
	if s.encryptor != nil {
		enc, err := s.encryptor.Encrypt(content)
		if err != nil {
			return nil, fmt.Errorf("encrypt content: %w", err)
		}
		storeContent = enc
	}

	var embJSON, metaJSON []byte
	var err error

	if len(emb) > 0 {
		embJSON, err = json.Marshal(emb)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal embedding: %w", err)
		}
	}

	if len(metadata) > 0 {
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO memory_chunks (id, content, embedding, metadata, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		chunk.ID, storeContent, string(embJSON), string(metaJSON), chunk.CreatedAt, chunk.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store chunk: %w", err)
	}

	// Store in vec0 table if sqlite-vec is enabled
	if s.enableVec && len(emb) > 0 {
		_, err = s.db.ExecContext(ctx,
			"INSERT OR REPLACE INTO memory_vectors (id, embedding) VALUES (?, ?)",
			chunk.ID, string(embJSON),
		)
		if err != nil {
			// Log but don't fail - vector search will fall back to in-memory
			_ = err
		}
	}

	// Prune if needed
	go s.pruneIfNeeded(context.Background())

	return chunk, nil
}

// StoreBatch stores multiple memory chunks.
func (s *VectorStore) StoreBatch(ctx context.Context, chunks []MemoryChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx,
		"INSERT INTO memory_chunks (id, content, embedding, metadata, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for i := range chunks {
		if chunks[i].ID == "" {
			chunks[i].ID = uuid.New().String()
		}
		chunks[i].CreatedAt = now
		chunks[i].UpdatedAt = now

		var embJSON, metaJSON []byte
		if len(chunks[i].Embedding) > 0 {
			embJSON, _ = json.Marshal(chunks[i].Embedding)
		}
		if len(chunks[i].Metadata) > 0 {
			metaJSON, _ = json.Marshal(chunks[i].Metadata)
		}

		// Encrypt content for storage
		storeContent := chunks[i].Content
		if s.encryptor != nil {
			if enc, err := s.encryptor.Encrypt(storeContent); err == nil {
				storeContent = enc
			}
		}

		_, err = stmt.ExecContext(ctx, chunks[i].ID, storeContent, string(embJSON), string(metaJSON), chunks[i].CreatedAt, chunks[i].UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to store chunk: %w", err)
		}

		// Store in vec0 table if sqlite-vec is enabled
		if s.enableVec && len(chunks[i].Embedding) > 0 {
			tx.ExecContext(ctx, "INSERT OR REPLACE INTO memory_vectors (id, embedding) VALUES (?, ?)",
				chunks[i].ID, string(embJSON))
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	go s.pruneIfNeeded(context.Background())

	return nil
}

// Get retrieves a memory chunk by ID.
func (s *VectorStore) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chunk MemoryChunk
	var embJSON, metaJSON sql.NullString

	err := s.db.QueryRowContext(ctx,
		"SELECT id, content, embedding, metadata, created_at, updated_at FROM memory_chunks WHERE id = ?",
		id,
	).Scan(&chunk.ID, &chunk.Content, &embJSON, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chunk: %w", err)
	}

	if embJSON.Valid && embJSON.String != "" {
		if err := json.Unmarshal([]byte(embJSON.String), &chunk.Embedding); err != nil {
			return nil, fmt.Errorf("failed to unmarshal embedding: %w", err)
		}
	}

	if metaJSON.Valid && metaJSON.String != "" {
		if err := json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	s.decryptChunkContent(&chunk)

	return &chunk, nil
}

// GetAll retrieves all memory chunks.
func (s *VectorStore) GetAll(ctx context.Context) ([]MemoryChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, content, metadata, created_at, updated_at FROM memory_chunks ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []MemoryChunk
	for rows.Next() {
		var chunk MemoryChunk
		var metaJSON sql.NullString

		if err := rows.Scan(&chunk.ID, &chunk.Content, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt); err != nil {
			continue
		}

		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata)
		}

		s.decryptChunkContent(&chunk)
		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// Delete deletes a memory chunk.
func (s *VectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM memory_chunks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete chunk: %w", err)
	}

	// Delete from vec0 table if sqlite-vec is enabled
	if s.enableVec {
		s.db.ExecContext(ctx, "DELETE FROM memory_vectors WHERE id = ?", id)
	}

	return nil
}

// SearchVector performs vector similarity search.
func (s *VectorStore) SearchVector(ctx context.Context, queryEmb []float32, limit int, minScore float32) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use sqlite-vec for fast KNN search if available
	if s.enableVec {
		return s.searchVectorVec(ctx, queryEmb, limit, minScore)
	}

	// Fallback to in-memory search
	return s.searchVectorFallback(ctx, queryEmb, limit, minScore)
}

// searchVectorVec uses sqlite-vec for fast KNN vector search.
func (s *VectorStore) searchVectorVec(ctx context.Context, queryEmb []float32, limit int, minScore float32) ([]VectorSearchResult, error) {
	embJSON, _ := json.Marshal(queryEmb)

	// Use sqlite-vec's KNN search with vec_distance_cosine
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			v.id,
			m.content,
			m.metadata,
			m.created_at,
			m.updated_at,
			vec_distance_cosine(v.embedding, ?) as distance
		FROM memory_vectors v
		JOIN memory_chunks m ON v.id = m.id
		ORDER BY distance ASC
		LIMIT ?
	`, string(embJSON), limit)
	if err != nil {
		// Fall back to in-memory search if sqlite-vec query fails
		return s.searchVectorFallback(ctx, queryEmb, limit, minScore)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var chunk MemoryChunk
		var metaJSON sql.NullString
		var distance float64

		if err := rows.Scan(&chunk.ID, &chunk.Content, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt, &distance); err != nil {
			continue
		}

		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata)
		}

		// Convert distance to similarity score (1 - distance for cosine)
		score := float32(1.0 - distance)
		if score >= minScore {
			s.decryptChunkContent(&chunk)
			results = append(results, VectorSearchResult{
				Chunk:     chunk,
				Score:     score,
				MatchType: "vector",
			})
		}
	}

	return results, nil
}

// searchVectorFallback performs in-memory vector similarity search.
func (s *VectorStore) searchVectorFallback(ctx context.Context, queryEmb []float32, limit int, minScore float32) ([]VectorSearchResult, error) {
	// Load all chunks with embeddings
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, content, embedding, metadata, created_at, updated_at FROM memory_chunks WHERE embedding IS NOT NULL AND embedding != ''",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunks: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult

	for rows.Next() {
		var chunk MemoryChunk
		var embJSON, metaJSON sql.NullString

		if err := rows.Scan(&chunk.ID, &chunk.Content, &embJSON, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt); err != nil {
			continue
		}

		if !embJSON.Valid || embJSON.String == "" {
			continue
		}

		if err := json.Unmarshal([]byte(embJSON.String), &chunk.Embedding); err != nil {
			continue
		}

		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata)
		}

		// Calculate cosine similarity in Go
		score := cosineSimilarity(queryEmb, chunk.Embedding)
		if score >= minScore {
			s.decryptChunkContent(&chunk)
			results = append(results, VectorSearchResult{
				Chunk:     chunk,
				Score:     score,
				MatchType: "vector",
			})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Limit results
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// cosineSimilarity calculates cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// sqrt32 calculates square root for float32.
func sqrt32(x float32) float32 {
	if x <= 0 {
		return 0
	}
	// Newton's method
	z := x / 2
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// SearchKeyword performs FTS5 keyword search.
func (s *VectorStore) SearchKeyword(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	if !s.enableFTS {
		return s.searchKeywordFallback(ctx, query, limit)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.content, m.embedding, m.metadata, m.created_at, m.updated_at, bm25(memory_chunks_fts) as score
		FROM memory_chunks_fts f
		JOIN memory_chunks m ON f.id = m.id
		WHERE memory_chunks_fts MATCH ?
		ORDER BY score
		LIMIT ?
	`, query, limit)
	if err != nil {
		// Fall back to LIKE search if FTS fails
		return s.searchKeywordFallback(ctx, query, limit)
	}
	defer rows.Close()

	var results []VectorSearchResult

	for rows.Next() {
		var chunk MemoryChunk
		var embJSON, metaJSON sql.NullString
		var score float64

		if err := rows.Scan(&chunk.ID, &chunk.Content, &embJSON, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt, &score); err != nil {
			continue
		}

		if embJSON.Valid && embJSON.String != "" {
			json.Unmarshal([]byte(embJSON.String), &chunk.Embedding)
		}
		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata)
		}

		// BM25 returns negative scores, lower is better
		// Convert to positive score where higher is better
		s.decryptChunkContent(&chunk)
		results = append(results, VectorSearchResult{
			Chunk:     chunk,
			Score:     float32(-score),
			MatchType: "keyword",
		})
	}

	return results, nil
}

// searchKeywordFallback uses LIKE for keyword search when FTS is not available.
func (s *VectorStore) searchKeywordFallback(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.QueryContext(ctx,
		"SELECT id, content, embedding, metadata, created_at, updated_at FROM memory_chunks WHERE content LIKE ? ORDER BY updated_at DESC LIMIT ?",
		"%"+query+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search chunks: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult

	for rows.Next() {
		var chunk MemoryChunk
		var embJSON, metaJSON sql.NullString

		if err := rows.Scan(&chunk.ID, &chunk.Content, &embJSON, &metaJSON, &chunk.CreatedAt, &chunk.UpdatedAt); err != nil {
			continue
		}

		if embJSON.Valid && embJSON.String != "" {
			json.Unmarshal([]byte(embJSON.String), &chunk.Embedding)
		}
		if metaJSON.Valid && metaJSON.String != "" {
			json.Unmarshal([]byte(metaJSON.String), &chunk.Metadata)
		}

		s.decryptChunkContent(&chunk)
		results = append(results, VectorSearchResult{
			Chunk:     chunk,
			Score:     1.0, // Default score for LIKE matches
			MatchType: "keyword",
		})
	}

	return results, nil
}

// Update updates a memory chunk.
func (s *VectorStore) Update(ctx context.Context, id string, content string, emb []float32, metadata map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Encrypt content for storage
	storeContent := content
	if s.encryptor != nil {
		enc, err := s.encryptor.Encrypt(content)
		if err != nil {
			return fmt.Errorf("encrypt content: %w", err)
		}
		storeContent = enc
	}

	var embJSON, metaJSON []byte
	var err error

	if len(emb) > 0 {
		embJSON, err = json.Marshal(emb)
		if err != nil {
			return fmt.Errorf("failed to marshal embedding: %w", err)
		}
	}

	if len(metadata) > 0 {
		metaJSON, err = json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx,
		"UPDATE memory_chunks SET content = ?, embedding = ?, metadata = ?, updated_at = ? WHERE id = ?",
		storeContent, string(embJSON), string(metaJSON), time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update chunk: %w", err)
	}

	return nil
}

// Count returns the number of chunks.
func (s *VectorStore) Count(ctx context.Context) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_chunks").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count chunks: %w", err)
	}
	return count, nil
}

// Prune removes old chunks to stay within maxChunks.
func (s *VectorStore) Prune(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.pruneUnsafe(ctx)
}

// pruneIfNeeded prunes if the store exceeds maxChunks.
func (s *VectorStore) pruneIfNeeded(ctx context.Context) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_chunks").Scan(&count)
	if err != nil || count <= s.maxChunks {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = s.pruneUnsafe(ctx)
}

// pruneUnsafe prunes without locking (caller must hold lock).
func (s *VectorStore) pruneUnsafe(ctx context.Context) (int, error) {
	// Delete oldest chunks to get back to 80% of maxChunks
	targetCount := int(float64(s.maxChunks) * 0.8)

	result, err := s.db.ExecContext(ctx, `
		DELETE FROM memory_chunks WHERE id IN (
			SELECT id FROM memory_chunks
			ORDER BY updated_at ASC
			LIMIT (
				SELECT MAX(0, COUNT(*) - ?) FROM memory_chunks
			)
		)
	`, targetCount)
	if err != nil {
		return 0, fmt.Errorf("failed to prune chunks: %w", err)
	}

	deleted, _ := result.RowsAffected()
	return int(deleted), nil
}

// Clear removes all chunks.
func (s *VectorStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, "DELETE FROM memory_chunks")
	if err != nil {
		return fmt.Errorf("failed to clear chunks: %w", err)
	}

	// Clear vec0 table if sqlite-vec is enabled
	if s.enableVec {
		s.db.ExecContext(ctx, "DELETE FROM memory_vectors")
	}

	return nil
}

// Close closes the database connection.
func (s *VectorStore) Close() error {
	return s.db.Close()
}

// Stats returns vector store statistics.
func (s *VectorStore) Stats(ctx context.Context) (VectorStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats VectorStoreStats
	stats.MaxChunks = s.maxChunks
	stats.EmbeddingDim = s.embeddingDim
	stats.FTSEnabled = s.enableFTS
	stats.VecEnabled = s.enableVec

	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memory_chunks").Scan(&stats.ChunkCount)
	if err != nil {
		return stats, err
	}

	err = s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM memory_chunks WHERE embedding IS NOT NULL AND embedding != ''",
	).Scan(&stats.EmbeddedCount)
	if err != nil {
		return stats, err
	}

	s.db.QueryRowContext(ctx,
		"SELECT MIN(created_at), MAX(updated_at) FROM memory_chunks",
	).Scan(&stats.OldestChunk, &stats.NewestChunk)

	return stats, nil
}

// VectorStoreStats holds vector store statistics.
type VectorStoreStats struct {
	ChunkCount    int       `json:"chunk_count"`
	EmbeddedCount int       `json:"embedded_count"`
	MaxChunks     int       `json:"max_chunks"`
	EmbeddingDim  int       `json:"embedding_dim"`
	FTSEnabled    bool      `json:"fts_enabled"`
	VecEnabled    bool      `json:"vec_enabled"` // sqlite-vec extension loaded
	OldestChunk   time.Time `json:"oldest_chunk,omitempty"`
	NewestChunk   time.Time `json:"newest_chunk,omitempty"`
}

// IsVecEnabled returns whether sqlite-vec is enabled.
func (s *VectorStore) IsVecEnabled() bool {
	return s.enableVec
}
