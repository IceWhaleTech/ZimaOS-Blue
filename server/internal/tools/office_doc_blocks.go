package tools

import (
	"strings"
)

func officeDocBlocksFromLegacyParagraphs(values []string) []officeDocBlock {
	out := make([]officeDocBlock, 0, len(values))
	for _, value := range values {
		out = append(out, officeDocBlocksFromLegacyParagraph(value)...)
	}
	return out
}

func officeDocBlocksFromBodyText(text string) []officeDocBlock {
	parts := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n")
	out := make([]officeDocBlock, 0, len(parts))
	for _, part := range parts {
		out = append(out, officeDocBlocksFromLegacyParagraph(part)...)
	}
	return out
}

func officeDocBlocksFromLegacyParagraph(text string) []officeDocBlock {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if text == "" {
		return nil
	}
	if alt, source, ok := officeMarkdownImageBlock(text); ok {
		return []officeDocBlock{{Kind: officeDocBlockImage, Text: alt, Source: source}}
	}
	if separator, ok := officeMarkdownThematicBreak(text); ok {
		return []officeDocBlock{{Kind: officeDocBlockSeparator, Text: separator}}
	}
	if code, ok := officeMarkdownFencedCodeBlock(text); ok {
		return []officeDocBlock{{Kind: officeDocBlockCode, Text: code}}
	}
	if lines := officeTrimmedLines(text); len(lines) > 0 {
		if quote, ok := officeMarkdownBlockquote(lines); ok {
			return []officeDocBlock{{Kind: officeDocBlockQuote, Text: quote}}
		}
	}
	return []officeDocBlock{{Kind: officeDocBlockParagraph, Text: text}}
}

type officeParsedSectionBody struct {
	Heading string
	Blocks  []officeDocBlock
	Bullets []string
	Table   *officeTableSpec
}

func officeParseMarkdownishSectionBody(text string) officeParsedSectionBody {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	parsed := officeParsedSectionBody{}

	var (
		paragraphLines []string
		quoteLines     []string
		bulletLines    []string
		tableLines     []string
		codeLines      []string
		codeFenceChar  byte
		codeFenceLen   int
		inCodeBlock    bool
	)

	hasStructuredContent := func() bool {
		return len(parsed.Blocks) > 0 || len(parsed.Bullets) > 0 || parsed.Table != nil
	}
	flushParagraph := func() {
		if len(paragraphLines) == 0 {
			return
		}
		if blocks := officeCompactStatParagraphBlocks(paragraphLines); len(blocks) > 0 {
			parsed.Blocks = append(parsed.Blocks, blocks...)
			paragraphLines = nil
			return
		}
		if text := officeJoinTextFragments(paragraphLines); text != "" {
			parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockParagraph, Text: text})
		}
		paragraphLines = nil
	}
	flushQuote := func() {
		if len(quoteLines) == 0 {
			return
		}
		if text, ok := officeMarkdownBlockquote(quoteLines); ok && text != "" {
			parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockQuote, Text: text})
		}
		quoteLines = nil
	}
	flushBullets := func() {
		if len(bulletLines) == 0 {
			return
		}
		parsed.Bullets = append(parsed.Bullets, officeMarkdownBullets(bulletLines)...)
		bulletLines = nil
	}
	flushTable := func() {
		if len(tableLines) == 0 {
			return
		}
		if table := officeParseMarkdownTable(tableLines); table != nil {
			if parsed.Table == nil {
				parsed.Table = table
			} else {
				lines := make([]string, 0, len(table.Headers)+len(table.Rows))
				if len(table.Headers) > 0 {
					lines = append(lines, strings.Join(table.Headers, " | "))
				}
				for _, row := range table.Rows {
					lines = append(lines, strings.Join(row, " | "))
				}
				if text := officeJoinTextFragments(lines); text != "" {
					parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockParagraph, Text: text})
				}
			}
		} else {
			paragraphLines = append(paragraphLines, tableLines...)
		}
		tableLines = nil
	}
	flushCode := func() {
		if !inCodeBlock {
			return
		}
		inCodeBlock = false
		codeFenceChar = 0
		codeFenceLen = 0
		if len(codeLines) == 0 {
			return
		}
		parsed.Blocks = append(parsed.Blocks, officeDocBlock{
			Kind: officeDocBlockCode,
			Text: strings.Join(codeLines, "\n"),
		})
		codeLines = nil
	}
	flushStructured := func() {
		flushParagraph()
		flushQuote()
		flushBullets()
		flushTable()
	}

	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if inCodeBlock {
			if officeMarkdownFenceMatches(line, codeFenceChar, codeFenceLen) {
				flushCode()
				continue
			}
			codeLines = append(codeLines, strings.TrimRight(rawLine, "\r"))
			continue
		}
		if line == "" {
			flushStructured()
			continue
		}
		if alt, source, ok := officeMarkdownImageBlock(line); ok {
			flushStructured()
			parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockImage, Text: alt, Source: source})
			continue
		}
		if separator, ok := officeMarkdownThematicBreak(line); ok {
			flushStructured()
			parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockSeparator, Text: separator})
			continue
		}
		if fenceChar, fenceLen, ok := officeMarkdownFenceDelimiter(line); ok {
			flushStructured()
			inCodeBlock = true
			codeFenceChar = fenceChar
			codeFenceLen = fenceLen
			codeLines = nil
			continue
		}
		if heading, _, ok := officeMarkdownHeading(line); ok {
			flushStructured()
			if parsed.Heading == "" && !hasStructuredContent() {
				parsed.Heading = heading
			} else {
				parsed.Blocks = append(parsed.Blocks, officeDocBlock{Kind: officeDocBlockParagraph, Text: heading})
			}
			continue
		}
		if strings.HasPrefix(line, ">") {
			flushParagraph()
			flushBullets()
			flushTable()
			quoteLines = append(quoteLines, line)
			continue
		}
		flushQuote()
		if _, ok := officeParseMarkdownListItem(line); ok {
			flushParagraph()
			flushTable()
			bulletLines = append(bulletLines, line)
			continue
		}
		flushBullets()
		if officeLooksLikeMarkdownTableRow(line) {
			flushParagraph()
			tableLines = append(tableLines, line)
			continue
		}
		flushTable()
		paragraphLines = append(paragraphLines, line)
	}

	flushStructured()
	flushCode()
	return parsed
}

func officeLooksLikeMarkdownTableRow(line string) bool {
	line = strings.TrimSpace(line)
	return strings.Count(line, "|") >= 2
}

func officeLooksLikeGeneratedSlideHeading(text string) bool {
	return officeGeneratedSlideHeadingPattern.MatchString(strings.TrimSpace(text))
}

func officeDocBlocksOrParagraphs(blocks []officeDocBlock, paragraphs []string) []officeDocBlock {
	out := make([]officeDocBlock, 0, len(blocks)+len(paragraphs))
	out = append(out, blocks...)
	out = append(out, officeDocBlocksFromLegacyParagraphs(paragraphs)...)
	return out
}

func officeCompactStatParagraphBlocks(lines []string) []officeDocBlock {
	if len(lines) < 2 || len(lines) > 6 {
		return nil
	}
	blocks := make([]officeDocBlock, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			return nil
		}
		if _, _, ok := officeSplitCompactStat(text); !ok {
			return nil
		}
		blocks = append(blocks, officeDocBlock{Kind: officeDocBlockParagraph, Text: text})
	}
	return blocks
}

func officeSplitCompactStat(text string) (string, string, bool) {
	for _, separator := range []string{":", "："} {
		left, right, ok := strings.Cut(text, separator)
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if left == "" || right == "" {
			continue
		}
		if runeCount(left) > 28 || runeCount(right) > 24 {
			continue
		}
		return left, right, true
	}
	return "", "", false
}

func officeMarkdownBlockquote(lines []string) (string, bool) {
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

func officeMarkdownImageBlock(block string) (string, string, bool) {
	lines := officeTrimmedLines(block)
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

func officeMarkdownFencedCodeBlock(block string) (string, bool) {
	block = strings.TrimSpace(strings.ReplaceAll(block, "\r\n", "\n"))
	if block == "" {
		return "", false
	}
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		return "", false
	}
	fenceChar, fenceLen, ok := officeMarkdownFenceDelimiter(lines[0])
	if !ok || !officeMarkdownFenceMatches(lines[len(lines)-1], fenceChar, fenceLen) {
		return "", false
	}
	return strings.Join(lines[1:len(lines)-1], "\n"), true
}

func officeMarkdownThematicBreak(block string) (string, bool) {
	lines := officeTrimmedLines(block)
	if len(lines) != 1 {
		return "", false
	}
	line := strings.TrimSpace(lines[0])
	if line == "" {
		return "", false
	}

	var (
		marker byte
		count  int
	)
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

func officeMarkdownFenceDelimiter(line string) (byte, int, bool) {
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

func officeMarkdownFenceMatches(line string, fenceChar byte, minCount int) bool {
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
