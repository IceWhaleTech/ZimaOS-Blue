package uiexec

import (
	"hash/fnv"
	"math"
)

// Embedder provides a local embedding fallback for fuzzy name matching.
// The default implementation is deterministic and fully local.
type Embedder interface {
	Embed(text string) []float32
}

// LocalNGramEmbedder turns normalized text into a hashed character n-gram vector.
// This is not a semantic model, but it provides robust approximate matching
// across minor typos and spacing/punctuation differences without any remote calls.
type LocalNGramEmbedder struct {
	Dim int
	N   int
}

func NewLocalNGramEmbedder() *LocalNGramEmbedder {
	return &LocalNGramEmbedder{Dim: 256, N: 3}
}

func (e *LocalNGramEmbedder) Embed(text string) []float32 {
	dim := e.Dim
	if dim <= 0 {
		dim = 256
	}
	n := e.N
	if n <= 0 {
		n = 3
	}

	normalized := normalizeText(text)
	if normalized == "" {
		return make([]float32, dim)
	}

	runes := []rune(normalized)
	vec := make([]float32, dim)
	if len(runes) < n {
		idx := hashToIndex(normalized, dim)
		vec[idx] = 1
		return normalizeVec(vec)
	}
	for i := 0; i+n <= len(runes); i++ {
		gram := string(runes[i : i+n])
		idx := hashToIndex(gram, dim)
		vec[idx]++
	}
	return normalizeVec(vec)
}

func hashToIndex(s string, dim int) int {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return int(h.Sum64() % uint64(dim))
}

func normalizeVec(vec []float32) []float32 {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum <= 0 {
		return vec
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= inv
	}
	return vec
}

func cosine(a []float32, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}
