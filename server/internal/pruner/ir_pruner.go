package pruner

import (
	"context"
	"fmt"
	"strings"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// IRPruner implements Backend using BM25 scoring with unified segmentation.
// Handles both code and non-code content automatically.
type IRPruner struct {
	config Config
	scorer *BM25Scorer
	cache  *LRUScoreCache
}

// NewIRPruner creates a new IR-based pruning backend.
func NewIRPruner(cfg Config) *IRPruner {
	capacity := cfg.CacheCapacity
	if capacity <= 0 {
		capacity = 256
	}
	return &IRPruner{
		config: cfg,
		scorer: NewBM25Scorer(1.2, 0.75),
		cache:  NewScoreCache(capacity),
	}
}

// Prune implements Backend. It detects content type, segments, scores, and prunes.
func (p *IRPruner) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := timeutil.NowTime()

	content := req.GetContent()
	if strings.TrimSpace(content) == "" {
		return &PruneResponse{PrunedContent: "", PrunedCode: ""}, nil
	}

	lineCount := strings.Count(content, "\n") + 1

	// Detect content type (use hint if provided)
	ct := req.ContentType
	if ct == ContentUnknown || ct == 0 {
		minLines := p.config.MinLines
		if minLines <= 0 {
			minLines = 50
		}
		ct = DetectContentType(content, minLines)
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = p.config.Threshold
	}

	// Check cache
	key := CacheKey(content, req.Query)
	scored, cached := p.cache.Get(key)

	if !cached {
		// Segment → Tokenize → Score → Boost
		segments := AutoSegmentize(content, ct)
		if len(segments) == 0 {
			origTokens := EstimateTokens(content)
			return &PruneResponse{
				PrunedContent:   content,
				PrunedCode:      content,
				ContentType:     ct,
				Score:           1.0,
				OriginalLines:   lineCount,
				KeptLines:       lineCount,
				OriginalTokens:  origTokens,
				PrunedTokens:    origTokens,
				CompressionRate: 1.0,
				LatencyMs:       msElapsed(start),
			}, nil
		}

		// Re-tokenize segments for non-code content using TextTokenize
		if ct != ContentCode {
			for i := range segments {
				segments[i].Tokens = TextTokenize(segments[i].Content)
			}
		}

		raw := p.scorer.Score(req.Query, segments)
		scored = BoostScoresWithQuery(raw, req.Query)

		// Add heading boost for non-code
		if ct != ContentCode {
			scored = boostNonCode(scored, lineCount)
		}

		p.cache.Put(key, scored)
	}

	// Compute token budget
	origTokens := EstimateTokens(content)
	keepRatio := 1.0 - threshold
	tokenBudget := int(float64(origTokens) * keepRatio)

	selected := SelectTopK(scored, tokenBudget)

	// Build output with pre-allocated builder
	var builder strings.Builder
	builder.Grow(len(content) / 2) // estimate ~50% kept
	lastEnd := -1
	keptLines := 0

	for _, ss := range selected {
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
	if lastEnd < lineCount-1 {
		gap := lineCount - 1 - lastEnd
		if gap > 0 {
			fmt.Fprintf(&builder, "(filtered %d lines)\n", gap)
		}
	}

	prunedContent := builder.String()
	prunedTokens := EstimateTokens(prunedContent)
	var compressionRate float64
	if origTokens > 0 {
		compressionRate = float64(prunedTokens) / float64(origTokens)
	}

	return &PruneResponse{
		PrunedContent:   prunedContent,
		PrunedCode:      prunedContent, // backward compat
		ContentType:     ct,
		Score:           1.0,
		OriginalLines:   lineCount,
		KeptLines:       keptLines,
		PrunedLines:     lineCount - keptLines,
		OriginalTokens:  origTokens,
		PrunedTokens:    prunedTokens,
		CompressionRate: compressionRate,
		LatencyMs:       msElapsed(start),
	}, nil
}

// Health always returns nil for the IR pruner.
func (p *IRPruner) Health(_ context.Context) error { return nil }

// Close is a no-op for the IR pruner.
func (p *IRPruner) Close() error { return nil }

// boostNonCode applies heading and position boosts for non-code content.
func boostNonCode(scored []ScoredSegment, totalLines int) []ScoredSegment {
	if len(scored) == 0 {
		return nil
	}

	result := make([]ScoredSegment, len(scored))
	copy(result, scored)

	posThreshold := totalLines / 10 // first/last 10%

	for i := range result {
		multiplier := 1.0

		// Heading boost
		if result[i].Segment.Kind == SegmentHeading {
			multiplier *= 1.2
		}

		// Position boost: first/last 10%
		if result[i].Segment.StartLine < posThreshold ||
			result[i].Segment.EndLine > totalLines-posThreshold {
			multiplier *= 1.1
		}

		result[i].Score *= multiplier
	}
	return result
}

func msElapsed(start time.Time) float64 {
	return float64(timeutil.SinceTime(start).Microseconds()) / 1000.0
}
