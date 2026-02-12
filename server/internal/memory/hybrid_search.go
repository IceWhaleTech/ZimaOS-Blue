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
	vectorStore      *VectorStore
	embeddingProvider embedding.Provider
	config           HybridSearchConfig
	mu               sync.RWMutex
}

// HybridSearchConfig holds hybrid search configuration.
type HybridSearchConfig struct {
	VectorWeight   float32 // Weight for vector search results (0-1)
	KeywordWeight  float32 // Weight for keyword search results (0-1)
	MinVectorScore float32 // Minimum vector similarity score
	MaxResults     int     // Maximum results to return
	EnableRerank   bool    // Enable result reranking
}

// HybridSearchResult represents a combined search result.
type HybridSearchResult struct {
	Chunk         MemoryChunk `json:"chunk"`
	VectorScore   float32     `json:"vector_score,omitempty"`
	KeywordScore  float32     `json:"keyword_score,omitempty"`
	CombinedScore float32     `json:"combined_score"`
	MatchTypes    []string    `json:"match_types"`
	Highlights    []string    `json:"highlights,omitempty"`    // Highlighted snippets
	MatchedTerms  []string    `json:"matched_terms,omitempty"` // Terms that matched
}

// SearchOptions holds optional search parameters.
type SearchOptions struct {
	Limit       int        // Maximum results
	StartDate   *time.Time // Filter by date range start
	EndDate     *time.Time // Filter by date range end
	Highlight   bool       // Enable highlighting
	HighlightTag string    // Tag for highlighting (default: <mark>)
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

	return &HybridSearcher{
		vectorStore:      store,
		embeddingProvider: provider,
		config: HybridSearchConfig{
			VectorWeight:   vectorWeight,
			KeywordWeight:  keywordWeight,
			MinVectorScore: float32(cfg.Search.MinScore),
			MaxResults:     cfg.Search.MaxResults,
			EnableRerank:   false,
		},
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

	// Perform both searches in parallel
	var vectorResults []VectorSearchResult
	var keywordResults []VectorSearchResult
	var vectorErr, keywordErr error

	var wg sync.WaitGroup
	wg.Add(2)

	// Vector search
	go func() {
		defer wg.Done()
		if h.embeddingProvider != nil {
			queryEmb, err := h.embeddingProvider.Embed(ctx, query)
			if err != nil {
				vectorErr = err
				return
			}
			vectorResults, vectorErr = h.vectorStore.SearchVector(ctx, queryEmb, limit*2, h.config.MinVectorScore)
		}
	}()

	// Keyword search
	go func() {
		defer wg.Done()
		keywordResults, keywordErr = h.vectorStore.SearchKeyword(ctx, query, limit*2)
	}()

	wg.Wait()

	// Combine results
	results := h.combineResults(vectorResults, keywordResults, vectorErr, keywordErr)

	// Apply date range filter
	if opts.StartDate != nil || opts.EndDate != nil {
		results = h.filterByDateRange(results, opts.StartDate, opts.EndDate)
	}

	// Add highlighting if enabled
	if opts.Highlight {
		tag := opts.HighlightTag
		if tag == "" {
			tag = "mark"
		}
		results = h.addHighlights(results, query, tag)
	}

	// Sort by combined score
	sort.Slice(results, func(i, j int) bool {
		return results[i].CombinedScore > results[j].CombinedScore
	})

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// filterByDateRange filters results by date range.
func (h *HybridSearcher) filterByDateRange(results []HybridSearchResult, start, end *time.Time) []HybridSearchResult {
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
func (h *HybridSearcher) addHighlights(results []HybridSearchResult, query string, tag string) []HybridSearchResult {
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

				// Find and highlight the term in context
				idx := strings.Index(contentLower, term)
				if idx >= 0 {
					// Get surrounding context (50 chars before and after)
					start := idx - 50
					if start < 0 {
						start = 0
					}
					end := idx + len(term) + 50
					if end > len(content) {
						end = len(content)
					}

					snippet := content[start:end]
					// Highlight the term (case-insensitive replacement)
					highlighted := highlightTerm(snippet, term, openTag, closeTag)

					// Add ellipsis if truncated
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

// SearchVectorOnly performs vector-only search.
func (h *HybridSearcher) SearchVectorOnly(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	if h.embeddingProvider == nil {
		return nil, fmt.Errorf("embedding provider not configured")
	}

	queryEmb, err := h.embeddingProvider.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	vectorResults, err := h.vectorStore.SearchVector(ctx, queryEmb, limit, h.config.MinVectorScore)
	if err != nil {
		return nil, err
	}

	results := make([]HybridSearchResult, len(vectorResults))
	for i, vr := range vectorResults {
		results[i] = HybridSearchResult{
			Chunk:         vr.Chunk,
			VectorScore:   vr.Score,
			CombinedScore: vr.Score,
			MatchTypes:    []string{"vector"},
		}
	}

	return results, nil
}

// SearchKeywordOnly performs keyword-only search.
func (h *HybridSearcher) SearchKeywordOnly(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	keywordResults, err := h.vectorStore.SearchKeyword(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]HybridSearchResult, len(keywordResults))
	for i, kr := range keywordResults {
		results[i] = HybridSearchResult{
			Chunk:         kr.Chunk,
			KeywordScore:  kr.Score,
			CombinedScore: kr.Score,
			MatchTypes:    []string{"keyword"},
		}
	}

	return results, nil
}

// combineResults merges vector and keyword results with weighted scoring.
func (h *HybridSearcher) combineResults(
	vectorResults []VectorSearchResult,
	keywordResults []VectorSearchResult,
	vectorErr, keywordErr error,
) []HybridSearchResult {
	// Build a map of chunk ID to result
	resultMap := make(map[string]*HybridSearchResult)

	// Normalize vector scores
	var maxVectorScore float32 = 1.0
	if len(vectorResults) > 0 && vectorErr == nil {
		for _, vr := range vectorResults {
			if vr.Score > maxVectorScore {
				maxVectorScore = vr.Score
			}
		}
	}

	// Normalize keyword scores
	var maxKeywordScore float32 = 1.0
	if len(keywordResults) > 0 && keywordErr == nil {
		for _, kr := range keywordResults {
			if kr.Score > maxKeywordScore {
				maxKeywordScore = kr.Score
			}
		}
	}

	// Add vector results
	if vectorErr == nil {
		for _, vr := range vectorResults {
			normalizedScore := vr.Score / maxVectorScore
			resultMap[vr.Chunk.ID] = &HybridSearchResult{
				Chunk:       vr.Chunk,
				VectorScore: normalizedScore,
				MatchTypes:  []string{"vector"},
			}
		}
	}

	// Add/merge keyword results
	if keywordErr == nil {
		for _, kr := range keywordResults {
			normalizedScore := kr.Score / maxKeywordScore
			if existing, ok := resultMap[kr.Chunk.ID]; ok {
				existing.KeywordScore = normalizedScore
				existing.MatchTypes = append(existing.MatchTypes, "keyword")
			} else {
				resultMap[kr.Chunk.ID] = &HybridSearchResult{
					Chunk:        kr.Chunk,
					KeywordScore: normalizedScore,
					MatchTypes:   []string{"keyword"},
				}
			}
		}
	}

	// Calculate combined scores
	results := make([]HybridSearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		// Weighted combination
		r.CombinedScore = r.VectorScore*h.config.VectorWeight + r.KeywordScore*h.config.KeywordWeight

		// Boost for matches in both
		if len(r.MatchTypes) > 1 {
			r.CombinedScore *= 1.2 // 20% boost for hybrid matches
			if r.CombinedScore > 1.0 {
				r.CombinedScore = 1.0
			}
		}

		results = append(results, *r)
	}

	return results
}

// Store stores content with automatic embedding.
func (h *HybridSearcher) Store(ctx context.Context, content string, metadata map[string]string) (*MemoryChunk, error) {
	var emb []float32
	var err error

	if h.embeddingProvider != nil {
		emb, err = h.embeddingProvider.Embed(ctx, content)
		if err != nil {
			// Log error but continue without embedding
			emb = nil
		}
	}

	return h.vectorStore.Store(ctx, content, emb, metadata)
}

// StoreBatch stores multiple contents with automatic embedding.
func (h *HybridSearcher) StoreBatch(ctx context.Context, contents []string, metadatas []map[string]string) error {
	chunks := make([]MemoryChunk, len(contents))

	// Generate embeddings if provider available
	var embeddings [][]float32
	if h.embeddingProvider != nil {
		var err error
		embeddings, err = h.embeddingProvider.EmbedBatch(ctx, contents)
		if err != nil {
			// Continue without embeddings
			embeddings = nil
		}
	}

	for i, content := range contents {
		chunks[i] = MemoryChunk{
			Content: content,
		}
		if embeddings != nil && i < len(embeddings) {
			chunks[i].Embedding = embeddings[i]
		}
		if metadatas != nil && i < len(metadatas) {
			chunks[i].Metadata = metadatas[i]
		}
	}

	return h.vectorStore.StoreBatch(ctx, chunks)
}

// Delete deletes a memory chunk.
func (h *HybridSearcher) Delete(ctx context.Context, id string) error {
	return h.vectorStore.Delete(ctx, id)
}

// Get retrieves a memory chunk by ID.
func (h *HybridSearcher) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return h.vectorStore.Get(ctx, id)
}

// GetAll retrieves all memory chunks.
func (h *HybridSearcher) GetAll(ctx context.Context) ([]MemoryChunk, error) {
	return h.vectorStore.GetAll(ctx)
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

// Recall retrieves relevant memories.
func (s *MemoryService) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	return s.Searcher.Search(ctx, query, limit)
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

// GetAll retrieves all memories.
func (s *MemoryService) GetAll(ctx context.Context) ([]MemoryChunk, error) {
	return s.Searcher.GetAll(ctx)
}

// Prune removes old memories.
func (s *MemoryService) Prune(ctx context.Context) (int, error) {
	return s.Searcher.Prune(ctx)
}

// Stats returns memory statistics.
func (s *MemoryService) Stats(ctx context.Context) (HybridSearchStats, error) {
	return s.Searcher.Stats(ctx)
}
