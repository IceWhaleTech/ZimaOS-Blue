//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func init() {
	sqlite_vec.Auto()
}

// quantizeFloat32ToInt8 converts float32 embeddings (typically [-1,1]) to int8 [-128,127]
// and serializes as a byte slice for sqlite-vec int8 columns.
func quantizeFloat32ToInt8(emb []float32) []byte {
	buf := make([]byte, len(emb))
	for i, v := range emb {
		// Clamp to [-1, 1] then scale to [-128, 127]
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		buf[i] = byte(int8(math.Round(float64(v) * 127)))
	}
	return buf
}

// serializeFloat32 serializes float32 embeddings to little-endian bytes for sqlite-vec queries.
// sqlite-vec accepts float32 queries against int8 columns (it quantizes internally for MATCH).
func serializeFloat32(emb []float32) []byte {
	buf := make([]byte, len(emb)*4)
	for i, v := range emb {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// VectorStoreConfig holds configuration for the vector store.
type VectorStoreConfig struct {
	DBPath       string
	EmbeddingDim int
	MaxChunks    int
	EnableFTS    bool
}

// VectorStore provides SQLite-backed storage with sqlite-vec + FTS5.
type VectorStore struct {
	db           *sql.DB
	embeddingDim int
	maxChunks    int
	enableFTS    bool
	mu           sync.RWMutex
}

// VectorSearchResult represents a vector search result.
type VectorSearchResult struct {
	Chunk MemoryChunk
	Score float32
}

// HybridSearchResult represents a combined search result.
type HybridSearchResult struct {
	Chunk         MemoryChunk `json:"chunk"`
	VectorScore   float32     `json:"vector_score,omitempty"`
	KeywordScore  float32     `json:"keyword_score,omitempty"`
	CombinedScore float32     `json:"combined_score"`
	MatchTypes    []string    `json:"match_types"`
	Highlights    []string    `json:"highlights,omitempty"`
	MatchedTerms  []string    `json:"matched_terms,omitempty"`
}

// VectorStoreStats holds vector store statistics.
type VectorStoreStats struct {
	ChunkCount   int    `json:"chunk_count"`
	MaxChunks    int    `json:"max_chunks"`
	EmbeddingDim int    `json:"embedding_dim"`
	FTSEnabled   bool   `json:"fts_enabled"`
	OldestChunk  string `json:"oldest_chunk,omitempty"`
	NewestChunk  string `json:"newest_chunk,omitempty"`
}

const vectorStoreSchema = `
CREATE TABLE IF NOT EXISTS memory_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chunk_id TEXT UNIQUE NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    metadata TEXT,
    embedding_model TEXT,
    importance REAL DEFAULT 0.5,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_memory_chunks_chunk_id ON memory_chunks(chunk_id);
CREATE INDEX IF NOT EXISTS idx_memory_chunks_created_at ON memory_chunks(created_at);
`

const ftsSchema = `
CREATE VIRTUAL TABLE IF NOT EXISTS memory_fts USING fts5(
    content,
    tags,
    content=memory_chunks,
    content_rowid=id,
    tokenize='porter unicode61'
);

CREATE TRIGGER IF NOT EXISTS memory_chunks_ai AFTER INSERT ON memory_chunks BEGIN
    INSERT INTO memory_fts(rowid, content, tags) VALUES (new.id, new.content, new.tags);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_ad AFTER DELETE ON memory_chunks BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, content, tags) VALUES('delete', old.id, old.content, old.tags);
END;

CREATE TRIGGER IF NOT EXISTS memory_chunks_au AFTER UPDATE ON memory_chunks BEGIN
    INSERT INTO memory_fts(memory_fts, rowid, content, tags) VALUES('delete', old.id, old.content, old.tags);
    INSERT INTO memory_fts(rowid, content, tags) VALUES (new.id, new.content, new.tags);
END;
`

// NewVectorStore creates a new vector store.
func NewVectorStore(cfg VectorStoreConfig) (*VectorStore, error) {
	if cfg.EmbeddingDim <= 0 {
		cfg.EmbeddingDim = 1536
	}
	if cfg.MaxChunks <= 0 {
		cfg.MaxChunks = 10000
	}

	db, err := sql.Open("sqlite3", cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open vector store db: %w", err)
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	// Create base schema
	if _, err := db.Exec(vectorStoreSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create vector store schema: %w", err)
	}

	// Create FTS5 tables + triggers
	if cfg.EnableFTS {
		if _, err := db.Exec(ftsSchema); err != nil {
			db.Close()
			return nil, fmt.Errorf("create FTS schema: %w", err)
		}
	}

	// Create sqlite-vec virtual table (int8 quantized)
	vecSQL := fmt.Sprintf(
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_vec USING vec0(embedding int8[%d])`,
		cfg.EmbeddingDim,
	)
	if _, err := db.Exec(vecSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("create vec0 table: %w", err)
	}

	return &VectorStore{
		db:           db,
		embeddingDim: cfg.EmbeddingDim,
		maxChunks:    cfg.MaxChunks,
		enableFTS:    cfg.EnableFTS,
	}, nil
}

// Store stores a memory chunk with its embedding.
func (s *VectorStore) Store(ctx context.Context, content string, emb []float32, metadata map[string]string, embModel string) (*MemoryChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()
	chunkID := NewEntryID()

	tagsJSON, _ := json.Marshal([]string{})
	metaJSON, _ := json.Marshal(metadata)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Insert into memory_chunks (FTS trigger fires automatically)
	res, err := tx.ExecContext(ctx,
		`INSERT INTO memory_chunks (chunk_id, content, tags, metadata, embedding_model, importance, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0.5, ?, ?)`,
		chunkID, content, string(tagsJSON), string(metaJSON), embModel, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert chunk: %w", err)
	}

	rowID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get rowid: %w", err)
	}

	// Insert into memory_vec (must be manual, vec0 doesn't support triggers)
	if len(emb) == s.embeddingDim {
		quantized := quantizeFloat32ToInt8(emb)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO memory_vec (rowid, embedding) VALUES (?, ?)`,
			rowID, quantized,
		); err != nil {
			return nil, fmt.Errorf("insert vec: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &MemoryChunk{
		ID:        chunkID,
		Content:   content,
		Metadata:  metadata,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SearchVector performs pure vector similarity search.
func (s *VectorStore) SearchVector(ctx context.Context, queryEmb []float32, limit int, minScore float32) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(queryEmb) != s.embeddingDim {
		return nil, fmt.Errorf("query embedding dim %d != store dim %d", len(queryEmb), s.embeddingDim)
	}

	serialized := serializeFloat32(queryEmb)

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.chunk_id, c.content, c.metadata, c.created_at, c.updated_at, v.distance
		FROM memory_vec v
		JOIN memory_chunks c ON c.id = v.rowid
		WHERE v.embedding MATCH ?
		ORDER BY v.distance
		LIMIT ?`,
		serialized, limit*3,
	)
	if err != nil {
		return nil, fmt.Errorf("search vector: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	for rows.Next() {
		var r VectorSearchResult
		var metaStr sql.NullString
		var distance float64
		if err := rows.Scan(&r.Chunk.ID, &r.Chunk.Content, &metaStr, &r.Chunk.CreatedAt, &r.Chunk.UpdatedAt, &distance); err != nil {
			continue
		}
		// cosine distance 0-2 → similarity 0-1
		r.Score = float32(1.0 - distance/2.0)
		if r.Score < minScore {
			continue
		}
		if metaStr.Valid {
			_ = json.Unmarshal([]byte(metaStr.String), &r.Chunk.Metadata)
		}
		results = append(results, r)
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// SearchKeyword performs FTS5 keyword search.
func (s *VectorStore) SearchKeyword(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.enableFTS {
		return nil, fmt.Errorf("FTS not enabled")
	}

	// Escape FTS5 special characters
	ftsQuery := escapeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT c.chunk_id, c.content, c.metadata, c.created_at, c.updated_at, f.rank
		FROM memory_fts f
		JOIN memory_chunks c ON c.id = f.rowid
		WHERE memory_fts MATCH ?
		ORDER BY f.rank
		LIMIT ?`,
		ftsQuery, limit*3,
	)
	if err != nil {
		return nil, fmt.Errorf("search keyword: %w", err)
	}
	defer rows.Close()

	var results []VectorSearchResult
	var ranks []float64
	for rows.Next() {
		var r VectorSearchResult
		var metaStr sql.NullString
		var rank float64
		if err := rows.Scan(&r.Chunk.ID, &r.Chunk.Content, &metaStr, &r.Chunk.CreatedAt, &r.Chunk.UpdatedAt, &rank); err != nil {
			continue
		}
		if metaStr.Valid {
			_ = json.Unmarshal([]byte(metaStr.String), &r.Chunk.Metadata)
		}
		r.Score = float32(rank) // raw BM25 rank (negative, lower=better)
		results = append(results, r)
		ranks = append(ranks, rank)
	}

	// Min-max normalize BM25 ranks to [0, 1]
	if len(ranks) > 0 {
		minR, maxR := ranks[0], ranks[0]
		for _, r := range ranks {
			if r < minR {
				minR = r
			}
			if r > maxR {
				maxR = r
			}
		}
		rangeR := maxR - minR
		for i := range results {
			if rangeR > 0 {
				// BM25: lower rank = better match, so invert
				results[i].Score = float32(1.0 - (ranks[i]-minR)/rangeR)
			} else {
				results[i].Score = 1.0
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// HybridSearch performs combined vector + keyword search with Go-layer normalization.
func (s *VectorStore) HybridSearch(ctx context.Context, queryEmb []float32, queryText string, limit int, vectorWeight, keywordWeight, minScore float32) ([]HybridSearchResult, error) {
	candidateLimit := limit * 3
	if candidateLimit < 30 {
		candidateLimit = 30
	}

	// Run both searches in parallel
	var vecResults []VectorSearchResult
	var ftsResults []VectorSearchResult
	var vecErr, ftsErr error
	var wg sync.WaitGroup

	if len(queryEmb) == s.embeddingDim {
		wg.Add(1)
		go func() {
			defer wg.Done()
			vecResults, vecErr = s.SearchVector(ctx, queryEmb, candidateLimit, 0)
		}()
	}

	if s.enableFTS && queryText != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ftsResults, ftsErr = s.SearchKeyword(ctx, queryText, candidateLimit)
		}()
	}

	wg.Wait()

	// Build result map by chunk ID
	resultMap := make(map[string]*HybridSearchResult)

	if vecErr == nil {
		for _, vr := range vecResults {
			resultMap[vr.Chunk.ID] = &HybridSearchResult{
				Chunk:       vr.Chunk,
				VectorScore: vr.Score,
				MatchTypes:  []string{"vector"},
			}
		}
	}

	if ftsErr == nil {
		for _, fr := range ftsResults {
			if existing, ok := resultMap[fr.Chunk.ID]; ok {
				existing.KeywordScore = fr.Score
				existing.MatchTypes = append(existing.MatchTypes, "keyword")
			} else {
				resultMap[fr.Chunk.ID] = &HybridSearchResult{
					Chunk:        fr.Chunk,
					KeywordScore: fr.Score,
					MatchTypes:   []string{"keyword"},
				}
			}
		}
	}

	// Calculate combined scores
	var results []HybridSearchResult
	for _, r := range resultMap {
		r.CombinedScore = r.VectorScore*vectorWeight + r.KeywordScore*keywordWeight
		// Boost for dual match
		if len(r.MatchTypes) > 1 {
			r.CombinedScore *= 1.2
			if r.CombinedScore > 1.0 {
				r.CombinedScore = 1.0
			}
		}
		if r.CombinedScore >= minScore {
			results = append(results, *r)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// Get retrieves a chunk by ID.
func (s *VectorStore) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var chunk MemoryChunk
	var metaStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT chunk_id, content, metadata, created_at, updated_at FROM memory_chunks WHERE chunk_id = ?`, id,
	).Scan(&chunk.ID, &chunk.Content, &metaStr, &chunk.CreatedAt, &chunk.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if metaStr.Valid {
		_ = json.Unmarshal([]byte(metaStr.String), &chunk.Metadata)
	}
	return &chunk, nil
}

// Delete removes a chunk by ID.
func (s *VectorStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get rowid first for vec deletion
	var rowID int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM memory_chunks WHERE chunk_id = ?`, id).Scan(&rowID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	// Delete from vec (manual)
	tx.ExecContext(ctx, `DELETE FROM memory_vec WHERE rowid = ?`, rowID)
	// Delete from chunks (FTS trigger handles FTS deletion)
	tx.ExecContext(ctx, `DELETE FROM memory_chunks WHERE id = ?`, rowID)

	return tx.Commit()
}

// Clear removes all data.
func (s *VectorStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.ExecContext(ctx, `DELETE FROM memory_vec`)
	tx.ExecContext(ctx, `DELETE FROM memory_chunks`)

	return tx.Commit()
}

// Prune removes oldest chunks when over capacity.
func (s *VectorStore) Prune(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_chunks`).Scan(&count); err != nil {
		return 0, err
	}

	if count <= s.maxChunks {
		return 0, nil
	}

	toDelete := count - s.maxChunks
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Delete oldest chunks (FTS trigger handles FTS, vec manual)
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM memory_chunks ORDER BY created_at ASC LIMIT ?`, toDelete)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()

	for _, id := range ids {
		tx.ExecContext(ctx, `DELETE FROM memory_vec WHERE rowid = ?`, id)
		tx.ExecContext(ctx, `DELETE FROM memory_chunks WHERE id = ?`, id)
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(ids), nil
}

// Stats returns store statistics.
func (s *VectorStore) Stats(ctx context.Context) (VectorStoreStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var stats VectorStoreStats
	stats.MaxChunks = s.maxChunks
	stats.EmbeddingDim = s.embeddingDim
	stats.FTSEnabled = s.enableFTS

	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_chunks`).Scan(&stats.ChunkCount)
	s.db.QueryRowContext(ctx, `SELECT MIN(created_at) FROM memory_chunks`).Scan(&stats.OldestChunk)
	s.db.QueryRowContext(ctx, `SELECT MAX(created_at) FROM memory_chunks`).Scan(&stats.NewestChunk)

	return stats, nil
}

// Close closes the database.
func (s *VectorStore) Close() error {
	return s.db.Close()
}

// escapeFTS5Query escapes special FTS5 characters.
func escapeFTS5Query(query string) string {
	// Split into words and quote each
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}
	var parts []string
	for _, w := range words {
		// Remove FTS5 operators
		w = strings.TrimFunc(w, func(r rune) bool {
			return r == '"' || r == '*' || r == '+' || r == '-' || r == '(' || r == ')' || r == ':'
		})
		if w != "" {
			parts = append(parts, `"`+w+`"`)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " OR ")
}
