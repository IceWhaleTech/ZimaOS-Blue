package tools

import (
	"encoding/xml"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	xlsxWorkbookRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	xlsxStylesRelType = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
)

var xlsxCellRefPattern = regexp.MustCompile(`(?i)(\$?)([A-Z]{1,3})(\$?)([0-9]+)`)

type xlsxWorkbookXML struct {
	XMLName xml.Name               `xml:"workbook"`
	XMLNS   string                 `xml:"xmlns,attr,omitempty"`
	XMLNSR  string                 `xml:"xmlns:r,attr,omitempty"`
	Sheets  []xlsxWorkbookSheetXML `xml:"sheets>sheet"`
}

type xlsxWorkbookSheetXML struct {
	Name    string `xml:"name,attr"`
	SheetID int    `xml:"sheetId,attr"`
	State   string `xml:"state,attr,omitempty"`
	RelID   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type xlsxRelationshipsXML struct {
	XMLName       xml.Name                `xml:"Relationships"`
	XMLNS         string                  `xml:"xmlns,attr,omitempty"`
	Relationships []xlsxRelationshipEntry `xml:"Relationship"`
}

type xlsxRelationshipEntry struct {
	ID     string `xml:"Id,attr"`
	Type   string `xml:"Type,attr,omitempty"`
	Target string `xml:"Target,attr"`
}

type xlsxSharedStringsXML struct {
	Items []xlsxSharedStringItem `xml:"si"`
}

type xlsxSharedStringItem struct {
	Text string            `xml:"t"`
	Runs []xlsxTextRunItem `xml:"r"`
}

type xlsxTextRunItem struct {
	Text string `xml:"t"`
}

type xlsxWorksheetXML struct {
	SheetPr    *xlsxSheetPrXML    `xml:"sheetPr"`
	SheetViews *xlsxSheetViewsXML `xml:"sheetViews"`
	Cols       *xlsxColsXML       `xml:"cols"`
	Rows       []xlsxRowXML       `xml:"sheetData>row"`
	AutoFilter *xlsxAutoFilterXML `xml:"autoFilter"`
	MergeCells *xlsxMergeCellsXML `xml:"mergeCells"`
}

type xlsxSheetPrXML struct {
	TabColor *xlsxTabColorXML `xml:"tabColor"`
}

type xlsxTabColorXML struct {
	RGB string `xml:"rgb,attr"`
}

type xlsxSheetViewsXML struct {
	SheetViews []xlsxSheetViewXML `xml:"sheetView"`
}

type xlsxSheetViewXML struct {
	Pane *xlsxPaneXML `xml:"pane"`
}

type xlsxPaneXML struct {
	TopLeftCell string `xml:"topLeftCell,attr"`
}

type xlsxColsXML struct {
	Cols []xlsxColXML `xml:"col"`
}

type xlsxColXML struct {
	Min   int     `xml:"min,attr"`
	Max   int     `xml:"max,attr"`
	Width float64 `xml:"width,attr"`
	Style int     `xml:"style,attr"`
}

type xlsxRowXML struct {
	Index  int           `xml:"r,attr"`
	Height float64       `xml:"ht,attr"`
	Cells  []xlsxCellXML `xml:"c"`
}

type xlsxCellXML struct {
	Ref       string            `xml:"r,attr"`
	Type      string            `xml:"t,attr"`
	Style     int               `xml:"s,attr"`
	Formula   *xlsxFormulaXML   `xml:"f"`
	Value     string            `xml:"v"`
	InlineStr *xlsxInlineStrXML `xml:"is"`
}

type xlsxFormulaXML struct {
	Type        string `xml:"t,attr"`
	Reference   string `xml:"ref,attr"`
	SharedIndex int    `xml:"si,attr"`
	Text        string `xml:",chardata"`
}

type xlsxInlineStrXML struct {
	Text string            `xml:"t"`
	Runs []xlsxTextRunItem `xml:"r"`
}

type xlsxAutoFilterXML struct {
	Ref string `xml:"ref,attr"`
}

type xlsxMergeCellsXML struct {
	Items []xlsxMergeCellXML `xml:"mergeCell"`
}

type xlsxMergeCellXML struct {
	Ref string `xml:"ref,attr"`
}

type xlsxMutableWorkbook struct {
	entries       []zipArchiveEntry
	entryIndex    map[string]int
	workbookXML   xlsxWorkbookXML
	workbookData  []byte
	workbookDirty bool
	stylesEntry   string
	styles        *xlsxMutableStyles
	relationships xlsxRelationshipsXML
	sheets        []*xlsxMutableSheet
}

type xlsxMutableSheet struct {
	name         string
	relID        string
	entryName    string
	originalData []byte
	dirty        bool
	styles       *xlsxMutableStyles
	build        officeXLSXSheetBuild
}

type xlsxSharedFormula struct {
	cellRef string
	text    string
}

func loadMutableXLSXWorkbook(path string) (*xlsxMutableWorkbook, error) {
	entries, err := readZipArchive(path)
	if err != nil {
		return nil, err
	}
	index := make(map[string]int, len(entries))
	for idx, entry := range entries {
		index[entry.Name] = idx
	}

	workbookEntry, ok := index["xl/workbook.xml"]
	if !ok {
		return nil, fmt.Errorf("xl/workbook.xml missing")
	}
	relsEntry, ok := index["xl/_rels/workbook.xml.rels"]
	if !ok {
		return nil, fmt.Errorf("xl/_rels/workbook.xml.rels missing")
	}

	var workbookXML xlsxWorkbookXML
	if err := xml.Unmarshal(entries[workbookEntry].Data, &workbookXML); err != nil {
		return nil, fmt.Errorf("decode workbook.xml: %w", err)
	}
	if workbookXML.XMLNS == "" {
		workbookXML.XMLNS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	}
	if workbookXML.XMLNSR == "" {
		workbookXML.XMLNSR = xlsxWorkbookRelNS
	}

	var relsXML xlsxRelationshipsXML
	if err := xml.Unmarshal(entries[relsEntry].Data, &relsXML); err != nil {
		return nil, fmt.Errorf("decode workbook relationships: %w", err)
	}
	if relsXML.XMLNS == "" {
		relsXML.XMLNS = "http://schemas.openxmlformats.org/package/2006/relationships"
	}

	stylesEntry := ""
	for _, rel := range relsXML.Relationships {
		if strings.TrimSpace(rel.Type) == xlsxStylesRelType {
			stylesEntry = normalizeXLSXWorkbookTarget(rel.Target)
			break
		}
	}
	if stylesEntry == "" {
		if _, ok := index["xl/styles.xml"]; ok {
			stylesEntry = "xl/styles.xml"
		}
	}

	var styles *xlsxMutableStyles
	if stylesEntry != "" {
		if stylesIdx, ok := index[stylesEntry]; ok {
			styles, err = loadMutableXLSXStyles(entries[stylesIdx].Data)
			if err != nil {
				return nil, fmt.Errorf("decode %s: %w", stylesEntry, err)
			}
		}
	}

	sharedStrings := make([]string, 0)
	if sharedIdx, ok := index["xl/sharedStrings.xml"]; ok {
		var sharedXML xlsxSharedStringsXML
		if err := xml.Unmarshal(entries[sharedIdx].Data, &sharedXML); err != nil {
			return nil, fmt.Errorf("decode sharedStrings.xml: %w", err)
		}
		for _, item := range sharedXML.Items {
			sharedStrings = append(sharedStrings, item.fullText())
		}
	}

	relTargets := make(map[string]string, len(relsXML.Relationships))
	for _, rel := range relsXML.Relationships {
		relTargets[rel.ID] = normalizeXLSXWorkbookTarget(rel.Target)
	}

	sheets := make([]*xlsxMutableSheet, 0, len(workbookXML.Sheets))
	for _, sheetMeta := range workbookXML.Sheets {
		entryName := relTargets[sheetMeta.RelID]
		if entryName == "" {
			return nil, fmt.Errorf("worksheet target missing for %s", sheetMeta.Name)
		}
		entryPos, ok := index[entryName]
		if !ok {
			return nil, fmt.Errorf("worksheet entry %s missing", entryName)
		}
		var sheetXML xlsxWorksheetXML
		if err := xml.Unmarshal(entries[entryPos].Data, &sheetXML); err != nil {
			return nil, fmt.Errorf("decode worksheet %s: %w", sheetMeta.Name, err)
		}
		build, err := xlsxWorksheetToBuild(sheetMeta.Name, sheetXML, sharedStrings, styles)
		if err != nil {
			return nil, fmt.Errorf("parse worksheet %s: %w", sheetMeta.Name, err)
		}
		sheets = append(sheets, &xlsxMutableSheet{
			name:         sheetMeta.Name,
			relID:        sheetMeta.RelID,
			entryName:    entryName,
			originalData: append([]byte(nil), entries[entryPos].Data...),
			styles:       styles,
			build:        build,
		})
	}

	return &xlsxMutableWorkbook{
		entries:       entries,
		entryIndex:    index,
		workbookXML:   workbookXML,
		workbookData:  append([]byte(nil), entries[workbookEntry].Data...),
		stylesEntry:   stylesEntry,
		styles:        styles,
		relationships: relsXML,
		sheets:        sheets,
	}, nil
}

func (i xlsxInlineStrXML) fullText() string {
	if strings.TrimSpace(i.Text) != "" || len(i.Runs) == 0 {
		return i.Text
	}
	var sb strings.Builder
	for _, run := range i.Runs {
		sb.WriteString(run.Text)
	}
	return sb.String()
}

func (i xlsxSharedStringItem) fullText() string {
	if strings.TrimSpace(i.Text) != "" || len(i.Runs) == 0 {
		return i.Text
	}
	var sb strings.Builder
	for _, run := range i.Runs {
		sb.WriteString(run.Text)
	}
	return sb.String()
}

func normalizeXLSXWorkbookTarget(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "/")
	if strings.HasPrefix(target, "xl/") {
		return path.Clean(target)
	}
	return path.Clean(path.Join("xl", target))
}

func xlsxWorksheetToBuild(name string, worksheet xlsxWorksheetXML, sharedStrings []string, styles *xlsxMutableStyles) (officeXLSXSheetBuild, error) {
	build := officeXLSXSheetBuild{
		Name:       name,
		AutoFilter: "",
	}
	if worksheet.SheetPr != nil && worksheet.SheetPr.TabColor != nil {
		build.TabColor = strings.TrimPrefix(strings.TrimSpace(worksheet.SheetPr.TabColor.RGB), "FF")
	}
	if worksheet.SheetViews != nil {
		for _, view := range worksheet.SheetViews.SheetViews {
			if view.Pane != nil && strings.TrimSpace(view.Pane.TopLeftCell) != "" {
				build.Freeze = strings.TrimSpace(view.Pane.TopLeftCell)
				break
			}
		}
	}
	if worksheet.AutoFilter != nil {
		build.AutoFilter = strings.TrimSpace(worksheet.AutoFilter.Ref)
	}
	if worksheet.MergeCells != nil {
		for _, merge := range worksheet.MergeCells.Items {
			if ref := strings.TrimSpace(merge.Ref); ref != "" {
				build.Merges = append(build.Merges, ref)
			}
		}
	}
	if worksheet.Cols != nil {
		maxCol := 0
		for _, col := range worksheet.Cols.Cols {
			if col.Max > maxCol {
				maxCol = col.Max
			}
		}
		if maxCol > 0 {
			build.ColWidths = make([]float64, maxCol)
			build.ColStyles = make([]int, maxCol)
			for _, col := range worksheet.Cols.Cols {
				width := col.Width
				for idx := col.Min - 1; idx < col.Max && idx < len(build.ColWidths); idx++ {
					build.ColWidths[idx] = width
					if col.Style > 0 {
						build.ColStyles[idx] = col.Style
					}
				}
			}
		}
	}

	sharedFormulae := make(map[int]xlsxSharedFormula)
	rows := make([]officeXLSXRow, 0, len(worksheet.Rows))
	for rowIdx, rowXML := range worksheet.Rows {
		row := officeXLSXRow{
			Height: rowXML.Height,
		}
		if row.Height == 0 {
			row.Height = 20
		}
		maxCol := 0
		for _, cellXML := range rowXML.Cells {
			colIndex := maxCol
			if parsed, ok := xlsxColumnIndex(cellXML.Ref); ok {
				colIndex = parsed
			}
			if colIndex+1 > maxCol {
				maxCol = colIndex + 1
			}
		}
		row.Cells = make([]officeXLSXCell, maxCol)
		rowNumber := rowXML.Index
		if rowNumber <= 0 {
			rowNumber = rowIdx + 1
		}
		for _, cellXML := range rowXML.Cells {
			colIndex := 0
			if parsed, ok := xlsxColumnIndex(cellXML.Ref); ok {
				colIndex = parsed
			}
			ref := strings.TrimSpace(cellXML.Ref)
			if ref == "" {
				ref = officeXLSXCellRef(colIndex, rowNumber)
			}
			row.Cells[colIndex] = xlsxDecodeCell(cellXML, sharedStrings, ref, sharedFormulae, styles)
		}
		rows = append(rows, row)
	}
	build.Rows = rows
	return build, nil
}

func xlsxDecodeCell(cell xlsxCellXML, sharedStrings []string, ref string, sharedFormulae map[int]xlsxSharedFormula, styles *xlsxMutableStyles) officeXLSXCell {
	result := officeXLSXCell{
		Style: cell.Style,
		Type:  strings.TrimSpace(cell.Type),
	}
	format := ""
	if styles != nil {
		format = styles.formatForStyle(cell.Style)
	}
	if format == "" {
		format = officeXLSXStyleFormat(cell.Style)
	}
	if format != "" {
		result.Format = format
	}
	formula := ""
	if cell.Formula != nil {
		formula = strings.TrimSpace(cell.Formula.Text)
		if strings.EqualFold(strings.TrimSpace(cell.Formula.Type), "shared") {
			if formula != "" {
				sharedFormulae[cell.Formula.SharedIndex] = xlsxSharedFormula{cellRef: ref, text: formula}
			} else if base, ok := sharedFormulae[cell.Formula.SharedIndex]; ok {
				formula = xlsxTranslateFormula(base.text, base.cellRef, ref)
			}
		}
	}
	result.Formula = formula

	switch strings.ToLower(strings.TrimSpace(cell.Type)) {
	case "s":
		index, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err == nil && index >= 0 && index < len(sharedStrings) {
			result.Value = sharedStrings[index]
			result.ForceString = true
		}
	case "inlinestr":
		if cell.InlineStr != nil {
			result.Value = cell.InlineStr.fullText()
			result.ForceString = true
		}
	case "b":
		result.Value = strings.TrimSpace(cell.Value) == "1"
	case "str", "e":
		result.Value = strings.TrimSpace(cell.Value)
		result.ForceString = true
	default:
		result.Value = xlsxDecodeNumericValue(strings.TrimSpace(cell.Value), result.Format)
		result.ForceString = result.Format == "date" || result.Format == "datetime"
	}
	if strings.TrimSpace(cell.Value) == "" && result.Value == nil && formula != "" {
		result.Value = nil
	}
	return result
}

func xlsxDecodeNumericValue(raw, format string) interface{} {
	if raw == "" {
		return nil
	}
	if format == "date" || format == "datetime" {
		if number, err := strconv.ParseFloat(raw, 64); err == nil {
			return xlsxExcelSerialToString(number, format)
		}
	}
	if i, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return f
	}
	return raw
}

func xlsxExcelSerialToString(serial float64, format string) string {
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	timestamp := base.Add(time.Duration(serial * float64(24*time.Hour)))
	if format == "datetime" {
		return timestamp.Format("2006-01-02 15:04:05")
	}
	return timestamp.Format("2006-01-02")
}

func officeXLSXStyleFormat(styleID int) string {
	switch styleID {
	case 26:
		return "integer"
	case 27:
		return "decimal"
	case 28:
		return "currency"
	case 29:
		return "percent"
	case 30:
		return "date"
	case 31:
		return "datetime"
	case 32:
		return "text"
	default:
		return ""
	}
}

func xlsxColumnIndex(ref string) (int, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return 0, false
	}
	col := 0
	seenLetter := false
	for _, r := range ref {
		switch {
		case r >= 'A' && r <= 'Z':
			col = col*26 + int(r-'A'+1)
			seenLetter = true
		case r >= 'a' && r <= 'z':
			col = col*26 + int(r-'a'+1)
			seenLetter = true
		case r >= '0' && r <= '9':
			if !seenLetter {
				return 0, false
			}
			return col - 1, true
		default:
			return 0, false
		}
	}
	if !seenLetter {
		return 0, false
	}
	return col - 1, true
}

func xlsxParseCellRef(ref string) (int, int, bool) {
	col, ok := xlsxColumnIndex(ref)
	if !ok {
		return 0, 0, false
	}
	row := 0
	for _, r := range ref {
		if r < '0' || r > '9' {
			continue
		}
		row = row*10 + int(r-'0')
	}
	if row <= 0 {
		return 0, 0, false
	}
	return col, row, true
}

func xlsxTranslateFormula(formula, oldRef, newRef string) string {
	oldCol, oldRow, ok := xlsxParseCellRef(oldRef)
	if !ok {
		return formula
	}
	newCol, newRow, ok := xlsxParseCellRef(newRef)
	if !ok {
		return formula
	}
	deltaCol := newCol - oldCol
	deltaRow := newRow - oldRow
	return xlsxCellRefPattern.ReplaceAllStringFunc(formula, func(match string) string {
		parts := xlsxCellRefPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		colAbs := parts[1] == "$"
		rowAbs := parts[3] == "$"
		col := xlsxLettersToIndex(parts[2])
		row, err := strconv.Atoi(parts[4])
		if err != nil {
			return match
		}
		if !colAbs {
			col += deltaCol
		}
		if !rowAbs {
			row += deltaRow
		}
		if col < 0 || row <= 0 {
			return match
		}
		replacement := ""
		if colAbs {
			replacement += "$"
		}
		replacement += officeXLSXColumnName(col)
		if rowAbs {
			replacement += "$"
		}
		replacement += strconv.Itoa(row)
		return replacement
	})
}

func xlsxLettersToIndex(letters string) int {
	col := 0
	for _, r := range strings.ToUpper(strings.TrimSpace(letters)) {
		if r < 'A' || r > 'Z' {
			break
		}
		col = col*26 + int(r-'A'+1)
	}
	return col - 1
}

func (w *xlsxMutableWorkbook) sheetByName(name string) *xlsxMutableSheet {
	trimmed := strings.TrimSpace(name)
	for idx := range w.sheets {
		if strings.EqualFold(strings.TrimSpace(w.sheets[idx].name), trimmed) {
			return w.sheets[idx]
		}
	}
	return nil
}

func (w *xlsxMutableWorkbook) firstVisibleSheet() *xlsxMutableSheet {
	for idx, meta := range w.workbookXML.Sheets {
		if strings.EqualFold(strings.TrimSpace(meta.State), "hidden") {
			continue
		}
		if idx < len(w.sheets) {
			return w.sheets[idx]
		}
	}
	if len(w.sheets) == 0 {
		return nil
	}
	return w.sheets[0]
}

func (w *xlsxMutableWorkbook) resolveSheet(name string) (*xlsxMutableSheet, error) {
	if sheet := w.sheetByName(name); sheet != nil {
		return sheet, nil
	}
	if strings.TrimSpace(name) == "" {
		if sheet := w.firstVisibleSheet(); sheet != nil {
			return sheet, nil
		}
	}
	return nil, fmt.Errorf("sheet %q not found", name)
}

func (w *xlsxMutableWorkbook) renameSheet(oldName, newName string) error {
	newName = officeSanitizeSheetName(newName)
	if newName == "" {
		return fmt.Errorf("new_name must be a non-empty sheet name")
	}
	for idx := range w.workbookXML.Sheets {
		if strings.EqualFold(strings.TrimSpace(w.workbookXML.Sheets[idx].Name), strings.TrimSpace(oldName)) {
			w.workbookXML.Sheets[idx].Name = newName
			if idx < len(w.sheets) {
				w.sheets[idx].name = newName
				w.sheets[idx].build.Name = newName
			}
			w.workbookDirty = true
			return nil
		}
	}
	return fmt.Errorf("sheet %q not found", oldName)
}

func (w *xlsxMutableWorkbook) save(path string) error {
	if w.workbookDirty {
		if idx, ok := w.entryIndex["xl/workbook.xml"]; ok {
			patched, err := patchXLSXWorkbookSheetNames(w.workbookData, w.workbookXML)
			if err != nil {
				return err
			}
			w.entries[idx].Data = patched
		}
	}

	if w.styles != nil && w.styles.dirty {
		if w.stylesEntry == "" {
			return fmt.Errorf("styles.xml entry missing")
		}
		idx, ok := w.entryIndex[w.stylesEntry]
		if !ok {
			return fmt.Errorf("styles entry %s missing", w.stylesEntry)
		}
		rendered, err := w.styles.render()
		if err != nil {
			return err
		}
		w.entries[idx].Data = rendered
	}

	for _, sheet := range w.sheets {
		if !sheet.dirty {
			continue
		}
		idx, ok := w.entryIndex[sheet.entryName]
		if !ok {
			return fmt.Errorf("worksheet entry %s missing", sheet.entryName)
		}
		w.entries[idx].Data = []byte(officeXLSXWorksheetXML(sheet.build))
	}
	return writeZipArchive(path, w.entries)
}

func patchXLSXWorkbookSheetNames(data []byte, workbook xlsxWorkbookXML) ([]byte, error) {
	updated := string(data)
	sheetTagPattern := regexp.MustCompile(`<sheet\b[^>]*/?>`)
	nameAttrPattern := regexp.MustCompile(`\bname="[^"]*"`)
	for _, sheet := range workbook.Sheets {
		relPattern := regexp.MustCompile(`\b(?:[\w]+:)?id="` + regexp.QuoteMeta(strings.TrimSpace(sheet.RelID)) + `"`)
		nameXML := officeXMLText(sheet.Name)
		matched := false
		updated = sheetTagPattern.ReplaceAllStringFunc(updated, func(tag string) string {
			if !relPattern.MatchString(tag) {
				return tag
			}
			matched = true
			next := nameAttrPattern.ReplaceAllString(tag, `name="`+nameXML+`"`)
			return next
		})
		if !matched {
			return nil, fmt.Errorf("sheet relationship %s not found in workbook.xml", sheet.RelID)
		}
	}
	return []byte(updated), nil
}

func xlsxHeadersForSheet(sheet officeXLSXSheetBuild) []string {
	if len(sheet.Rows) == 0 {
		return nil
	}
	headers := make([]string, 0, len(sheet.Rows[0].Cells))
	for idx, cell := range sheet.Rows[0].Cells {
		header := strings.TrimSpace(officeCellString(cell.Value))
		if header == "" {
			header = fmt.Sprintf("Column %d", idx+1)
		}
		headers = append(headers, header)
	}
	return headers
}

func xlsxColumnSpecsForSheet(sheet officeXLSXSheetBuild) []officeColumnSpec {
	headers := xlsxHeadersForSheet(sheet)
	specs := make([]officeColumnSpec, 0, len(headers))
	for idx, header := range headers {
		specs = append(specs, officeColumnSpec{
			Header: header,
			Key:    header,
			Kind:   inferOfficeColumnKind(officeColumnSpec{Header: header, Key: header}, xlsxSheetRowValues(sheet), idx),
		})
	}
	return specs
}

func xlsxSheetRowValues(sheet officeXLSXSheetBuild) [][]interface{} {
	rows := make([][]interface{}, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := make([]interface{}, 0, len(row.Cells))
		for _, cell := range row.Cells {
			values = append(values, cell.Value)
		}
		rows = append(rows, values)
	}
	return rows
}

func xlsxNormalizeIncomingRows(sheet officeXLSXSheetBuild, raw interface{}) ([][]interface{}, error) {
	columns := xlsxColumnSpecsForSheet(sheet)
	_, rows, err := normalizeOfficeRows(columns, raw)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func xlsxBuildDataRow(sheet officeXLSXSheetBuild, styles *xlsxMutableStyles, columns []officeColumnSpec, rowIndex int, raw []interface{}) (officeXLSXRow, error) {
	row := officeXLSXRow{
		Height: 20,
		Cells:  make([]officeXLSXCell, len(columns)),
	}
	for colIndex, column := range columns {
		var rawValue interface{}
		if colIndex < len(raw) {
			rawValue = raw[colIndex]
		}
		cellInput, _ := officeParseStructuredCell(rawValue)
		value := rawValue
		if cellInput.HasShape {
			value = cellInput.Value
		}
		style, err := xlsxResolveStyleID(styles, sheet, rowIndex, colIndex, 0, column, cellInput.Format, value)
		if err != nil {
			return officeXLSXRow{}, err
		}
		row.Cells[colIndex] = officeXLSXCell{
			Value:       value,
			Formula:     cellInput.Formula,
			Type:        cellInput.Type,
			Format:      cellInput.Format,
			Style:       style,
			ForceString: officeShouldWriteStringCell(column, rawValue, cellInput),
		}
	}
	return row, nil
}

func xlsxApplyAppendRows(sheet *xlsxMutableSheet, rawRows interface{}) error {
	rows, err := xlsxNormalizeIncomingRows(sheet.build, rawRows)
	if err != nil {
		return err
	}
	columns := xlsxColumnSpecsForSheet(sheet.build)
	baseIndex := xlsxMax(0, len(sheet.build.Rows)-1)
	for idx, raw := range rows {
		built, err := xlsxBuildDataRow(sheet.build, sheet.styles, columns, baseIndex+idx, raw)
		if err != nil {
			return err
		}
		sheet.build.Rows = append(sheet.build.Rows, built)
	}
	if sheet.build.AutoFilter != "" {
		sheet.build.AutoFilter = officeXLSXRangeRef(0, 1, len(columns)-1, len(sheet.build.Rows))
	}
	sheet.dirty = true
	return nil
}

func xlsxApplyInsertRows(sheet *xlsxMutableSheet, startRow, count int, rawRows interface{}) error {
	if startRow <= 0 {
		return fmt.Errorf("row must be >= 1")
	}
	rows, err := xlsxNormalizeIncomingRows(sheet.build, rawRows)
	if err != nil {
		return err
	}
	insertAt := startRow - 1
	if insertAt < 0 {
		insertAt = 0
	}
	if insertAt > len(sheet.build.Rows) {
		insertAt = len(sheet.build.Rows)
	}
	columns := xlsxColumnSpecsForSheet(sheet.build)
	inserted := make([]officeXLSXRow, 0, xlsxMax(count, len(rows)))
	for idx := 0; idx < len(rows); idx++ {
		built, err := xlsxBuildDataRow(sheet.build, sheet.styles, columns, insertAt+idx, rows[idx])
		if err != nil {
			return err
		}
		inserted = append(inserted, built)
	}
	for len(inserted) < count {
		built, err := xlsxBuildDataRow(sheet.build, sheet.styles, columns, insertAt+len(inserted), nil)
		if err != nil {
			return err
		}
		inserted = append(inserted, built)
	}
	sheet.build.Rows = append(sheet.build.Rows[:insertAt], append(inserted, sheet.build.Rows[insertAt:]...)...)
	xlsxReindexMovedFormulas(sheet.build.Rows, insertAt+len(inserted), len(inserted))
	if sheet.build.AutoFilter != "" {
		sheet.build.AutoFilter = officeXLSXRangeRef(0, 1, len(columns)-1, len(sheet.build.Rows))
	}
	sheet.dirty = true
	return nil
}

func xlsxApplyDeleteRows(sheet *xlsxMutableSheet, startRow, count int) error {
	if startRow <= 0 {
		return fmt.Errorf("row must be >= 1")
	}
	if count <= 0 {
		count = 1
	}
	start := startRow - 1
	if start >= len(sheet.build.Rows) {
		return nil
	}
	end := start + count
	if end > len(sheet.build.Rows) {
		end = len(sheet.build.Rows)
	}
	sheet.build.Rows = append(sheet.build.Rows[:start], sheet.build.Rows[end:]...)
	xlsxReindexMovedFormulas(sheet.build.Rows, start, -count)
	columns := xlsxColumnSpecsForSheet(sheet.build)
	if sheet.build.AutoFilter != "" && len(columns) > 0 && len(sheet.build.Rows) > 0 {
		sheet.build.AutoFilter = officeXLSXRangeRef(0, 1, len(columns)-1, len(sheet.build.Rows))
	}
	sheet.dirty = true
	return nil
}

func xlsxReindexMovedFormulas(rows []officeXLSXRow, start, delta int) {
	for rowIndex := start; rowIndex < len(rows); rowIndex++ {
		for colIndex := range rows[rowIndex].Cells {
			formula := strings.TrimSpace(rows[rowIndex].Cells[colIndex].Formula)
			if formula == "" {
				continue
			}
			oldRef := officeXLSXCellRef(colIndex, rowIndex+1-delta)
			newRef := officeXLSXCellRef(colIndex, rowIndex+1)
			rows[rowIndex].Cells[colIndex].Formula = xlsxTranslateFormula(formula, oldRef, newRef)
		}
	}
}

func xlsxEnsureCell(sheet *xlsxMutableSheet, rowIndex, colIndex int) {
	for len(sheet.build.Rows) <= rowIndex {
		sheet.build.Rows = append(sheet.build.Rows, officeXLSXRow{Height: 20})
	}
	if len(sheet.build.Rows[rowIndex].Cells) <= colIndex {
		grow := make([]officeXLSXCell, colIndex+1)
		copy(grow, sheet.build.Rows[rowIndex].Cells)
		sheet.build.Rows[rowIndex].Cells = grow
	}
	if len(sheet.build.ColWidths) <= colIndex {
		grow := make([]float64, colIndex+1)
		copy(grow, sheet.build.ColWidths)
		for idx := range grow {
			if grow[idx] == 0 {
				grow[idx] = 12
			}
		}
		sheet.build.ColWidths = grow
	}
	if len(sheet.build.ColStyles) <= colIndex {
		grow := make([]int, colIndex+1)
		copy(grow, sheet.build.ColStyles)
		sheet.build.ColStyles = grow
	}
}

func xlsxApplyUpdateCells(sheet *xlsxMutableSheet, raw interface{}) error {
	cellMap, err := xlsxCoerceCellMap(raw)
	if err != nil {
		return err
	}
	columns := xlsxColumnSpecsForSheet(sheet.build)
	for ref, rawValue := range cellMap {
		colIndex, rowNumber, ok := xlsxParseCellRef(ref)
		if !ok {
			return fmt.Errorf("invalid cell reference %q", ref)
		}
		rowIndex := rowNumber - 1
		xlsxEnsureCell(sheet, rowIndex, colIndex)
		cellInput, _ := officeParseStructuredCell(rawValue)
		value := rawValue
		if cellInput.HasShape {
			value = cellInput.Value
		}
		column := officeColumnSpec{Header: fmt.Sprintf("Column %d", colIndex+1)}
		if colIndex < len(columns) {
			column = columns[colIndex]
		}
		style, err := xlsxResolveStyleID(sheet.styles, sheet.build, rowIndex, colIndex, sheet.build.Rows[rowIndex].Cells[colIndex].Style, column, cellInput.Format, value)
		if err != nil {
			return err
		}
		sheet.build.Rows[rowIndex].Cells[colIndex] = officeXLSXCell{
			Value:       value,
			Formula:     cellInput.Formula,
			Type:        cellInput.Type,
			Format:      cellInput.Format,
			Style:       style,
			ForceString: officeShouldWriteStringCell(column, rawValue, cellInput),
		}
	}
	sheet.dirty = true
	return nil
}

func xlsxCoerceCellMap(raw interface{}) (map[string]interface{}, error) {
	if raw == nil {
		return nil, fmt.Errorf("cells are required")
	}
	if values, ok := coerceCompatMap(raw); ok {
		out := make(map[string]interface{}, len(values))
		for key, value := range values {
			out[strings.ToUpper(strings.TrimSpace(key))] = value
		}
		return out, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("cells must be an object or array")
	}
	out := make(map[string]interface{}, len(items))
	for idx, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("cells[%d] must be an object", idx)
		}
		ref := strings.ToUpper(strings.TrimSpace(anyToStringForLLM(firstMapValue(m, "cell", "ref"))))
		if ref == "" {
			return nil, fmt.Errorf("cells[%d] requires cell/ref", idx)
		}
		if value, ok := m["value"]; ok {
			if _, hasFormula := m["formula"]; hasFormula || len(m) > 1 {
				out[ref] = map[string]interface{}{
					"value":   value,
					"formula": m["formula"],
					"type":    m["type"],
					"format":  m["format"],
				}
				continue
			}
			out[ref] = value
			continue
		}
		out[ref] = m
	}
	return out, nil
}

func xlsxSheetRecords(sheet officeXLSXSheetBuild) []map[string]interface{} {
	headers := xlsxHeadersForSheet(sheet)
	if len(headers) == 0 || len(sheet.Rows) <= 1 {
		return nil
	}
	records := make([]map[string]interface{}, 0, len(sheet.Rows)-1)
	for _, row := range sheet.Rows[1:] {
		record := make(map[string]interface{}, len(headers))
		nonEmpty := false
		for idx, header := range headers {
			var value interface{}
			if idx < len(row.Cells) {
				value = row.Cells[idx].Value
				if value == nil {
					value = ""
				}
			}
			record[header] = value
			if strings.TrimSpace(anyToStringForLLM(value)) != "" {
				nonEmpty = true
			}
		}
		if nonEmpty {
			records = append(records, record)
		}
	}
	return records
}

func xlsxSheetFormulaCount(sheet officeXLSXSheetBuild) int {
	count := 0
	for _, row := range sheet.Rows {
		for _, cell := range row.Cells {
			if strings.TrimSpace(cell.Formula) != "" {
				count++
			}
		}
	}
	return count
}

func xlsxSheetColumnKinds(sheet officeXLSXSheetBuild) map[string]string {
	headers := xlsxHeadersForSheet(sheet)
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for idx, header := range headers {
		kinds := make(map[string]struct{})
		for _, row := range sheet.Rows[1:] {
			if idx >= len(row.Cells) {
				continue
			}
			kind := xlsxCellKind(row.Cells[idx])
			if kind != "" {
				kinds[kind] = struct{}{}
			}
		}
		out[header] = xlsxCollapseKinds(kinds)
	}
	return out
}

func xlsxCellKind(cell officeXLSXCell) string {
	if format := officeNormalizeCellFormat(cell.Format); format != "" && format != "general" {
		return format
	}
	switch cell.Value.(type) {
	case nil:
		return ""
	case bool:
		return "boolean"
	case int, int64:
		return "integer"
	case float64:
		return "decimal"
	case string:
		if strings.TrimSpace(cell.Value.(string)) == "" {
			return ""
		}
		return "text"
	default:
		return "text"
	}
}

func xlsxCollapseKinds(kinds map[string]struct{}) string {
	if len(kinds) == 0 {
		return ""
	}
	if len(kinds) == 1 {
		for kind := range kinds {
			return kind
		}
	}
	return "mixed"
}

func xlsxChangedCellCount(left, right officeXLSXSheetBuild) int {
	maxRows := xlsxMax(len(left.Rows), len(right.Rows))
	maxCols := 0
	for _, row := range left.Rows {
		if len(row.Cells) > maxCols {
			maxCols = len(row.Cells)
		}
	}
	for _, row := range right.Rows {
		if len(row.Cells) > maxCols {
			maxCols = len(row.Cells)
		}
	}
	count := 0
	for rowIndex := 0; rowIndex < maxRows; rowIndex++ {
		for colIndex := 0; colIndex < maxCols; colIndex++ {
			leftValue, leftFormula := xlsxCellSnapshot(left, rowIndex, colIndex)
			rightValue, rightFormula := xlsxCellSnapshot(right, rowIndex, colIndex)
			if leftValue != rightValue || leftFormula != rightFormula {
				count++
			}
		}
	}
	return count
}

func xlsxCellSnapshot(sheet officeXLSXSheetBuild, rowIndex, colIndex int) (string, string) {
	if rowIndex >= len(sheet.Rows) || colIndex >= len(sheet.Rows[rowIndex].Cells) {
		return "", ""
	}
	cell := sheet.Rows[rowIndex].Cells[colIndex]
	return anyToStringForLLM(cell.Value), strings.TrimSpace(cell.Formula)
}

func xlsxTopRows(sheet officeXLSXSheetBuild, metric string, limit int) []map[string]interface{} {
	records := xlsxSheetRecords(sheet)
	sort.SliceStable(records, func(i, j int) bool {
		left, _ := officeCellNumber(records[i][metric])
		right, _ := officeCellNumber(records[j][metric])
		if left == right {
			return anyToStringForLLM(records[i][metric]) < anyToStringForLLM(records[j][metric])
		}
		return left > right
	})
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records
}

func xlsxGroupTotals(sheet officeXLSXSheetBuild, groupBy, metric string) map[string]float64 {
	records := xlsxSheetRecords(sheet)
	out := make(map[string]float64)
	for _, record := range records {
		key := strings.TrimSpace(anyToStringForLLM(record[groupBy]))
		if key == "" {
			continue
		}
		value, ok := officeCellNumber(record[metric])
		if !ok {
			continue
		}
		out[key] += value
	}
	return out
}

func xlsxMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}
