//go:build cgo

package memory

import (
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
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

// serializeFloat32 serializes float32 embeddings to little-endian bytes for sqlite-vec helper functions.
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
	readDB       *sql.DB
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

type memoryChunkRow struct {
	ID        int64     `zorm:"id"`
	ChunkID   string    `zorm:"chunk_id"`
	Content   string    `zorm:"content"`
	Tags      string    `zorm:"tags"`
	Metadata  string    `zorm:"metadata"`
	CreatedAt time.Time `zorm:"created_at"`
	UpdatedAt time.Time `zorm:"updated_at"`
}

type memoryChunkIDRow struct {
	ID int64 `zorm:"id"`
}

func memoryChunkFromRow(row memoryChunkRow) MemoryChunk {
	chunk := MemoryChunk{
		ID:        row.ChunkID,
		Content:   row.Content,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	if strings.TrimSpace(row.Metadata) != "" {
		_ = json.Unmarshal([]byte(row.Metadata), &chunk.Metadata)
	}
	return chunk
}

func vectorStoreStringFromMapValue(row z.V, key string) string {
	value, ok := vectorStoreValueFromMapKey(row, key)
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func vectorStoreIntFromMapValue(row z.V, key string) int {
	value, ok := vectorStoreValueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case uint:
		return int(typed)
	case uint8:
		return int(typed)
	case uint16:
		return int(typed)
	case uint32:
		return int(typed)
	case uint64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case []byte:
		n, _ := strconv.Atoi(string(typed))
		return n
	case string:
		n, _ := strconv.Atoi(typed)
		return n
	default:
		return 0
	}
}

func vectorStoreFloat64FromMapValue(row z.V, key string) float64 {
	value, ok := vectorStoreValueFromMapKey(row, key)
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case int8:
		return float64(typed)
	case int16:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	case uint:
		return float64(typed)
	case uint8:
		return float64(typed)
	case uint16:
		return float64(typed)
	case uint32:
		return float64(typed)
	case uint64:
		return float64(typed)
	case []byte:
		n, _ := strconv.ParseFloat(string(typed), 64)
		return n
	case string:
		n, _ := strconv.ParseFloat(typed, 64)
		return n
	default:
		return 0
	}
}

func vectorStoreTimeFromMapValue(row z.V, key string) time.Time {
	value, ok := vectorStoreValueFromMapKey(row, key)
	if !ok || value == nil {
		return time.Time{}
	}
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		return parseVectorStoreTime(typed)
	case []byte:
		return parseVectorStoreTime(string(typed))
	default:
		return parseVectorStoreTime(fmt.Sprint(typed))
	}
}

func parseVectorStoreTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

func vectorSearchResultFromMapRow(row z.V) (VectorSearchResult, float64) {
	result := VectorSearchResult{
		Chunk: MemoryChunk{
			ID:        vectorStoreStringFromMapValue(row, "chunk_id"),
			Content:   vectorStoreStringFromMapValue(row, "content"),
			CreatedAt: vectorStoreTimeFromMapValue(row, "created_at"),
			UpdatedAt: vectorStoreTimeFromMapValue(row, "updated_at"),
		},
	}
	if metadata := strings.TrimSpace(vectorStoreStringFromMapValue(row, "metadata")); metadata != "" {
		_ = json.Unmarshal([]byte(metadata), &result.Chunk.Metadata)
	}
	applySourceIDToChunk(&result.Chunk)
	rank := vectorStoreFloat64FromMapValue(row, "rank")
	result.Score = float32(rank)
	return result, rank
}

func vectorSearchResultFromDistanceMapRow(row z.V) (VectorSearchResult, float64) {
	result := VectorSearchResult{
		Chunk: MemoryChunk{
			ID:        vectorStoreStringFromMapValue(row, "chunk_id"),
			Content:   vectorStoreStringFromMapValue(row, "content"),
			CreatedAt: vectorStoreTimeFromMapValue(row, "created_at"),
			UpdatedAt: vectorStoreTimeFromMapValue(row, "updated_at"),
		},
	}
	if metadata := strings.TrimSpace(vectorStoreStringFromMapValue(row, "metadata")); metadata != "" {
		_ = json.Unmarshal([]byte(metadata), &result.Chunk.Metadata)
	}
	applySourceIDToChunk(&result.Chunk)
	distance := vectorStoreFloat64FromMapValue(row, "distance")
	// cosine distance 0-2 -> similarity 0-1
	result.Score = float32(1.0 - distance/2.0)
	return result, distance
}

func vectorStoreValueFromMapKey(row z.V, key string) (interface{}, bool) {
	if row == nil {
		return nil, false
	}
	if value, ok := row[key]; ok {
		return value, true
	}
	for rawKey, value := range row {
		if vectorStoreNormalizeMapKey(rawKey) == key {
			return value, true
		}
	}
	return nil, false
}

func vectorStoreNormalizeMapKey(key string) string {
	key = strings.TrimSpace(strings.Trim(key, "`"))
	if key == "" {
		return ""
	}
	fields := strings.Fields(key)
	if len(fields) >= 3 && strings.EqualFold(fields[len(fields)-2], "as") {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if len(fields) >= 2 {
		return strings.Trim(fields[len(fields)-1], "`")
	}
	if dot := strings.LastIndex(key, "."); dot >= 0 {
		return strings.Trim(key[dot+1:], "`")
	}
	return key
}

const vectorStoreSchema = `
CREATE TABLE IF NOT EXISTS memory_chunks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    chunk_id TEXT UNIQUE NOT NULL,
    content TEXT NOT NULL,
    tags TEXT,
    metadata TEXT,
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

func supportsFTS5(db *sql.DB) bool {
	if db == nil {
		return false
	}

	rows, err := db.Query(`SELECT name FROM pragma_module_list WHERE name = 'fts5'`)
	if err == nil {
		defer rows.Close()
		if rows.Next() {
			return true
		}
	}

	if _, err := db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS temp.memory_fts5_probe USING fts5(content)`); err != nil {
		return false
	}
	_, _ = db.Exec(`DROP TABLE IF EXISTS temp.memory_fts5_probe`)
	return true
}

func dropMemoryFTSTriggers(db *sql.DB) error {
	if db == nil {
		return nil
	}
	for _, stmt := range []string{
		`DROP TRIGGER IF EXISTS memory_chunks_ai`,
		`DROP TRIGGER IF EXISTS memory_chunks_ad`,
		`DROP TRIGGER IF EXISTS memory_chunks_au`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func isMissingFTSModuleError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such module") && strings.Contains(msg, "fts5")
}

func extractTagValues(metadata map[string]string) []string {
	if len(metadata) == 0 {
		return nil
	}
	tags := make([]string, 0, len(metadata))
	for key, value := range metadata {
		if !strings.HasPrefix(key, "tag_") {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	sort.Strings(tags)
	return tags
}

func keywordFallbackTerms(query string) []string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(words) == 0 {
		return nil
	}
	terms := make([]string, 0, len(words))
	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		word = strings.Trim(word, `"'*+-():,.;!?[]{} `)
		if word == "" {
			continue
		}
		if _, ok := seen[word]; ok {
			continue
		}
		seen[word] = struct{}{}
		terms = append(terms, word)
	}
	return terms
}

func keywordFallbackScore(query string, terms []string, content string, tags string) float32 {
	haystack := strings.ToLower(strings.TrimSpace(content + " " + tags))
	if haystack == "" || len(terms) == 0 {
		return 0
	}
	matches := 0
	for _, term := range terms {
		if strings.Contains(haystack, term) {
			matches++
		}
	}
	if matches == 0 {
		return 0
	}
	score := float32(matches) / float32(len(terms))
	if normalized := strings.ToLower(strings.TrimSpace(query)); normalized != "" && strings.Contains(haystack, normalized) {
		score += 0.25
	}
	if score > 1 {
		return 1
	}
	return score
}

func ensureVectorStoreParentDir(dbPath string) error {
	trimmed := strings.TrimSpace(dbPath)
	// SQLite special DSNs do not map to filesystem directories.
	if trimmed == "" || trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:") {
		return nil
	}
	dir := filepath.Dir(trimmed)
	if dir == "" || dir == "." {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create vector store dir %q: %w", dir, err)
	}
	return nil
}

func openVectorStoreReaderDB(dbPath string) (*sql.DB, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" || dbPath == ":memory:" || strings.HasPrefix(dbPath, "file:") {
		return nil, nil
	}
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
	db, err := dbutil.OpenSQLiteWithRecovery(dsn, dbPath, func(db *sql.DB) error {
		db.SetMaxOpenConns(4)
		db.SetMaxIdleConns(2)
		if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
			return fmt.Errorf("set vector store reader busy timeout: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

// NewVectorStore creates a new vector store.
func NewVectorStore(cfg VectorStoreConfig) (*VectorStore, error) {
	if cfg.EmbeddingDim <= 0 {
		cfg.EmbeddingDim = 1536
	}
	if cfg.MaxChunks <= 0 {
		cfg.MaxChunks = 10000
	}
	if err := ensureVectorStoreParentDir(cfg.DBPath); err != nil {
		return nil, err
	}

	ftsEnabled := cfg.EnableFTS
	db, err := dbutil.OpenSQLiteWithRecoveryAndRecreate(
		cfg.DBPath+"?_journal_mode=WAL&_busy_timeout=5000",
		cfg.DBPath,
		func(db *sql.DB) error {
			db.SetMaxOpenConns(4)
			db.SetMaxIdleConns(2)
			if _, err := db.Exec("PRAGMA synchronous=FULL"); err != nil {
				return fmt.Errorf("set vector store synchronous mode: %w", err)
			}
			if _, err := db.Exec("PRAGMA wal_autocheckpoint=1000"); err != nil {
				return fmt.Errorf("set vector store wal autocheckpoint: %w", err)
			}

			if _, err := db.Exec(vectorStoreSchema); err != nil {
				return fmt.Errorf("create vector store schema: %w", err)
			}

			if !ftsEnabled {
				if err := dropMemoryFTSTriggers(db); err != nil {
					return fmt.Errorf("disable FTS triggers: %w", err)
				}
			}

			if ftsEnabled {
				if !supportsFTS5(db) {
					ftsEnabled = false
					if err := dropMemoryFTSTriggers(db); err != nil {
						return fmt.Errorf("disable FTS triggers: %w", err)
					}
				} else if _, err := db.Exec(ftsSchema); err != nil {
					if !isMissingFTSModuleError(err) {
						return fmt.Errorf("create FTS schema: %w", err)
					}
					ftsEnabled = false
					if err := dropMemoryFTSTriggers(db); err != nil {
						return fmt.Errorf("disable FTS triggers: %w", err)
					}
				}
			}

			vecSQL := fmt.Sprintf(
				`CREATE VIRTUAL TABLE IF NOT EXISTS memory_vec USING vec0(embedding int8[%d])`,
				cfg.EmbeddingDim,
			)
			if _, err := db.Exec(vecSQL); err != nil {
				return fmt.Errorf("create vec0 table: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("open vector store db: %w", err)
	}

	readDB, readErr := openVectorStoreReaderDB(cfg.DBPath)
	if readErr != nil || readDB == nil {
		readDB = db
	}

	return &VectorStore{
		db:           db,
		readDB:       readDB,
		embeddingDim: cfg.EmbeddingDim,
		maxChunks:    cfg.MaxChunks,
		enableFTS:    ftsEnabled,
	}, nil
}

func (s *VectorStore) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

// Store stores a memory chunk with its embedding.
func (s *VectorStore) Store(ctx context.Context, content string, emb []float32, metadata map[string]string) (*MemoryChunk, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := timeutil.NowTime()
	chunkID := NewEntryID()
	if sourceID := sourceIDFromMetadata(metadata); sourceID != "" {
		chunkID = sourceID + "#" + chunkID
	}

	tagsText := strings.Join(extractTagValues(metadata), " ")
	metaJSON, _ := json.Marshal(metadata)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Insert into memory_chunks (FTS trigger fires automatically)
	rowID, err := z.TableContext(ctx, tx, "memory_chunks").Insert(z.V{
		"chunk_id":   chunkID,
		"content":    content,
		"tags":       tagsText,
		"metadata":   string(metaJSON),
		"importance": 0.5,
		"created_at": now,
		"updated_at": now,
	})
	if err != nil {
		return nil, fmt.Errorf("insert chunk: %w", err)
	}

	// Insert into memory_vec (must be manual, vec0 doesn't support triggers)
	if len(emb) == s.embeddingDim {
		serialized := serializeFloat32(emb)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO memory_vec (rowid, embedding) VALUES (?, vec_quantize_int8(?, 'unit'))`,
			int64(rowID), serialized,
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

	var rows []z.V
	if _, err := z.TableContext(ctx, s.reader(), "memory_vec v").Select(&rows,
		z.Fields(
			"c.chunk_id as chunk_id",
			"c.content as content",
			"c.metadata as metadata",
			"c.created_at as created_at",
			"c.updated_at as updated_at",
			"v.distance as distance",
		),
		z.InnerJoin("memory_chunks c", z.Expr("c.id = v.rowid")),
		z.Where(
			z.Expr("v.embedding MATCH vec_quantize_int8(vec_f32(?), 'unit')", serialized),
			z.Expr("k = ?", limit*3),
		),
		z.OrderBy("v.distance"),
	); err != nil {
		return nil, fmt.Errorf("search vector: %w", err)
	}

	var results []VectorSearchResult
	for _, row := range rows {
		r, _ := vectorSearchResultFromDistanceMapRow(row)
		if r.Score < minScore {
			continue
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
		return s.searchKeywordFallback(ctx, query, limit)
	}

	// Escape FTS5 special characters
	ftsQuery := escapeFTS5Query(query)
	if ftsQuery == "" {
		return nil, nil
	}

	var rows []z.V
	if _, err := z.TableContext(ctx, s.reader(), "memory_chunks c").Select(&rows,
		z.Fields(
			"c.chunk_id as chunk_id",
			"c.content as content",
			"c.metadata as metadata",
			"c.created_at as created_at",
			"c.updated_at as updated_at",
			"f.rank as rank",
		),
		z.InnerJoin("memory_fts f", z.Expr("c.id = f.rowid")),
		z.Where(z.Expr("memory_fts MATCH ?", ftsQuery)),
		z.OrderBy("f.rank"),
		z.Limit(limit*3),
	); err != nil {
		if isMissingFTSModuleError(err) {
			return s.searchKeywordFallback(ctx, query, limit)
		}
		return nil, fmt.Errorf("search keyword: %w", err)
	}

	var results []VectorSearchResult
	var ranks []float64
	for _, row := range rows {
		r, rank := vectorSearchResultFromMapRow(row)
		// raw BM25 rank (negative, lower=better)
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

func (s *VectorStore) searchKeywordFallback(ctx context.Context, query string, limit int) ([]VectorSearchResult, error) {
	terms := keywordFallbackTerms(query)
	if len(terms) == 0 {
		return nil, nil
	}
	if limit <= 0 {
		limit = 10
	}

	candidateLimit := limit * 5
	if candidateLimit < 25 {
		candidateLimit = 25
	}

	clauses := make([]string, 0, len(terms))
	args := make([]interface{}, 0, len(terms)*2)
	for _, term := range terms {
		pattern := "%" + term + "%"
		clauses = append(clauses, `(LOWER(content) LIKE ? OR LOWER(tags) LIKE ?)`)
		args = append(args, pattern, pattern)
	}
	whereArgs := append([]interface{}{strings.Join(clauses, ` OR `)}, args...)
	var rows []memoryChunkRow
	if _, err := z.TableContext(ctx, s.reader(), "memory_chunks").Select(&rows,
		z.Where(whereArgs...),
		z.OrderBy("updated_at DESC"),
		z.Limit(candidateLimit),
	); err != nil {
		return nil, fmt.Errorf("search keyword fallback: %w", err)
	}

	results := make([]VectorSearchResult, 0, limit)
	for i := range rows {
		result := VectorSearchResult{Chunk: memoryChunkFromRow(rows[i])}
		applySourceIDToChunk(&result.Chunk)
		result.Score = keywordFallbackScore(query, terms, result.Chunk.Content, rows[i].Tags)
		if result.Score <= 0 {
			continue
		}
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].Chunk.UpdatedAt.After(results[j].Chunk.UpdatedAt)
		}
		return results[i].Score > results[j].Score
	})

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

	var rows []memoryChunkRow
	if _, err := z.TableContext(ctx, s.reader(), "memory_chunks").Select(&rows,
		z.Where(z.Eq("chunk_id", id)),
		z.Limit(1),
	); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	chunk := memoryChunkFromRow(rows[0])
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

	var rows []memoryChunkRow
	if _, err := z.TableContext(ctx, tx, "memory_chunks").Select(&rows); err != nil {
		return err
	}

	rowIDs := make([]int64, 0, 4)
	for i := range rows {
		if rows[i].ChunkID == id {
			rowIDs = append(rowIDs, rows[i].ID)
			continue
		}
		if strings.TrimSpace(rows[i].Metadata) == "" {
			continue
		}
		metadata := map[string]string{}
		if err := json.Unmarshal([]byte(rows[i].Metadata), &metadata); err != nil {
			continue
		}
		if sourceIDFromMetadata(metadata) == id {
			rowIDs = append(rowIDs, rows[i].ID)
		}
	}
	if len(rowIDs) == 0 {
		return nil
	}

	for _, rowID := range rowIDs {
		_, _ = z.TableContext(ctx, tx, "memory_vec").Delete(z.Where(z.Eq("rowid", rowID)))
	}
	if _, err := z.TableContext(ctx, tx, "memory_chunks").Delete(z.Where(z.In("id", rowIDs))); err != nil {
		return err
	}

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

	if _, err := z.TableContext(ctx, tx, "memory_vec").Delete(z.Where(z.Expr("1=1"))); err != nil {
		return err
	}
	if _, err := z.TableContext(ctx, tx, "memory_chunks").Delete(z.Where(z.Expr("1=1"))); err != nil {
		return err
	}

	return tx.Commit()
}

// Prune removes oldest chunks when over capacity.
func (s *VectorStore) Prune(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var countRows []z.V
	if _, err := z.TableContext(ctx, s.reader(), "memory_chunks").Select(&countRows,
		z.Fields("COUNT(*) as chunk_count"),
		z.Limit(1),
	); err != nil {
		return 0, err
	}
	count := 0
	if len(countRows) > 0 {
		count = vectorStoreIntFromMapValue(countRows[0], "chunk_count")
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
	var rows []memoryChunkIDRow
	if _, err := z.TableContext(ctx, tx, "memory_chunks").Select(&rows,
		z.OrderBy("created_at ASC"),
		z.Limit(toDelete),
	); err != nil {
		return 0, err
	}
	var ids []int64
	for i := range rows {
		ids = append(ids, rows[i].ID)
	}

	for _, id := range ids {
		if _, err := z.TableContext(ctx, tx, "memory_vec").Delete(z.Where(z.Eq("rowid", id))); err != nil {
			return 0, err
		}
	}
	if len(ids) > 0 {
		if _, err := z.TableContext(ctx, tx, "memory_chunks").Delete(z.Where(z.In("id", ids))); err != nil {
			return 0, err
		}
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

	var rows []z.V
	if _, err := z.TableContext(ctx, s.reader(), "memory_chunks").Select(&rows,
		z.Fields(
			"COUNT(*) as chunk_count",
			"MIN(created_at) as oldest_chunk",
			"MAX(created_at) as newest_chunk",
		),
		z.Limit(1),
	); err != nil {
		return stats, err
	}
	if len(rows) > 0 {
		stats.ChunkCount = vectorStoreIntFromMapValue(rows[0], "chunk_count")
		stats.OldestChunk = vectorStoreStringFromMapValue(rows[0], "oldest_chunk")
		stats.NewestChunk = vectorStoreStringFromMapValue(rows[0], "newest_chunk")
	}

	return stats, nil
}

// Close closes the database.
func (s *VectorStore) Close() error {
	if s == nil {
		return nil
	}
	if s.readDB != nil && s.readDB != s.db {
		_ = s.readDB.Close()
	}
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
