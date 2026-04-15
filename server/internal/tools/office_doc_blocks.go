package tools

import "strings"

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

func officeDocBlocksOrParagraphs(blocks []officeDocBlock, paragraphs []string) []officeDocBlock {
	out := make([]officeDocBlock, 0, len(blocks)+len(paragraphs))
	out = append(out, blocks...)
	out = append(out, officeDocBlocksFromLegacyParagraphs(paragraphs)...)
	return out
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
