package uiexec

import (
	"math"
	"sort"
	"strings"
)

type Finder struct {
	nodes    []Node
	embedder Embedder

	normalizedName []string
	nameEmbedding  [][]float32
}

func NewFinder(nodes []Node) *Finder {
	return NewFinderWithEmbedder(nodes, NewLocalNGramEmbedder())
}

func NewFinderWithEmbedder(nodes []Node, embedder Embedder) *Finder {
	copied := append([]Node(nil), nodes...)
	for i := range copied {
		copied[i].Actions = normalizeActions(copied[i].Actions)
	}
	f := &Finder{
		nodes:          copied,
		embedder:       embedder,
		normalizedName: make([]string, len(copied)),
		nameEmbedding:  make([][]float32, len(copied)),
	}
	for i, n := range copied {
		f.normalizedName[i] = normalizeText(n.Name)
		if embedder != nil && strings.TrimSpace(n.Name) != "" {
			f.nameEmbedding[i] = embedder.Embed(n.Name)
		}
	}
	return f
}

func (f *Finder) FindNode(query Query) *Node {
	if f == nil || len(f.nodes) == 0 {
		return nil
	}
	roleKey := strings.TrimSpace(strings.ToLower(query.Role))
	nameKey := normalizeText(query.Name)
	approxKey := normalizeText(query.NameApprox)
	requiredCap := normalizeText(query.RequireCapability)

	type scored struct {
		idx   int
		score float64
	}
	candidates := make([]scored, 0, 16)

	for i, n := range f.nodes {
		if roleKey != "" && !strings.EqualFold(strings.TrimSpace(n.Role), roleKey) {
			continue
		}
		if requiredCap != "" && !hasAction(n.Actions, requiredCap) {
			continue
		}

		score := 1.0
		if nameKey != "" {
			s := stringFuzzyScore(f.normalizedName[i], nameKey)
			if s <= 0 {
				continue
			}
			score += s * 100
		}
		if approxKey != "" {
			s := approxScore(f, i, approxKey)
			if s < 0.35 {
				continue
			}
			score += s * 120
		}
		candidates = append(candidates, scored{idx: i, score: score})
	}

	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(i int, j int) bool {
		if candidates[i].score == candidates[j].score {
			return f.nodes[candidates[i].idx].ID < f.nodes[candidates[j].idx].ID
		}
		return candidates[i].score > candidates[j].score
	})
	best := f.nodes[candidates[0].idx]
	return &best
}

func stringFuzzyScore(candidateNormalized string, queryNormalized string) float64 {
	if queryNormalized == "" {
		return 0
	}
	if candidateNormalized == queryNormalized {
		return 1.0
	}
	if strings.Contains(candidateNormalized, queryNormalized) {
		return 0.85
	}
	overlap := tokenOverlapScore(candidateNormalized, queryNormalized)
	if overlap >= 0.2 {
		return 0.5 + overlap*0.4
	}
	return 0
}

func approxScore(f *Finder, idx int, approxQuery string) float64 {
	if f == nil {
		return 0
	}
	candidate := f.normalizedName[idx]
	if candidate == "" || approxQuery == "" {
		return 0
	}
	editSim := normalizedEditSimilarity(candidate, approxQuery)

	embedSim := 0.0
	if f.embedder != nil {
		qvec := f.embedder.Embed(approxQuery)
		cvec := f.nameEmbedding[idx]
		if len(cvec) == 0 {
			cvec = f.embedder.Embed(candidate)
		}
		// Local hashed n-gram vectors are non-negative and normalized, so cosine is in [0..1].
		embedSim = cosine(qvec, cvec)
	}
	return math.Max(editSim, embedSim)
}

func normalizedEditSimilarity(a string, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	dist := levenshtein([]rune(a), []rune(b))
	maxLen := len([]rune(a))
	if lb := len([]rune(b)); lb > maxLen {
		maxLen = lb
	}
	if maxLen == 0 {
		return 0
	}
	sim := 1.0 - float64(dist)/float64(maxLen)
	if sim < 0 {
		return 0
	}
	return sim
}

func levenshtein(a []rune, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// O(min(len(a),len(b))) memory DP.
	if len(a) < len(b) {
		a, b = b, a
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := current[j-1] + 1
			sub := prev[j-1] + cost
			current[j] = minInt(del, ins, sub)
		}
		prev = current
	}
	return prev[len(b)]
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	best := values[0]
	for _, v := range values[1:] {
		if v < best {
			best = v
		}
	}
	return best
}
