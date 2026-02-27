package pruner

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	mdImageRe         = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	mdLinkRe          = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	mdAutoLinkRe      = regexp.MustCompile(`<((?:https?|mailto):[^>]+)>`)
	mdUnorderedListRe = regexp.MustCompile(`^\s*[-*+]\s+`)
	mdOrderedListRe   = regexp.MustCompile(`^\s*\d+[.)]\s+`)
	mdTaskListRe      = regexp.MustCompile(`^\[(?: |x|X)\]\s+`)
	mdHeadingRe       = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	mdBlockQuoteRe    = regexp.MustCompile(`^\s*>\s*`)
	mdRuleRe          = regexp.MustCompile(`^\s*([-*_]\s*){3,}\s*$`)
	mdInlineCleaner   = strings.NewReplacer("**", "", "__", "", "*", "", "_", "", "~~", "", "`", "")
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

		// Skip italic instruction lines (*...*) — these are user-facing edit hints,
		// not useful content for the LLM (e.g. "*Blue maintains this file automatically.*")
		if isItalicHint(line) {
			continue
		}

		// Skip empty placeholder fields like "- **Name:**" (no value after colon)
		if isEmptyField(line) {
			continue
		}

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

	// If only headings remain (no actual content), return empty — the caller
	// can skip injecting this file entirely.
	if onlyHeadings(out) {
		return ""
	}

	return strings.Join(out, "\n")
}

// MarkdownToText converts markdown-like content into compact plain text for prompt context:
//   - Runs CompactMarkdown first
//   - Removes common markdown markers (headings/lists/quotes/fences/emphasis)
//   - Converts links/images to readable text
//   - Collapses blank lines to at most one
func MarkdownToText(s string) string {
	s = CompactMarkdown(s)
	if s == "" {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(lines))
	blanks := 0
	inFence := false

	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			blanks++
			if blanks <= 1 {
				out = append(out, "")
			}
			continue
		}
		blanks = 0

		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			// Keep code text but avoid markdown wrappers.
			line = strings.TrimSpace(line)
		}

		line = mdRuleRe.ReplaceAllString(line, "")
		line = mdHeadingRe.ReplaceAllString(line, "")
		line = mdBlockQuoteRe.ReplaceAllString(line, "")
		line = mdUnorderedListRe.ReplaceAllString(line, "")
		line = mdOrderedListRe.ReplaceAllString(line, "")
		line = mdTaskListRe.ReplaceAllString(line, "")
		line = mdImageRe.ReplaceAllString(line, "$1")
		line = mdLinkRe.ReplaceAllString(line, "$1")
		line = mdAutoLinkRe.ReplaceAllString(line, "$1")
		line = mdInlineCleaner.Replace(line)
		line = strings.ReplaceAll(line, "|", " ")
		line = strings.Join(strings.Fields(line), " ")

		if line == "" {
			blanks++
			if blanks <= 1 {
				out = append(out, "")
			}
			continue
		}
		blanks = 0
		out = append(out, line)
	}

	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return strings.Join(out, "\n")
}

// MarkdownToTextMinimal is an ultra-compact mode:
// converts markdown to plain text, then collapses all whitespace/newlines into single spaces.
func MarkdownToTextMinimal(s string) string {
	text := MarkdownToText(s)
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

// onlyHeadings returns true if every non-blank line is a markdown heading.
// Used to detect files where all content was stripped, leaving only skeleton headings.
func onlyHeadings(lines []string) bool {
	hasHeading := false
	for _, l := range lines {
		if l == "" {
			continue
		}
		if !isMarkdownHeading(l) {
			return false
		}
		hasHeading = true
	}
	return hasHeading
}

// isMarkdownHeading returns true if the line starts with one or more '#'.
func isMarkdownHeading(line string) bool {
	return len(line) > 0 && line[0] == '#'
}

// isItalicHint returns true if the line is a standalone italic hint like
// "*Blue maintains this file automatically. You can also edit it directly.*"
// These are user-facing instructions, not useful content for the LLM.
func isItalicHint(line string) bool {
	trimmed := strings.TrimSpace(line)
	return len(trimmed) > 2 && trimmed[0] == '*' && trimmed[len(trimmed)-1] == '*' && !strings.HasPrefix(trimmed, "**")
}

// isEmptyField returns true if the line is a list item with a bold label but no value,
// e.g. "- **Name:**" or "- **Timezone:**". These are unfilled template placeholders.
func isEmptyField(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- **") {
		return false
	}
	// Check if it ends with ":**" or ":** " (no value after the label)
	after := strings.TrimPrefix(trimmed, "- ")
	// Strip the bold label: "**Label:**" or "**Label:** "
	if idx := strings.Index(after, ":**"); idx >= 0 {
		rest := strings.TrimSpace(after[idx+3:])
		return rest == "" || rest == "*"
	}
	return false
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
