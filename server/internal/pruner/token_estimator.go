package pruner

import (
	"unicode"
	"unicode/utf8"
)

// EstimateTokens provides a rough token count estimate.
// Uses ~4 chars/token for ASCII, ~1.5 chars/token for CJK.
// Optimized: fast path for pure ASCII, sampling for mixed content.
func EstimateTokens(text string) int {
	n := len(text)
	if n == 0 {
		return 0
	}

	// Fast path: if byte length == rune count, it's pure ASCII
	if n == utf8.RuneCountInString(text) {
		tokens := n / 4
		if tokens == 0 {
			tokens = 1
		}
		return tokens
	}

	// Mixed content: sample first 256 runes to estimate CJK ratio
	sampleSize := 256
	asciiChars := 0
	cjkChars := 0
	sampled := 0

	for _, r := range text {
		if sampled >= sampleSize {
			break
		}
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) ||
			unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hiragana, r) {
			cjkChars++
		} else {
			asciiChars++
		}
		sampled++
	}

	totalRunes := utf8.RuneCountInString(text)
	if sampled == 0 {
		return n / 4
	}

	// Extrapolate from sample
	cjkRatio := float64(cjkChars) / float64(sampled)
	estCJK := int(cjkRatio * float64(totalRunes))
	estASCII := totalRunes - estCJK

	tokens := estASCII/4 + int(float64(estCJK)/1.5)
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}
