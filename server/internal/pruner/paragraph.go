package pruner

import (
	"strings"
)

// SegmentizeParagraphs splits non-code text into heading and paragraph segments.
// Headings are lines starting with # (markdown). Paragraphs are separated by blank lines.
func SegmentizeParagraphs(text string) []Segment {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	ensureDetectorRegexes()

	lines := strings.Split(text, "\n")
	var segments []Segment
	var buf []string
	bufStart := -1

	flush := func(endLine int) {
		if len(buf) == 0 {
			return
		}
		content := strings.Join(buf, "\n")
		if strings.TrimSpace(content) == "" {
			buf = buf[:0]
			bufStart = -1
			return
		}
		segments = append(segments, NewSegment(bufStart, endLine, SegmentParagraph, "", content, codeTokenize(content)))
		buf = buf[:0]
		bufStart = -1
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Heading detection
		if len(trimmed) > 0 && trimmed[0] == '#' && markdownHeadingRe.MatchString(trimmed) {
			flush(i - 1)
			segments = append(segments, NewSegment(i, i, SegmentHeading, trimmed, line, codeTokenize(trimmed)))
			continue
		}

		// Blank line = paragraph boundary
		if trimmed == "" {
			flush(i - 1)
			continue
		}

		if bufStart < 0 {
			bufStart = i
		}
		buf = append(buf, line)
	}
	flush(len(lines) - 1)

	return segments
}

// SegmentizeLogs splits log output into groups separated by blank lines.
func SegmentizeLogs(text string) []Segment {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var segments []Segment
	var buf []string
	bufStart := -1

	flush := func(endLine int) {
		if len(buf) == 0 {
			return
		}
		content := strings.Join(buf, "\n")
		if strings.TrimSpace(content) == "" {
			buf = buf[:0]
			bufStart = -1
			return
		}
		segments = append(segments, NewSegment(bufStart, endLine, SegmentLogGroup, "", content, codeTokenize(content)))
		buf = buf[:0]
		bufStart = -1
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush(i - 1)
			continue
		}
		if bufStart < 0 {
			bufStart = i
		}
		buf = append(buf, line)
	}
	flush(len(lines) - 1)

	return segments
}

// SegmentizeData splits JSON/YAML content into top-level key segments.
// For JSON objects, each top-level key becomes a segment.
func SegmentizeData(text string) []Segment {
	if strings.TrimSpace(text) == "" {
		return nil
	}

	lines := strings.Split(text, "\n")
	var segments []Segment
	var buf []string
	bufStart := -1
	depth := 0

	flush := func(endLine int) {
		if len(buf) == 0 {
			return
		}
		content := strings.Join(buf, "\n")
		if strings.TrimSpace(content) == "" {
			buf = buf[:0]
			bufStart = -1
			return
		}
		name := extractDataKey(buf[0])
		segments = append(segments, NewSegment(bufStart, endLine, SegmentDataKey, name, content, codeTokenize(content)))
		buf = buf[:0]
		bufStart = -1
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip outer braces/brackets
		if trimmed == "{" || trimmed == "}" || trimmed == "[" || trimmed == "]" {
			continue
		}

		// Track nesting depth
		openCount := strings.Count(line, "{") + strings.Count(line, "[")
		closeCount := strings.Count(line, "}") + strings.Count(line, "]")

		if depth == 0 && bufStart >= 0 && openCount == 0 && closeCount == 0 {
			// Still at top level, same key continues
		}

		if depth == 0 && len(buf) > 0 && openCount == 0 {
			// New top-level key at depth 0
			if isTopLevelKey(trimmed) {
				flush(i - 1)
			}
		}

		if bufStart < 0 {
			bufStart = i
		}
		buf = append(buf, line)
		depth += openCount - closeCount
		if depth < 0 {
			depth = 0
		}

		// If we returned to depth 0, flush
		if depth == 0 && len(buf) > 1 {
			flush(i)
		}
	}
	flush(len(lines) - 1)

	return segments
}

// isTopLevelKey checks if a line looks like a JSON/YAML top-level key.
func isTopLevelKey(trimmed string) bool {
	return strings.Contains(trimmed, ":") || strings.Contains(trimmed, "\":")
}

// extractDataKey extracts the key name from a JSON/YAML line.
func extractDataKey(line string) string {
	trimmed := strings.TrimSpace(line)
	// JSON: "key": value
	if idx := strings.Index(trimmed, "\":"); idx > 0 {
		start := strings.Index(trimmed, "\"")
		if start >= 0 && start < idx {
			return trimmed[start+1 : idx]
		}
	}
	// YAML: key: value
	if idx := strings.Index(trimmed, ":"); idx > 0 {
		return strings.TrimSpace(trimmed[:idx])
	}
	return trimmed
}

// AutoSegmentize dispatches to the correct segmenter based on content type.
func AutoSegmentize(content string, ct ContentType) []Segment {
	switch ct {
	case ContentCode:
		return Segmentize(content)
	case ContentDoc:
		return SegmentizeParagraphs(content)
	case ContentLog:
		return SegmentizeLogs(content)
	case ContentData:
		return SegmentizeData(content)
	default:
		// Fallback: treat as paragraphs
		return SegmentizeParagraphs(content)
	}
}
