package pruner

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// segmentTokens returns the pre-computed token count or estimates it.
func segmentTokens(seg *Segment) int {
	if seg.TokenCount > 0 {
		return seg.TokenCount
	}
	return EstimateTokens(seg.Content)
}

// SelectTopK selects the highest-scoring segments that fit within the token budget.
// Segments are returned sorted by their original position (StartLine).
func SelectTopK(scored []ScoredSegment, tokenBudget int) []ScoredSegment {
	if len(scored) == 0 {
		return nil
	}

	// Sort by score descending
	sorted := make([]ScoredSegment, len(scored))
	copy(sorted, scored)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Score > sorted[j].Score
	})

	var selected []ScoredSegment
	usedTokens := 0
	for _, ss := range sorted {
		tokens := segmentTokens(&ss.Segment)
		if usedTokens+tokens > tokenBudget && len(selected) > 0 {
			break
		}
		selected = append(selected, ss)
		usedTokens += tokens
	}

	// Re-sort by original position for coherent output
	sort.Slice(selected, func(i, j int) bool {
		return selected[i].Segment.StartLine < selected[j].Segment.StartLine
	})
	return selected
}

// boostRules defines structural boost multipliers by segment kind.
var boostRules = map[SegmentKind]float64{
	SegmentFunction: 1.3,
	SegmentClass:    1.25,
	SegmentBlock:    1.1,
	SegmentLines:    1.0,
}

// importKeywords are terms that indicate import/include segments.
var importKeywords = []string{"import", "require", "include", "use", "from"}

// BoostScores applies rule-based weight boosting to scored segments.
// Structural segments (functions, classes) get a multiplier boost.
// Import/include segments get an additional boost.
func BoostScores(scored []ScoredSegment) []ScoredSegment {
	return BoostScoresWithQuery(scored, "")
}

// BoostScoresWithQuery applies rule-based weight boosting with query-aware name matching.
// Segments whose name matches query terms get a significant boost.
func BoostScoresWithQuery(scored []ScoredSegment, query string) []ScoredSegment {
	if len(scored) == 0 {
		return nil
	}

	queryTokens := codeTokenize(query)
	querySet := make(map[string]bool, len(queryTokens))
	for _, qt := range queryTokens {
		querySet[qt] = true
	}

	// Pre-build import keyword set for O(1) lookup
	importSet := make(map[string]bool, len(importKeywords))
	for _, kw := range importKeywords {
		importSet[kw] = true
	}

	result := make([]ScoredSegment, len(scored))
	copy(result, scored)

	for i := range result {
		multiplier := boostRules[result[i].Segment.Kind]
		if multiplier == 0 {
			multiplier = 1.0
		}

		// Import boost: check pre-tokenized tokens (fast path)
		// Falls back to content scan if tokens are empty (e.g. manually constructed segments)
		importFound := false
		if len(result[i].Segment.Tokens) > 0 {
			for _, tok := range result[i].Segment.Tokens {
				if importSet[tok] {
					importFound = true
					break
				}
			}
		} else {
			contentLower := strings.ToLower(result[i].Segment.Content)
			for _, kw := range importKeywords {
				if strings.Contains(contentLower, kw) {
					importFound = true
					break
				}
			}
		}
		if importFound {
			multiplier *= 1.15
		}

		// Name-match boost: if segment name contains query terms, big boost
		if len(querySet) > 0 && result[i].Segment.Name != "" {
			nameTokens := codeTokenize(result[i].Segment.Name)
			for _, nt := range nameTokens {
				if querySet[nt] {
					multiplier *= 1.5
					break
				}
			}
		}

		result[i].Score *= multiplier
	}
	return result
}

// BM25Backend implements Backend using BM25 scoring with segment indexing.
type BM25Backend struct {
	config Config
	scorer *BM25Scorer
	cache  *LRUScoreCache
}

// NewBM25Backend creates a new BM25-based pruning backend.
func NewBM25Backend(cfg Config) *BM25Backend {
	return &BM25Backend{
		config: cfg,
		scorer: NewBM25Scorer(1.2, 0.75),
		cache:  NewScoreCache(256),
	}
}

// Prune implements Backend using the BM25 + segment pipeline.
func (b *BM25Backend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := time.Now()

	lines := strings.Split(req.Code, "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return &PruneResponse{PrunedCode: req.Code}, nil
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = b.config.Threshold
	}

	// Check cache
	key := CacheKey(req.Code, req.Query)
	scored, cached := b.cache.Get(key)

	if !cached {
		// Segmentize → Score → Boost (with query-aware name matching)
		segments := Segmentize(req.Code)
		raw := b.scorer.Score(req.Query, segments)
		scored = BoostScoresWithQuery(raw, req.Query)
		b.cache.Put(key, scored)
	}

	// Compute token budget: threshold controls aggressiveness
	// threshold=0.5 → keep ~40% of tokens, threshold=0.3 → keep ~60%
	origTokens := EstimateTokens(req.Code)
	keepRatio := 1.0 - threshold
	tokenBudget := int(float64(origTokens) * keepRatio)

	selected := SelectTopK(scored, tokenBudget)

	// Build output: selected segments + filtered markers for gaps
	var builder strings.Builder
	builder.Grow(len(req.Code) / 2)
	lastEnd := -1
	keptLines := 0

	for _, ss := range selected {
		// Insert filtered marker for gap
		gap := ss.Segment.StartLine - lastEnd - 1
		if gap > 0 {
			fmt.Fprintf(&builder, "(filtered %d lines)\n", gap)
		}
		builder.WriteString(ss.Segment.Content)
		builder.WriteByte('\n')
		keptLines += ss.Segment.EndLine - ss.Segment.StartLine + 1
		lastEnd = ss.Segment.EndLine
	}

	// Trailing gap
	if lastEnd < len(lines)-1 {
		gap := len(lines) - 1 - lastEnd
		if gap > 0 {
			fmt.Fprintf(&builder, "(filtered %d lines)\n", gap)
		}
	}

	prunedCode := builder.String()
	prunedTokens := EstimateTokens(prunedCode)
	var compressionRate float64
	if origTokens > 0 {
		compressionRate = float64(prunedTokens) / float64(origTokens)
	}

	return &PruneResponse{
		PrunedCode:      prunedCode,
		Score:           1.0,
		OriginalLines:   len(lines),
		KeptLines:       keptLines,
		PrunedLines:     len(lines) - keptLines,
		OriginalTokens:  origTokens,
		PrunedTokens:    prunedTokens,
		CompressionRate: compressionRate,
		LatencyMs:       float64(time.Since(start).Microseconds()) / 1000.0,
	}, nil
}

// Health always returns nil for the BM25 backend.
func (b *BM25Backend) Health(_ context.Context) error {
	return nil
}

// Close is a no-op for the BM25 backend.
func (b *BM25Backend) Close() error {
	return nil
}
