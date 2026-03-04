//go:build !cgo

package memory

import (
	"context"
	"fmt"
	"sync"
)

// VectorStoreConfig holds configuration for the vector store.
type VectorStoreConfig struct {
	DBPath       string
	EmbeddingDim int
	MaxChunks    int
	EnableFTS    bool
}

// VectorStore provides a no-cgo compatibility stub.
type VectorStore struct {
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

var errVectorStoreNoCGO = fmt.Errorf("vector store requires cgo/sqlite-vec (build with CGO_ENABLED=1)")

// NewVectorStore creates a no-cgo stub and returns an availability error.
func NewVectorStore(cfg VectorStoreConfig) (*VectorStore, error) {
	if cfg.EmbeddingDim <= 0 {
		cfg.EmbeddingDim = 1536
	}
	if cfg.MaxChunks <= 0 {
		cfg.MaxChunks = 10000
	}
	return nil, errVectorStoreNoCGO
}

// Store is unavailable in !cgo builds.
func (s *VectorStore) Store(context.Context, string, []float32, map[string]string, string) (*MemoryChunk, error) {
	return nil, errVectorStoreNoCGO
}

// SearchVector is unavailable in !cgo builds.
func (s *VectorStore) SearchVector(context.Context, []float32, int, float32) ([]VectorSearchResult, error) {
	return nil, errVectorStoreNoCGO
}

// SearchKeyword is unavailable in !cgo builds.
func (s *VectorStore) SearchKeyword(context.Context, string, int) ([]VectorSearchResult, error) {
	return nil, errVectorStoreNoCGO
}

// HybridSearch is unavailable in !cgo builds.
func (s *VectorStore) HybridSearch(context.Context, []float32, string, int, float32, float32, float32) ([]HybridSearchResult, error) {
	return nil, errVectorStoreNoCGO
}

// Get is unavailable in !cgo builds.
func (s *VectorStore) Get(context.Context, string) (*MemoryChunk, error) {
	return nil, errVectorStoreNoCGO
}

// Delete is unavailable in !cgo builds.
func (s *VectorStore) Delete(context.Context, string) error {
	return errVectorStoreNoCGO
}

// Clear is unavailable in !cgo builds.
func (s *VectorStore) Clear(context.Context) error {
	return errVectorStoreNoCGO
}

// Prune is unavailable in !cgo builds.
func (s *VectorStore) Prune(context.Context) (int, error) {
	return 0, errVectorStoreNoCGO
}

// Stats returns a zero-value report for !cgo builds.
func (s *VectorStore) Stats(context.Context) (VectorStoreStats, error) {
	if s == nil {
		return VectorStoreStats{}, nil
	}
	return VectorStoreStats{
		ChunkCount:   0,
		MaxChunks:    s.maxChunks,
		EmbeddingDim: s.embeddingDim,
		FTSEnabled:   s.enableFTS,
	}, nil
}

// Close is a no-op for !cgo builds.
func (s *VectorStore) Close() error {
	return nil
}
