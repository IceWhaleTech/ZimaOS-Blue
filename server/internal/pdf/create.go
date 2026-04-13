package pdf

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	createPageWidth    = 612.0
	createPageHeight   = 792.0
	createMarginLeft   = 54.0
	createMarginTop    = 60.0
	createMarginBottom = 54.0
)

const (
	createFontRegular = "F1"
	createFontBold    = "F2"
)

var createPDFTextEscaper = strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")

type CreateRequest struct {
	Title      string
	Subtitle   string
	Summary    string
	Paragraphs []string
	Notes      []string
	Sections   []CreateSection
}

type CreateSection struct {
	Heading    string
	Paragraphs []string
	Bullets    []string
	Table      *CreateTable
}

type CreateTable struct {
	Headers []string
	Rows    [][]string
}

type CreateResult struct {
	PageCount int
	LineCount int
	CharCount int
	Warnings  []string
}

type createStyledLine struct {
	Text      string
	Font      string
	FontSize  float64
	GapAfter  float64
	CharCount int
}

func CreateDocument(req CreateRequest) ([]byte, CreateResult, error) {
	lines, warnings := buildCreateStyledLines(req)
	if len(lines) == 0 {
		return nil, CreateResult{}, fmt.Errorf("pdf create requires title, summary, sections, paragraphs, notes, or content")
	}
	pageStreams, lineCount, charCount := paginateCreateLines(lines)
	if len(pageStreams) == 0 {
		return nil, CreateResult{}, fmt.Errorf("pdf create produced no pages")
	}
	return buildCreatePDFBytes(pageStreams), CreateResult{
		PageCount: len(pageStreams),
		LineCount: lineCount,
		CharCount: charCount,
		Warnings:  warnings,
	}, nil
}

func buildCreateStyledLines(req CreateRequest) ([]createStyledLine, []string) {
	lines := make([]createStyledLine, 0, 32)
	warnings := make([]string, 0, 1)
	replacedRunes := false

	appendLine := func(text, font string, fontSize, gapAfter float64) {
		sanitized, replaced := sanitizeCreateText(text)
		if sanitized == "" {
			return
		}
		replacedRunes = replacedRunes || replaced
		lines = append(lines, createStyledLine{
			Text:      sanitized,
			Font:      font,
			FontSize:  fontSize,
			GapAfter:  gapAfter,
			CharCount: utf8.RuneCountInString(sanitized),
		})
	}

	appendBodyParagraphs := func(items []string) {
		for _, item := range items {
			appendLine(item, createFontRegular, 11, 6)
		}
	}

	appendBullets := func(items []string) {
		for _, item := range items {
			appendLine("- "+item, createFontRegular, 11, 4)
		}
	}

	appendLine(req.Title, createFontBold, 22, 8)
	appendLine(req.Subtitle, createFontRegular, 14, 12)

	if strings.TrimSpace(req.Summary) != "" {
		appendLine("Summary", createFontBold, 13, 3)
		appendLine(req.Summary, createFontRegular, 11, 10)
	}

	appendBodyParagraphs(req.Paragraphs)

	for _, section := range req.Sections {
		appendLine(section.Heading, createFontBold, 16, 5)
		appendBodyParagraphs(section.Paragraphs)
		appendBullets(section.Bullets)
		if section.Table != nil {
			if len(section.Table.Headers) > 0 {
				appendLine(strings.Join(section.Table.Headers, " | "), createFontBold, 10, 2)
			}
			for _, row := range section.Table.Rows {
				appendLine(strings.Join(row, " | "), createFontRegular, 10, 2)
			}
			if len(section.Table.Headers) > 0 || len(section.Table.Rows) > 0 {
				lines = append(lines, createStyledLine{GapAfter: 6})
			}
		}
	}

	if len(req.Notes) > 0 {
		appendLine("Notes", createFontBold, 12, 3)
		appendBullets(req.Notes)
	}

	if replacedRunes {
		warnings = append(warnings, "native_pdf_ir create replaced unsupported characters with simpler ASCII glyphs")
	}
	return lines, warnings
}

func sanitizeCreateText(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", false
	}
	var b strings.Builder
	b.Grow(len(text))
	replaced := false
	for _, r := range text {
		switch r {
		case '\n', '\r', '\t', '\u00a0':
			b.WriteByte(' ')
			replaced = replaced || r != ' '
		case '“', '”', '„', '‟':
			b.WriteByte('"')
			replaced = true
		case '‘', '’', '‚', '‛':
			b.WriteByte('\'')
			replaced = true
		case '—', '–', '−':
			b.WriteByte('-')
			replaced = true
		case '•':
			b.WriteByte('-')
			replaced = true
		case '…':
			b.WriteString("...")
			replaced = true
		default:
			if r >= 32 && r <= 126 {
				b.WriteRune(r)
				continue
			}
			replaced = true
			b.WriteByte('?')
		}
	}
	collapsed := strings.Join(strings.Fields(b.String()), " ")
	return strings.TrimSpace(collapsed), replaced
}

func paginateCreateLines(lines []createStyledLine) ([]string, int, int) {
	pageStreams := make([]string, 0, 1)
	var page bytes.Buffer
	y := createPageHeight - createMarginTop
	lineCount := 0
	charCount := 0

	flushPage := func() {
		if page.Len() == 0 {
			return
		}
		pageStreams = append(pageStreams, page.String())
		page.Reset()
	}

	for _, line := range lines {
		if strings.TrimSpace(line.Text) == "" {
			y -= 10 + line.GapAfter
			if y < createMarginBottom {
				flushPage()
				y = createPageHeight - createMarginTop
			}
			continue
		}
		wrapped := wrapCreateText(line.Text, createMaxCharsPerLine(line.FontSize))
		for idx, item := range wrapped {
			lineHeight := line.FontSize * 1.35
			if y-lineHeight < createMarginBottom {
				flushPage()
				y = createPageHeight - createMarginTop
			}
			writeCreateTextCommand(&page, line.Font, line.FontSize, createMarginLeft, y, item)
			y -= lineHeight
			lineCount++
			if idx == len(wrapped)-1 {
				y -= line.GapAfter
			}
		}
		charCount += line.CharCount
	}
	flushPage()
	if len(pageStreams) == 0 {
		pageStreams = append(pageStreams, "")
	}
	return pageStreams, lineCount, charCount
}

func createMaxCharsPerLine(fontSize float64) int {
	if fontSize <= 0 {
		fontSize = 11
	}
	width := createPageWidth - (createMarginLeft * 2)
	maxChars := int(width / (fontSize * 0.58))
	if maxChars < 20 {
		return 20
	}
	return maxChars
}

func wrapCreateText(text string, limit int) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return []string{text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := make([]string, 0, len(words))
	current := ""
	currentRunes := 0
	flushCurrent := func() {
		if strings.TrimSpace(current) == "" {
			return
		}
		lines = append(lines, current)
		current = ""
		currentRunes = 0
	}
	for _, word := range words {
		wordRunes := utf8.RuneCountInString(word)
		if wordRunes > limit {
			flushCurrent()
			lines = append(lines, splitCreateLongWord(word, limit)...)
			continue
		}
		if current == "" {
			current = word
			currentRunes = wordRunes
			continue
		}
		if currentRunes+1+wordRunes <= limit {
			current += " " + word
			currentRunes += 1 + wordRunes
			continue
		}
		flushCurrent()
		current = word
		currentRunes = wordRunes
	}
	flushCurrent()
	return lines
}

func splitCreateLongWord(word string, limit int) []string {
	runes := []rune(word)
	if len(runes) <= limit || limit <= 0 {
		return []string{word}
	}
	parts := make([]string, 0, (len(runes)/limit)+1)
	for len(runes) > 0 {
		n := limit
		if len(runes) < n {
			n = len(runes)
		}
		parts = append(parts, string(runes[:n]))
		runes = runes[n:]
	}
	return parts
}

func writeCreateTextCommand(buf *bytes.Buffer, font string, fontSize, x, y float64, text string) {
	buf.WriteString("BT\n/")
	buf.WriteString(font)
	buf.WriteByte(' ')
	buf.WriteString(strconv.FormatFloat(fontSize, 'f', 2, 64))
	buf.WriteString(" Tf\n1 0 0 1 ")
	buf.WriteString(strconv.FormatFloat(x, 'f', 2, 64))
	buf.WriteByte(' ')
	buf.WriteString(strconv.FormatFloat(y, 'f', 2, 64))
	buf.WriteString(" Tm\n(")
	buf.WriteString(escapeCreatePDFText(text))
	buf.WriteString(") Tj\nET\n")
}

func buildCreatePDFBytes(pageStreams []string) []byte {
	pageCount := len(pageStreams)
	regularFontID := 3 + (pageCount * 2)
	boldFontID := regularFontID + 1
	totalObjects := boldFontID
	objects := make([]string, totalObjects+1)

	kids := make([]string, 0, pageCount)
	for idx, stream := range pageStreams {
		pageID := 3 + (idx * 2)
		contentID := pageID + 1
		kids = append(kids, fmt.Sprintf("%d 0 R", pageID))
		objects[pageID] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> >>",
			createPageWidth,
			createPageHeight,
			contentID,
			regularFontID,
			boldFontID,
		)
		objects[contentID] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream)
	}

	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	objects[2] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), pageCount)
	objects[regularFontID] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	objects[boldFontID] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>"

	var buf bytes.Buffer
	streamBytes := 0
	for _, stream := range pageStreams {
		streamBytes += len(stream)
	}
	buf.Grow(1024 + streamBytes + len(pageStreams)*256)
	buf.WriteString("%PDF-1.4\n%\xD0\xD4\xC5\xD8\n")
	offsets := make([]int, totalObjects+1)
	for objectID := 1; objectID <= totalObjects; objectID++ {
		offsets[objectID] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", objectID, objects[objectID])
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", totalObjects+1)
	buf.WriteString("0000000000 65535 f \n")
	for objectID := 1; objectID <= totalObjects; objectID++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[objectID])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", totalObjects+1, xrefStart)
	return buf.Bytes()
}

func escapeCreatePDFText(text string) string {
	return createPDFTextEscaper.Replace(text)
}
