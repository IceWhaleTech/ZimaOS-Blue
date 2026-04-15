package officemd

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

type BlockKind string

const (
	BlockParagraph BlockKind = "paragraph"
	BlockQuote     BlockKind = "quote"
	BlockCode      BlockKind = "code"
	BlockSeparator BlockKind = "separator"
	BlockImage     BlockKind = "image"
)

type Block struct {
	Kind   BlockKind
	Text   string
	Source string
}

type TableSpec struct {
	Headers          []string
	Rows             [][]string
	ColumnWidths     []float64
	ColumnAlignments []string
}

type Section struct {
	Heading         string
	ParagraphBlocks []Block
	Paragraphs      []string
	Bullets         []string
	Table           *TableSpec
}

type DocSpec struct {
	Title           string
	Subtitle        string
	Summary         string
	Sections        []Section
	ParagraphBlocks []Block
	Paragraphs      []string
	Notes           []string
}

var (
	orderedListPattern = regexp.MustCompile(`^\d+[\.\)]\s+`)
)

func ParseDocument(content string) DocSpec {
	spec := DocSpec{}
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	blocks := strings.Split(normalized, "\n\n")
	currentSection := -1

	appendTopParagraph := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		lower := strings.ToLower(text)
		if strings.HasPrefix(lower, "summary:") || strings.HasPrefix(lower, "摘要：") || strings.HasPrefix(lower, "摘要:") {
			text = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(text, "Summary:"), "摘要:"), "摘要："))
			if text != "" {
				spec.Summary = text
			}
			return
		}
		if spec.Title != "" && spec.Subtitle == "" && spec.Summary == "" && len(spec.ParagraphBlocks) == 0 && len(spec.Paragraphs) == 0 && runeCount(text) <= 96 {
			spec.Subtitle = text
			return
		}
		if spec.Summary == "" {
			spec.Summary = text
			return
		}
		spec.ParagraphBlocks = append(spec.ParagraphBlocks, Block{Kind: BlockParagraph, Text: text})
	}

	appendBlock := func(block Block) {
		block.Text = strings.TrimSpace(block.Text)
		if block.Kind == BlockImage {
			block.Source = strings.TrimSpace(block.Source)
			if block.Source == "" {
				return
			}
		} else if block.Text == "" {
			return
		}
		if currentSection >= 0 && currentSection < len(spec.Sections) {
			spec.Sections[currentSection].ParagraphBlocks = append(spec.Sections[currentSection].ParagraphBlocks, block)
			return
		}
		if block.Kind == BlockParagraph {
			appendTopParagraph(block.Text)
			return
		}
		spec.ParagraphBlocks = append(spec.ParagraphBlocks, block)
	}

	startSection := func(heading string) {
		spec.Sections = append(spec.Sections, Section{Heading: strings.TrimSpace(heading)})
		currentSection = len(spec.Sections) - 1
	}

	for _, block := range blocks {
		block = strings.TrimSpace(strings.ReplaceAll(block, "\r\n", "\n"))
		if block == "" {
			continue
		}

		firstLine, rest := splitFirstMarkdownLine(block)
		if heading, level, ok := markdownHeading(firstLine); ok {
			if level == 1 && spec.Title == "" && currentSection < 0 && spec.Summary == "" && len(spec.ParagraphBlocks) == 0 && len(spec.Paragraphs) == 0 {
				spec.Title = heading
			} else {
				startSection(heading)
			}
			block = strings.TrimSpace(rest)
			if block == "" {
				continue
			}
		}

		if alt, source, ok := markdownImageBlock(block); ok {
			appendBlock(Block{Kind: BlockImage, Text: alt, Source: source})
			continue
		}
		if separator, ok := markdownThematicBreak(block); ok {
			appendBlock(Block{Kind: BlockSeparator, Text: separator})
			continue
		}
		if code, ok := markdownFencedCodeBlock(block); ok {
			appendBlock(Block{Kind: BlockCode, Text: code})
			continue
		}

		lines := trimmedLines(block)
		if len(lines) == 0 {
			continue
		}
		if quote, ok := markdownBlockquote(lines); ok {
			appendBlock(Block{Kind: BlockQuote, Text: quote})
			continue
		}
		if allBulletLines(lines) {
			bullets := markdownBullets(lines)
			if len(bullets) == 0 {
				continue
			}
			if currentSection >= 0 && currentSection < len(spec.Sections) {
				spec.Sections[currentSection].Bullets = append(spec.Sections[currentSection].Bullets, bullets...)
			} else {
				startSection("Highlights")
				spec.Sections[currentSection].Bullets = append(spec.Sections[currentSection].Bullets, bullets...)
			}
			continue
		}
		if table := parseMarkdownTable(lines); table != nil {
			if currentSection >= 0 && currentSection < len(spec.Sections) {
				spec.Sections[currentSection].Table = table
			} else {
				startSection("Table")
				spec.Sections[currentSection].Table = table
			}
			continue
		}
		appendBlock(Block{Kind: BlockParagraph, Text: joinTextFragments(lines)})
	}

	return spec
}

func splitFirstMarkdownLine(text string) (string, string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return strings.TrimSpace(text[:idx]), text[idx+1:]
	}
	return strings.TrimSpace(text), ""
}

func trimmedLines(block string) []string {
	rawLines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func markdownHeading(line string) (string, int, bool) {
	line = strings.TrimSpace(line)
	for level := 1; level <= 3; level++ {
		prefix := strings.Repeat("#", level) + " "
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), level, true
		}
	}
	return "", 0, false
}

func allBulletLines(lines []string) bool {
	if len(lines) == 0 {
		return false
	}
	for _, line := range lines {
		if _, ok := parseMarkdownListItem(line); !ok {
			return false
		}
	}
	return true
}

func markdownBullets(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if bullet, ok := parseMarkdownListItem(line); ok && bullet != "" {
			out = append(out, bullet)
		}
	}
	return out
}

func parseMarkdownListItem(line string) (string, bool) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "- [x] "), strings.HasPrefix(line, "- [X] "), strings.HasPrefix(line, "* [x] "), strings.HasPrefix(line, "* [X] "):
		return "☑ " + strings.TrimSpace(line[6:]), true
	case strings.HasPrefix(line, "- [ ] "), strings.HasPrefix(line, "* [ ] "):
		return "☐ " + strings.TrimSpace(line[6:]), true
	}
	if isOrderedListItem(line) {
		return line, true
	}
	switch {
	case strings.HasPrefix(line, "- "):
		return strings.TrimSpace(strings.TrimPrefix(line, "- ")), true
	case strings.HasPrefix(line, "* "):
		return strings.TrimSpace(strings.TrimPrefix(line, "* ")), true
	case strings.HasPrefix(line, "• "):
		return strings.TrimSpace(strings.TrimPrefix(line, "• ")), true
	}
	return "", false
}

func isOrderedListItem(line string) bool {
	return orderedListPattern.MatchString(strings.TrimSpace(line))
}

func markdownBlockquote(lines []string) (string, bool) {
	if len(lines) == 0 {
		return "", false
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, ">") {
			return "", false
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, ">"))
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n")), true
}

func markdownImageBlock(block string) (string, string, bool) {
	lines := trimmedLines(block)
	if len(lines) != 1 {
		return "", "", false
	}
	line := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(line, "![") {
		return "", "", false
	}
	altEnd := strings.Index(line, "](")
	if altEnd < 2 || !strings.HasSuffix(line, ")") {
		return "", "", false
	}
	alt := line[2:altEnd]
	source := strings.TrimSpace(line[altEnd+2 : len(line)-1])
	if source == "" {
		return "", "", false
	}
	if strings.HasPrefix(source, "<") && strings.HasSuffix(source, ">") && len(source) >= 2 {
		source = strings.TrimSpace(source[1 : len(source)-1])
	}
	return strings.TrimSpace(alt), source, true
}

func markdownFencedCodeBlock(block string) (string, bool) {
	block = strings.TrimSpace(strings.ReplaceAll(block, "\r\n", "\n"))
	if block == "" {
		return "", false
	}
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		return "", false
	}
	fenceChar, fenceLen, ok := markdownFenceDelimiter(lines[0])
	if !ok || !markdownFenceMatches(lines[len(lines)-1], fenceChar, fenceLen) {
		return "", false
	}
	return strings.Join(lines[1:len(lines)-1], "\n"), true
}

func markdownThematicBreak(block string) (string, bool) {
	lines := trimmedLines(block)
	if len(lines) != 1 {
		return "", false
	}
	line := strings.TrimSpace(lines[0])
	if line == "" {
		return "", false
	}
	var marker byte
	count := 0
	for idx := 0; idx < len(line); idx++ {
		ch := line[idx]
		switch ch {
		case ' ', '\t':
			continue
		case '-', '*', '_':
			if marker == 0 {
				marker = ch
			} else if ch != marker {
				return "", false
			}
			count++
		default:
			return "", false
		}
	}
	if count < 3 || marker == 0 {
		return "", false
	}
	return strings.Repeat(string(marker), 3), true
}

func markdownFenceDelimiter(line string) (byte, int, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return 0, 0, false
	}
	fenceChar := line[0]
	if fenceChar != '`' && fenceChar != '~' {
		return 0, 0, false
	}
	count := 0
	for count < len(line) && line[count] == fenceChar {
		count++
	}
	if count < 3 {
		return 0, 0, false
	}
	return fenceChar, count, true
}

func markdownFenceMatches(line string, fenceChar byte, minCount int) bool {
	line = strings.TrimSpace(line)
	if len(line) < minCount {
		return false
	}
	for idx := 0; idx < len(line); idx++ {
		if line[idx] != fenceChar {
			return false
		}
	}
	return true
}

func renderedListLine(text, defaultPrefix string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if isOrderedListItem(text) || isTaskListItem(text) {
		return text
	}
	return defaultPrefix + text
}

func isTaskListItem(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "☑ ") || strings.HasPrefix(text, "☐ ")
}

func parseMarkdownTable(lines []string) *TableSpec {
	if len(lines) < 2 {
		return nil
	}
	for _, line := range lines {
		if !strings.Contains(line, "|") {
			return nil
		}
	}
	headers := markdownTableCells(lines[0])
	if len(headers) == 0 || !markdownTableDivider(lines[1], len(headers)) {
		return nil
	}
	rows := make([][]string, 0, len(lines)-2)
	for _, line := range lines[2:] {
		cells := markdownTableCells(line)
		if len(cells) == 0 {
			continue
		}
		for len(cells) < len(headers) {
			cells = append(cells, "")
		}
		rows = append(rows, cells)
	}
	return &TableSpec{Headers: headers, Rows: rows}
}

func markdownTableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func markdownTableDivider(line string, columns int) bool {
	cells := markdownTableCells(line)
	if len(cells) != columns {
		return false
	}
	for _, cell := range cells {
		cell = strings.TrimSpace(strings.Trim(cell, ":"))
		if cell == "" {
			return false
		}
		for _, r := range cell {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func joinTextFragments(fragments []string) string {
	var sb strings.Builder
	for _, fragment := range fragments {
		part := normalizeInlineWhitespace(fragment)
		if part == "" {
			continue
		}
		if sb.Len() == 0 {
			sb.WriteString(part)
			continue
		}
		prev := lastNonSpaceRune(sb.String())
		next := firstNonSpaceRune(part)
		if shouldInsertFragmentSpace(prev, next) {
			sb.WriteByte(' ')
		}
		sb.WriteString(part)
	}
	return strings.TrimSpace(sb.String())
}

func normalizeInlineWhitespace(text string) string {
	var sb strings.Builder
	lastWasSpace := false
	for _, r := range text {
		switch r {
		case '\u200b', '\u200c', '\u200d', '\ufeff':
			continue
		case '\u00a0', '\u3000':
			r = ' '
		}
		if unicode.IsSpace(r) {
			if sb.Len() == 0 || lastWasSpace {
				continue
			}
			sb.WriteByte(' ')
			lastWasSpace = true
			continue
		}
		sb.WriteRune(r)
		lastWasSpace = false
	}
	return strings.TrimSpace(sb.String())
}

func firstNonSpaceRune(text string) rune {
	for _, r := range text {
		if !unicode.IsSpace(r) {
			return r
		}
	}
	return 0
}

func lastNonSpaceRune(text string) rune {
	runes := []rune(text)
	for idx := len(runes) - 1; idx >= 0; idx-- {
		if !unicode.IsSpace(runes[idx]) {
			return runes[idx]
		}
	}
	return 0
}

func shouldInsertFragmentSpace(prev, next rune) bool {
	if prev == 0 || next == 0 {
		return false
	}
	if isOpenPunctuation(prev) || isClosePunctuation(next) {
		return false
	}
	if isSentenceEnding(prev) {
		return !isCJK(next)
	}
	if isCJK(prev) || isCJK(next) {
		return false
	}
	return true
}

func isSentenceEnding(r rune) bool {
	return strings.ContainsRune("。！？!?；;.:：", r)
}

func isOpenPunctuation(r rune) bool {
	return strings.ContainsRune("([{<\"'“‘【《「『", r)
}

func isClosePunctuation(r rune) bool {
	return strings.ContainsRune(")]}>\"'”’】》」』，。！？!?；;：:、,.", r)
}

func isCJK(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func runeCount(text string) int {
	if text == "" {
		return 0
	}
	return utf8.RuneCountInString(text)
}

func RenderedListLine(text, defaultPrefix string) string {
	return renderedListLine(text, defaultPrefix)
}

func JoinTextFragments(values []string) string {
	return joinTextFragments(values)
}
