package pruner

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// CompactMarkdown strips noise from markdown text to reduce token usage:
//   - Removes HTML comments (<!-- ... -->)
//   - Collapses 3+ consecutive blank lines into 2 (preserves paragraph breaks)
//   - Strips trailing whitespace from each line
//   - Removes empty placeholder sections (heading followed only by blank lines before next heading)
func CompactMarkdown(s string) string {
	if s == "" {
		return ""
	}

	// Strip HTML comments (single-line and multi-line)
	s = stripHTMLComments(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blanks := 0

	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")

		// Skip empty placeholder sections: a heading followed only by blank lines
		// before the next heading at the same or higher level (or EOF).
		if isMarkdownHeading(line) && isEmptySection(line, lines, i+1) {
			// Skip this heading and its trailing blank lines
			for i+1 < len(lines) && strings.TrimSpace(lines[i+1]) == "" {
				i++
			}
			continue
		}

		if line == "" {
			blanks++
			if blanks <= 2 {
				out = append(out, "")
			}
			continue
		}
		blanks = 0
		out = append(out, line)
	}

	// Trim trailing blank lines
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}

	return strings.Join(out, "\n")
}

// isMarkdownHeading returns true if the line starts with one or more '#'.
func isMarkdownHeading(line string) bool {
	return len(line) > 0 && line[0] == '#'
}

// isEmptySection returns true if lines[start:] contains only blank lines
// until the next heading at the same or higher level (or EOF).
// currentHeading is the heading line being checked.
func isEmptySection(currentHeading string, lines []string, start int) bool {
	level := headingLevel(currentHeading)
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Hit non-blank line: if it's a heading at same/higher level, section was empty.
		// If it's a sub-heading or content, section is NOT empty.
		if isMarkdownHeading(trimmed) {
			return headingLevel(trimmed) <= level
		}
		return false // non-blank, non-heading content
	}
	// Reached EOF — section was empty
	return true
}

// headingLevel returns the number of '#' characters at the start of a heading line.
func headingLevel(line string) int {
	n := 0
	for _, c := range line {
		if c == '#' {
			n++
		} else {
			break
		}
	}
	return n
}

// stripHTMLComments removes <!-- ... --> comments (including multi-line).
func stripHTMLComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for {
		idx := strings.Index(s, "<!--")
		if idx < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:idx])
		s = s[idx+4:]
		end := strings.Index(s, "-->")
		if end < 0 {
			// Unclosed comment — drop the rest
			break
		}
		s = s[end+3:]
	}
	return b.String()
}

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
