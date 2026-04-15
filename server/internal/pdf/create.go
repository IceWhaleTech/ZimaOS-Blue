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
	Align     string
	Font      string
	FontSize  float64
	GapAfter  float64
	CharCount int
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

	appendLine := func(text, font string, fontSize, gapAfter float64) {
		sanitized, replaced := sanitizeCreateText(text, fontPlan)
		if sanitized == "" {
			return
		}
		replacedRunes = replacedRunes || replaced
		lines = append(lines, createStyledLine{
			Text:      sanitized,
			Align:     createTextAlign(sanitized),
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
				replaced = true
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
	case '…':
		return "...", true
	default:
		return "", false
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
		if strings.TrimSpace(line.Text) == "" {
			doc.Ln(createBlankLineHeight + line.GapAfter)
			continue
		}

		family, style := fontPlan.fontFor(line.Font)
		doc.SetFont(family, style, line.FontSize)
		align := line.Align
		if align == "" {
			align = "L"
		}
		wrapped := doc.SplitText(line.Text, textWidth)
		if len(wrapped) == 0 {
			wrapped = []string{line.Text}
		}
		lineCount += len(wrapped)
		if align == "R" && createHasArabicLetters(line.Text) {
			for _, segment := range wrapped {
				doc.CellFormat(textWidth, line.FontSize*createLineHeightScale, createShapeArabicVisual(segment), "", 2, align, false, 0, "")
			}
		} else {
			doc.MultiCell(textWidth, line.FontSize*createLineHeightScale, line.Text, "", align, false)
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
