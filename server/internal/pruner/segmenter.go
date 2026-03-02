package pruner

import (
	"regexp"
	"strings"
)

// segmentPattern defines a regex pattern for detecting code boundaries.
type segmentPattern struct {
	re   *regexp.Regexp
	kind SegmentKind
}

// Language-agnostic segment boundary patterns.
// These detect function/method/class definitions across Go, Python, JS/TS, Rust, Java.
var segmentPatterns = []segmentPattern{
	// Go: func, type struct/interface
	{re: regexp.MustCompile(`^\s*func\s+`), kind: SegmentFunction},
	{re: regexp.MustCompile(`^\s*type\s+\w+\s+(struct|interface)\s*\{`), kind: SegmentClass},
	// Python: def, class
	{re: regexp.MustCompile(`^\s*def\s+\w+\s*\(`), kind: SegmentFunction},
	{re: regexp.MustCompile(`^\s*class\s+\w+`), kind: SegmentClass},
	// JS/TS: function, class, arrow (const x = (...) =>)
	{re: regexp.MustCompile(`^\s*function\s+\w+`), kind: SegmentFunction},
	{re: regexp.MustCompile(`^\s*class\s+\w+`), kind: SegmentClass},
	// Rust: fn, impl, struct, enum
	{re: regexp.MustCompile(`^\s*(pub\s+)?fn\s+\w+`), kind: SegmentFunction},
	{re: regexp.MustCompile(`^\s*impl\s+`), kind: SegmentClass},
	// Java: public/private/protected methods and classes
	{re: regexp.MustCompile(`^\s*(public|private|protected)\s+class\s+`), kind: SegmentClass},
}

// Segmentize splits code into semantic segments (functions, classes, blocks).
// It detects boundaries using regex patterns for multiple languages,
// then groups remaining lines into SegmentLines blocks.
func Segmentize(code string) []Segment {
	if code == "" {
		return nil
	}

	lines := strings.Split(code, "\n")
	if len(lines) == 0 {
		return nil
	}

	// Find all boundary lines
	type boundary struct {
		line int
		kind SegmentKind
		name string
	}
	var boundaries []boundary

	for i, line := range lines {
		for _, pat := range segmentPatterns {
			if pat.re.MatchString(line) {
				name := extractName(line)
				boundaries = append(boundaries, boundary{line: i, kind: pat.kind, name: name})
				break // first match wins
			}
		}
	}

	// No boundaries found — return entire code as one SegmentLines
	if len(boundaries) == 0 {
		content := strings.Join(lines, "\n")
		return []Segment{NewSegment(0, len(lines)-1, SegmentLines, "", content, codeTokenize(content))}
	}

	var segments []Segment

	// Lines before first boundary → SegmentLines
	if boundaries[0].line > 0 {
		content := strings.Join(lines[:boundaries[0].line], "\n")
		if strings.TrimSpace(content) != "" {
			segments = append(segments, NewSegment(0, boundaries[0].line-1, SegmentLines, "", content, codeTokenize(content)))
		}
	}

	// Process each boundary
	for i, b := range boundaries {
		endLine := len(lines) - 1
		if i+1 < len(boundaries) {
			endLine = boundaries[i+1].line - 1
		}
		// Find closing brace for brace-delimited languages
		endLine = findBlockEnd(lines, b.line, endLine)

		content := strings.Join(lines[b.line:endLine+1], "\n")
		segments = append(segments, NewSegment(b.line, endLine, b.kind, b.name, content, codeTokenize(content)))
	}

	return segments
}

// findBlockEnd finds the end of a brace-delimited block starting at startLine.
// For indentation-based languages (Python), it finds the last indented line.
func findBlockEnd(lines []string, startLine, maxEnd int) int {
	if startLine >= len(lines) {
		return startLine
	}

	startTrimmed := strings.TrimSpace(lines[startLine])

	// Check if this is a brace-delimited block
	if strings.Contains(startTrimmed, "{") {
		depth := 0
		for i := startLine; i <= maxEnd && i < len(lines); i++ {
			depth += strings.Count(lines[i], "{") - strings.Count(lines[i], "}")
			if depth <= 0 {
				return i
			}
		}
		return maxEnd
	}

	// Indentation-based (Python): find last line with deeper indentation
	baseIndent := leadingSpaces(lines[startLine])
	lastIndented := startLine
	for i := startLine + 1; i <= maxEnd && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue // skip blank lines
		}
		if leadingSpaces(lines[i]) > baseIndent {
			lastIndented = i
		} else {
			break
		}
	}
	return lastIndented
}

// leadingSpaces counts the number of leading whitespace characters.
func leadingSpaces(s string) int {
	count := 0
	for _, r := range s {
		if r == ' ' {
			count++
		} else if r == '\t' {
			count += 4
		} else {
			break
		}
	}
	return count
}

// extractName extracts a human-readable name from a code boundary line.
func extractName(line string) string {
	trimmed := strings.TrimSpace(line)
	// Truncate at opening paren or brace
	for _, delim := range []string{"(", "{"} {
		if idx := strings.Index(trimmed, delim); idx > 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
	}
	// Limit length
	if len(trimmed) > 60 {
		trimmed = trimmed[:60]
	}
	return trimmed
}
