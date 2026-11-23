package querylang

import (
	_ "embed"
	"strings"
	"sync"
)

//go:embed data/simplified-zh-evidence.txt
var simplifiedChineseEvidenceData string

// OpenCC's simplified forms, excluding self mappings and Japanese Shinjitai.
// This is a modern-script heuristic, not a complete Japanese kanji inventory.
// Source pins, derivation and Apache-2.0 attribution are in data/README.md.
var simplifiedChineseEvidence = sync.OnceValue(func() map[rune]bool {
	characters := make(map[rune]bool)
	for _, r := range strings.TrimSpace(simplifiedChineseEvidenceData) {
		characters[r] = true
	}
	return characters
})

func hasSimplifiedChineseEvidence(query string) bool {
	characters := simplifiedChineseEvidence()
	for _, r := range query {
		if characters[r] {
			return true
		}
	}
	return false
}
