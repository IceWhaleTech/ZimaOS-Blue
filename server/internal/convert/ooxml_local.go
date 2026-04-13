package convert

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	pathpkg "path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func readDOCXLocal(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx archive: %w", err)
	}
	defer reader.Close()

	file := findZipFile(reader.File, "word/document.xml")
	if file == nil {
		return "", fmt.Errorf("docx document.xml missing")
	}
	text, err := extractOOXMLText(file, ooxmlTextExtractConfig{
		ParagraphElements: map[string]struct{}{"p": {}},
		TextElements:      map[string]struct{}{"t": {}, "instrText": {}},
		BreakElements:     map[string]string{"tab": "\t", "br": "\n", "cr": "\n"},
	})
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("docx document contained no readable text")
	}
	return text, nil
}

func readPPTXLocal(path string) (string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open pptx archive: %w", err)
	}
	defer reader.Close()

	slides := collectSortedZipFiles(reader.File, "ppt/slides/slide", ".xml")
	if len(slides) == 0 {
		return "", fmt.Errorf("pptx slides missing")
	}

	parts := make([]string, 0, len(slides))
	for idx, slide := range slides {
		slideText, err := extractOOXMLText(slide, ooxmlTextExtractConfig{
			ParagraphElements: map[string]struct{}{"p": {}},
			TextElements:      map[string]struct{}{"t": {}},
			BreakElements:     map[string]string{"br": "\n", "tab": "\t"},
		})
		if err != nil {
			return "", err
		}
		chartText, err := extractPPTXChartText(reader.File, slide.Name)
		if err != nil {
			return "", err
		}
		text := strings.TrimSpace(strings.TrimSpace(slideText) + "\n\n" + strings.TrimSpace(chartText))
		if text == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("[Slide %d]\n%s", idx+1, text))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("pptx slides contained no readable text")
	}
	return strings.Join(parts, "\n\n"), nil
}

type pptxLocalRelationships struct {
	Relationships []pptxLocalRelationship `xml:"Relationship"`
}

type pptxLocalRelationship struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr"`
	Target string `xml:"Target,attr"`
}

func extractPPTXChartText(files []*zip.File, slideName string) (string, error) {
	relsName := pptxSlideRelsName(slideName)
	if relsName == "" {
		return "", nil
	}
	relsFile := findZipFile(files, relsName)
	if relsFile == nil {
		return "", nil
	}
	rc, err := relsFile.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", relsFile.Name, err)
	}
	defer rc.Close()

	var rels pptxLocalRelationships
	if err := xml.NewDecoder(rc).Decode(&rels); err != nil {
		return "", fmt.Errorf("decode %s: %w", relsFile.Name, err)
	}

	parts := make([]string, 0, len(rels.Relationships))
	for _, rel := range rels.Relationships {
		if !strings.HasSuffix(strings.TrimSpace(rel.Type), "/chart") {
			continue
		}
		target := strings.TrimSpace(rel.Target)
		if target == "" {
			continue
		}
		chartPath := pathpkg.Clean(pathpkg.Join(pathpkg.Dir(slideName), target))
		chartFile := findZipFile(files, chartPath)
		if chartFile == nil {
			continue
		}
		text, err := extractOOXMLText(chartFile, ooxmlTextExtractConfig{
			ParagraphElements: map[string]struct{}{"p": {}, "pt": {}, "tx": {}},
			TextElements:      map[string]struct{}{"t": {}, "v": {}},
		})
		if err != nil {
			return "", err
		}
		dateLabels, err := extractPPTXDateChartLabelText(files, chartFile)
		if err != nil {
			return "", err
		}
		text = strings.TrimSpace(strings.TrimSpace(text) + "\n\n" + strings.TrimSpace(dateLabels))
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}

func extractPPTXDateChartLabelText(files []*zip.File, file *zip.File) (string, error) {
	if file == nil {
		return "", nil
	}
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", file.Name, err)
	}
	defer rc.Close()

	data, err := readSpreadsheetBytes(rc, int(file.UncompressedSize64))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file.Name, err)
	}
	xmlText := string(data)
	if !strings.Contains(xmlText, "<c:dateAx") {
		return "", nil
	}
	axisFormatCode := pptxDateAxisFormatCode(xmlText)
	if axisFormatCode == "" {
		axisFormatCode = "yyyy-mm-dd"
	}

	var (
		out  []string
		seen = make(map[string]struct{})
	)
	for _, block := range pptxDateChartCategoryBlocks(xmlText) {
		formatCode := axisFormatCode
		if formatStart := strings.Index(block, "<c:formatCode>"); formatStart >= 0 {
			formatStart += len("<c:formatCode>")
			if formatEnd := strings.Index(block[formatStart:], "</c:formatCode>"); formatEnd >= 0 {
				formatCode = strings.TrimSpace(block[formatStart : formatStart+formatEnd])
			}
		}
		valuePos := 0
		for {
			valueStart := strings.Index(block[valuePos:], "<c:v>")
			if valueStart < 0 {
				break
			}
			valueStart += valuePos + len("<c:v>")
			valueEnd := strings.Index(block[valueStart:], "</c:v>")
			if valueEnd < 0 {
				break
			}
			raw := strings.TrimSpace(block[valueStart : valueStart+valueEnd])
			if formatted, ok := pptxDateChartSerialToString(raw, formatCode); ok {
				if _, exists := seen[formatted]; !exists {
					seen[formatted] = struct{}{}
					out = append(out, formatted)
				}
			}
			valuePos = valueStart + valueEnd + len("</c:v>")
		}
	}
	if len(out) > 0 {
		return strings.Join(out, "\n\n"), nil
	}
	embedded, err := extractPPTXEmbeddedDateChartLabelText(files, file.Name, xmlText, axisFormatCode)
	if err != nil {
		return "", err
	}
	return embedded, nil
}

type pptxDateChartCategoryBlock struct {
	Start int
	Text  string
}

var (
	pptxDateChartNumLitBlockPattern = regexp.MustCompile(`(?s)<c:cat>\s*<c:numLit>(.*?)</c:numLit>\s*</c:cat>`)
	pptxDateChartNumRefBlockPattern = regexp.MustCompile(`(?s)<c:cat>\s*<c:numRef>(.*?)</c:numRef>\s*</c:cat>`)
	pptxDateChartNumCachePattern    = regexp.MustCompile(`(?s)<c:numCache>(.*?)</c:numCache>`)
	pptxChartFormulaRefPattern      = regexp.MustCompile(`(?s)<c:cat>\s*<c:numRef>.*?<c:f>(.*?)</c:f>.*?</c:numRef>\s*</c:cat>`)
	pptxChartExternalDataPattern    = regexp.MustCompile(`(?s)<c:externalData\b[^>]*\br:id="([^"]+)"`)
)

func pptxDateChartCategoryBlocks(xmlText string) []string {
	var blocks []pptxDateChartCategoryBlock
	for _, match := range pptxDateChartNumLitBlockPattern.FindAllStringSubmatchIndex(xmlText, -1) {
		if len(match) >= 4 {
			blocks = append(blocks, pptxDateChartCategoryBlock{
				Start: match[2],
				Text:  xmlText[match[2]:match[3]],
			})
		}
	}
	for _, match := range pptxDateChartNumRefBlockPattern.FindAllStringSubmatchIndex(xmlText, -1) {
		if len(match) < 4 {
			continue
		}
		inner := xmlText[match[2]:match[3]]
		cacheMatch := pptxDateChartNumCachePattern.FindStringSubmatchIndex(inner)
		if len(cacheMatch) >= 4 {
			blocks = append(blocks, pptxDateChartCategoryBlock{
				Start: match[2] + cacheMatch[2],
				Text:  inner[cacheMatch[2]:cacheMatch[3]],
			})
		}
	}

	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].Start < blocks[j].Start
	})
	out := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block.Text) == "" {
			continue
		}
		out = append(out, block.Text)
	}
	return out
}

func extractPPTXEmbeddedDateChartLabelText(files []*zip.File, chartName, chartXML, axisFormatCode string) (string, error) {
	workbookData, err := pptxEmbeddedWorkbookData(files, chartName, chartXML)
	if err != nil || len(workbookData) == 0 {
		return "", err
	}
	archive, err := loadEmbeddedSpreadsheetArchive(workbookData)
	if err != nil {
		return "", nil
	}
	formulas := pptxChartFormulaRefs(chartXML)
	if len(formulas) == 0 {
		return "", nil
	}
	var (
		out  []string
		seen = make(map[string]struct{})
	)
	for _, formula := range formulas {
		ref, ok := parsePPTXChartRangeFormula(formula)
		if !ok {
			continue
		}
		rows, err := archive.rowsForSheet(ref.SheetName)
		if err != nil {
			continue
		}
		for _, label := range pptxLabelsForSpreadsheetRange(rows, ref, axisFormatCode) {
			if _, exists := seen[label]; exists {
				continue
			}
			seen[label] = struct{}{}
			out = append(out, label)
		}
	}
	return strings.Join(out, "\n\n"), nil
}

type pptxChartRangeRef struct {
	SheetName string
	StartCol  int
	StartRow  int
	EndCol    int
	EndRow    int
}

type embeddedSpreadsheetArchive struct {
	workbookXML   xlsxWorkbookXML
	relTargets    map[string]string
	files         map[string]*zip.File
	sharedStrings []string
	styles        map[int]spreadsheetStyleInfo
	sheetCache    map[string][][]spreadsheetCell
}

func loadEmbeddedSpreadsheetArchive(data []byte) (*embeddedSpreadsheetArchive, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(reader.File))
	for _, file := range reader.File {
		files[file.Name] = file
	}
	workbookFile, ok := files["xl/workbook.xml"]
	if !ok {
		return nil, fmt.Errorf("xlsx workbook.xml missing")
	}
	workbookRelsFile, ok := files["xl/_rels/workbook.xml.rels"]
	if !ok {
		return nil, fmt.Errorf("xlsx workbook relationships missing")
	}
	var workbookXML xlsxWorkbookXML
	if err := decodeZipXML(workbookFile, &workbookXML); err != nil {
		return nil, err
	}
	var relsXML xlsxRelationshipsXML
	if err := decodeZipXML(workbookRelsFile, &relsXML); err != nil {
		return nil, err
	}

	sharedStrings := make([]string, 0)
	if sharedFile, ok := files["xl/sharedStrings.xml"]; ok {
		parsed, err := parseSpreadsheetSharedStrings(sharedFile)
		if err != nil {
			return nil, err
		}
		sharedStrings = parsed
	}
	styles := make(map[int]spreadsheetStyleInfo)
	if stylesFile, ok := files["xl/styles.xml"]; ok {
		parsed, err := decodeSpreadsheetStyles(stylesFile)
		if err != nil {
			return nil, err
		}
		styles = parsed
	}
	relTargets := make(map[string]string, len(relsXML.Relationships))
	for _, rel := range relsXML.Relationships {
		relTargets[rel.ID] = normalizeWorkbookTarget(rel.Target)
	}
	return &embeddedSpreadsheetArchive{
		workbookXML:   workbookXML,
		relTargets:    relTargets,
		files:         files,
		sharedStrings: sharedStrings,
		styles:        styles,
		sheetCache:    make(map[string][][]spreadsheetCell),
	}, nil
}

func (a *embeddedSpreadsheetArchive) rowsForSheet(name string) ([][]spreadsheetCell, error) {
	if rows, ok := a.sheetCache[name]; ok {
		return rows, nil
	}
	for _, sheetMeta := range a.workbookXML.Sheets {
		if strings.TrimSpace(sheetMeta.Name) != name {
			continue
		}
		target := a.relTargets[sheetMeta.RelID]
		if strings.TrimSpace(target) == "" {
			return nil, fmt.Errorf("missing worksheet target for relationship %q", sheetMeta.RelID)
		}
		sheetFile, ok := a.files[target]
		if !ok {
			return nil, fmt.Errorf("worksheet %q missing from archive", target)
		}
		rows, err := parseSpreadsheetWorksheet(sheetFile, a.sharedStrings, a.styles)
		if err != nil {
			return nil, err
		}
		a.sheetCache[name] = rows
		return rows, nil
	}
	return nil, fmt.Errorf("sheet %q missing", name)
}

func pptxEmbeddedWorkbookData(files []*zip.File, chartName, chartXML string) ([]byte, error) {
	relID := pptxChartExternalDataRelID(chartXML)
	if relID == "" {
		return nil, nil
	}
	relsName := pptxChartRelsName(chartName)
	if relsName == "" {
		return nil, nil
	}
	relsFile := findZipFile(files, relsName)
	if relsFile == nil {
		return nil, nil
	}
	rc, err := relsFile.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var rels pptxLocalRelationships
	if err := xml.NewDecoder(rc).Decode(&rels); err != nil {
		return nil, err
	}
	for _, rel := range rels.Relationships {
		if strings.TrimSpace(rel.ID) != relID {
			continue
		}
		target := strings.TrimSpace(rel.Target)
		if target == "" {
			return nil, nil
		}
		workbookPath := pathpkg.Clean(pathpkg.Join(pathpkg.Dir(chartName), target))
		workbookFile := findZipFile(files, workbookPath)
		if workbookFile == nil {
			return nil, nil
		}
		rc, err := workbookFile.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return readSpreadsheetBytes(rc, int(workbookFile.UncompressedSize64))
	}
	return nil, nil
}

func pptxChartExternalDataRelID(chartXML string) string {
	match := pptxChartExternalDataPattern.FindStringSubmatch(chartXML)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func pptxChartFormulaRefs(chartXML string) []string {
	matches := pptxChartFormulaRefPattern.FindAllStringSubmatch(chartXML, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}
		formula := strings.TrimSpace(match[1])
		if formula == "" {
			continue
		}
		out = append(out, formula)
	}
	return out
}

func parsePPTXChartRangeFormula(formula string) (pptxChartRangeRef, bool) {
	formula = strings.TrimSpace(formula)
	bang := strings.LastIndex(formula, "!")
	if bang <= 0 || bang >= len(formula)-1 {
		return pptxChartRangeRef{}, false
	}
	sheetName := strings.TrimSpace(formula[:bang])
	rangeRef := strings.TrimSpace(formula[bang+1:])
	sheetName = strings.Trim(sheetName, "'")
	sheetName = strings.ReplaceAll(sheetName, "''", "'")

	startRef := rangeRef
	endRef := rangeRef
	if cut := strings.Index(rangeRef, ":"); cut >= 0 {
		startRef = rangeRef[:cut]
		endRef = rangeRef[cut+1:]
	}
	startCol, startRow, ok := spreadsheetParseCellRef(strings.ReplaceAll(startRef, "$", ""))
	if !ok {
		return pptxChartRangeRef{}, false
	}
	endCol, endRow, ok := spreadsheetParseCellRef(strings.ReplaceAll(endRef, "$", ""))
	if !ok {
		return pptxChartRangeRef{}, false
	}
	if endRow < startRow {
		startRow, endRow = endRow, startRow
	}
	if endCol < startCol {
		startCol, endCol = endCol, startCol
	}
	return pptxChartRangeRef{
		SheetName: sheetName,
		StartCol:  startCol,
		StartRow:  startRow,
		EndCol:    endCol,
		EndRow:    endRow,
	}, true
}

func pptxLabelsForSpreadsheetRange(rows [][]spreadsheetCell, ref pptxChartRangeRef, axisFormatCode string) []string {
	out := make([]string, 0, ref.EndRow-ref.StartRow+1)
	appendCell := func(rowIndex, colIndex int) {
		if rowIndex < 0 || rowIndex >= len(rows) {
			return
		}
		if colIndex < 0 || colIndex >= len(rows[rowIndex]) {
			return
		}
		label := pptxSpreadsheetChartLabel(rows[rowIndex][colIndex], axisFormatCode)
		if strings.TrimSpace(label) != "" {
			out = append(out, label)
		}
	}

	switch {
	case ref.StartCol == ref.EndCol:
		for rowIndex := ref.StartRow - 1; rowIndex <= ref.EndRow-1; rowIndex++ {
			appendCell(rowIndex, ref.StartCol)
		}
	case ref.StartRow == ref.EndRow:
		for colIndex := ref.StartCol; colIndex <= ref.EndCol; colIndex++ {
			appendCell(ref.StartRow-1, colIndex)
		}
	default:
		for rowIndex := ref.StartRow - 1; rowIndex <= ref.EndRow-1; rowIndex++ {
			for colIndex := ref.StartCol; colIndex <= ref.EndCol; colIndex++ {
				appendCell(rowIndex, colIndex)
			}
		}
	}
	return out
}

func pptxSpreadsheetChartLabel(cell spreadsheetCell, axisFormatCode string) string {
	if axisFormatCode != "" {
		if raw := strings.TrimSpace(cell.RawValue); raw != "" {
			if formatted, ok := pptxDateChartSerialToString(raw, axisFormatCode); ok {
				return formatted
			}
		}
	}
	if strings.TrimSpace(cell.Display) != "" {
		return strings.TrimSpace(cell.Display)
	}
	if cell.Value != nil {
		return strings.TrimSpace(fmt.Sprint(cell.Value))
	}
	return ""
}

func pptxDateAxisFormatCode(xmlText string) string {
	pos := 0
	for {
		start := strings.Index(xmlText[pos:], "<c:dateAx")
		if start < 0 {
			return ""
		}
		start += pos
		end := strings.Index(xmlText[start:], "</c:dateAx>")
		if end < 0 {
			return ""
		}
		block := xmlText[start : start+end]
		if formatCode := pptxTagAttributeValue(block, "<c:numFmt", "formatCode"); formatCode != "" {
			return strings.TrimSpace(formatCode)
		}
		pos = start + end + len("</c:dateAx>")
	}
}

func pptxTagAttributeValue(xmlText, tagStart, attribute string) string {
	start := strings.Index(xmlText, tagStart)
	if start < 0 {
		return ""
	}
	end := strings.Index(xmlText[start:], ">")
	if end < 0 {
		return ""
	}
	tag := xmlText[start : start+end]
	pattern := attribute + `="`
	valueStart := strings.Index(tag, pattern)
	if valueStart < 0 {
		return ""
	}
	valueStart += len(pattern)
	valueEnd := strings.Index(tag[valueStart:], `"`)
	if valueEnd < 0 {
		return ""
	}
	return tag[valueStart : valueStart+valueEnd]
}

func pptxDateChartSerialToString(raw, formatCode string) (string, bool) {
	serial, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return "", false
	}
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	timestamp := base.Add(time.Duration(serial * float64(24*time.Hour)))
	return timestamp.Format(pptxDateChartTimeLayout(formatCode)), true
}

func pptxDateChartTimeLayout(formatCode string) string {
	lower := pptxNormalizeDateChartFormatCode(formatCode)
	if layout, ok := pptxDateChartCustomLayout(lower); ok {
		return layout
	}
	hasTime := strings.Contains(lower, ":") || strings.Contains(lower, "h")
	hasDate := strings.Contains(lower, "y") || strings.Contains(lower, "d")
	if hasTime {
		if hasDate {
			if strings.Contains(lower, "s") {
				return "2006-01-02 15:04:05"
			}
			return "2006-01-02 15:04"
		}
		if strings.Contains(lower, "s") {
			return "15:04:05"
		}
		return "15:04"
	}
	return "2006-01-02"
}

func pptxNormalizeDateChartFormatCode(formatCode string) string {
	formatCode = strings.TrimSpace(formatCode)
	if cut := strings.Index(formatCode, ";"); cut >= 0 {
		formatCode = formatCode[:cut]
	}
	var out strings.Builder
	out.Grow(len(formatCode))
	inBracket := false
	for i := 0; i < len(formatCode); i++ {
		ch := formatCode[i]
		switch {
		case inBracket:
			if ch == ']' {
				inBracket = false
			}
		case ch == '[':
			inBracket = true
		case ch == '"' || ch == '\\':
			continue
		case ch == '_' || ch == '*':
			if i+1 < len(formatCode) {
				i++
			}
		default:
			out.WriteByte(ch)
		}
	}
	return strings.Join(strings.Fields(strings.ToLower(out.String())), " ")
}

func pptxDateChartCustomLayout(formatCode string) (string, bool) {
	switch formatCode {
	case "m/d/yyyy":
		return "1/2/2006", true
	case "mm/dd/yyyy":
		return "01/02/2006", true
	case "m/d/yy":
		return "1/2/06", true
	case "mm/dd/yy":
		return "01/02/06", true
	case "m/yyyy":
		return "1/2006", true
	case "mm/yyyy":
		return "01/2006", true
	case "yyyy-mm":
		return "2006-01", true
	case "yyyy-m":
		return "2006-1", true
	case "mmm-yy":
		return "Jan-06", true
	case "mmm-yyyy":
		return "Jan-2006", true
	case "mmm yy":
		return "Jan 06", true
	case "mmm yyyy":
		return "Jan 2006", true
	case "mmmm-yy":
		return "January-06", true
	case "mmmm-yyyy":
		return "January-2006", true
	case "mmmm yy":
		return "January 06", true
	case "mmmm yyyy":
		return "January 2006", true
	default:
		return "", false
	}
}

func ooxmlPartRelsName(partName string) string {
	if partName == "" {
		return ""
	}
	dir := pathpkg.Dir(partName)
	base := pathpkg.Base(partName)
	if dir == "." || base == "." {
		return ""
	}
	return pathpkg.Clean(pathpkg.Join(dir, "_rels", base+".rels"))
}

func pptxSlideRelsName(slideName string) string {
	return ooxmlPartRelsName(slideName)
}

func pptxChartRelsName(chartName string) string {
	return ooxmlPartRelsName(chartName)
}

type ooxmlTextExtractConfig struct {
	ParagraphElements map[string]struct{}
	TextElements      map[string]struct{}
	BreakElements     map[string]string
}

func extractOOXMLText(file *zip.File, cfg ooxmlTextExtractConfig) (string, error) {
	if file == nil {
		return "", fmt.Errorf("ooxml file is required")
	}
	rc, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", file.Name, err)
	}
	defer rc.Close()
	data, err := readSpreadsheetBytes(rc, int(file.UncompressedSize64))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file.Name, err)
	}
	if text, err := extractOOXMLTextBytes(data, cfg); err == nil {
		return text, nil
	}
	return extractOOXMLTextDecoder(bytes.NewReader(data), file.Name, cfg)
}

func extractOOXMLTextDecoder(r io.Reader, name string, cfg ooxmlTextExtractConfig) (string, error) {
	decoder := xml.NewDecoder(r)
	var (
		captureText bool
		current     strings.Builder
		out         strings.Builder
	)

	flushParagraph := func() {
		text := normalizeOOXMLParagraphText(current.String())
		current.Reset()
		if text != "" {
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(text)
		}
	}

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("decode %s: %w", name, err)
		}
		switch typed := token.(type) {
		case xml.StartElement:
			name := typed.Name.Local
			if _, ok := cfg.TextElements[name]; ok {
				captureText = true
			}
			if replacement, ok := cfg.BreakElements[name]; ok {
				current.WriteString(replacement)
			}
		case xml.EndElement:
			name := typed.Name.Local
			if _, ok := cfg.TextElements[name]; ok {
				captureText = false
			}
			if _, ok := cfg.ParagraphElements[name]; ok {
				flushParagraph()
			}
		case xml.CharData:
			if captureText {
				current.Write(typed)
			}
		}
	}
	flushParagraph()
	return out.String(), nil
}

func extractOOXMLTextBytes(data []byte, cfg ooxmlTextExtractConfig) (string, error) {
	var (
		captureText bool
		current     strings.Builder
		out         strings.Builder
	)

	flushParagraph := func() {
		text := normalizeOOXMLParagraphText(current.String())
		current.Reset()
		if text != "" {
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(text)
		}
	}

	for idx := 0; idx < len(data); {
		lt := bytes.IndexByte(data[idx:], '<')
		if lt < 0 {
			if captureText && idx < len(data) {
				current.WriteString(spreadsheetXMLText(data[idx:]))
			}
			break
		}
		lt += idx
		if captureText && lt > idx {
			current.WriteString(spreadsheetXMLText(data[idx:lt]))
		}
		next, localName, isEnd, selfClosing, cdataText, err := parseOOXMLTag(data, lt)
		if err != nil {
			return "", err
		}
		if cdataText != nil {
			if captureText {
				current.Write(cdataText)
			}
			idx = next
			continue
		}
		if localName == "" {
			idx = next
			continue
		}
		if isEnd {
			if _, ok := cfg.TextElements[localName]; ok {
				captureText = false
			}
			if _, ok := cfg.ParagraphElements[localName]; ok {
				flushParagraph()
			}
			idx = next
			continue
		}
		if _, ok := cfg.TextElements[localName]; ok {
			captureText = true
			if selfClosing {
				captureText = false
			}
		}
		if replacement, ok := cfg.BreakElements[localName]; ok {
			current.WriteString(replacement)
		}
		idx = next
	}

	flushParagraph()
	return out.String(), nil
}

func parseOOXMLTag(data []byte, start int) (next int, localName string, isEnd bool, selfClosing bool, cdataText []byte, err error) {
	if start+1 >= len(data) {
		return 0, "", false, false, nil, fmt.Errorf("truncated xml tag")
	}
	switch {
	case bytes.HasPrefix(data[start:], []byte("<!--")):
		end := bytes.Index(data[start+4:], []byte("-->"))
		if end < 0 {
			return 0, "", false, false, nil, fmt.Errorf("xml comment close missing")
		}
		return start + 4 + end + 3, "", false, false, nil, nil
	case bytes.HasPrefix(data[start:], []byte("<![CDATA[")):
		end := bytes.Index(data[start+9:], []byte("]]>"))
		if end < 0 {
			return 0, "", false, false, nil, fmt.Errorf("xml cdata close missing")
		}
		end += start + 9
		return end + 3, "", false, false, data[start+9 : end], nil
	case bytes.HasPrefix(data[start:], []byte("<?")):
		end := bytes.Index(data[start+2:], []byte("?>"))
		if end < 0 {
			return 0, "", false, false, nil, fmt.Errorf("xml processing instruction close missing")
		}
		return start + 2 + end + 2, "", false, false, nil, nil
	case bytes.HasPrefix(data[start:], []byte("<!")):
		end := bytes.IndexByte(data[start+2:], '>')
		if end < 0 {
			return 0, "", false, false, nil, fmt.Errorf("xml declaration close missing")
		}
		return start + 2 + end + 1, "", false, false, nil, nil
	}

	tagEnd, selfClosing, err := spreadsheetFindTagEnd(data, start)
	if err != nil {
		return 0, "", false, false, nil, err
	}
	body := data[start+1 : tagEnd]
	if len(body) == 0 {
		return 0, "", false, false, nil, fmt.Errorf("empty xml tag")
	}
	if body[0] == '/' {
		isEnd = true
		body = body[1:]
	}
	body = bytes.TrimSpace(body)
	for len(body) > 0 && body[len(body)-1] == '/' {
		body = bytes.TrimSpace(body[:len(body)-1])
	}
	nameEnd := 0
	for nameEnd < len(body) && !isSpreadsheetXMLSpace(body[nameEnd]) {
		nameEnd++
	}
	if nameEnd == 0 {
		return 0, "", false, false, nil, fmt.Errorf("xml tag name missing")
	}
	name := body[:nameEnd]
	if colon := bytes.IndexByte(name, ':'); colon >= 0 {
		name = name[colon+1:]
	}
	return tagEnd + 1, string(name), isEnd, selfClosing, nil, nil
}

func normalizeOOXMLParagraphText(raw string) string {
	raw = strings.ReplaceAll(raw, "\u00a0", " ")
	var cleaned strings.Builder
	lineStart := 0
	wrote := false
	for idx := 0; idx <= len(raw); idx++ {
		if idx < len(raw) && raw[idx] != '\n' {
			continue
		}
		line := strings.TrimSpace(raw[lineStart:idx])
		if line != "" {
			if wrote {
				cleaned.WriteByte('\n')
			}
			cleaned.WriteString(line)
			wrote = true
		}
		lineStart = idx + 1
	}
	return cleaned.String()
}

func findZipFile(files []*zip.File, name string) *zip.File {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, file := range files {
		if strings.ToLower(strings.TrimSpace(file.Name)) == target {
			return file
		}
	}
	return nil
}

func collectSortedZipFiles(files []*zip.File, prefix, suffix string) []*zip.File {
	type numberedFile struct {
		order int
		file  *zip.File
	}
	items := make([]numberedFile, 0, len(files))
	lowerPrefix := strings.ToLower(prefix)
	lowerSuffix := strings.ToLower(suffix)
	for _, file := range files {
		name := strings.ToLower(strings.TrimSpace(file.Name))
		if !strings.HasPrefix(name, lowerPrefix) || !strings.HasSuffix(name, lowerSuffix) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(name, lowerPrefix), lowerSuffix)
		order, err := strconv.Atoi(middle)
		if err != nil {
			continue
		}
		items = append(items, numberedFile{order: order, file: file})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order == items[j].order {
			return items[i].file.Name < items[j].file.Name
		}
		return items[i].order < items[j].order
	})
	out := make([]*zip.File, 0, len(items))
	for _, item := range items {
		out = append(out, item.file)
	}
	return out
}
