package humanizer

import (
	"regexp"
	"strings"
	"unicode"
)

// ChunkMode determines the chunking strategy for outbound messages.
type ChunkMode int

const (
	// ChunkLength splits only when exceeding the limit.
	ChunkLength ChunkMode = iota
	// ChunkNewline prefers breaking on paragraph boundaries (blank lines).
	ChunkNewline
)

// DefaultChunkLimit is the default maximum chunk size in bytes.
const DefaultChunkLimit = 4000

// FenceSpan represents a fenced code block region in the text.
type FenceSpan struct {
	Start    int    // byte offset of the opening fence line
	End      int    // byte offset of the closing fence line end (or len(text) if unclosed)
	OpenLine string // full opening line (e.g. "```go")
	Marker   string // fence marker (e.g. "```")
	Indent   string // leading indent (0-3 spaces)
}

// parseFenceSpans scans text for Markdown fenced code blocks (``` or ~~~).
// Unclosed fences are included with End = len(text).
// Uses strings.IndexByte for line scanning instead of strings.Split.
func parseFenceSpans(text string) []FenceSpan {
	var spans []FenceSpan
	type openFence struct {
		start     int
		markerCh  byte
		markerLen int
		openLine  string
		marker    string
		indent    string
	}
	var open *openFence

	offset := 0
	for offset <= len(text) {
		// Find end of current line.
		nl := strings.IndexByte(text[offset:], '\n')
		var lineEnd int
		if nl == -1 {
			lineEnd = len(text)
		} else {
			lineEnd = offset + nl
		}
		line := text[offset:lineEnd]

		if indent, marker, ok := parseFenceLine(line); ok {
			markerCh := marker[0]
			markerLen := len(marker)
			if open == nil {
				open = &openFence{
					start:     offset,
					markerCh:  markerCh,
					markerLen: markerLen,
					openLine:  line,
					marker:    marker,
					indent:    indent,
				}
			} else if open.markerCh == markerCh && markerLen >= open.markerLen {
				spans = append(spans, FenceSpan{
					Start:    open.start,
					End:      lineEnd,
					OpenLine: open.openLine,
					Marker:   open.marker,
					Indent:   open.indent,
				})
				open = nil
			}
		}

		if nl == -1 {
			break
		}
		offset = lineEnd + 1
	}

	// Unclosed fence spans to end of text.
	if open != nil {
		spans = append(spans, FenceSpan{
			Start:    open.start,
			End:      len(text),
			OpenLine: open.openLine,
			Marker:   open.marker,
			Indent:   open.indent,
		})
	}

	return spans
}

// fenceLineRe matches a Markdown fence line: 0-3 spaces indent + 3+ backticks or tildes.
var fenceLineRe = regexp.MustCompile(`^( {0,3})(`+ "`{3,}" + `|~{3,})(.*)$`)

// parseFenceLine checks if a line is a fence opener/closer.
// Returns (indent, marker, true) or ("", "", false).
func parseFenceLine(line string) (indent, marker string, ok bool) {
	m := fenceLineRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

// findFenceSpanAt returns the FenceSpan containing index, or nil.
// An index at the exact start or end boundary is NOT considered inside.
func findFenceSpanAt(spans []FenceSpan, index int) *FenceSpan {
	for i := range spans {
		if index > spans[i].Start && index < spans[i].End {
			return &spans[i]
		}
	}
	return nil
}

// isSafeFenceBreak returns true if index is NOT inside any fenced code block.
func isSafeFenceBreak(spans []FenceSpan, index int) bool {
	return findFenceSpanAt(spans, index) == nil
}

// breakpoints holds the result of scanning for paren-aware break positions.
type breakpoints struct {
	lastNewline    int
	lastWhitespace int
}

// scanParenAwareBreakpoints scans window for the last newline and last whitespace
// positions that are outside parentheses and pass the isAllowed filter.
func scanParenAwareBreakpoints(window string, isAllowed func(int) bool) breakpoints {
	bp := breakpoints{lastNewline: -1, lastWhitespace: -1}
	depth := 0

	for i := 0; i < len(window); i++ {
		if isAllowed != nil && !isAllowed(i) {
			continue
		}
		ch := window[i]
		switch ch {
		case '(':
			depth++
			continue
		case ')':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 {
			continue
		}
		if ch == '\n' {
			bp.lastNewline = i
		} else if ch == ' ' || ch == '\t' || ch == '\r' {
			bp.lastWhitespace = i
		}
	}

	return bp
}

// ChunkText splits text into chunks of at most limit bytes.
// Prefers breaking at newlines, then whitespace, then hard-cuts.
// Parenthesis-aware: avoids breaking inside (...).
func ChunkText(text string, limit int) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > limit {
		window := remaining[:limit]

		bp := scanParenAwareBreakpoints(window, nil)

		breakIdx := bp.lastNewline
		if breakIdx <= 0 {
			breakIdx = bp.lastWhitespace
		}
		if breakIdx <= 0 {
			breakIdx = limit
		}

		chunk := strings.TrimRightFunc(remaining[:breakIdx], unicode.IsSpace)
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}

		// Skip separator if we broke on whitespace/newline.
		nextStart := breakIdx
		if breakIdx < len(remaining) && isWhitespace(remaining[breakIdx]) {
			nextStart = breakIdx + 1
		}
		remaining = strings.TrimLeftFunc(remaining[nextStart:], unicode.IsSpace)
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}

	return chunks
}

// ChunkMarkdownText splits text into chunks of at most limit bytes,
// handling fenced code blocks: when a split falls inside a fence,
// the current chunk gets a closing fence marker and the next chunk
// gets the opening fence line prepended.
func ChunkMarkdownText(text string, limit int) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}
	if len(text) <= limit {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > limit {
		spans := parseFenceSpans(remaining)
		window := remaining[:limit]

		softBreak := pickSafeBreakIndex(window, spans)
		breakIdx := softBreak
		if breakIdx <= 0 {
			breakIdx = limit
		}

		initialFence := findFenceSpanAt(spans, breakIdx)
		if isSafeFenceBreak(spans, breakIdx) {
			initialFence = nil
		}

		var fenceToSplit *FenceSpan
		if initialFence != nil {
			fenceToSplit = initialFence
			closeLine := initialFence.Indent + initialFence.Marker
			maxIdxIfNeedNewline := limit - (len(closeLine) + 1)

			if maxIdxIfNeedNewline <= 0 {
				fenceToSplit = nil
				breakIdx = limit
			} else {
				minProgressIdx := initialFence.Start + len(initialFence.OpenLine) + 2
				if minProgressIdx > len(remaining) {
					minProgressIdx = len(remaining)
				}
				maxIdxIfAlreadyNewline := limit - len(closeLine)

				pickedNewline := false
				lastNL := strings.LastIndexByte(remaining[:max(0, maxIdxIfAlreadyNewline)], '\n')
				for lastNL != -1 {
					candidateBreak := lastNL + 1
					if candidateBreak < minProgressIdx {
						break
					}
					candidateFence := findFenceSpanAt(spans, candidateBreak)
					if candidateFence != nil && candidateFence.Start == initialFence.Start {
						breakIdx = candidateBreak
						if breakIdx < 1 {
							breakIdx = 1
						}
						pickedNewline = true
						break
					}
					if lastNL == 0 {
						break
					}
					lastNL = strings.LastIndexByte(remaining[:lastNL], '\n')
				}

				if !pickedNewline {
					if minProgressIdx > maxIdxIfAlreadyNewline {
						fenceToSplit = nil
						breakIdx = limit
					} else {
						breakIdx = minProgressIdx
						if maxIdxIfNeedNewline > breakIdx {
							breakIdx = maxIdxIfNeedNewline
						}
					}
				}
			}

			// Re-check if we're still inside the same fence.
			fenceAtBreak := findFenceSpanAt(spans, breakIdx)
			if fenceAtBreak == nil || fenceAtBreak.Start != initialFence.Start {
				fenceToSplit = nil
			}
		}

		rawChunk := remaining[:breakIdx]
		if len(rawChunk) == 0 {
			break
		}

		brokeOnSep := breakIdx < len(remaining) && isWhitespace(remaining[breakIdx])
		nextStart := breakIdx
		if brokeOnSep {
			nextStart = breakIdx + 1
		}
		next := remaining[nextStart:]

		if fenceToSplit != nil {
			closeLine := fenceToSplit.Indent + fenceToSplit.Marker
			if strings.HasSuffix(rawChunk, "\n") {
				rawChunk = rawChunk + closeLine
			} else {
				rawChunk = rawChunk + "\n" + closeLine
			}
			next = fenceToSplit.OpenLine + "\n" + next
		} else {
			next = stripLeadingNewlines(next)
		}

		chunks = append(chunks, rawChunk)
		remaining = next
	}

	if len(remaining) > 0 {
		chunks = append(chunks, remaining)
	}

	return chunks
}

// paragraphBreakRe matches paragraph boundaries: blank lines (possibly with whitespace).
var paragraphBreakRe = regexp.MustCompile(`\n[\t ]*\n+`)

// ChunkByParagraph splits text at paragraph boundaries (blank lines).
// Fenced code blocks are respected — blank lines inside fences are not split points.
// Falls back to ChunkText for oversized paragraphs.
func ChunkByParagraph(text string, limit int) []string {
	if text == "" {
		return nil
	}
	if limit <= 0 {
		return []string{text}
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Fast path: no paragraph separators.
	if !paragraphBreakRe.MatchString(normalized) {
		if len(normalized) <= limit {
			return []string{normalized}
		}
		return ChunkText(normalized, limit)
	}

	spans := parseFenceSpans(normalized)

	var parts []string
	matches := paragraphBreakRe.FindAllStringIndex(normalized, -1)
	lastIndex := 0
	for _, m := range matches {
		idx := m[0]
		if !isSafeFenceBreak(spans, idx) {
			continue
		}
		parts = append(parts, normalized[lastIndex:idx])
		lastIndex = m[1]
	}
	parts = append(parts, normalized[lastIndex:])

	var chunks []string
	for _, part := range parts {
		paragraph := strings.TrimRight(part, " \t\n\r")
		if strings.TrimSpace(paragraph) == "" {
			continue
		}
		if len(paragraph) <= limit {
			chunks = append(chunks, paragraph)
		} else {
			chunks = append(chunks, ChunkText(paragraph, limit)...)
		}
	}

	return chunks
}

// ChunkTextWithMode dispatches to the appropriate chunker based on mode.
func ChunkTextWithMode(text string, limit int, mode ChunkMode) []string {
	if mode == ChunkNewline {
		return ChunkByParagraph(text, limit)
	}
	return ChunkText(text, limit)
}

// ChunkMarkdownTextWithMode dispatches to the appropriate markdown-aware chunker.
// In newline mode, first splits by paragraph, then uses markdown-aware splitting
// for oversized paragraphs.
func ChunkMarkdownTextWithMode(text string, limit int, mode ChunkMode) []string {
	if mode == ChunkNewline {
		paragraphChunks := ChunkByParagraph(text, limit)
		var out []string
		for _, chunk := range paragraphChunks {
			if len(chunk) <= limit {
				out = append(out, chunk)
			} else {
				nested := ChunkMarkdownText(chunk, limit)
				if len(nested) == 0 && chunk != "" {
					out = append(out, chunk)
				} else {
					out = append(out, nested...)
				}
			}
		}
		return out
	}
	return ChunkMarkdownText(text, limit)
}

// pickSafeBreakIndex finds the best break point in window that is outside fenced code blocks.
func pickSafeBreakIndex(window string, spans []FenceSpan) int {
	bp := scanParenAwareBreakpoints(window, func(index int) bool {
		return isSafeFenceBreak(spans, index)
	})
	if bp.lastNewline > 0 {
		return bp.lastNewline
	}
	if bp.lastWhitespace > 0 {
		return bp.lastWhitespace
	}
	return -1
}

func stripLeadingNewlines(s string) string {
	i := 0
	for i < len(s) && s[i] == '\n' {
		i++
	}
	if i > 0 {
		return s[i:]
	}
	return s
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ChunkIR splits an IR into chunks of at most limit bytes,
// correctly slicing style and link spans for each chunk.
func ChunkIR(ir IR, limit int) []IR {
	if ir.Text == "" {
		return nil
	}
	if limit <= 0 || len(ir.Text) <= limit {
		return []IR{ir}
	}

	textChunks := ChunkText(ir.Text, limit)
	results := make([]IR, 0, len(textChunks))
	cursor := 0

	for idx, chunk := range textChunks {
		if chunk == "" {
			continue
		}
		// Skip whitespace between chunks
		if idx > 0 {
			for cursor < len(ir.Text) && isWhitespace(ir.Text[cursor]) {
				cursor++
			}
		}
		start := cursor
		end := start + len(chunk)
		if end > len(ir.Text) {
			end = len(ir.Text)
		}
		results = append(results, IR{
			Text:   chunk,
			Styles: SliceStyleSpans(ir.Styles, start, end),
			Links:  SliceLinkSpans(ir.Links, start, end),
		})
		cursor = end
	}

	return results
}
