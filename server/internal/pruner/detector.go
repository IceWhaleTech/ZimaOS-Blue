package pruner

import (
	"regexp"
	"strings"
)

// codeKeywords are programming tokens that indicate source code.
var codeKeywords = []string{
	"func ", "def ", "class ", "import ", "package ",
	"return ", "if ", "for ", "while ", "switch ",
	"const ", "var ", "let ", "type ", "struct ",
	"interface ", "enum ", "public ", "private ",
	"async ", "await ", "try ", "catch ", "throw ",
	"#include", "#define", "#import",
}

// codeBrackets are bracket patterns common in code.
var codeBrackets = []string{"{", "}", "()", "[]", "=>", "->", "::"}

// logTimestampRe matches common log timestamp patterns.
var logTimestampRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}`)

// markdownHeadingRe matches markdown headings.
var markdownHeadingRe = regexp.MustCompile(`^#{1,6}\s+\S`)

// IsCodeContent returns true if content appears to be source code
// and exceeds the minimum line threshold.
func IsCodeContent(content string, minLines int) bool {
	return DetectContentType(content, minLines) == ContentCode
}

// DetectContentType classifies content into code, documentation, logs, data, or unknown.
// It samples only the first 100 lines for keyword/pattern detection to avoid
// scanning the entire content (which can be very large).
func DetectContentType(content string, minLines int) ContentType {
	lineCount := strings.Count(content, "\n") + 1
	if lineCount < minLines || len(content) == 0 {
		return ContentUnknown
	}

	// Quick trim check for empty content (avoid full TrimSpace)
	if content[0] <= ' ' || content[len(content)-1] <= ' ' {
		if strings.TrimSpace(content) == "" {
			return ContentUnknown
		}
	}

	// Check for JSON/YAML data using first/last non-whitespace bytes
	firstByte := firstNonSpace(content)
	lastByte := lastNonSpace(content)
	if (firstByte == '{' && lastByte == '}') || (firstByte == '[' && lastByte == ']') {
		return ContentData
	}

	// Extract first 100 lines as a single substring (no Split allocation)
	sample := sampleLines(content, 100)
	sampleLineCount := strings.Count(sample, "\n") + 1

	// Count indicators on the sample only
	logLines := 0
	headingLines := 0
	codeScore := 0

	// Scan sample line by line without allocating a []string
	remaining := sample
	for remaining != "" {
		var line string
		if idx := strings.IndexByte(remaining, '\n'); idx >= 0 {
			line = remaining[:idx]
			remaining = remaining[idx+1:]
		} else {
			line = remaining
			remaining = ""
		}
		if logTimestampRe.MatchString(line) {
			logLines++
		}
		trimmedLine := strings.TrimSpace(line)
		if markdownHeadingRe.MatchString(trimmedLine) {
			headingLines++
		}
	}

	// Log detection: >30% of sampled lines have timestamps
	if logLines > sampleLineCount*3/10 {
		return ContentLog
	}

	// Code detection: scan sample only (not full content)
	if strings.Contains(sample, "```") {
		return ContentCode
	}

	sampleLower := strings.ToLower(sample)
	for _, kw := range codeKeywords {
		if strings.Contains(sampleLower, kw) {
			codeScore++
		}
	}
	for _, br := range codeBrackets {
		if strings.Contains(sample, br) {
			codeScore++
		}
	}
	// Average line length heuristic (use byte length as proxy for rune count)
	if lineCount > 0 && len(content)/lineCount < 120 {
		codeScore++
	}

	if codeScore >= 3 {
		return ContentCode
	}

	// Markdown/doc detection: has headings
	if headingLines >= 2 {
		return ContentDoc
	}

	return ContentUnknown
}

// sampleLines returns the first n lines of content as a substring (zero-alloc for the common case).
func sampleLines(content string, n int) string {
	off := 0
	for i := 0; i < n; i++ {
		idx := strings.IndexByte(content[off:], '\n')
		if idx < 0 {
			return content // fewer than n lines
		}
		off += idx + 1
	}
	return content[:off]
}

// firstNonSpace returns the first non-whitespace byte, or 0 if none.
func firstNonSpace(s string) byte {
	for i := 0; i < len(s); i++ {
		if s[i] > ' ' {
			return s[i]
		}
	}
	return 0
}

// lastNonSpace returns the last non-whitespace byte, or 0 if none.
func lastNonSpace(s string) byte {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] > ' ' {
			return s[i]
		}
	}
	return 0
}
