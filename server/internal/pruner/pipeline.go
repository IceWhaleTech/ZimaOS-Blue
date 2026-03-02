package pruner

import (
	"container/heap"
	"context"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"sort"
	"strings"
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

	// Keep the best segment as fallback so outputs are never fully empty.
	bestIdx := 0
	for i := 1; i < len(scored); i++ {
		if topKCandidateBetter(i, scored[i], bestIdx, scored[bestIdx]) {
			bestIdx = i
		}
	}

	h := make(topKMinHeap, 0, minInt(len(scored), 32))
	usedTokens := 0
	for i, ss := range scored {
		tokens := segmentTokens(&ss.Segment)
		if tokens < 0 {
			tokens = 0
		}

		cand := topKCandidate{
			idx:    i,
			scored: ss,
			tokens: tokens,
		}

		if usedTokens+tokens <= tokenBudget {
			heap.Push(&h, cand)
			usedTokens += tokens
			continue
		}
		if h.Len() == 0 {
			continue
		}
		if !topKCandidateBetter(cand.idx, cand.scored, h[0].idx, h[0].scored) {
			continue
		}

		removed := make([]topKCandidate, 0, 4)
		for usedTokens+tokens > tokenBudget && h.Len() > 0 {
			worst := h[0]
			if !topKCandidateBetter(cand.idx, cand.scored, worst.idx, worst.scored) {
				break
			}
			popped := heap.Pop(&h).(topKCandidate)
			usedTokens -= popped.tokens
			removed = append(removed, popped)
		}
		if usedTokens+tokens <= tokenBudget {
			heap.Push(&h, cand)
			usedTokens += tokens
			continue
		}
		for _, r := range removed {
			heap.Push(&h, r)
			usedTokens += r.tokens
		}
	}

	selected := make([]topKCandidate, 0, h.Len()+1)
	selectedIdx := make(map[int]struct{}, h.Len()+1)
	for h.Len() > 0 {
		item := heap.Pop(&h).(topKCandidate)
		selected = append(selected, item)
		selectedIdx[item.idx] = struct{}{}
	}
	if _, ok := selectedIdx[bestIdx]; !ok {
		selected = append(selected, topKCandidate{
			idx:    bestIdx,
			scored: scored[bestIdx],
			tokens: segmentTokens(&scored[bestIdx].Segment),
		})
		selectedIdx[bestIdx] = struct{}{}
	}

	// Re-sort by original position for coherent output.
	sort.Slice(selected, func(i, j int) bool {
		if selected[i].scored.Segment.StartLine == selected[j].scored.Segment.StartLine {
			return selected[i].idx < selected[j].idx
		}
		return selected[i].scored.Segment.StartLine < selected[j].scored.Segment.StartLine
	})

	out := make([]ScoredSegment, len(selected))
	for i := range selected {
		out[i] = selected[i].scored
	}
	return out
}

type topKCandidate struct {
	idx    int
	scored ScoredSegment
	tokens int
}

type topKMinHeap []topKCandidate

func (h topKMinHeap) Len() int { return len(h) }

func (h topKMinHeap) Less(i, j int) bool {
	// Min-heap by score (worst on top). Tie-break keeps earlier segments.
	if h[i].scored.Score != h[j].scored.Score {
		return h[i].scored.Score < h[j].scored.Score
	}
	if h[i].scored.Segment.StartLine != h[j].scored.Segment.StartLine {
		return h[i].scored.Segment.StartLine > h[j].scored.Segment.StartLine
	}
	return h[i].idx > h[j].idx
}

func (h topKMinHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *topKMinHeap) Push(x any) {
	*h = append(*h, x.(topKCandidate))
}

func (h *topKMinHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

func topKCandidateBetter(i int, a ScoredSegment, j int, b ScoredSegment) bool {
	if a.Score != b.Score {
		return a.Score > b.Score
	}
	if a.Segment.StartLine != b.Segment.StartLine {
		return a.Segment.StartLine < b.Segment.StartLine
	}
	return i < j
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// importKeywords are terms that indicate import/include segments.
var importKeywords = []string{"import", "require", "include", "use", "from"}

func isImportToken(tok string) bool {
	switch tok {
	case "import", "require", "include", "use", "from":
		return true
	default:
		return false
	}
}

// BoostScores applies rule-based weight boosting to scored segments.
// Structural segments (functions, classes) get a multiplier boost.
// Import/include segments get an additional boost.
func BoostScores(scored []ScoredSegment) []ScoredSegment {
	return BoostScoresWithQuery(scored, "")
}

// BoostScoresWithQuery applies rule-based weight boosting with query-aware name matching.
// Segments whose name matches query terms get a significant boost.
func BoostScoresWithQuery(scored []ScoredSegment, query string) []ScoredSegment {
	return boostScoresWithQuery(scored, query, true)
}

func boostScoresWithQueryInPlace(scored []ScoredSegment, query string) []ScoredSegment {
	return boostScoresWithQuery(scored, query, false)
}

func boostScoresWithQuery(scored []ScoredSegment, query string, clone bool) []ScoredSegment {
	if len(scored) == 0 {
		return nil
	}

	queryTokens := codeTokenize(query)
	var querySet map[string]struct{}
	if len(queryTokens) > 0 {
		querySet = make(map[string]struct{}, len(queryTokens))
		for _, qt := range queryTokens {
			querySet[qt] = struct{}{}
		}
	}

	result := scored
	if clone {
		result = make([]ScoredSegment, len(scored))
		copy(result, scored)
	}

	for i := range result {
		multiplier := structuralMultiplier(result[i].Segment.Kind)

		// Import boost: check pre-tokenized tokens (fast path)
		// Falls back to content scan if tokens are empty (e.g. manually constructed segments)
		importFound := false
		if len(result[i].Segment.Tokens) > 0 {
			for _, tok := range result[i].Segment.Tokens {
				if isImportToken(tok) {
					importFound = true
					break
				}
			}
		} else {
			importFound = containsAnyImportKeywordFoldASCII(result[i].Segment.Content)
		}
		if importFound {
			multiplier *= 1.15
		}

		// Name-match boost: if segment name contains query terms, big boost
		if querySet != nil && result[i].Segment.Name != "" {
			nameTokens := result[i].Segment.NameTokens
			if len(nameTokens) == 0 {
				nameTokens = codeTokenize(result[i].Segment.Name)
			}
			for _, nt := range nameTokens {
				if _, ok := querySet[nt]; ok {
					multiplier *= 1.5
					break
				}
			}
		}

		result[i].Score *= multiplier
	}
	return result
}

func containsAnyImportKeywordFoldASCII(s string) bool {
	for _, kw := range importKeywords {
		if containsASCIIFold(s, kw) {
			return true
		}
	}
	return false
}

func containsASCIIFold(s, sub string) bool {
	n := len(s)
	m := len(sub)
	if m == 0 {
		return true
	}
	if m > n {
		return false
	}
	for i := 0; i <= n-m; i++ {
		j := 0
		for ; j < m; j++ {
			c := s[i+j]
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c != sub[j] {
				break
			}
		}
		if j == m {
			return true
		}
	}
	return false
}

func structuralMultiplier(kind SegmentKind) float64 {
	switch kind {
	case SegmentFunction:
		return 1.3
	case SegmentClass:
		return 1.25
	case SegmentBlock:
		return 1.1
	default:
		return 1.0
	}
}

// BM25Backend implements Backend using BM25 scoring with segment indexing.
type BM25Backend struct {
	config Config
	scorer *BM25Scorer
	cache  *LRUScoreCache
}

// NewBM25Backend creates a new BM25-based pruning backend.
func NewBM25Backend(cfg Config) *BM25Backend {
	capacity := cfg.CacheCapacity
	if capacity <= 0 {
		capacity = 256
	}
	return &BM25Backend{
		config: cfg,
		scorer: NewBM25Scorer(1.2, 0.75),
		cache:  NewScoreCache(capacity),
	}
}

// Prune implements Backend using the BM25 + segment pipeline.
func (b *BM25Backend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := timeutil.NowTime()

	lines := strings.Split(req.Code, "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return &PruneResponse{PrunedCode: req.Code}, nil
	}

	threshold := resolveThreshold(req.Threshold, b.config.Threshold)

	// Check cache
	key := CacheKey(req.Code, req.Query)
	scored, cached := b.cache.Get(key)

	if !cached {
		// Segmentize → Score → Boost (with query-aware name matching)
		segments := Segmentize(req.Code)
		raw := b.scorer.Score(req.Query, segments)
		scored = boostScoresWithQueryInPlace(raw, req.Query)
		b.cache.Put(key, scored)
	}

	// Compute token budget: threshold controls aggressiveness
	// threshold=0.5 → keep ~50% of tokens, threshold=0.3 → keep ~70%
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
		LatencyMs:       float64(timeutil.SinceTime(start).Microseconds()) / 1000.0,
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
