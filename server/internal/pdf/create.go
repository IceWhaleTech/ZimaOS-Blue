package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-pdf/fpdf"
)

const (
	createPageWidth         = 612.0
	createPageHeight        = 792.0
	createMarginLeft        = 54.0
	createMarginTop         = 60.0
	createMarginBottom      = 54.0
	createLineHeightScale   = 1.35
	createBlankLineHeight   = 10.0
	createCoreFontFamily    = "Helvetica"
	createUnicodeFontFamily = "nativepdfunicode"
)

const (
	createFontRegular = "F1"
	createFontBold    = "F2"
)

const (
	createTableCellPaddingX    = 8.0
	createTableCellPaddingY    = 5.0
	createTableBorderWidth     = 0.75
	createTableLineHeightScale = 1.2
	createDividerLineWidth     = 0.9
	createDividerInset         = 6.0
	createListMarkerGap        = 8.0
	createListMinMarkerWidth   = 12.0
	createDividerSentinel      = "────────"
)

type CreateRequest struct {
	Title         string
	Subtitle      string
	Summary       string
	Paragraphs    []string
	Notes         []string
	Sections      []CreateSection
	TitleColor    string
	SubtitleColor string
	HeadingColor  string
	BodyColor     string
	MutedColor    string
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
	Align     string
	Font      string
	FontSize  float64
	GapAfter  float64
	CharCount int
	ColorR    int
	ColorG    int
	ColorB    int
	Table     *createStyledTable
	ListItem  *createStyledListItem
	Divider   *createStyledDivider
}

type createStyledTable struct {
	Headers     []createStyledTableCell
	Rows        [][]createStyledTableCell
	FontSize    float64
	HeaderTextR int
	HeaderTextG int
	HeaderTextB int
	BodyTextR   int
	BodyTextG   int
	BodyTextB   int
	BorderR     int
	BorderG     int
	BorderB     int
	HeaderFillR int
	HeaderFillG int
	HeaderFillB int
	RowFillR    int
	RowFillG    int
	RowFillB    int
}

type createStyledTableCell struct {
	Text  string
	Align string
}

type createStyledListItem struct {
	Marker string
	Text   string
	Align  string
}

type createStyledDivider struct {
	LineR int
	LineG int
	LineB int
}

type createFontPlan struct {
	family       string
	unicodeBytes []byte
	supportsRune func(rune) bool
}

func (p createFontPlan) hasUnicodeFont() bool {
	return len(p.unicodeBytes) > 0 && p.supportsRune != nil
}

func (p createFontPlan) fontFor(id string) (string, string) {
	family := p.family
	if strings.TrimSpace(family) == "" {
		family = createCoreFontFamily
	}
	switch id {
	case createFontBold:
		return family, "B"
	default:
		return family, ""
	}
}

func CreateDocument(req CreateRequest) ([]byte, CreateResult, error) {
	fontPlan := resolveCreateFontPlan(createRequestTextValues(req))
	lines, warnings := buildCreateStyledLines(req, fontPlan)
	if len(lines) == 0 {
		return nil, CreateResult{}, fmt.Errorf("pdf create requires title, summary, sections, paragraphs, notes, or content")
	}

	data, pageCount, lineCount, err := renderCreatePDF(lines, fontPlan)
	if err != nil {
		return nil, CreateResult{}, err
	}

	charCount := 0
	for _, line := range lines {
		charCount += line.CharCount
	}

	return data, CreateResult{
		PageCount: pageCount,
		LineCount: lineCount,
		CharCount: charCount,
		Warnings:  warnings,
	}, nil
}

func resolveCreateFontPlan(textValues []string) createFontPlan {
	required := collectCreateRequiredRunes(textValues)
	if len(required) == 0 {
		return createFontPlan{family: createCoreFontFamily}
	}

	fontBytes, err := resolveCreateUnicodeFont(required)
	if err != nil {
		return createFontPlan{family: createCoreFontFamily}
	}
	supportsRune, err := createFontSupportFunc(fontBytes)
	if err != nil {
		return createFontPlan{family: createCoreFontFamily}
	}
	return createFontPlan{
		family:       createUnicodeFontFamily,
		unicodeBytes: fontBytes,
		supportsRune: supportsRune,
	}
}

func createRequestTextValues(req CreateRequest) []string {
	values := make([]string, 0, 16)
	appendText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		values = append(values, text)
		if createHasArabicLetters(text) {
			values = append(values, createShapeArabicVisual(text))
		}
	}

	appendText(req.Title)
	appendText(req.Subtitle)
	appendText(req.Summary)
	for _, paragraph := range req.Paragraphs {
		appendText(paragraph)
	}
	for _, note := range req.Notes {
		appendText(note)
	}
	for _, section := range req.Sections {
		appendText(section.Heading)
		for _, paragraph := range section.Paragraphs {
			appendText(paragraph)
		}
		for _, bullet := range section.Bullets {
			appendText(bullet)
		}
		if section.Table != nil {
			for _, header := range section.Table.Headers {
				appendText(header)
			}
			for _, row := range section.Table.Rows {
				for _, cell := range row {
					appendText(cell)
				}
			}
		}
	}
	return values
}

func buildCreateStyledLines(req CreateRequest, fontPlan createFontPlan) ([]createStyledLine, []string) {
	lines := make([]createStyledLine, 0, 32)
	warnings := make([]string, 0, 1)
	replacedRunes := false

	appendDivider := func(color string, gapAfter float64) {
		lineR, lineG, lineB := createBlendRGB(color, 0.78)
		lines = append(lines, createStyledLine{
			GapAfter: gapAfter,
			Divider: &createStyledDivider{
				LineR: lineR,
				LineG: lineG,
				LineB: lineB,
			},
		})
	}

	appendLine := func(text, font string, fontSize, gapAfter float64, color string) {
		if createTextIsDivider(text) {
			appendDivider(req.MutedColor, gapAfter)
			return
		}
		text = cleanCreateInlineMarkdown(text)
		if createTextIsDivider(text) {
			appendDivider(req.MutedColor, gapAfter)
			return
		}
		sanitized, replaced := sanitizeCreateText(text, fontPlan)
		if sanitized == "" {
			return
		}
		replacedRunes = replacedRunes || replaced
		colorR, colorG, colorB := createColorRGB(color)
		lines = append(lines, createStyledLine{
			Text:      sanitized,
			Align:     createTextAlign(sanitized),
			Font:      font,
			FontSize:  fontSize,
			GapAfter:  gapAfter,
			CharCount: utf8.RuneCountInString(sanitized),
			ColorR:    colorR,
			ColorG:    colorG,
			ColorB:    colorB,
		})
	}

	appendBodyParagraphs := func(items []string) {
		for idx := 0; idx < len(items); idx++ {
			cleaned := cleanCreateInlineMarkdown(items[idx])
			if title, body, ok := createMonthParagraphParts(cleaned); ok {
				appendLine(title, createFontBold, 12, 2, req.HeadingColor)
				if body != "" {
					appendLine(body, createFontRegular, 11, 6, req.BodyColor)
				}
				continue
			}
			if createLooksLikeMonthTitleText(cleaned) && idx+1 < len(items) {
				if stars, body, ok := createMonthRatingLeadParts(cleanCreateInlineMarkdown(items[idx+1])); ok {
					appendLine(strings.TrimSpace(cleaned+" "+stars), createFontBold, 12, 2, req.HeadingColor)
					if body != "" {
						appendLine(body, createFontRegular, 11, 6, req.BodyColor)
					}
					idx++
					continue
				}
			}
			switch {
			case createLooksLikeMonthTitleParagraph(cleaned):
				appendLine(cleaned, createFontBold, 12, 2, req.HeadingColor)
				continue
			default:
				if lead, body, ok := createLeadInParagraphParts(cleaned); ok {
					appendLine(lead, createFontBold, 12, 1, req.HeadingColor)
					appendLine(body, createFontRegular, 11, 6, req.BodyColor)
					continue
				}
			}
			appendLine(items[idx], createFontRegular, 11, 6, req.BodyColor)
		}
	}

	appendListItems := func(items []string, color string) {
		colorR, colorG, colorB := createColorRGB(color)
		for _, item := range items {
			marker, content := createParseListItem(item)
			content = cleanCreateInlineMarkdown(content)
			sanitizedText, textReplaced := sanitizeCreateText(content, fontPlan)
			if sanitizedText == "" {
				continue
			}
			sanitizedMarker, markerReplaced := sanitizeCreateText(marker, fontPlan)
			if sanitizedMarker == "" {
				sanitizedMarker = "-"
			}
			replacedRunes = replacedRunes || textReplaced || markerReplaced
			lines = append(lines, createStyledLine{
				Font:      createFontRegular,
				FontSize:  11,
				GapAfter:  4,
				CharCount: utf8.RuneCountInString(sanitizedMarker + " " + sanitizedText),
				ColorR:    colorR,
				ColorG:    colorG,
				ColorB:    colorB,
				ListItem: &createStyledListItem{
					Marker: sanitizedMarker,
					Text:   sanitizedText,
					Align:  createTextAlign(sanitizedText),
				},
			})
		}
	}

	appendTable := func(table *CreateTable) {
		if table == nil {
			return
		}

		headerTextR, headerTextG, headerTextB := createColorRGB(req.HeadingColor)
		bodyTextR, bodyTextG, bodyTextB := createColorRGB(req.BodyColor)
		borderR, borderG, borderB := createBlendRGB(req.MutedColor, 0.82)
		headerFillR, headerFillG, headerFillB := createBlendRGB(req.HeadingColor, 0.92)
		rowFillR, rowFillG, rowFillB := createBlendRGB(req.MutedColor, 0.96)

		styled := createStyledTable{
			Headers:     make([]createStyledTableCell, 0, len(table.Headers)),
			Rows:        make([][]createStyledTableCell, 0, len(table.Rows)),
			FontSize:    10,
			HeaderTextR: headerTextR,
			HeaderTextG: headerTextG,
			HeaderTextB: headerTextB,
			BodyTextR:   bodyTextR,
			BodyTextG:   bodyTextG,
			BodyTextB:   bodyTextB,
			BorderR:     borderR,
			BorderG:     borderG,
			BorderB:     borderB,
			HeaderFillR: headerFillR,
			HeaderFillG: headerFillG,
			HeaderFillB: headerFillB,
			RowFillR:    rowFillR,
			RowFillG:    rowFillG,
			RowFillB:    rowFillB,
		}

		charCount := 0
		styleCell := func(text string) (createStyledTableCell, bool) {
			sanitized, replaced := sanitizeCreateText(cleanCreateInlineMarkdown(text), fontPlan)
			replacedRunes = replacedRunes || replaced
			charCount += utf8.RuneCountInString(sanitized)
			return createStyledTableCell{
				Text:  sanitized,
				Align: createTextAlign(sanitized),
			}, strings.TrimSpace(sanitized) != ""
		}

		headersHaveText := false
		for _, header := range table.Headers {
			cell, hasText := styleCell(header)
			headersHaveText = headersHaveText || hasText
			styled.Headers = append(styled.Headers, cell)
		}

		for _, row := range table.Rows {
			styledRow := make([]createStyledTableCell, 0, len(row))
			rowHasText := false
			for _, cellText := range row {
				cell, hasText := styleCell(cellText)
				rowHasText = rowHasText || hasText
				styledRow = append(styledRow, cell)
			}
			if rowHasText {
				styled.Rows = append(styled.Rows, styledRow)
			}
		}

		if !headersHaveText && len(styled.Rows) == 0 {
			return
		}

		lines = append(lines, createStyledLine{
			GapAfter:  8,
			CharCount: charCount,
			Table:     &styled,
		})
	}

	appendLine(req.Title, createFontBold, 22, 8, req.TitleColor)
	appendLine(req.Subtitle, createFontRegular, 14, 12, req.SubtitleColor)

	if strings.TrimSpace(req.Summary) != "" {
		appendLine("Summary", createFontBold, 13, 3, req.HeadingColor)
		appendLine(req.Summary, createFontRegular, 11, 10, req.BodyColor)
	}

	appendBodyParagraphs(req.Paragraphs)

	for _, section := range req.Sections {
		appendLine(section.Heading, createFontBold, 16, 5, req.HeadingColor)
		appendBodyParagraphs(section.Paragraphs)
		appendListItems(section.Bullets, req.BodyColor)
		appendTable(section.Table)
	}

	if len(req.Notes) > 0 {
		appendLine("Notes", createFontBold, 12, 3, req.HeadingColor)
		appendListItems(req.Notes, req.MutedColor)
	}

	if replacedRunes {
		warnings = append(warnings, "native_pdf_ir create replaced unsupported characters with simpler ASCII glyphs")
	}
	return lines, warnings
}

func cleanCreateInlineMarkdown(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	cleaned := strings.NewReplacer(
		"  \r\n", " · ",
		"  \n", " · ",
		"\r\n", " · ",
		"\n", " · ",
		"**", "",
		"__", "",
		"`", "",
		"~~", "",
	).Replace(text)
	return strings.TrimSpace(cleaned)
}

func createTextIsDivider(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if text == createDividerSentinel {
		return true
	}
	if utf8.RuneCountInString(text) < 3 {
		return false
	}
	return strings.Trim(text, "-*_─") == ""
}

func createParseListItem(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	switch {
	case strings.HasPrefix(text, "☑ "):
		return "☑", strings.TrimSpace(strings.TrimPrefix(text, "☑ "))
	case strings.HasPrefix(text, "☐ "):
		return "☐", strings.TrimSpace(strings.TrimPrefix(text, "☐ "))
	}
	if marker, content, ok := createOrderedListParts(text); ok {
		return marker, content
	}
	switch {
	case strings.HasPrefix(text, "- "):
		return "•", strings.TrimSpace(strings.TrimPrefix(text, "- "))
	case strings.HasPrefix(text, "* "):
		return "•", strings.TrimSpace(strings.TrimPrefix(text, "* "))
	case strings.HasPrefix(text, "• "):
		return "•", strings.TrimSpace(strings.TrimPrefix(text, "• "))
	default:
		return "•", text
	}
}

func createLeadInParagraphParts(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if text == "" || strings.Contains(text, " | ") || createLooksLikeMonthTitleText(text) {
		return "", "", false
	}

	for _, separator := range []string{"：", ":"} {
		left, right, ok := strings.Cut(text, separator)
		if !ok {
			continue
		}
		left = strings.TrimSpace(left)
		right = strings.TrimSpace(right)
		if left == "" || right == "" {
			return "", "", false
		}
		if utf8.RuneCountInString(left) > 18 {
			return "", "", false
		}
		if strings.ContainsAny(left, "。！？.!?|") || strings.Contains(right, " | ") {
			return "", "", false
		}
		return left + separator, right, true
	}

	return "", "", false
}

func createLooksLikeMonthTitleText(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || !strings.Contains(text, "（") {
		return false
	}

	for _, prefix := range []string{"正月", "二月", "三月", "四月", "五月", "六月", "七月", "八月", "九月", "十月", "十一月", "十二月"} {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

func createLooksLikeMonthTitleParagraph(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" || !createLooksLikeMonthTitleText(text) || !strings.ContainsAny(text, "★☆") {
		return false
	}
	return true
}

func createMonthParagraphParts(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if !createLooksLikeMonthTitleParagraph(text) {
		return "", "", false
	}

	closeIndex := strings.Index(text, "）")
	closeTokenLen := len("）")
	if closeIndex < 0 {
		closeIndex = strings.Index(text, ")")
		closeTokenLen = len(")")
	}
	if closeIndex <= 0 || closeIndex+closeTokenLen >= len(text) {
		return "", "", false
	}

	title := strings.TrimSpace(text[:closeIndex+closeTokenLen])
	rest := strings.TrimSpace(text[closeIndex+closeTokenLen:])
	stars, body, ok := createMonthRatingLeadParts(rest)
	if !ok {
		return "", "", false
	}

	title = strings.TrimSpace(title + " " + stars)
	for _, prefix := range []string{"危险", "最佳"} {
		if body == prefix {
			title = strings.TrimSpace(title + " " + prefix)
			body = ""
			break
		}
		if strings.HasPrefix(body, prefix) {
			trimmed := strings.TrimSpace(strings.TrimPrefix(body, prefix))
			if trimmed != "" {
				title = strings.TrimSpace(title + " " + prefix)
				body = trimmed
			}
			break
		}
	}
	return title, body, true
}

func createMonthRatingLeadParts(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}

	runes := []rune(text)
	idx := 0
	for idx < len(runes) && (runes[idx] == '★' || runes[idx] == '☆') {
		idx++
	}
	if idx == 0 {
		return "", "", false
	}

	stars := strings.TrimSpace(string(runes[:idx]))
	body := strings.TrimSpace(string(runes[idx:]))
	return stars, body, true
}

func createOrderedListParts(text string) (string, string, bool) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", false
	}
	prefixLen := 0
	for prefixLen < len(text) && text[prefixLen] >= '0' && text[prefixLen] <= '9' {
		prefixLen++
	}
	if prefixLen == 0 || prefixLen >= len(text) {
		return "", "", false
	}
	if text[prefixLen] != '.' && text[prefixLen] != ')' {
		return "", "", false
	}
	markerEnd := prefixLen + 1
	if markerEnd >= len(text) || text[markerEnd] != ' ' {
		return "", "", false
	}
	return text[:markerEnd], strings.TrimSpace(text[markerEnd+1:]), true
}

func sanitizeCreateText(text string, fontPlan createFontPlan) (string, bool) {
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
			continue
		}

		if createRuneWritable(r, fontPlan) {
			b.WriteRune(r)
			continue
		}

		if replacement, ok := createRuneFallback(r); ok {
			if createStringWritable(replacement, fontPlan) {
				b.WriteString(replacement)
				if !(replacement == "" && createRuneIsSilentFormattingDrop(r)) {
					replaced = true
				}
				continue
			}
		}

		if r >= 32 && r <= 126 {
			b.WriteRune(r)
			continue
		}

		if fontPlan.hasUnicodeFont() && unicode.IsGraphic(r) && fontPlan.supportsRune(r) {
			b.WriteRune(r)
			continue
		}

		replaced = true
		b.WriteByte('?')
	}

	collapsed := strings.Join(strings.Fields(b.String()), " ")
	return strings.TrimSpace(collapsed), replaced
}

func createRuneWritable(r rune, fontPlan createFontPlan) bool {
	if r < 32 || r == 127 {
		return false
	}
	if r <= 126 {
		return true
	}
	if !fontPlan.hasUnicodeFont() || !unicode.IsGraphic(r) {
		return false
	}
	return fontPlan.supportsRune(r)
}

func createStringWritable(text string, fontPlan createFontPlan) bool {
	for _, r := range text {
		if !createRuneWritable(r, fontPlan) {
			return false
		}
	}
	return true
}

func createRuneFallback(r rune) (string, bool) {
	switch r {
	case '“', '”', '„', '‟':
		return `"`, true
	case '‘', '’', '‚', '‛':
		return `'`, true
	case '—', '–', '−':
		return "-", true
	case '•':
		return "-", true
	case '⚠':
		return "[!]", true
	case '☑':
		return "[x]", true
	case '☐':
		return "[ ]", true
	case '…':
		return "...", true
	case '\u200b', '\u200c', '\u200d', '\ufe0f':
		return "", true
	default:
		if createRuneShouldDropWhenUnsupported(r) {
			return "", true
		}
		return "", false
	}
}

func createRuneShouldDropWhenUnsupported(r rune) bool {
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Cf, r) {
		return true
	}
	switch {
	case r >= 0x1F000 && r <= 0x1FAFF:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		return true
	default:
		return false
	}
}

func createRuneIsSilentFormattingDrop(r rune) bool {
	switch r {
	case '\u200b', '\u200c', '\u200d', '\ufe0f':
		return true
	default:
		return false
	}
}

func renderCreatePDF(lines []createStyledLine, fontPlan createFontPlan) ([]byte, int, int, error) {
	doc := fpdf.NewCustom(&fpdf.InitType{
		OrientationStr: "P",
		UnitStr:        "pt",
		Size: fpdf.SizeType{
			Wd: createPageWidth,
			Ht: createPageHeight,
		},
	})
	doc.SetCompression(false)
	doc.SetMargins(createMarginLeft, createMarginTop, createMarginLeft)
	doc.SetAutoPageBreak(true, createMarginBottom)
	doc.SetCreator("ZimaOS Blue native_pdf_ir", true)
	if fontPlan.hasUnicodeFont() {
		doc.AddUTF8FontFromBytes(createUnicodeFontFamily, "", fontPlan.unicodeBytes)
		doc.AddUTF8FontFromBytes(createUnicodeFontFamily, "B", fontPlan.unicodeBytes)
	}
	doc.AddPage()

	lineCount := 0
	textWidth := createPageWidth - (createMarginLeft * 2)

	for _, line := range lines {
		if line.Table != nil {
			renderedLines, err := renderCreateTable(doc, line.Table, textWidth, fontPlan)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}
		if line.Divider != nil {
			renderedLines, err := renderCreateDivider(doc, line.Divider, textWidth)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}
		if line.ListItem != nil {
			renderedLines, err := renderCreateListItem(doc, line.ListItem, line.Font, line.FontSize, textWidth, fontPlan, line.ColorR, line.ColorG, line.ColorB)
			if err != nil {
				return nil, 0, 0, err
			}
			lineCount += renderedLines
			if line.GapAfter > 0 {
				doc.Ln(line.GapAfter)
			}
			continue
		}

		if strings.TrimSpace(line.Text) == "" {
			doc.Ln(createBlankLineHeight + line.GapAfter)
			continue
		}

		family, style := fontPlan.fontFor(line.Font)
		doc.SetFont(family, style, line.FontSize)
		doc.SetTextColor(line.ColorR, line.ColorG, line.ColorB)
		align := line.Align
		if align == "" {
			align = "L"
		}
		wrapped := createSplitText(doc, line.Text, textWidth)
		if len(wrapped) == 0 {
			wrapped = []string{line.Text}
		}
		lineCount += len(wrapped)
		for _, segment := range wrapped {
			if align == "R" && createHasArabicLetters(segment) {
				segment = createShapeArabicVisual(segment)
			}
			doc.CellFormat(textWidth, line.FontSize*createLineHeightScale, segment, "", 2, align, false, 0, "")
		}
		if line.GapAfter > 0 {
			doc.Ln(line.GapAfter)
		}
	}

	var out bytes.Buffer
	if err := doc.Output(&out); err != nil {
		return nil, 0, 0, err
	}
	return out.Bytes(), doc.PageNo(), lineCount, nil
}

func renderCreateDivider(doc *fpdf.Fpdf, divider *createStyledDivider, textWidth float64) (int, error) {
	if doc == nil || divider == nil {
		return 0, nil
	}

	lineY := doc.GetY() + 3
	if lineY > createPageHeight-createMarginBottom {
		doc.AddPage()
		lineY = doc.GetY() + 3
	}

	left := createMarginLeft + createDividerInset
	right := createMarginLeft + textWidth - createDividerInset
	if right <= left {
		left = createMarginLeft
		right = createMarginLeft + textWidth
	}

	doc.SetDrawColor(divider.LineR, divider.LineG, divider.LineB)
	doc.SetLineWidth(createDividerLineWidth)
	doc.Line(left, lineY, right, lineY)
	doc.SetXY(createMarginLeft, lineY)
	return 1, nil
}

func renderCreateListItem(doc *fpdf.Fpdf, item *createStyledListItem, fontID string, fontSize float64, textWidth float64, fontPlan createFontPlan, textR, textG, textB int) (int, error) {
	if doc == nil || item == nil {
		return 0, nil
	}

	family, style := fontPlan.fontFor(fontID)
	doc.SetFont(family, style, fontSize)
	doc.SetTextColor(textR, textG, textB)

	lineHeight := fontSize * createLineHeightScale
	if doc.GetY()+lineHeight > createPageHeight-createMarginBottom {
		doc.AddPage()
	}

	marker := strings.TrimSpace(item.Marker)
	if marker == "" {
		marker = "-"
	}
	markerWidth := doc.GetStringWidth(marker)
	if markerWidth < createListMinMarkerWidth {
		markerWidth = createListMinMarkerWidth
	}
	markerWidth += createListMarkerGap

	contentWidth := textWidth - markerWidth
	if contentWidth < 48 {
		contentWidth = textWidth - (createListMinMarkerWidth + createListMarkerGap)
		markerWidth = createListMinMarkerWidth + createListMarkerGap
	}

	segments := createSplitText(doc, item.Text, contentWidth)
	if len(segments) == 0 {
		segments = []string{item.Text}
	}

	align := item.Align
	if align == "" {
		align = "L"
	}
	startX := createMarginLeft

	renderSegment := func(segment string) {
		if align == "R" && createHasArabicLetters(segment) {
			segment = createShapeArabicVisual(segment)
		}
		doc.CellFormat(contentWidth, lineHeight, segment, "", 2, align, false, 0, "")
	}

	doc.SetX(startX)
	doc.CellFormat(markerWidth, lineHeight, marker, "", 0, "R", false, 0, "")
	renderSegment(segments[0])

	for _, segment := range segments[1:] {
		doc.SetX(startX + markerWidth)
		renderSegment(segment)
	}

	return len(segments), nil
}

func renderCreateTable(doc *fpdf.Fpdf, table *createStyledTable, textWidth float64, fontPlan createFontPlan) (int, error) {
	if table == nil {
		return 0, nil
	}
	columnCount := createTableColumnCount(table)
	if columnCount == 0 {
		return 0, nil
	}

	widths := createTableColumnWidths(columnCount, textWidth)
	lineCount := 0

	renderHeader := func() error {
		if len(table.Headers) == 0 {
			return nil
		}
		count, fits, err := renderCreateTableRow(doc, table.Headers, widths, createFontBold, table.FontSize, fontPlan, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
		if err != nil {
			return err
		}
		if !fits {
			doc.AddPage()
			count, fits, err = renderCreateTableRow(doc, table.Headers, widths, createFontBold, table.FontSize, fontPlan, table.HeaderTextR, table.HeaderTextG, table.HeaderTextB, table.HeaderFillR, table.HeaderFillG, table.HeaderFillB, table.BorderR, table.BorderG, table.BorderB, true)
			if err != nil {
				return err
			}
			if !fits {
				return fmt.Errorf("pdf table header too tall to fit on a single page")
			}
		}
		lineCount += count
		return nil
	}

	if err := renderHeader(); err != nil {
		return 0, err
	}

	for rowIndex, row := range table.Rows {
		fill := rowIndex%2 == 0
		count, fits, err := renderCreateTableRow(doc, row, widths, createFontRegular, table.FontSize, fontPlan, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
		if err != nil {
			return 0, err
		}
		if !fits {
			doc.AddPage()
			if err := renderHeader(); err != nil {
				return 0, err
			}
			count, fits, err = renderCreateTableRow(doc, row, widths, createFontRegular, table.FontSize, fontPlan, table.BodyTextR, table.BodyTextG, table.BodyTextB, table.RowFillR, table.RowFillG, table.RowFillB, table.BorderR, table.BorderG, table.BorderB, fill)
			if err != nil {
				return 0, err
			}
			if !fits {
				return 0, fmt.Errorf("pdf table row too tall to fit on a single page")
			}
		}
		lineCount += count
	}

	return lineCount, nil
}

func renderCreateTableRow(doc *fpdf.Fpdf, cells []createStyledTableCell, widths []float64, fontID string, fontSize float64, fontPlan createFontPlan, textR, textG, textB int, fillR, fillG, fillB int, borderR, borderG, borderB int, fill bool) (int, bool, error) {
	family, style := fontPlan.fontFor(fontID)
	doc.SetFont(family, style, fontSize)

	lineHeight := fontSize * createTableLineHeightScale
	innerWidths := make([]float64, len(widths))
	wrapped := make([][]string, len(widths))
	maxLines := 1

	for idx, width := range widths {
		innerWidth := width - (createTableCellPaddingX * 2)
		if innerWidth < 12 {
			innerWidth = width
		}
		innerWidths[idx] = innerWidth

		text := ""
		if idx < len(cells) {
			text = cells[idx].Text
		}
		segments := createSplitText(doc, text, innerWidth)
		if len(segments) == 0 {
			segments = []string{""}
		}
		wrapped[idx] = segments
		if len(segments) > maxLines {
			maxLines = len(segments)
		}
	}

	rowHeight := createTableCellPaddingY*2 + float64(maxLines)*lineHeight
	if doc.GetY()+rowHeight > createPageHeight-createMarginBottom {
		return 0, false, nil
	}

	doc.SetLineWidth(createTableBorderWidth)
	startX := createMarginLeft
	startY := doc.GetY()
	x := startX

	for idx, width := range widths {
		doc.SetDrawColor(borderR, borderG, borderB)
		if fill {
			doc.SetFillColor(fillR, fillG, fillB)
			doc.Rect(x, startY, width, rowHeight, "DF")
		} else {
			doc.Rect(x, startY, width, rowHeight, "D")
		}

		align := "L"
		if idx < len(cells) && strings.TrimSpace(cells[idx].Align) != "" {
			align = cells[idx].Align
		}

		doc.SetXY(x+createTableCellPaddingX, startY+createTableCellPaddingY)
		doc.SetTextColor(textR, textG, textB)

		for _, segment := range wrapped[idx] {
			if align == "R" && createHasArabicLetters(segment) {
				segment = createShapeArabicVisual(segment)
			}
			doc.CellFormat(innerWidths[idx], lineHeight, segment, "", 2, align, false, 0, "")
		}

		x += width
		doc.SetXY(x, startY)
	}

	doc.SetXY(createMarginLeft, startY+rowHeight)
	return maxLines, true, nil
}

func createTableColumnCount(table *createStyledTable) int {
	if table == nil {
		return 0
	}
	count := len(table.Headers)
	for _, row := range table.Rows {
		if len(row) > count {
			count = len(row)
		}
	}
	return count
}

func createTableColumnWidths(columnCount int, totalWidth float64) []float64 {
	widths := make([]float64, columnCount)
	if columnCount == 0 {
		return widths
	}
	baseWidth := totalWidth / float64(columnCount)
	remaining := totalWidth
	for idx := range widths {
		width := baseWidth
		if idx == len(widths)-1 {
			width = remaining
		} else {
			remaining -= width
		}
		widths[idx] = width
	}
	return widths
}

func createSplitText(doc *fpdf.Fpdf, text string, width float64) []string {
	if doc == nil {
		return nil
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	runes := []rune(text)
	for len(runes) > 0 && runes[len(runes)-1] == '\n' {
		runes = runes[:len(runes)-1]
	}
	if len(runes) == 0 {
		return nil
	}

	maxWidth := width - 2*doc.GetCellMargin()
	if maxWidth <= 0 {
		maxWidth = width
	}

	lines := make([]string, 0, 4)
	start := 0
	lastSpaceBreak := -1
	lastCJKBreak := -1
	lineWidth := 0.0

	for idx := 0; idx < len(runes); {
		r := runes[idx]
		if r == '\n' {
			lines = append(lines, string(runes[start:idx]))
			idx++
			start = idx
			lastSpaceBreak = -1
			lastCJKBreak = -1
			lineWidth = 0
			continue
		}

		lineWidth += doc.GetStringWidth(string(r))
		if unicode.IsSpace(r) {
			lastSpaceBreak = idx
		} else if createSplitTextTreatsRuneAsCJK(r) {
			lastCJKBreak = idx
		}

		if lineWidth > maxWidth {
			lastBreak := lastSpaceBreak
			if lastBreak < start {
				lastBreak = lastCJKBreak
			}
			if lastBreak >= start {
				end, next := createSplitTextBreakRange(runes, start, idx, lastBreak)
				if end == start {
					end = idx + 1
					next = end
				}
				lines = append(lines, string(runes[start:end]))
				start = next
				idx = start
			} else {
				if idx == start {
					idx++
				}
				lines = append(lines, string(runes[start:idx]))
				start = idx
			}
			lastSpaceBreak = -1
			lastCJKBreak = -1
			lineWidth = 0
			continue
		}

		idx++
	}

	if start < len(runes) {
		lines = append(lines, string(runes[start:]))
	}
	return lines
}

func createSplitTextBreakRange(runes []rune, start, current, lastBreak int) (int, int) {
	if lastBreak < start || lastBreak >= len(runes) {
		return current, current
	}
	breakRune := runes[lastBreak]
	if unicode.IsSpace(breakRune) {
		return lastBreak, lastBreak + 1
	}
	if lastBreak == current {
		return lastBreak, lastBreak
	}
	return lastBreak + 1, lastBreak + 1
}

func createSplitTextTreatsRuneAsCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FA5
}

func createColorRGB(raw string) (int, int, int) {
	value := strings.TrimSpace(strings.TrimPrefix(raw, "#"))
	if len(value) != 6 {
		return 17, 24, 39
	}
	var rgb [3]int
	for idx := 0; idx < 3; idx++ {
		component := value[idx*2 : idx*2+2]
		var parsed int
		if _, err := fmt.Sscanf(component, "%02X", &parsed); err != nil {
			if _, err := fmt.Sscanf(strings.ToUpper(component), "%02X", &parsed); err != nil {
				return 17, 24, 39
			}
		}
		rgb[idx] = parsed
	}
	return rgb[0], rgb[1], rgb[2]
}

func createBlendRGB(raw string, whiteMix float64) (int, int, int) {
	if whiteMix < 0 {
		whiteMix = 0
	}
	if whiteMix > 1 {
		whiteMix = 1
	}
	r, g, b := createColorRGB(raw)
	blend := func(component int) int {
		return int(float64(component)*(1-whiteMix) + 255*whiteMix)
	}
	return blend(r), blend(g), blend(b)
}
