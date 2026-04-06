package pdf

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
)

const (
	headerFooterZoneRatio = 0.18
	tableColumnTolerance  = 18.0
	lineMergeGapPoints    = 14.0
)

var headerFooterDigitPattern = regexp.MustCompile(`\d+`)

type BBox struct {
	Left   float64 `json:"left"`
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
}

type OutlineEntry struct {
	Title      string `json:"title"`
	Level      int    `json:"level"`
	PageNumber int    `json:"page_number,omitempty"`
}

type PageBlock struct {
	ID           int         `json:"id,omitempty"`
	Kind         string      `json:"kind"`
	PageNumber   int         `json:"page_number"`
	BBox         BBox        `json:"bbox"`
	Text         string      `json:"text,omitempty"`
	Markdown     string      `json:"markdown,omitempty"`
	HeadingLevel int         `json:"heading_level,omitempty"`
	Children     []PageBlock `json:"children,omitempty"`
}

type PageTableCell struct {
	RowNumber    int    `json:"row_number"`
	ColumnNumber int    `json:"column_number"`
	RowSpan      int    `json:"row_span"`
	ColumnSpan   int    `json:"column_span"`
	BBox         BBox   `json:"bbox"`
	Text         string `json:"text,omitempty"`
}

type PageTableRow struct {
	RowNumber int             `json:"row_number"`
	Cells     []PageTableCell `json:"cells"`
}

type PageTable struct {
	ID              int            `json:"id,omitempty"`
	PageNumber      int            `json:"page_number"`
	BBox            BBox           `json:"bbox"`
	NumberOfRows    int            `json:"number_of_rows"`
	NumberOfColumns int            `json:"number_of_columns"`
	Markdown        string         `json:"markdown,omitempty"`
	Rows            []PageTableRow `json:"rows"`
}

type pdfPageGeometry struct {
	Width  float64
	Height float64
}

type pdfStructuredRect struct {
	Text       string
	BBox       BBox
	FontSize   float64
	FontWeight int
	FontName   string
}

type pdfStructuredLine struct {
	Fragments  []pdfStructuredRect
	BBox       BBox
	Text       string
	RawText    string
	FontSize   float64
	FontWeight int
	FontName   string
}

type pdfStructuredPage struct {
	PageNumber int
	Geometry   pdfPageGeometry
	Lines      []*pdfStructuredLine
	RawText    string
	Text       string
}

type pdfPageSource struct {
	PageNumber   int
	Geometry     pdfPageGeometry
	Structured   *pdfStructuredPage
	RawText      string
	Text         string
	Source       string
	Warnings     []string
	OCRResult    ocrruntime.Result
	VisionResult VisionResult
	OCRUsed      bool
	VisionUsed   bool
}

type pdfPresentation struct {
	Text        string
	Markdown    string
	Blocks      []PageBlock
	Tables      []PageTable
	HeadingText []OutlineEntry
}

type pdfFontProfile struct {
	BodySize     float64
	HeadingSizes []float64
	DefaultLevel int
}

type pdfLayoutContext struct {
	headers map[string]struct{}
	footers map[string]struct{}
	fonts   pdfFontProfile
}

func buildStructuredLines(rects []pdfStructuredRect) []*pdfStructuredLine {
	if len(rects) == 0 {
		return nil
	}
	sort.SliceStable(rects, func(i, j int) bool {
		if nearlyEqual(rects[i].BBox.Top, rects[j].BBox.Top, 1.5) {
			return rects[i].BBox.Left < rects[j].BBox.Left
		}
		return rects[i].BBox.Top < rects[j].BBox.Top
	})

	lines := make([]*pdfStructuredLine, 0, len(rects))
	for _, rect := range rects {
		assigned := false
		for i := len(lines) - 1; i >= 0; i-- {
			line := lines[i]
			if shouldJoinStructuredLine(line, rect) {
				line.Fragments = append(line.Fragments, rect)
				line.BBox = line.BBox.union(rect.BBox)
				assigned = true
				break
			}
			if rect.BBox.Top-line.BBox.Bottom > lineMergeGapPoints {
				break
			}
		}
		if !assigned {
			lines = append(lines, &pdfStructuredLine{
				Fragments: []pdfStructuredRect{rect},
				BBox:      rect.BBox,
			})
		}
	}

	for _, line := range lines {
		sort.SliceStable(line.Fragments, func(i, j int) bool {
			return line.Fragments[i].BBox.Left < line.Fragments[j].BBox.Left
		})
		line.RawText = buildStructuredLineRawText(line.Fragments)
		line.Text = normalizePDFLine(line.RawText)
		line.FontSize = dominantFontSize(line.Fragments)
		line.FontWeight = dominantFontWeight(line.Fragments)
		line.FontName = dominantFontName(line.Fragments)
		if strings.TrimSpace(line.Text) == "" {
			line.Text = normalizeText(line.RawText)
		}
	}

	sort.SliceStable(lines, func(i, j int) bool {
		if nearlyEqual(lines[i].BBox.Top, lines[j].BBox.Top, 2.0) {
			return lines[i].BBox.Left < lines[j].BBox.Left
		}
		return lines[i].BBox.Top < lines[j].BBox.Top
	})
	return sortStructuredLinesReadingOrder(lines)
}

func shouldJoinStructuredLine(line *pdfStructuredLine, rect pdfStructuredRect) bool {
	if line == nil {
		return false
	}
	lineMid := (line.BBox.Top + line.BBox.Bottom) / 2
	rectMid := (rect.BBox.Top + rect.BBox.Bottom) / 2
	threshold := math.Max(3, math.Max(line.BBox.height(), rect.BBox.height())*0.7)
	if math.Abs(lineMid-rectMid) <= threshold {
		return true
	}
	return bboxVerticalOverlap(line.BBox, rect.BBox) >= 0.45
}

func buildStructuredPageRawText(lines []*pdfStructuredLine) string {
	if len(lines) == 0 {
		return ""
	}
	rawLines := make([]string, 0, len(lines))
	var prev *pdfStructuredLine
	for _, line := range lines {
		if line == nil || strings.TrimSpace(line.RawText) == "" {
			continue
		}
		if prev != nil {
			gap := line.BBox.Top - prev.BBox.Bottom
			if gap > math.Max(lineMergeGapPoints, prev.BBox.height()*1.4) {
				rawLines = append(rawLines, "")
			}
		}
		rawLines = append(rawLines, strings.TrimSpace(line.RawText))
		prev = line
	}
	return strings.TrimSpace(strings.Join(rawLines, "\n"))
}

func buildStructuredLineRawText(fragments []pdfStructuredRect) string {
	if len(fragments) == 0 {
		return ""
	}
	var builder strings.Builder
	for idx, fragment := range fragments {
		if idx > 0 {
			prev := fragments[idx-1]
			builder.WriteString(strings.Repeat(" ", structuredGapSpaces(prev, fragment)))
		}
		builder.WriteString(strings.TrimSpace(fragment.Text))
	}
	return strings.TrimSpace(builder.String())
}

func structuredGapSpaces(prev, next pdfStructuredRect) int {
	fontSize := math.Max(prev.FontSize, next.FontSize)
	if fontSize <= 0 {
		fontSize = 12
	}
	gap := next.BBox.Left - prev.BBox.Right
	switch {
	case gap <= fontSize*0.35:
		return 0
	case gap <= fontSize*1.1:
		return 1
	case gap <= fontSize*2.0:
		return 3
	default:
		return 5
	}
}

func sortStructuredLinesReadingOrder(lines []*pdfStructuredLine) []*pdfStructuredLine {
	if len(lines) <= 1 {
		return lines
	}
	bounds := unionLineBounds(lines)
	pageWidth := bounds.width()
	if pageWidth <= 0 {
		pageWidth = 1
	}

	wide := make([]*pdfStructuredLine, 0, len(lines))
	narrow := make([]*pdfStructuredLine, 0, len(lines))
	for _, line := range lines {
		if line == nil {
			continue
		}
		if line.BBox.width() >= pageWidth*0.72 {
			wide = append(wide, line)
			continue
		}
		narrow = append(narrow, line)
	}
	if len(wide) == 0 || len(narrow) == 0 {
		return sortStructuredLinesSimple(lines)
	}

	sort.SliceStable(wide, func(i, j int) bool { return wide[i].BBox.Top < wide[j].BBox.Top })
	used := make(map[*pdfStructuredLine]struct{}, len(narrow))
	ordered := make([]*pdfStructuredLine, 0, len(lines))
	for _, wideLine := range wide {
		segment := make([]*pdfStructuredLine, 0)
		for _, line := range narrow {
			if _, ok := used[line]; ok {
				continue
			}
			if line.BBox.Top < wideLine.BBox.Top {
				segment = append(segment, line)
				used[line] = struct{}{}
			}
		}
		ordered = append(ordered, sortStructuredColumns(segment)...)
		ordered = append(ordered, wideLine)
	}
	remaining := make([]*pdfStructuredLine, 0)
	for _, line := range narrow {
		if _, ok := used[line]; ok {
			continue
		}
		remaining = append(remaining, line)
	}
	ordered = append(ordered, sortStructuredColumns(remaining)...)
	return ordered
}

func sortStructuredColumns(lines []*pdfStructuredLine) []*pdfStructuredLine {
	if len(lines) <= 1 {
		return sortStructuredLinesSimple(lines)
	}
	sorted := make([]*pdfStructuredLine, 0, len(lines))
	for _, line := range lines {
		if line != nil {
			sorted = append(sorted, line)
		}
	}
	if len(sorted) <= 1 {
		return sorted
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].BBox.centerX() < sorted[j].BBox.centerX()
	})

	bestGap := 0.0
	bestIndex := -1
	for i := 0; i < len(sorted)-1; i++ {
		gap := sorted[i+1].BBox.centerX() - sorted[i].BBox.centerX()
		if gap > bestGap {
			bestGap = gap
			bestIndex = i
		}
	}
	bounds := unionLineBounds(sorted)
	if bestGap < math.Max(bounds.width()*0.12, 36) || bestIndex < 0 {
		return sortStructuredRows(sorted)
	}

	left := append([]*pdfStructuredLine(nil), sorted[:bestIndex+1]...)
	right := append([]*pdfStructuredLine(nil), sorted[bestIndex+1:]...)
	if len(left) == 0 || len(right) == 0 {
		return sortStructuredRows(sorted)
	}

	left = sortStructuredRows(left)
	right = sortStructuredRows(right)
	return append(left, right...)
}

func sortStructuredRows(lines []*pdfStructuredLine) []*pdfStructuredLine {
	if len(lines) <= 1 {
		return sortStructuredLinesSimple(lines)
	}
	sorted := sortStructuredLinesSimple(lines)
	bestGap := 0.0
	bestIndex := -1
	for i := 0; i < len(sorted)-1; i++ {
		gap := sorted[i+1].BBox.Top - sorted[i].BBox.Bottom
		if gap > bestGap {
			bestGap = gap
			bestIndex = i
		}
	}
	if bestGap < math.Max(medianLineHeight(sorted)*1.4, 18) || bestIndex < 0 {
		return sorted
	}
	top := append([]*pdfStructuredLine(nil), sorted[:bestIndex+1]...)
	bottom := append([]*pdfStructuredLine(nil), sorted[bestIndex+1:]...)
	return append(sortStructuredRows(top), sortStructuredRows(bottom)...)
}

func sortStructuredLinesSimple(lines []*pdfStructuredLine) []*pdfStructuredLine {
	out := make([]*pdfStructuredLine, 0, len(lines))
	for _, line := range lines {
		if line != nil {
			out = append(out, line)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if nearlyEqual(out[i].BBox.Top, out[j].BBox.Top, 2.0) {
			return out[i].BBox.Left < out[j].BBox.Left
		}
		return out[i].BBox.Top < out[j].BBox.Top
	})
	return out
}

func buildPDFLayoutContext(sources []pdfPageSource) pdfLayoutContext {
	ctx := pdfLayoutContext{
		headers: map[string]struct{}{},
		footers: map[string]struct{}{},
		fonts:   buildPDFFontProfile(sources),
	}
	if len(sources) < 2 {
		return ctx
	}

	headerCounts := make(map[string]int)
	footerCounts := make(map[string]int)
	for _, source := range sources {
		if source.Structured == nil || len(source.Structured.Lines) == 0 {
			continue
		}
		lines := sortStructuredLinesSimple(source.Structured.Lines)
		for _, line := range candidateHeaderLines(lines, source.Geometry) {
			key := headerFooterKey(line.Text)
			if key != "" {
				headerCounts[key]++
			}
		}
		for _, line := range candidateFooterLines(lines, source.Geometry) {
			key := headerFooterKey(line.Text)
			if key != "" {
				footerCounts[key]++
			}
		}
	}
	for key, count := range headerCounts {
		if count >= 2 {
			ctx.headers[key] = struct{}{}
		}
	}
	for key, count := range footerCounts {
		if count >= 2 {
			ctx.footers[key] = struct{}{}
		}
	}
	return ctx
}

func buildPDFFontProfile(sources []pdfPageSource) pdfFontProfile {
	sizes := make([]float64, 0, 64)
	for _, source := range sources {
		if source.Structured == nil {
			continue
		}
		for _, line := range source.Structured.Lines {
			if line == nil || line.FontSize <= 0 || strings.TrimSpace(line.Text) == "" {
				continue
			}
			sizes = append(sizes, line.FontSize)
		}
	}
	if len(sizes) == 0 {
		return pdfFontProfile{BodySize: 12, DefaultLevel: 2}
	}
	sort.Float64s(sizes)
	body := sizes[len(sizes)/2]
	headingSizes := make([]float64, 0, 4)
	for i := len(sizes) - 1; i >= 0; i-- {
		size := sizes[i]
		if size < body*1.12 {
			break
		}
		if len(headingSizes) == 0 || !nearlyEqual(headingSizes[len(headingSizes)-1], size, 0.75) {
			headingSizes = append(headingSizes, size)
		}
	}
	return pdfFontProfile{BodySize: body, HeadingSizes: headingSizes, DefaultLevel: 2}
}

func buildPDFPresentation(source pdfPageSource, ctx pdfLayoutContext, includeHeadersFooters bool) pdfPresentation {
	if source.Structured == nil || len(source.Structured.Lines) == 0 || source.Source != "text" {
		return buildSyntheticPDFPresentation(source)
	}
	lines := source.Structured.Lines
	filtered := make([]*pdfStructuredLine, 0, len(lines))
	headerLines := make([]*pdfStructuredLine, 0, 2)
	footerLines := make([]*pdfStructuredLine, 0, 2)
	for _, line := range lines {
		if line == nil || strings.TrimSpace(line.Text) == "" {
			continue
		}
		if ctx.isHeader(line, source.Geometry) {
			headerLines = append(headerLines, line)
			continue
		}
		if ctx.isFooter(line, source.Geometry) {
			footerLines = append(footerLines, line)
			continue
		}
		filtered = append(filtered, line)
	}
	if len(filtered) == 0 && (len(headerLines) > 0 || len(footerLines) > 0) {
		filtered = append(filtered, headerLines...)
		filtered = append(filtered, footerLines...)
	}

	presentation := pdfPresentation{}
	nextID := 1
	appendBlock := func(block PageBlock) {
		if strings.TrimSpace(block.Text) == "" && strings.TrimSpace(block.Markdown) == "" && len(block.Children) == 0 {
			return
		}
		if block.ID == 0 {
			block.ID = nextID
			nextID++
		}
		presentation.Blocks = append(presentation.Blocks, block)
	}

	if includeHeadersFooters && len(headerLines) > 0 {
		appendBlock(buildPlainBlock("header", source.PageNumber, headerLines, 0))
	}

	for i := 0; i < len(filtered); {
		if table, consumed, ok := buildPDFTable(source.PageNumber, filtered, i, nextID); ok {
			table.ID = nextID
			nextID++
			presentation.Tables = append(presentation.Tables, table)
			appendBlock(PageBlock{
				ID:         table.ID,
				Kind:       "table",
				PageNumber: source.PageNumber,
				BBox:       table.BBox,
				Text:       plainTextFromTable(table),
				Markdown:   table.Markdown,
			})
			i += consumed
			continue
		}
		if isPDFListLikeLine(filtered[i].Text) {
			block, consumed := buildPDFListBlock(source.PageNumber, filtered[i:])
			block.ID = nextID
			nextID++
			appendBlock(block)
			i += consumed
			continue
		}
		if level := ctx.headingLevel(filtered[i]); level > 0 {
			block := buildPlainBlock("heading", source.PageNumber, []*pdfStructuredLine{filtered[i]}, level)
			block.ID = nextID
			nextID++
			appendBlock(block)
			presentation.HeadingText = append(presentation.HeadingText, OutlineEntry{
				Title:      block.Text,
				Level:      level,
				PageNumber: source.PageNumber,
			})
			i++
			continue
		}
		block, consumed := buildPDFParagraphBlock(source.PageNumber, filtered[i:], ctx)
		block.ID = nextID
		nextID++
		appendBlock(block)
		i += consumed
	}

	if includeHeadersFooters && len(footerLines) > 0 {
		appendBlock(buildPlainBlock("footer", source.PageNumber, footerLines, 0))
	}

	presentation.Text = plainTextFromBlocks(presentation.Blocks)
	presentation.Markdown = markdownFromBlocks(presentation.Blocks)
	return presentation
}

func buildSyntheticPDFPresentation(source pdfPageSource) pdfPresentation {
	lines := splitSyntheticLines(source.Text)
	if len(lines) == 0 {
		return pdfPresentation{}
	}
	presentation := pdfPresentation{}
	nextID := 1
	for i := 0; i < len(lines); {
		line := lines[i]
		if isPDFListLikeLine(line) {
			items := make([]PageBlock, 0, 4)
			markdownLines := make([]string, 0, 4)
			for i < len(lines) && isPDFListLikeLine(lines[i]) {
				item := PageBlock{
					ID:         nextID,
					Kind:       "list_item",
					PageNumber: source.PageNumber,
					Text:       normalizePDFListPrefix(lines[i]),
					Markdown:   normalizePDFListPrefix(lines[i]),
				}
				nextID++
				items = append(items, item)
				markdownLines = append(markdownLines, item.Markdown)
				i++
			}
			presentation.Blocks = append(presentation.Blocks, PageBlock{
				ID:         nextID,
				Kind:       "list",
				PageNumber: source.PageNumber,
				Text:       strings.Join(markdownLines, "\n"),
				Markdown:   strings.Join(markdownLines, "\n"),
				Children:   items,
			})
			nextID++
			continue
		}
		level := 0
		if isPDFHeadingLikeLine(line) {
			level = 2
		}
		kind := "paragraph"
		markdown := line
		if level > 0 {
			kind = "heading"
			markdown = strings.Repeat("#", level) + " " + line
			presentation.HeadingText = append(presentation.HeadingText, OutlineEntry{
				Title:      line,
				Level:      level,
				PageNumber: source.PageNumber,
			})
		}
		presentation.Blocks = append(presentation.Blocks, PageBlock{
			ID:           nextID,
			Kind:         kind,
			PageNumber:   source.PageNumber,
			Text:         line,
			Markdown:     markdown,
			HeadingLevel: level,
		})
		nextID++
		i++
	}
	presentation.Text = plainTextFromBlocks(presentation.Blocks)
	presentation.Markdown = markdownFromBlocks(presentation.Blocks)
	return presentation
}

func buildPDFTable(pageNumber int, lines []*pdfStructuredLine, start int, nextID int) (PageTable, int, bool) {
	if start >= len(lines) || !looksLikeStructuredTableLine(lines[start]) {
		return PageTable{}, 0, false
	}
	group := make([]*pdfStructuredLine, 0, 4)
	for idx := start; idx < len(lines); idx++ {
		line := lines[idx]
		if !looksLikeStructuredTableLine(line) {
			break
		}
		if len(group) > 0 && !similarStructuredColumns(group[len(group)-1], line) {
			break
		}
		group = append(group, line)
	}
	if len(group) < 2 {
		return PageTable{}, 0, false
	}
	anchors := collectTableAnchors(group)
	if len(anchors) < 2 {
		return PageTable{}, 0, false
	}

	table := PageTable{
		PageNumber:      pageNumber,
		NumberOfRows:    len(group),
		NumberOfColumns: len(anchors),
	}
	for rowIdx, line := range group {
		row := PageTableRow{RowNumber: rowIdx + 1, Cells: make([]PageTableCell, 0, len(anchors))}
		cellTexts := make([]string, len(anchors))
		cellBoxes := make([]BBox, len(anchors))
		cellSet := make([]bool, len(anchors))
		for _, fragment := range line.Fragments {
			colIdx := closestAnchorIndex(fragment.BBox.Left, anchors)
			if colIdx < 0 {
				continue
			}
			if cellSet[colIdx] {
				cellTexts[colIdx] = strings.TrimSpace(cellTexts[colIdx] + " " + fragment.Text)
				cellBoxes[colIdx] = cellBoxes[colIdx].union(fragment.BBox)
			} else {
				cellTexts[colIdx] = strings.TrimSpace(fragment.Text)
				cellBoxes[colIdx] = fragment.BBox
				cellSet[colIdx] = true
			}
		}
		for colIdx := range anchors {
			cell := PageTableCell{
				RowNumber:    rowIdx + 1,
				ColumnNumber: colIdx + 1,
				RowSpan:      1,
				ColumnSpan:   1,
				BBox:         cellBoxes[colIdx],
				Text:         cellTexts[colIdx],
			}
			row.Cells = append(row.Cells, cell)
			table.BBox = table.BBox.union(cell.BBox)
		}
		table.Rows = append(table.Rows, row)
	}
	table.Markdown = markdownFromTable(table)
	return table, len(group), true
}

func buildPDFListBlock(pageNumber int, lines []*pdfStructuredLine) (PageBlock, int) {
	items := make([]PageBlock, 0, 4)
	textLines := make([]string, 0, 4)
	var blockBBox BBox
	consumed := 0
	for consumed < len(lines) {
		line := lines[consumed]
		if line == nil || !isPDFListLikeLine(line.Text) {
			break
		}
		itemText := normalizePDFListPrefix(line.Text)
		items = append(items, PageBlock{
			Kind:       "list_item",
			PageNumber: pageNumber,
			BBox:       line.BBox,
			Text:       itemText,
			Markdown:   itemText,
		})
		blockBBox = blockBBox.union(line.BBox)
		textLines = append(textLines, itemText)
		consumed++
	}
	return PageBlock{
		Kind:       "list",
		PageNumber: pageNumber,
		BBox:       blockBBox,
		Text:       strings.Join(textLines, "\n"),
		Markdown:   strings.Join(textLines, "\n"),
		Children:   items,
	}, max(1, consumed)
}

func buildPDFParagraphBlock(pageNumber int, lines []*pdfStructuredLine, ctx pdfLayoutContext) (PageBlock, int) {
	group := make([]*pdfStructuredLine, 0, 4)
	group = append(group, lines[0])
	consumed := 1
	for consumed < len(lines) {
		current := lines[consumed-1]
		next := lines[consumed]
		if ctx.headingLevel(next) > 0 || isPDFListLikeLine(next.Text) || looksLikeStructuredTableLine(next) {
			break
		}
		if paragraphGap(current, next) > math.Max(lineMergeGapPoints, current.BBox.height()*1.5) && !shouldMergePDFLines(current.Text, next.Text) {
			break
		}
		group = append(group, next)
		consumed++
	}
	return buildPlainBlock("paragraph", pageNumber, group, 0), consumed
}

func buildPlainBlock(kind string, pageNumber int, lines []*pdfStructuredLine, level int) PageBlock {
	textLines := make([]string, 0, len(lines))
	var box BBox
	for _, line := range lines {
		if line == nil || strings.TrimSpace(line.Text) == "" {
			continue
		}
		textLines = append(textLines, line.Text)
		box = box.union(line.BBox)
	}
	text := mergeTextLines(textLines)
	markdown := text
	if kind == "heading" && level > 0 {
		markdown = strings.Repeat("#", level) + " " + text
	}
	return PageBlock{
		Kind:         kind,
		PageNumber:   pageNumber,
		BBox:         box,
		Text:         text,
		Markdown:     markdown,
		HeadingLevel: level,
	}
}

func candidateHeaderLines(lines []*pdfStructuredLine, geometry pdfPageGeometry) []*pdfStructuredLine {
	if len(lines) == 0 {
		return nil
	}
	out := make([]*pdfStructuredLine, 0, 2)
	zone := geometry.Height * headerFooterZoneRatio
	if zone <= 0 {
		zone = 96
	}
	for _, line := range lines {
		if line == nil {
			continue
		}
		if line.BBox.centerY() <= zone || len(out) < 2 {
			out = append(out, line)
			if len(out) >= 2 && line.BBox.centerY() > zone {
				break
			}
		}
	}
	return out
}

func candidateFooterLines(lines []*pdfStructuredLine, geometry pdfPageGeometry) []*pdfStructuredLine {
	if len(lines) == 0 {
		return nil
	}
	out := make([]*pdfStructuredLine, 0, 2)
	zone := geometry.Height * (1 - headerFooterZoneRatio)
	if zone <= 0 {
		zone = math.Max(geometry.Height-96, 0)
	}
	for idx := len(lines) - 1; idx >= 0 && len(out) < 2; idx-- {
		line := lines[idx]
		if line == nil {
			continue
		}
		if line.BBox.centerY() >= zone || len(out) < 2 {
			out = append(out, line)
		}
	}
	return out
}

func headerFooterKey(text string) string {
	text = strings.TrimSpace(collapsePDFSpaces(strings.ToLower(text)))
	if text == "" {
		return ""
	}
	text = headerFooterDigitPattern.ReplaceAllString(text, "#")
	if utf8.RuneCountInString(text) < 3 {
		return ""
	}
	return text
}

func (ctx pdfLayoutContext) isHeader(line *pdfStructuredLine, geometry pdfPageGeometry) bool {
	if line == nil {
		return false
	}
	if line.BBox.centerY() > geometry.Height*headerFooterZoneRatio && geometry.Height > 0 {
		return false
	}
	_, ok := ctx.headers[headerFooterKey(line.Text)]
	return ok
}

func (ctx pdfLayoutContext) isFooter(line *pdfStructuredLine, geometry pdfPageGeometry) bool {
	if line == nil {
		return false
	}
	if geometry.Height > 0 && line.BBox.centerY() < geometry.Height*(1-headerFooterZoneRatio) {
		return false
	}
	_, ok := ctx.footers[headerFooterKey(line.Text)]
	return ok
}

func (ctx pdfLayoutContext) headingLevel(line *pdfStructuredLine) int {
	if line == nil || strings.TrimSpace(line.Text) == "" || isPDFListLikeLine(line.Text) || looksLikeStructuredTableLine(line) {
		return 0
	}
	bodySize := ctx.fonts.BodySize
	if bodySize <= 0 {
		bodySize = 12
	}
	if line.FontSize <= 0 && !isPDFHeadingLikeLine(line.Text) {
		return 0
	}
	if line.FontSize < bodySize*1.08 && line.FontWeight < 600 && !isPDFHeadingLikeLine(line.Text) {
		return 0
	}
	if !isPDFHeadingLikeLine(line.Text) && line.FontSize < bodySize*1.18 {
		return 0
	}
	for idx, size := range ctx.fonts.HeadingSizes {
		if nearlyEqual(size, line.FontSize, 0.75) {
			return minInt(idx+1, 6)
		}
	}
	if line.FontSize >= bodySize*1.45 {
		return 1
	}
	if line.FontSize >= bodySize*1.25 {
		return 2
	}
	return max(1, ctx.fonts.DefaultLevel)
}

func looksLikeStructuredTableLine(line *pdfStructuredLine) bool {
	if line == nil || isPDFListLikeLine(line.Text) {
		return false
	}
	if len(line.Fragments) < 2 {
		return false
	}
	wideGaps := 0
	for idx := 1; idx < len(line.Fragments); idx++ {
		gap := line.Fragments[idx].BBox.Left - line.Fragments[idx-1].BBox.Right
		if gap >= tableColumnTolerance*0.65 {
			wideGaps++
		}
	}
	return wideGaps >= 1 || looksLikePDFTableLine(line.RawText)
}

func similarStructuredColumns(prev, next *pdfStructuredLine) bool {
	if prev == nil || next == nil {
		return false
	}
	prevAnchors := lineAnchors(prev)
	nextAnchors := lineAnchors(next)
	if len(prevAnchors) < 2 || len(nextAnchors) < 2 {
		return false
	}
	if absInt(len(prevAnchors)-len(nextAnchors)) > 1 {
		return false
	}
	shared := minInt(len(prevAnchors), len(nextAnchors))
	matches := 0
	for idx := 0; idx < shared; idx++ {
		if math.Abs(prevAnchors[idx]-nextAnchors[idx]) <= tableColumnTolerance {
			matches++
		}
	}
	return matches >= minInt(2, shared)
}

func collectTableAnchors(lines []*pdfStructuredLine) []float64 {
	allAnchors := make([]float64, 0, 8)
	for _, line := range lines {
		allAnchors = append(allAnchors, lineAnchors(line)...)
	}
	if len(allAnchors) == 0 {
		return nil
	}
	sort.Float64s(allAnchors)
	anchors := make([]float64, 0, len(allAnchors))
	for _, anchor := range allAnchors {
		if len(anchors) == 0 || math.Abs(anchors[len(anchors)-1]-anchor) > tableColumnTolerance {
			anchors = append(anchors, anchor)
			continue
		}
		anchors[len(anchors)-1] = (anchors[len(anchors)-1] + anchor) / 2
	}
	return anchors
}

func lineAnchors(line *pdfStructuredLine) []float64 {
	if line == nil || len(line.Fragments) == 0 {
		return nil
	}
	out := make([]float64, 0, len(line.Fragments))
	for _, fragment := range line.Fragments {
		out = append(out, fragment.BBox.Left)
	}
	sort.Float64s(out)
	return out
}

func closestAnchorIndex(value float64, anchors []float64) int {
	if len(anchors) == 0 {
		return -1
	}
	bestIdx := 0
	bestDist := math.Abs(anchors[0] - value)
	for idx := 1; idx < len(anchors); idx++ {
		distance := math.Abs(anchors[idx] - value)
		if distance < bestDist {
			bestDist = distance
			bestIdx = idx
		}
	}
	return bestIdx
}

func mergeTextLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	current := ""
	merged := make([]string, 0, len(lines))
	flush := func() {
		if strings.TrimSpace(current) == "" {
			return
		}
		merged = append(merged, strings.TrimSpace(current))
		current = ""
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		if current == "" {
			current = line
			continue
		}
		if shouldMergePDFLines(current, line) {
			current = joinPDFWrappedLines(current, line)
			continue
		}
		flush()
		current = line
	}
	flush()
	return strings.Join(merged, "\n\n")
}

func plainTextFromBlocks(blocks []PageBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func markdownFromBlocks(blocks []PageBlock) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		markdown := strings.TrimSpace(block.Markdown)
		if markdown == "" {
			markdown = strings.TrimSpace(block.Text)
		}
		if markdown == "" {
			continue
		}
		parts = append(parts, markdown)
	}
	return strings.Join(parts, "\n\n")
}

func markdownFromTable(table PageTable) string {
	if len(table.Rows) == 0 {
		return ""
	}
	lines := make([]string, 0, len(table.Rows)+1)
	headerCells := table.Rows[0].Cells
	lines = append(lines, markdownTableRow(headerCells))
	separator := make([]string, 0, len(headerCells))
	for range headerCells {
		separator = append(separator, "---")
	}
	lines = append(lines, "| "+strings.Join(separator, " | ")+" |")
	for _, row := range table.Rows[1:] {
		lines = append(lines, markdownTableRow(row.Cells))
	}
	return strings.Join(lines, "\n")
}

func markdownTableRow(cells []PageTableCell) string {
	values := make([]string, 0, len(cells))
	for _, cell := range cells {
		value := strings.TrimSpace(cell.Text)
		value = strings.ReplaceAll(value, "|", "\\|")
		values = append(values, value)
	}
	return "| " + strings.Join(values, " | ") + " |"
}

func plainTextFromTable(table PageTable) string {
	rows := make([]string, 0, len(table.Rows))
	for _, row := range table.Rows {
		values := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			values = append(values, strings.TrimSpace(cell.Text))
		}
		rows = append(rows, strings.Join(values, "\t"))
	}
	return strings.Join(rows, "\n")
}

func splitSyntheticLines(text string) []string {
	text = normalizeText(text)
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		lines = append(lines, part)
	}
	return lines
}

func paragraphGap(current, next *pdfStructuredLine) float64 {
	if current == nil || next == nil {
		return math.MaxFloat64
	}
	return math.Max(0, next.BBox.Top-current.BBox.Bottom)
}

func bboxVerticalOverlap(a, b BBox) float64 {
	top := math.Max(a.Top, b.Top)
	bottom := math.Min(a.Bottom, b.Bottom)
	if bottom <= top {
		return 0
	}
	overlap := bottom - top
	height := math.Min(a.height(), b.height())
	if height <= 0 {
		return 0
	}
	return overlap / height
}

func unionLineBounds(lines []*pdfStructuredLine) BBox {
	var out BBox
	for _, line := range lines {
		if line == nil {
			continue
		}
		out = out.union(line.BBox)
	}
	return out
}

func dominantFontSize(fragments []pdfStructuredRect) float64 {
	if len(fragments) == 0 {
		return 0
	}
	sizes := make([]float64, 0, len(fragments))
	for _, fragment := range fragments {
		if fragment.FontSize > 0 {
			sizes = append(sizes, fragment.FontSize)
		}
	}
	if len(sizes) == 0 {
		return 0
	}
	sort.Float64s(sizes)
	return sizes[len(sizes)/2]
}

func dominantFontWeight(fragments []pdfStructuredRect) int {
	best := 0
	for _, fragment := range fragments {
		if fragment.FontWeight > best {
			best = fragment.FontWeight
		}
	}
	return best
}

func dominantFontName(fragments []pdfStructuredRect) string {
	counts := make(map[string]int, len(fragments))
	bestName := ""
	bestCount := 0
	for _, fragment := range fragments {
		name := strings.TrimSpace(fragment.FontName)
		if name == "" {
			continue
		}
		counts[name]++
		if counts[name] > bestCount {
			bestCount = counts[name]
			bestName = name
		}
	}
	return bestName
}

func medianLineHeight(lines []*pdfStructuredLine) float64 {
	if len(lines) == 0 {
		return 0
	}
	heights := make([]float64, 0, len(lines))
	for _, line := range lines {
		if line == nil || line.BBox.height() <= 0 {
			continue
		}
		heights = append(heights, line.BBox.height())
	}
	if len(heights) == 0 {
		return 0
	}
	sort.Float64s(heights)
	return heights[len(heights)/2]
}

func (b BBox) union(other BBox) BBox {
	if other.width() == 0 && other.height() == 0 && other.Left == 0 && other.Top == 0 && other.Right == 0 && other.Bottom == 0 {
		return b
	}
	if b.width() == 0 && b.height() == 0 && b.Left == 0 && b.Top == 0 && b.Right == 0 && b.Bottom == 0 {
		return other
	}
	return BBox{
		Left:   math.Min(b.Left, other.Left),
		Top:    math.Min(b.Top, other.Top),
		Right:  math.Max(b.Right, other.Right),
		Bottom: math.Max(b.Bottom, other.Bottom),
	}
}

func (b BBox) width() float64 {
	return math.Max(0, b.Right-b.Left)
}

func (b BBox) height() float64 {
	return math.Max(0, b.Bottom-b.Top)
}

func (b BBox) centerX() float64 {
	return (b.Left + b.Right) / 2
}

func (b BBox) centerY() float64 {
	return (b.Top + b.Bottom) / 2
}

func nearlyEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
