package pruner

import (
	"math"
	"strings"
	"unicode"
)

// ContentType identifies the type of content being pruned.
type ContentType int

const (
	ContentCode    ContentType = iota // Source code (Go, Python, JS, etc.)
	ContentDoc                        // Documentation, markdown, prose
	ContentLog                        // Log output, structured text
	ContentData                       // JSON, YAML, structured data
	ContentUnknown                    // Fallback — treated as non-code
)

// SegmentKind identifies the type of segment.
type SegmentKind int

const (
	// Code segments
	SegmentFunction SegmentKind = iota
	SegmentClass
	SegmentBlock
	SegmentLines

	// Non-code segments
	SegmentParagraph
	SegmentHeading
	SegmentSentence
	SegmentLogGroup
	SegmentDataKey
)

// Segment represents a code unit (function, class, block, or line range).
type Segment struct {
	StartLine  int
	EndLine    int
	Kind       SegmentKind
	Name       string
	Content    string
	Tokens     []string // pre-tokenized content
	TokenCount int      // pre-computed token estimate (0 = not set)
}

// NewSegment creates a Segment with pre-computed token count.
func NewSegment(startLine, endLine int, kind SegmentKind, name, content string, tokens []string) Segment {
	return Segment{
		StartLine:  startLine,
		EndLine:    endLine,
		Kind:       kind,
		Name:       name,
		Content:    content,
		Tokens:     tokens,
		TokenCount: EstimateTokens(content),
	}
}

// ScoredSegment pairs a segment with its relevance score.
type ScoredSegment struct {
	Segment Segment
	Score   float64
}

// Scorer is the pluggable scoring interface.
type Scorer interface {
	Score(query string, segments []Segment) []ScoredSegment
}

// BM25Scorer implements Scorer using Okapi BM25.
type BM25Scorer struct {
	K1 float64 // term frequency saturation (default 1.2)
	B  float64 // length normalization (default 0.75)
}

// NewBM25Scorer creates a BM25 scorer with the given parameters.
func NewBM25Scorer(k1, b float64) *BM25Scorer {
	return &BM25Scorer{K1: k1, B: b}
}

// Score computes BM25 relevance scores for each segment against the query.
func (s *BM25Scorer) Score(query string, segments []Segment) []ScoredSegment {
	if len(segments) == 0 {
		return nil
	}

	queryTokens := codeTokenize(query)
	if len(queryTokens) == 0 {
		results := make([]ScoredSegment, len(segments))
		for i, seg := range segments {
			results[i] = ScoredSegment{Segment: seg, Score: 0}
		}
		return results
	}

	// Compute average document length
	totalLen := 0
	for _, seg := range segments {
		totalLen += len(seg.Tokens)
	}
	avgDL := float64(totalLen) / float64(len(segments))

	// Build document frequency for IDF (reuse seen map to reduce allocations)
	df := make(map[string]int)
	seen := make(map[string]bool)
	for _, seg := range segments {
		for k := range seen {
			delete(seen, k)
		}
		for _, tok := range seg.Tokens {
			if !seen[tok] {
				df[tok]++
				seen[tok] = true
			}
		}
	}

	n := float64(len(segments))
	if avgDL < 1 {
		avgDL = 1
	}

	// Pre-compute IDF for query terms
	idfCache := make(map[string]float64, len(queryTokens))
	for _, qt := range queryTokens {
		docFreq := float64(df[qt])
		idfCache[qt] = math.Log((n-docFreq+0.5)/(docFreq+0.5) + 1.0)
	}

	results := make([]ScoredSegment, len(segments))
	for i, seg := range segments {
		score := 0.0
		tf := buildTF(seg.Tokens)
		dl := float64(len(seg.Tokens))

		for _, qt := range queryTokens {
			termFreq := float64(tf[qt])
			if termFreq == 0 {
				continue
			}
			tfNorm := (termFreq * (s.K1 + 1)) / (termFreq + s.K1*(1-s.B+s.B*dl/avgDL))
			score += idfCache[qt] * tfNorm
		}
		results[i] = ScoredSegment{Segment: seg, Score: score}
	}
	return results
}

// buildTF builds a term frequency map from a token slice.
func buildTF(tokens []string) map[string]int {
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}
	return tf
}

// codeTokenize splits an identifier or code text into normalized tokens.
// It handles camelCase, snake_case, and mixed patterns.
func codeTokenize(text string) []string {
	if text == "" {
		return nil
	}

	// First split on non-alphanumeric boundaries (spaces, punctuation, underscores)
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	var tokens []string
	for _, word := range words {
		// Split camelCase / PascalCase
		tokens = append(tokens, splitCamelCase(word)...)
	}

	// Lowercase and filter short tokens
	result := make([]string, 0, len(tokens))
	for _, t := range tokens {
		lower := strings.ToLower(t)
		if len(lower) >= 2 {
			result = append(result, lower)
		}
	}
	return result
}

// splitCamelCase splits a word on camelCase boundaries.
// "parseHTTPResponse" -> ["parse", "HTTP", "Response"]
func splitCamelCase(s string) []string {
	if s == "" {
		return nil
	}

	var parts []string
	runes := []rune(s)
	start := 0

	for i := 1; i < len(runes); i++ {
		// Split on lower->upper transition: "parseHTTP" -> "parse" + "HTTP"
		if unicode.IsLower(runes[i-1]) && unicode.IsUpper(runes[i]) {
			parts = append(parts, string(runes[start:i]))
			start = i
			continue
		}
		// Split on upper->upper->lower: "HTTPResponse" -> "HTTP" + "Response"
		if i+1 < len(runes) && unicode.IsUpper(runes[i-1]) && unicode.IsUpper(runes[i]) && unicode.IsLower(runes[i+1]) {
			parts = append(parts, string(runes[start:i]))
			start = i
			continue
		}
	}
	parts = append(parts, string(runes[start:]))
	return parts
}
