package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
)

// HybridSearcher combines vector and keyword search.
type HybridSearcher struct {
	vectorStore       *VectorStore
	embeddingProvider embedding.Provider
	config            HybridSearchConfig
	scorer            *ImportanceScorer
	mu                sync.RWMutex
}

// HybridSearchConfig holds hybrid search configuration.
type HybridSearchConfig struct {
	VectorWeight  float32 // Weight for vector search (default 0.7)
	KeywordWeight float32 // Weight for keyword search (default 0.3)
	MinScore      float32 // Minimum combined score (default 0.5)
	MaxResults    int     // Maximum results to return (default 10)
}

// SearchOptions holds optional search parameters.
type SearchOptions struct {
	Limit        int
	StartDate    *time.Time
	EndDate      *time.Time
	Highlight    bool
	HighlightTag string
}

// NewHybridSearcher creates a new hybrid searcher.
func NewHybridSearcher(store *VectorStore, provider embedding.Provider, cfg config.MemoryConfig) *HybridSearcher {
	vectorWeight := float32(cfg.Search.VectorWeight)
	if vectorWeight == 0 {
		vectorWeight = 0.7
	}
	keywordWeight := float32(cfg.Search.KeywordWeight)
	if keywordWeight == 0 {
		keywordWeight = 0.3
	}
	minScore := float32(cfg.Search.MinScore)
	if minScore == 0 {
		minScore = 0.5
	}
	maxResults := cfg.Search.MaxResults
	if maxResults == 0 {
		maxResults = 10
	}

	return &HybridSearcher{
		vectorStore:       store,
		embeddingProvider: provider,
		config: HybridSearchConfig{
			VectorWeight:  vectorWeight,
			KeywordWeight: keywordWeight,
			MinScore:      minScore,
			MaxResults:    maxResults,
		},
		scorer: NewImportanceScorer(DefaultImportanceConfig()),
	}
}

// Search performs hybrid search combining vector and keyword search.
func (h *HybridSearcher) Search(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	return h.SearchWithOptions(ctx, query, SearchOptions{Limit: limit})
}

// SearchWithOptions performs hybrid search with additional options.
func (h *HybridSearcher) SearchWithOptions(ctx context.Context, query string, opts SearchOptions) ([]HybridSearchResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	limit := opts.Limit
	if limit == 0 {
		limit = h.config.MaxResults
	}
	if limit == 0 {
		limit = 10
	}

	var results []HybridSearchResult
	var err error

	// Try full hybrid search (vector + keyword)
	if h.embeddingProvider != nil {
		queryEmb, embErr := h.embeddingProvider.Embed(ctx, query)
		if embErr == nil {
			results, err = h.vectorStore.HybridSearch(
				ctx, queryEmb, query, limit,
				h.config.VectorWeight, h.config.KeywordWeight, h.config.MinScore,
			)
			if err == nil {
				results = h.postProcess(results, query, opts)
				return results, nil
			}
		}
		// Embedding failed — fall through to keyword-only
	}

	// Degradation: keyword-only search via FTS5
	keywordResults, err := h.vectorStore.SearchKeyword(ctx, query, limit*2)
	if err != nil {
		return nil, fmt.Errorf("keyword search: %w", err)
	}

	results = make([]HybridSearchResult, 0, len(keywordResults))
	for _, kr := range keywordResults {
		if kr.Score >= h.config.MinScore {
			results = append(results, HybridSearchResult{
				Chunk:         kr.Chunk,
				KeywordScore:  kr.Score,
				CombinedScore: kr.Score,
				MatchTypes:    []string{"keyword"},
			})
		}
	}

	results = h.postProcess(results, query, opts)
	return results, nil
}

// postProcess applies reranking, date filtering, highlighting, and limit.
func (h *HybridSearcher) postProcess(results []HybridSearchResult, query string, opts SearchOptions) []HybridSearchResult {
	// ImportanceScorer rerank
	if h.scorer != nil {
		for i := range results {
			chunk := &results[i].Chunk
			mc := &MemoryChunk{
				ID:        chunk.ID,
				Content:   chunk.Content,
				Metadata:  chunk.Metadata,
				CreatedAt: chunk.CreatedAt,
				UpdatedAt: chunk.UpdatedAt,
			}
			importance := h.scorer.Score(mc)
			// Blend: 80% search score + 20% importance
			results[i].CombinedScore = results[i].CombinedScore*0.8 + importance*0.2
		}
	}

	// Date range filter
	if opts.StartDate != nil || opts.EndDate != nil {
		results = filterByDateRange(results, opts.StartDate, opts.EndDate)
	}

	// Highlighting
	if opts.Highlight {
		tag := opts.HighlightTag
		if tag == "" {
			tag = "mark"
		}
		results = addHighlights(results, query, tag)
	}

	// Re-sort after reranking
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	limit := opts.Limit
	if limit == 0 {
		limit = h.config.MaxResults
	}
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// Store stores content with automatic embedding.
func (h *HybridSearcher) Store(ctx context.Context, content string, metadata map[string]string) (*MemoryChunk, error) {
	var emb []float32
	var embModel string

	if h.embeddingProvider != nil {
		var err error
		emb, err = h.embeddingProvider.Embed(ctx, content)
		if err != nil {
			// Continue without embedding
			emb = nil
		} else {
			embModel = h.embeddingProvider.Model()
		}
	}

	return h.vectorStore.Store(ctx, content, emb, metadata, embModel)
}

// Delete deletes a memory chunk.
func (h *HybridSearcher) Delete(ctx context.Context, id string) error {
	return h.vectorStore.Delete(ctx, id)
}

// Get retrieves a memory chunk by ID.
func (h *HybridSearcher) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return h.vectorStore.Get(ctx, id)
}

// Prune removes old chunks.
func (h *HybridSearcher) Prune(ctx context.Context) (int, error) {
	return h.vectorStore.Prune(ctx)
}

// Clear removes all chunks.
func (h *HybridSearcher) Clear(ctx context.Context) error {
	return h.vectorStore.Clear(ctx)
}

// Stats returns statistics.
func (h *HybridSearcher) Stats(ctx context.Context) (HybridSearchStats, error) {
	vectorStats, err := h.vectorStore.Stats(ctx)
	if err != nil {
		return HybridSearchStats{}, err
	}

	return HybridSearchStats{
		VectorStoreStats:  vectorStats,
		VectorWeight:      h.config.VectorWeight,
		KeywordWeight:     h.config.KeywordWeight,
		EmbeddingProvider: h.getProviderName(),
	}, nil
}

func (h *HybridSearcher) getProviderName() string {
	if h.embeddingProvider == nil {
		return ""
	}
	return h.embeddingProvider.Name()
}

// HybridSearchStats holds hybrid search statistics.
type HybridSearchStats struct {
	VectorStoreStats  VectorStoreStats `json:"vector_store"`
	VectorWeight      float32          `json:"vector_weight"`
	KeywordWeight     float32          `json:"keyword_weight"`
	EmbeddingProvider string           `json:"embedding_provider,omitempty"`
}

// MemoryService provides a high-level interface for memory operations.
type MemoryService struct {
	Searcher *HybridSearcher
}

// NewMemoryService creates a new memory service.
func NewMemoryService(searcher *HybridSearcher) *MemoryService {
	return &MemoryService{Searcher: searcher}
}

// GetSearcher returns the underlying HybridSearcher.
func (s *MemoryService) GetSearcher() *HybridSearcher {
	return s.Searcher
}

// Remember stores a memory.
func (s *MemoryService) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	metadata := make(map[string]string)
	if len(tags) > 0 {
		for i, tag := range tags {
			metadata[fmt.Sprintf("tag_%d", i)] = tag
		}
	}
	return s.Searcher.Store(ctx, content, metadata)
}

// Recall retrieves relevant memories as SearchResult (matches MemoryBackend interface).
func (s *MemoryService) Recall(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	hybridResults, err := s.Searcher.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	results := make([]SearchResult, len(hybridResults))
	for i, hr := range hybridResults {
		results[i] = SearchResult{
			Chunk:        hr.Chunk,
			Score:        hr.CombinedScore,
			KeywordScore: hr.KeywordScore,
			MatchTypes:   hr.MatchTypes,
		}
	}
	return results, nil
}

// Forget removes a memory.
func (s *MemoryService) Forget(ctx context.Context, id string) error {
	return s.Searcher.Delete(ctx, id)
}

// ForgetAll removes all memories.
func (s *MemoryService) ForgetAll(ctx context.Context) error {
	return s.Searcher.Clear(ctx)
}

// Get retrieves a memory by ID.
func (s *MemoryService) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return s.Searcher.Get(ctx, id)
}

// Prune removes old memories.
func (s *MemoryService) Prune(ctx context.Context) (int, error) {
	return s.Searcher.Prune(ctx)
}

// Stats returns memory statistics.
func (s *MemoryService) Stats(ctx context.Context) (*MemoryStats, error) {
	hs, err := s.Searcher.Stats(ctx)
	if err != nil {
		return nil, err
	}
	return &MemoryStats{
		TotalChunks: hs.VectorStoreStats.ChunkCount,
		OldestChunk: hs.VectorStoreStats.OldestChunk,
		NewestChunk: hs.VectorStoreStats.NewestChunk,
		Backend:     "local",
	}, nil
}

// Name returns the backend name.
func (s *MemoryService) Name() string {
	return "local"
}

// --- Helper functions ---

// filterByDateRange filters results by date range.
func filterByDateRange(results []HybridSearchResult, start, end *time.Time) []HybridSearchResult {
	filtered := make([]HybridSearchResult, 0, len(results))
	for _, r := range results {
		if start != nil && r.Chunk.CreatedAt.Before(*start) {
			continue
		}
		if end != nil && r.Chunk.CreatedAt.After(*end) {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// addHighlights adds highlighted snippets to results.
func addHighlights(results []HybridSearchResult, query string, tag string) []HybridSearchResult {
	queryTerms := strings.Fields(strings.ToLower(query))
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"

	for i := range results {
		content := results[i].Chunk.Content
		contentLower := strings.ToLower(content)

		var matchedTerms []string
		var highlights []string

		for _, term := range queryTerms {
			if strings.Contains(contentLower, term) {
				matchedTerms = append(matchedTerms, term)

				idx := strings.Index(contentLower, term)
				if idx >= 0 {
					start := idx - 50
					if start < 0 {
						start = 0
					}
					end := idx + len(term) + 50
					if end > len(content) {
						end = len(content)
					}

					snippet := content[start:end]
					highlighted := highlightTerm(snippet, term, openTag, closeTag)

					if start > 0 {
						highlighted = "..." + highlighted
					}
					if end < len(content) {
						highlighted = highlighted + "..."
					}

					highlights = append(highlights, highlighted)
				}
			}
		}

		results[i].MatchedTerms = matchedTerms
		results[i].Highlights = highlights
	}

	return results
}

// highlightTerm highlights a term in text (case-insensitive).
func highlightTerm(text, term, openTag, closeTag string) string {
	textLower := strings.ToLower(text)
	termLower := strings.ToLower(term)

	var result strings.Builder
	lastEnd := 0

	for {
		idx := strings.Index(textLower[lastEnd:], termLower)
		if idx < 0 {
			break
		}
		idx += lastEnd

		result.WriteString(text[lastEnd:idx])
		result.WriteString(openTag)
		result.WriteString(text[idx : idx+len(term)])
		result.WriteString(closeTag)
		lastEnd = idx + len(term)
	}

	result.WriteString(text[lastEnd:])
	return result.String()
}
