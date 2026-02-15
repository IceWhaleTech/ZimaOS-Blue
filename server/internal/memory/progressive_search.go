package memory

import (
	"context"
	"strings"
	"time"
)

// SearchDepth controls how much content is returned per result.
type SearchDepth int

const (
	// SearchDepthIndex returns compact index: ID + title + date + type (~50-100 tokens/result).
	SearchDepthIndex SearchDepth = 1
	// SearchDepthContext returns snippets with surrounding context (~100-200 tokens/result).
	SearchDepthContext SearchDepth = 2
	// SearchDepthFull returns complete content (~500-1000 tokens/result).
	SearchDepthFull SearchDepth = 3
)

// IndexResult is the Layer 1 compact result.
type IndexResult struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Date     string   `json:"date"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags,omitempty"`
	Score    float32  `json:"score"`
	TokenEst int      `json:"token_est"`
}

// ContextResult is the Layer 2 result with surrounding context.
type ContextResult struct {
	IndexResult
	Snippet    string   `json:"snippet"`
	Context    []string `json:"context,omitempty"`
	Highlights []string `json:"highlights,omitempty"`
}

// DetailResult is the Layer 3 full content result.
type DetailResult struct {
	ContextResult
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt string            `json:"created_at"`
	UpdatedAt string            `json:"updated_at"`
}

// ProgressiveSearchRequest is the API request for progressive search.
type ProgressiveSearchRequest struct {
	Query string      `json:"query"`
	Depth SearchDepth `json:"depth"`
	IDs   []string    `json:"ids,omitempty"`
	Limit int         `json:"limit,omitempty"`
}

// ProgressiveSearchResponse is the API response.
type ProgressiveSearchResponse struct {
	Depth      SearchDepth `json:"depth"`
	Query      string      `json:"query"`
	TotalFound int         `json:"total_found"`
	TokensUsed int         `json:"tokens_used"`
	Results    interface{} `json:"results"`
}

// ProgressiveSearcher wraps existing search with progressive disclosure.
type ProgressiveSearcher struct {
	searcher *HybridSearcher
}

// NewProgressiveSearcher creates a new progressive searcher.
func NewProgressiveSearcher(searcher *HybridSearcher) *ProgressiveSearcher {
	return &ProgressiveSearcher{searcher: searcher}
}

// Search dispatches to the appropriate layer based on depth.
func (p *ProgressiveSearcher) Search(ctx context.Context, req ProgressiveSearchRequest) (*ProgressiveSearchResponse, error) {
	if req.Limit <= 0 {
		switch req.Depth {
		case SearchDepthIndex:
			req.Limit = 20
		default:
			req.Limit = 5
		}
	}

	switch req.Depth {
	case SearchDepthContext:
		results, err := p.SearchContext(ctx, req.Query, req.IDs, req.Limit)
		if err != nil {
			return nil, err
		}
		return &ProgressiveSearchResponse{
			Depth:      SearchDepthContext,
			Query:      req.Query,
			TotalFound: len(results),
			TokensUsed: estimateContextTokens(results),
			Results:    results,
		}, nil

	case SearchDepthFull:
		results, err := p.SearchDetail(ctx, req.IDs)
		if err != nil {
			return nil, err
		}
		return &ProgressiveSearchResponse{
			Depth:      SearchDepthFull,
			Query:      req.Query,
			TotalFound: len(results),
			TokensUsed: estimateDetailTokens(results),
			Results:    results,
		}, nil

	default: // SearchDepthIndex
		results, err := p.SearchIndex(ctx, req.Query, req.Limit)
		if err != nil {
			return nil, err
		}
		return &ProgressiveSearchResponse{
			Depth:      SearchDepthIndex,
			Query:      req.Query,
			TotalFound: len(results),
			TokensUsed: estimateIndexTokens(results),
			Results:    results,
		}, nil
	}
}

// SearchIndex performs Layer 1 search: compact index with no full content.
func (p *ProgressiveSearcher) SearchIndex(ctx context.Context, query string, limit int) ([]IndexResult, error) {
	hybridResults, err := p.searcher.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	results := make([]IndexResult, 0, len(hybridResults))
	for _, r := range hybridResults {
		results = append(results, IndexResult{
			ID:       r.Chunk.ID,
			Title:    extractTitle(r.Chunk.Content),
			Date:     formatTime(r.Chunk.CreatedAt),
			Type:     inferType(r.Chunk.Metadata),
			Tags:     extractTags(r.Chunk.Metadata),
			Score:    r.CombinedScore,
			TokenEst: estimateTokens(r.Chunk.Content),
		})
	}
	return results, nil
}

// SearchContext performs Layer 2 search: snippets with context.
func (p *ProgressiveSearcher) SearchContext(ctx context.Context, query string, ids []string, limit int) ([]ContextResult, error) {
	var chunks []HybridSearchResult

	if len(ids) > 0 {
		// Fetch specific IDs
		for _, id := range ids {
			chunk, err := p.searcher.Get(ctx, id)
			if err != nil {
				continue
			}
			chunks = append(chunks, HybridSearchResult{Chunk: *chunk, CombinedScore: 1.0})
		}
	} else if query != "" {
		var err error
		chunks, err = p.searcher.Search(ctx, query, limit)
		if err != nil {
			return nil, err
		}
	}

	results := make([]ContextResult, 0, len(chunks))
	for _, r := range chunks {
		results = append(results, ContextResult{
			IndexResult: IndexResult{
				ID:       r.Chunk.ID,
				Title:    extractTitle(r.Chunk.Content),
				Date:     formatTime(r.Chunk.CreatedAt),
				Type:     inferType(r.Chunk.Metadata),
				Tags:     extractTags(r.Chunk.Metadata),
				Score:    r.CombinedScore,
				TokenEst: estimateTokens(r.Chunk.Content),
			},
			Snippet:    extractSnippet(r.Chunk.Content, query, 200),
			Context:    extractContextLines(r.Chunk.Content, query, 2),
			Highlights: r.Highlights,
		})
	}
	return results, nil
}

// SearchDetail performs Layer 3 search: full content for specific IDs.
func (p *ProgressiveSearcher) SearchDetail(ctx context.Context, ids []string) ([]DetailResult, error) {
	results := make([]DetailResult, 0, len(ids))
	for _, id := range ids {
		chunk, err := p.searcher.Get(ctx, id)
		if err != nil {
			continue
		}
		results = append(results, DetailResult{
			ContextResult: ContextResult{
				IndexResult: IndexResult{
					ID:       chunk.ID,
					Title:    extractTitle(chunk.Content),
					Date:     formatTime(chunk.CreatedAt),
					Type:     inferType(chunk.Metadata),
					Tags:     extractTags(chunk.Metadata),
					Score:    1.0,
					TokenEst: estimateTokens(chunk.Content),
				},
				Snippet: extractSnippet(chunk.Content, "", 200),
			},
			Content:   chunk.Content,
			Metadata:  chunk.Metadata,
			CreatedAt: formatTime(chunk.CreatedAt),
			UpdatedAt: formatTime(chunk.UpdatedAt),
		})
	}
	return results, nil
}

// --- Helper functions ---

// extractTitle returns the first line or first 80 chars of content.
func extractTitle(content string) string {
	if idx := strings.IndexByte(content, '\n'); idx > 0 && idx <= 80 {
		return strings.TrimSpace(content[:idx])
	}
	if len(content) > 80 {
		return strings.TrimRight(content[:80], " ") + "..."
	}
	return content
}

// extractSnippet returns a ~maxLen char excerpt around the best keyword match.
func extractSnippet(content, query string, maxLen int) string {
	if content == "" {
		return ""
	}
	if query == "" || maxLen <= 0 {
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerContent, lowerQuery)
	if idx < 0 {
		// No exact match, return beginning
		if len(content) > maxLen {
			return content[:maxLen] + "..."
		}
		return content
	}

	// Center the snippet around the match
	half := maxLen / 2
	start := idx - half
	if start < 0 {
		start = 0
	}
	end := start + maxLen
	if end > len(content) {
		end = len(content)
		start = end - maxLen
		if start < 0 {
			start = 0
		}
	}

	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}

// extractContextLines returns N lines before and after the first match.
func extractContextLines(content, query string, n int) []string {
	lines := strings.Split(content, "\n")
	if len(lines) <= n*2+1 {
		return lines
	}

	if query == "" {
		// Return first few lines
		end := n*2 + 1
		if end > len(lines) {
			end = len(lines)
		}
		return lines[:end]
	}

	lowerQuery := strings.ToLower(query)
	matchIdx := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), lowerQuery) {
			matchIdx = i
			break
		}
	}
	if matchIdx < 0 {
		matchIdx = 0
	}

	start := matchIdx - n
	if start < 0 {
		start = 0
	}
	end := matchIdx + n + 1
	if end > len(lines) {
		end = len(lines)
	}
	return lines[start:end]
}

func inferType(metadata map[string]string) string {
	if t, ok := metadata["type"]; ok {
		return t
	}
	if _, ok := metadata["layer"]; ok {
		return metadata["layer"]
	}
	return "memory"
}

func extractTags(metadata map[string]string) []string {
	var tags []string
	for k, v := range metadata {
		if strings.HasPrefix(k, "tag_") {
			tags = append(tags, v)
		}
	}
	return tags
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

// estimateTokens estimates token count (~4 chars per token).
func estimateTokens(content string) int {
	n := len(content) / 4
	if n == 0 && len(content) > 0 {
		n = 1
	}
	return n
}

func estimateIndexTokens(results []IndexResult) int {
	total := 0
	for _, r := range results {
		// ~30 tokens per index entry (id + title + date + type + score)
		total += 30 + len(r.Title)/4
	}
	return total
}

func estimateContextTokens(results []ContextResult) int {
	total := 0
	for _, r := range results {
		total += 30 + estimateTokens(r.Snippet)
		for _, c := range r.Context {
			total += estimateTokens(c)
		}
	}
	return total
}

func estimateDetailTokens(results []DetailResult) int {
	total := 0
	for _, r := range results {
		total += estimateTokens(r.Content) + 30
	}
	return total
}
