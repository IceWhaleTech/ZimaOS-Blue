package pruner

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode"
)

// LocalBackend implements Backend using a pure-Go scoring algorithm.
// Inspired by SWE-Pruner's line-level approach, it uses TF-IDF relevance
// scoring combined with structural heuristics to identify and prune
// low-relevance code lines without requiring a neural model.
//
// Reference: https://arxiv.org/abs/2601.16746
type LocalBackend struct {
	config Config
}

// NewLocalBackend creates a new Go-native pruning backend.
func NewLocalBackend(cfg Config) *LocalBackend {
	return &LocalBackend{config: cfg}
}

// lineScore holds the relevance score for a single line.
type lineScore struct {
	index int
	score float64
	text  string
}

// structuralKeywords are tokens that mark structurally important lines.
// These lines are always kept regardless of relevance score.
var structuralKeywords = map[string]float64{
	"package":   1.0,
	"import":    0.9,
	"func":      0.95,
	"type":      0.95,
	"struct":    0.95,
	"interface": 0.95,
	"class":     0.95,
	"def":       0.95,
	"return":    0.7,
	"if":        0.6,
	"else":      0.6,
	"for":       0.6,
	"switch":    0.6,
	"case":      0.6,
	"const":     0.8,
	"var":       0.7,
	"let":       0.7,
	"export":    0.8,
	"module":    0.8,
	"require":   0.8,
	"#include":  0.9,
	"#define":   0.8,
}

// Prune scores each line and removes low-relevance lines.
func (b *LocalBackend) Prune(ctx context.Context, req PruneRequest) (*PruneResponse, error) {
	start := time.Now()

	lines := strings.Split(req.Code, "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return &PruneResponse{PrunedCode: req.Code}, nil
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = b.config.Threshold
	}

	// Build query terms for TF-IDF matching
	queryTerms := tokenize(req.Query)

	// Score each line
	scores := make([]lineScore, len(lines))
	for i, line := range lines {
		scores[i] = lineScore{
			index: i,
			score: scoreLine(line, queryTerms, i, len(lines)),
			text:  line,
		}
	}

	// Build pruned output: group consecutive low-score lines into markers
	var builder strings.Builder
	pruneStart := -1
	pruneCount := 0
	keptLines := 0

	for i, ls := range scores {
		if ls.score >= threshold {
			// Flush any pending pruned block
			if pruneCount > 0 {
				fmt.Fprintf(&builder, "(filtered %d lines)\n", pruneCount)
				pruneStart = -1
				pruneCount = 0
			}
			builder.WriteString(ls.text)
			if i < len(scores)-1 {
				builder.WriteByte('\n')
			}
			keptLines++
		} else {
			if pruneStart == -1 {
				pruneStart = i
			}
			pruneCount++
		}
	}
	// Flush trailing pruned block
	if pruneCount > 0 {
		fmt.Fprintf(&builder, "(filtered %d lines)\n", pruneCount)
	}

	prunedCode := builder.String()
	origTokens := EstimateTokens(req.Code)
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

// Health always returns nil for the local backend.
func (b *LocalBackend) Health(_ context.Context) error {
	return nil
}

// Close is a no-op for the local backend.
func (b *LocalBackend) Close() error {
	return nil
}

// scoreLine computes a relevance score for a single line (0.0–1.0).
// The score combines: structural importance, query relevance, and position.
func scoreLine(line string, queryTerms map[string]int, lineIdx, totalLines int) float64 {
	trimmed := strings.TrimSpace(line)

	// Empty lines and pure whitespace get a base score
	if trimmed == "" {
		return 0.3
	}

	// Bracket-only lines (closing braces) are structurally important
	if trimmed == "}" || trimmed == "};" || trimmed == ")" || trimmed == "]" {
		return 0.85
	}

	score := 0.0

	// 1. Structural keyword boost (0–1.0)
	firstWord := firstToken(trimmed)
	if boost, ok := structuralKeywords[firstWord]; ok {
		score = math.Max(score, boost)
	}

	// 2. Comment lines get moderate score
	if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") {
		score = math.Max(score, 0.4)
	}

	// 3. Query relevance via term overlap (TF-IDF inspired)
	if len(queryTerms) > 0 {
		lineTerms := tokenize(trimmed)
		overlap := 0
		for term := range queryTerms {
			if _, ok := lineTerms[term]; ok {
				overlap++
			}
		}
		if overlap > 0 {
			relevance := float64(overlap) / float64(len(queryTerms))
			// IDF-like boost: rarer matches score higher
			idfBoost := math.Log2(float64(len(queryTerms)+1)) / math.Log2(float64(len(queryTerms)+2))
			queryScore := math.Min(relevance*idfBoost*1.5, 1.0)
			score = math.Max(score, queryScore)
		}
	}

	// 4. Position bias: first and last 10% of file are more important
	posRatio := float64(lineIdx) / float64(max(totalLines, 1))
	if posRatio < 0.1 || posRatio > 0.9 {
		score = math.Max(score, 0.7)
	}

	// 5. Lines with assignments or function calls get a small boost
	if strings.Contains(trimmed, "=") || strings.Contains(trimmed, "(") {
		score = math.Max(score, 0.45)
	}

	return score
}

// tokenize splits text into lowercase word tokens with camelCase/snake_case expansion,
// returning term frequencies. This enables query expansion so "authenticateUser"
// matches lines containing "authenticate" or "user".
func tokenize(text string) map[string]int {
	terms := make(map[string]int)
	if text == "" {
		return terms
	}
	// Use codeTokenize for camelCase/snake_case splitting
	expanded := codeTokenize(text)
	for _, w := range expanded {
		terms[w]++
	}
	// Also add the original unsplit tokens for exact matching
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for _, w := range words {
		if len(w) >= 2 {
			terms[w]++
		}
	}
	return terms
}

// firstToken returns the first whitespace-delimited token of a line.
func firstToken(line string) string {
	for i, r := range line {
		if unicode.IsSpace(r) {
			return strings.ToLower(line[:i])
		}
	}
	return strings.ToLower(line)
}
