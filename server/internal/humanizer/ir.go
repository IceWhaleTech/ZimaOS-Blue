package humanizer

import (
	"sort"
	"strings"
)

// Style represents a Markdown formatting style.
type Style int

const (
	StyleBold Style = iota
	StyleItalic
	StyleStrikethrough
	StyleCode
	StyleCodeBlock
	StyleSpoiler
)

// StyleSpan marks a region of text with a specific style.
type StyleSpan struct {
	Start int
	End   int
	Style Style
}

// LinkSpan marks a region of text that is a hyperlink.
type LinkSpan struct {
	Start int
	End   int
	Href  string
}

// IR is the intermediate representation of parsed Markdown.
// Text contains the plain text with all Markdown syntax removed.
// Styles and Links record where formatting was applied.
type IR struct {
	Text   string
	Styles []StyleSpan
	Links  []LinkSpan
}

// ParseOptions controls Markdown parsing behavior.
type ParseOptions struct {
	HeadingStyle     string // "none" or "bold" (default: "none")
	BlockquotePrefix string // prefix for blockquote lines (e.g. "> ")
	TableMode        string // "off", "bullets", or "code" (default: "off")
}

// parser holds state during a single Parse call.
type parser struct {
	opts ParseOptions
	out  strings.Builder

	styles []StyleSpan
	links  []LinkSpan

	// Fence tracking
	inFence    bool
	fenceChar  byte
	fenceLen   int
	fenceStart int // byte offset in out where fence content starts

	// List tracking
	listStack []listState

	// Table tracking
	inTable     bool
	tableRows   [][]string // accumulated rows (first row = headers)
	tableDivSeen bool
}

type listState struct {
	ordered bool
	index   int
}

// Parse converts Markdown text into an IR.
// Uses single-pass line scanning with strings.IndexByte for performance.
func Parse(markdown string, opts ParseOptions) IR {
	p := &parser{
		opts:   opts,
		styles: make([]StyleSpan, 0, 16),
		links:  make([]LinkSpan, 0, 4),
	}
	p.out.Grow(len(markdown))

	offset := 0
	for offset < len(markdown) {
		nl := strings.IndexByte(markdown[offset:], '\n')
		var line string
		if nl < 0 {
			line = markdown[offset:]
			offset = len(markdown)
		} else {
			line = markdown[offset : offset+nl]
			offset += nl + 1
		}
		p.processLine(line)
	}

	// Close any open fence
	if p.inFence {
		p.closeFence()
	}
	// Flush table
	if p.inTable {
		p.flushTable()
	}

	text := strings.TrimRight(p.out.String(), "\n ")

	// Clamp spans to text length (TrimRight may have shortened it)
	textLen := len(text)
	styles := p.styles[:0]
	for _, s := range p.styles {
		if s.Start >= textLen {
			continue
		}
		if s.End > textLen {
			s.End = textLen
		}
		if s.End > s.Start {
			styles = append(styles, s)
		}
	}
	links := p.links[:0]
	for _, l := range p.links {
		if l.Start >= textLen {
			continue
		}
		if l.End > textLen {
			l.End = textLen
		}
		if l.End > l.Start {
			links = append(links, l)
		}
	}

	return IR{
		Text:   text,
		Styles: styles,
		Links:  links,
	}
}

func (p *parser) processLine(line string) {
	// Inside a fenced code block — check for closing fence
	if p.inFence {
		trimmed := strings.TrimSpace(line)
		if indent, marker, ok := parseFenceLine(trimmed); ok && marker[0] == p.fenceChar && len(marker) >= p.fenceLen {
			_ = indent
			p.closeFence()
			return
		}
		// Append raw line inside fence
		p.out.WriteString(line)
		p.out.WriteByte('\n')
		return
	}

	trimmed := strings.TrimSpace(line)

	// Blank line
	if trimmed == "" {
		// End table if active
		if p.inTable {
			p.flushTable()
		}
		p.appendParagraphSep()
		return
	}

	// Fence opening
	if _, marker, ok := parseFenceLine(trimmed); ok {
		if p.inTable {
			p.flushTable()
		}
		p.inFence = true
		p.fenceChar = marker[0]
		p.fenceLen = len(marker)
		p.fenceStart = p.out.Len()
		return
	}

	// Horizontal rule: ---, ***, ___
	if isHorizontalRule(trimmed) {
		if p.inTable {
			p.flushTable()
		}
		// HR acts as a paragraph separator — don't emit extra newline
		p.appendParagraphSep()
		return
	}

	// Table row: starts and ends with |
	if trimmed[0] == '|' && trimmed[len(trimmed)-1] == '|' {
		p.processTableRow(trimmed)
		return
	}

	// If we were in a table but this line isn't a table row, flush
	if p.inTable {
		p.flushTable()
	}

	// Heading: # ... ######
	if trimmed[0] == '#' {
		if level, content := parseHeading(trimmed); level > 0 {
			start := p.out.Len()
			p.processInline(content)
			end := p.out.Len()
			if p.opts.HeadingStyle == "bold" && end > start {
				p.styles = append(p.styles, StyleSpan{Start: start, End: end, Style: StyleBold})
			}
			p.appendParagraphSep()
			return
		}
	}

	// Blockquote: > text
	if trimmed[0] == '>' {
		content := trimmed[1:]
		if len(content) > 0 && content[0] == ' ' {
			content = content[1:]
		}
		if p.opts.BlockquotePrefix != "" {
			p.out.WriteString(p.opts.BlockquotePrefix)
		}
		p.processInline(content)
		p.out.WriteByte('\n')
		return
	}

	// Unordered list: - item, * item, + item
	if (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && len(trimmed) > 1 && trimmed[1] == ' ' {
		p.out.WriteString("• ")
		p.processInline(trimmed[2:])
		p.out.WriteByte('\n')
		return
	}

	// Ordered list: 1. item
	if isOrderedListItem(trimmed) {
		dotIdx := strings.IndexByte(trimmed, '.')
		if dotIdx > 0 && dotIdx < len(trimmed)-1 && trimmed[dotIdx+1] == ' ' {
			p.out.WriteString(trimmed[:dotIdx+1])
			p.out.WriteByte(' ')
			p.processInline(trimmed[dotIdx+2:])
			p.out.WriteByte('\n')
			return
		}
	}

	// Regular paragraph line
	p.processInline(trimmed)
	p.out.WriteByte('\n')
}

// processInline parses inline Markdown elements within a line.
func (p *parser) processInline(text string) {
	i := 0
	for i < len(text) {
		ch := text[i]

		// Bold: **text** or __text__
		if (ch == '*' || ch == '_') && i+1 < len(text) && text[i+1] == ch {
			if end := findClosingDouble(text, i+2, ch); end >= 0 {
				start := p.out.Len()
				p.processInline(text[i+2 : end])
				p.styles = append(p.styles, StyleSpan{Start: start, End: p.out.Len(), Style: StyleBold})
				i = end + 2
				continue
			}
		}

		// Strikethrough: ~~text~~
		if ch == '~' && i+1 < len(text) && text[i+1] == '~' {
			if end := findClosingDouble(text, i+2, '~'); end >= 0 {
				start := p.out.Len()
				p.processInline(text[i+2 : end])
				p.styles = append(p.styles, StyleSpan{Start: start, End: p.out.Len(), Style: StyleStrikethrough})
				i = end + 2
				continue
			}
		}

		// Spoiler: ||text||
		if ch == '|' && i+1 < len(text) && text[i+1] == '|' {
			if end := findClosingDouble(text, i+2, '|'); end >= 0 {
				start := p.out.Len()
				p.processInline(text[i+2 : end])
				p.styles = append(p.styles, StyleSpan{Start: start, End: p.out.Len(), Style: StyleSpoiler})
				i = end + 2
				continue
			}
		}

		// Italic: *text* or _text_ (single)
		if (ch == '*' || ch == '_') && i+1 < len(text) && text[i+1] != ch {
			if end := findClosingSingle(text, i+1, ch); end >= 0 {
				start := p.out.Len()
				p.processInline(text[i+1 : end])
				p.styles = append(p.styles, StyleSpan{Start: start, End: p.out.Len(), Style: StyleItalic})
				i = end + 1
				continue
			}
		}

		// Inline code: `text`
		if ch == '`' {
			// Count consecutive backticks
			tickLen := 1
			for i+tickLen < len(text) && text[i+tickLen] == '`' {
				tickLen++
			}
			// Find matching closing backticks
			closeIdx := strings.Index(text[i+tickLen:], text[i:i+tickLen])
			if closeIdx >= 0 {
				codeContent := text[i+tickLen : i+tickLen+closeIdx]
				start := p.out.Len()
				p.out.WriteString(codeContent)
				p.styles = append(p.styles, StyleSpan{Start: start, End: p.out.Len(), Style: StyleCode})
				i = i + tickLen + closeIdx + tickLen
				continue
			}
		}

		// Image: ![alt](url) — emit "(image: alt)" for IM, stripped by voice later
		if ch == '!' && i+1 < len(text) && text[i+1] == '[' {
			if altEnd := strings.IndexByte(text[i+2:], ']'); altEnd >= 0 {
				altEnd += i + 2
				if altEnd+1 < len(text) && text[altEnd+1] == '(' {
					if urlEnd := strings.IndexByte(text[altEnd+2:], ')'); urlEnd >= 0 {
						alt := text[i+2 : altEnd]
						if alt != "" {
							p.out.WriteString("(image: ")
							p.out.WriteString(alt)
							p.out.WriteByte(')')
						}
						i = altEnd + 2 + urlEnd + 1
						continue
					}
				}
			}
		}

		// Link: [text](url)
		if ch == '[' {
			if labelEnd := findMatchingBracket(text, i); labelEnd >= 0 {
				if labelEnd+1 < len(text) && text[labelEnd+1] == '(' {
					if urlEnd := strings.IndexByte(text[labelEnd+2:], ')'); urlEnd >= 0 {
						label := text[i+1 : labelEnd]
						href := text[labelEnd+2 : labelEnd+2+urlEnd]
						start := p.out.Len()
						p.processInline(label)
						end := p.out.Len()
						if end > start && href != "" {
							p.links = append(p.links, LinkSpan{Start: start, End: end, Href: href})
						}
						i = labelEnd + 2 + urlEnd + 1
						continue
					}
				}
			}
		}

		// Regular character
		p.out.WriteByte(ch)
		i++
	}
}

func (p *parser) closeFence() {
	start := p.fenceStart
	end := p.out.Len()
	if end > start {
		p.styles = append(p.styles, StyleSpan{Start: start, End: end, Style: StyleCodeBlock})
	}
	p.inFence = false
	if end > start {
		// Ensure trailing newline after code block
		text := p.out.String()
		if text[len(text)-1] != '\n' {
			p.out.WriteByte('\n')
		}
	}
}

func (p *parser) appendParagraphSep() {
	if p.out.Len() == 0 {
		return
	}
	text := p.out.String()
	// Avoid double blank lines
	if len(text) >= 2 && text[len(text)-1] == '\n' && text[len(text)-2] == '\n' {
		return
	}
	if text[len(text)-1] != '\n' {
		p.out.WriteByte('\n')
	}
	p.out.WriteByte('\n')
}

// Table processing

func (p *parser) processTableRow(line string) {
	cells := parseTableCells(line)

	// Check if this is a divider row (|---|---|)
	if isTableDivider(cells) {
		p.tableDivSeen = true
		if !p.inTable {
			p.inTable = true
		}
		return
	}

	if !p.inTable {
		p.inTable = true
	}
	p.tableRows = append(p.tableRows, cells)
}

func (p *parser) flushTable() {
	if len(p.tableRows) == 0 {
		p.inTable = false
		p.tableDivSeen = false
		return
	}

	mode := p.opts.TableMode
	if mode == "" {
		mode = "off"
	}

	switch mode {
	case "bullets":
		p.renderTableBullets()
	case "code":
		p.renderTableCode()
	default:
		// "off" — render as plain text lines
		p.renderTablePlain()
	}

	p.tableRows = nil
	p.inTable = false
	p.tableDivSeen = false
}

func (p *parser) renderTableBullets() {
	if len(p.tableRows) == 0 {
		return
	}
	headers := p.tableRows[0]
	dataRows := p.tableRows[1:]

	for _, row := range dataRows {
		if len(row) > 0 {
			// First cell as label (bold)
			start := p.out.Len()
			p.out.WriteString(row[0])
			end := p.out.Len()
			if end > start {
				p.styles = append(p.styles, StyleSpan{Start: start, End: end, Style: StyleBold})
			}
			p.out.WriteByte('\n')
		}
		for i := 1; i < len(row); i++ {
			p.out.WriteString("• ")
			if i < len(headers) && headers[i] != "" {
				p.out.WriteString(headers[i])
				p.out.WriteString(": ")
			}
			p.out.WriteString(row[i])
			p.out.WriteByte('\n')
		}
		p.out.WriteByte('\n')
	}
}

func (p *parser) renderTableCode() {
	if len(p.tableRows) == 0 {
		return
	}

	// Compute column widths
	colCount := 0
	for _, row := range p.tableRows {
		if len(row) > colCount {
			colCount = len(row)
		}
	}
	widths := make([]int, colCount)
	for _, row := range p.tableRows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	start := p.out.Len()

	// Header row
	if len(p.tableRows) > 0 {
		p.writeTableRow(p.tableRows[0], widths)
		// Divider
		p.out.WriteByte('|')
		for _, w := range widths {
			p.out.WriteByte(' ')
			for j := 0; j < maxInt(3, w); j++ {
				p.out.WriteByte('-')
			}
			p.out.WriteString(" |")
		}
		p.out.WriteByte('\n')
	}

	for _, row := range p.tableRows[1:] {
		p.writeTableRow(row, widths)
	}

	end := p.out.Len()
	if end > start {
		p.styles = append(p.styles, StyleSpan{Start: start, End: end, Style: StyleCodeBlock})
	}
	p.out.WriteByte('\n')
}

func (p *parser) writeTableRow(row []string, widths []int) {
	p.out.WriteByte('|')
	for i, w := range widths {
		p.out.WriteByte(' ')
		cell := ""
		if i < len(row) {
			cell = row[i]
		}
		p.out.WriteString(cell)
		pad := w - len(cell)
		for j := 0; j < pad; j++ {
			p.out.WriteByte(' ')
		}
		p.out.WriteString(" |")
	}
	p.out.WriteByte('\n')
}

func (p *parser) renderTablePlain() {
	for _, row := range p.tableRows {
		p.out.WriteString(strings.Join(row, " | "))
		p.out.WriteByte('\n')
	}
}

// Helpers

func parseHeading(line string) (level int, content string) {
	i := 0
	for i < len(line) && line[i] == '#' {
		i++
	}
	if i == 0 || i > 6 {
		return 0, ""
	}
	if i >= len(line) || line[i] != ' ' {
		return 0, ""
	}
	return i, strings.TrimSpace(line[i+1:])
}

func isHorizontalRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	ch := line[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for _, r := range line {
		if byte(r) == ch {
			count++
		} else if r != ' ' {
			return false
		}
	}
	return count >= 3
}

func isOrderedListItem(line string) bool {
	i := 0
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	return i > 0 && i < len(line)-1 && line[i] == '.' && line[i+1] == ' '
}

func findClosingDouble(text string, start int, ch byte) int {
	for i := start; i < len(text)-1; i++ {
		if text[i] == ch && text[i+1] == ch {
			return i
		}
	}
	return -1
}

func findClosingSingle(text string, start int, ch byte) int {
	for i := start; i < len(text); i++ {
		if text[i] == ch {
			// Don't match if preceded by backslash
			if i > 0 && text[i-1] == '\\' {
				continue
			}
			return i
		}
	}
	return -1
}

func findMatchingBracket(text string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(text); i++ {
		if text[i] == '[' {
			depth++
		} else if text[i] == ']' {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func parseTableCells(line string) []string {
	// Strip leading/trailing |
	inner := line
	if len(inner) > 0 && inner[0] == '|' {
		inner = inner[1:]
	}
	if len(inner) > 0 && inner[len(inner)-1] == '|' {
		inner = inner[:len(inner)-1]
	}
	parts := strings.Split(inner, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

func isTableDivider(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		c = strings.Trim(c, ":-")
		if c != "" {
			return false
		}
	}
	return true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// SliceStyleSpans returns style spans that overlap [start, end), shifted to 0-based.
func SliceStyleSpans(spans []StyleSpan, start, end int) []StyleSpan {
	if len(spans) == 0 {
		return nil
	}
	var result []StyleSpan
	for _, s := range spans {
		ss := maxInt(s.Start, start)
		se := minInt(s.End, end)
		if se > ss {
			result = append(result, StyleSpan{Start: ss - start, End: se - start, Style: s.Style})
		}
	}
	return mergeStyleSpans(result)
}

// SliceLinkSpans returns link spans that overlap [start, end), shifted to 0-based.
func SliceLinkSpans(spans []LinkSpan, start, end int) []LinkSpan {
	if len(spans) == 0 {
		return nil
	}
	var result []LinkSpan
	for _, s := range spans {
		ss := maxInt(s.Start, start)
		se := minInt(s.End, end)
		if se > ss {
			result = append(result, LinkSpan{Start: ss - start, End: se - start, Href: s.Href})
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mergeStyleSpans(spans []StyleSpan) []StyleSpan {
	if len(spans) <= 1 {
		return spans
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].Start != spans[j].Start {
			return spans[i].Start < spans[j].Start
		}
		if spans[i].End != spans[j].End {
			return spans[i].End < spans[j].End
		}
		return spans[i].Style < spans[j].Style
	})
	merged := spans[:1]
	for _, s := range spans[1:] {
		prev := &merged[len(merged)-1]
		if prev.Style == s.Style && s.Start <= prev.End {
			if s.End > prev.End {
				prev.End = s.End
			}
			continue
		}
		merged = append(merged, s)
	}
	return merged
}
