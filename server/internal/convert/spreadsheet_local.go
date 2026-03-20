package convert

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const workbookRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

type spreadsheetWorkbook struct {
	Source     string             `json:"source"`
	SheetCount int                `json:"sheet_count"`
	Sheets     []spreadsheetSheet `json:"sheets"`
}

type spreadsheetSheet struct {
	Name     string                   `json:"name"`
	Headers  []string                 `json:"headers,omitempty"`
	Rows     [][]interface{}          `json:"rows"`
	Records  []map[string]interface{} `json:"records,omitempty"`
	RowCount int                      `json:"row_count"`
}

type spreadsheetCell struct {
	Display string
	Value   interface{}
}

type xlsxWorkbookXML struct {
	Sheets []xlsxWorkbookSheetXML `xml:"sheets>sheet"`
}

type xlsxWorkbookSheetXML struct {
	Name  string `xml:"name,attr"`
	RelID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

type xlsxRelationshipsXML struct {
	Relationships []xlsxRelationshipXML `xml:"Relationship"`
}

type xlsxRelationshipXML struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
}

type xlsxSharedStringsXML struct {
	Items []xlsxSharedStringItemXML `xml:"si"`
}

type xlsxSharedStringItemXML struct {
	Text string           `xml:"t"`
	Runs []xlsxTextRunXML `xml:"r"`
}

type xlsxWorksheetXML struct {
	Rows []xlsxRowXML `xml:"sheetData>row"`
}

type xlsxRowXML struct {
	Index int           `xml:"r,attr"`
	Cells []xlsxCellXML `xml:"c"`
}

type xlsxCellXML struct {
	Ref       string            `xml:"r,attr"`
	Type      string            `xml:"t,attr"`
	Value     string            `xml:"v"`
	InlineStr *xlsxInlineStrXML `xml:"is"`
}

type xlsxInlineStrXML struct {
	Text string           `xml:"t"`
	Runs []xlsxTextRunXML `xml:"r"`
}

type xlsxTextRunXML struct {
	Text string `xml:"t"`
}

func convertSpreadsheetLocally(outputDir string, source ResolvedSource, target string) (string, string, bool, error) {
	sourceExt := docExt(source.Path)
	target = normalizeFormat(target, "")
	if sourceExt != "xlsx" {
		return "", "", false, nil
	}
	switch target {
	case "json", "csv", "txt", "md":
	default:
		return "", "", false, nil
	}

	workbook, err := loadSpreadsheetWorkbook(source.Path)
	if err != nil {
		return "", "", true, err
	}

	base := trimExt(source.Name)
	outputPath := filepath.Join(outputDir, base+"."+target)
	var preview string
	switch target {
	case "json":
		preview, err = writeSpreadsheetJSON(outputPath, workbook)
	case "csv":
		preview, err = writeSpreadsheetCSV(outputPath, workbook)
	case "txt":
		preview, err = writeSpreadsheetText(outputPath, workbook)
	case "md":
		preview, err = writeSpreadsheetMarkdown(outputPath, workbook)
	}
	if err != nil {
		return "", "", true, err
	}
	return outputPath, preview, true, nil
}

func loadSpreadsheetWorkbook(xlsxPath string) (*spreadsheetWorkbook, error) {
	reader, err := zip.OpenReader(xlsxPath)
	if err != nil {
		return nil, fmt.Errorf("open xlsx archive: %w", err)
	}
	defer reader.Close()

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
		return nil, fmt.Errorf("decode workbook.xml: %w", err)
	}
	var relsXML xlsxRelationshipsXML
	if err := decodeZipXML(workbookRelsFile, &relsXML); err != nil {
		return nil, fmt.Errorf("decode workbook.xml.rels: %w", err)
	}

	sharedStrings := make([]string, 0)
	if sharedFile, ok := files["xl/sharedStrings.xml"]; ok {
		var sharedXML xlsxSharedStringsXML
		if err := decodeZipXML(sharedFile, &sharedXML); err != nil {
			return nil, fmt.Errorf("decode sharedStrings.xml: %w", err)
		}
		for _, item := range sharedXML.Items {
			sharedStrings = append(sharedStrings, item.fullText())
		}
	}

	relTargets := make(map[string]string, len(relsXML.Relationships))
	for _, rel := range relsXML.Relationships {
		relTargets[rel.ID] = normalizeWorkbookTarget(rel.Target)
	}

	workbook := &spreadsheetWorkbook{
		Source: filepath.Base(xlsxPath),
		Sheets: make([]spreadsheetSheet, 0, len(workbookXML.Sheets)),
	}
	for _, sheetMeta := range workbookXML.Sheets {
		target := relTargets[sheetMeta.RelID]
		if strings.TrimSpace(target) == "" {
			return nil, fmt.Errorf("missing worksheet target for relationship %q", sheetMeta.RelID)
		}
		sheetFile, ok := files[target]
		if !ok {
			return nil, fmt.Errorf("worksheet %q missing from archive", target)
		}
		var worksheetXML xlsxWorksheetXML
		if err := decodeZipXML(sheetFile, &worksheetXML); err != nil {
			return nil, fmt.Errorf("decode worksheet %q: %w", sheetMeta.Name, err)
		}
		rows, err := buildSpreadsheetRows(worksheetXML.Rows, sharedStrings)
		if err != nil {
			return nil, fmt.Errorf("parse worksheet %q: %w", sheetMeta.Name, err)
		}
		sheet := spreadsheetSheet{
			Name:     sheetMeta.Name,
			Rows:     convertSpreadsheetRowsToInterfaces(rows),
			RowCount: len(rows),
		}
		if len(rows) > 0 {
			sheet.Headers = uniqueSpreadsheetHeaders(rows[0])
			sheet.Records = spreadsheetRecordsFromRows(rows)
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	workbook.SheetCount = len(workbook.Sheets)
	return workbook, nil
}

func decodeZipXML(file *zip.File, out interface{}) error {
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	return xml.NewDecoder(rc).Decode(out)
}

func normalizeWorkbookTarget(target string) string {
	target = strings.TrimSpace(target)
	target = strings.TrimPrefix(target, "/")
	if strings.HasPrefix(target, "xl/") {
		return path.Clean(target)
	}
	return path.Clean(path.Join("xl", target))
}

func buildSpreadsheetRows(xmlRows []xlsxRowXML, sharedStrings []string) ([][]spreadsheetCell, error) {
	rows := make([][]spreadsheetCell, 0, len(xmlRows))
	for _, rowXML := range xmlRows {
		maxCol := 0
		indexed := make(map[int]spreadsheetCell, len(rowXML.Cells))
		nextCol := 0
		for _, cellXML := range rowXML.Cells {
			col := nextCol
			if parsed, ok := spreadsheetColumnIndex(cellXML.Ref); ok {
				col = parsed
			}
			cell, err := decodeSpreadsheetCell(cellXML, sharedStrings)
			if err != nil {
				return nil, err
			}
			indexed[col] = cell
			if col+1 > maxCol {
				maxCol = col + 1
			}
			nextCol = col + 1
		}
		row := make([]spreadsheetCell, maxCol)
		for idx := 0; idx < maxCol; idx++ {
			row[idx] = indexed[idx]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func decodeSpreadsheetCell(cell xlsxCellXML, sharedStrings []string) (spreadsheetCell, error) {
	text := spreadsheetCellText(cell, sharedStrings)
	switch strings.ToLower(strings.TrimSpace(cell.Type)) {
	case "b":
		value := strings.TrimSpace(cell.Value) == "1"
		if value {
			return spreadsheetCell{Display: "true", Value: true}, nil
		}
		return spreadsheetCell{Display: "false", Value: false}, nil
	case "n", "":
		if strings.TrimSpace(cell.Value) == "" {
			return spreadsheetCell{}, nil
		}
		if i, err := strconv.ParseInt(strings.TrimSpace(cell.Value), 10, 64); err == nil {
			return spreadsheetCell{Display: strconv.FormatInt(i, 10), Value: i}, nil
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(cell.Value), 64); err == nil {
			return spreadsheetCell{Display: strings.TrimSpace(cell.Value), Value: f}, nil
		}
	case "inlinestr", "s", "str", "e":
		return spreadsheetCell{Display: text, Value: text}, nil
	default:
		if text != "" {
			return spreadsheetCell{Display: text, Value: text}, nil
		}
	}
	if text == "" {
		return spreadsheetCell{}, nil
	}
	return spreadsheetCell{Display: text, Value: text}, nil
}

func spreadsheetCellText(cell xlsxCellXML, sharedStrings []string) string {
	switch strings.ToLower(strings.TrimSpace(cell.Type)) {
	case "inlinestr":
		if cell.InlineStr != nil {
			return cell.InlineStr.fullText()
		}
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(cell.Value))
		if err == nil && idx >= 0 && idx < len(sharedStrings) {
			return sharedStrings[idx]
		}
	case "str", "e":
		return strings.TrimSpace(cell.Value)
	}
	if cell.InlineStr != nil {
		if text := cell.InlineStr.fullText(); text != "" {
			return text
		}
	}
	return strings.TrimSpace(cell.Value)
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

func (i xlsxSharedStringItemXML) fullText() string {
	if strings.TrimSpace(i.Text) != "" || len(i.Runs) == 0 {
		return i.Text
	}
	var sb strings.Builder
	for _, run := range i.Runs {
		sb.WriteString(run.Text)
	}
	return sb.String()
}

func spreadsheetColumnIndex(ref string) (int, bool) {
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

func convertSpreadsheetRowsToInterfaces(rows [][]spreadsheetCell) [][]interface{} {
	out := make([][]interface{}, 0, len(rows))
	for _, row := range rows {
		values := make([]interface{}, 0, len(row))
		for _, cell := range row {
			if cell.Value == nil {
				values = append(values, "")
				continue
			}
			values = append(values, cell.Value)
		}
		out = append(out, values)
	}
	return out
}

func uniqueSpreadsheetHeaders(headerRow []spreadsheetCell) []string {
	headers := make([]string, 0, len(headerRow))
	seen := make(map[string]int, len(headerRow))
	for idx, cell := range headerRow {
		base := strings.TrimSpace(cell.Display)
		if base == "" {
			base = fmt.Sprintf("Column_%d", idx+1)
		}
		seen[base]++
		name := base
		if count := seen[base]; count > 1 {
			name = fmt.Sprintf("%s_%d", base, count)
		}
		headers = append(headers, name)
	}
	return headers
}

func spreadsheetRecordsFromRows(rows [][]spreadsheetCell) []map[string]interface{} {
	if len(rows) <= 1 {
		return nil
	}
	headers := uniqueSpreadsheetHeaders(rows[0])
	records := make([]map[string]interface{}, 0, len(rows)-1)
	for _, row := range rows[1:] {
		record := make(map[string]interface{}, len(headers))
		nonEmpty := false
		for idx, header := range headers {
			if idx >= len(row) || row[idx].Value == nil || row[idx].Display == "" {
				record[header] = ""
				continue
			}
			record[header] = row[idx].Value
			nonEmpty = true
		}
		if nonEmpty {
			records = append(records, record)
		}
	}
	return records
}

func writeSpreadsheetJSON(path string, workbook *spreadsheetWorkbook) (string, error) {
	data, err := json.MarshalIndent(workbook, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", err
	}
	return spreadsheetPreview(workbook), nil
}

func writeSpreadsheetCSV(path string, workbook *spreadsheetWorkbook) (string, error) {
	var buf bytes.Buffer
	for idx, sheet := range workbook.Sheets {
		if idx > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString("# Sheet: ")
		buf.WriteString(sheet.Name)
		buf.WriteString("\n")
		writer := csv.NewWriter(&buf)
		for _, row := range sheet.Rows {
			record := make([]string, 0, len(row))
			for _, cell := range row {
				record = append(record, spreadsheetString(cell))
			}
			if err := writer.Write(record); err != nil {
				return "", err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return "", err
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o640); err != nil {
		return "", err
	}
	return spreadsheetPreview(workbook), nil
}

func writeSpreadsheetText(path string, workbook *spreadsheetWorkbook) (string, error) {
	var buf bytes.Buffer
	for idx, sheet := range workbook.Sheets {
		if idx > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString("Sheet: ")
		buf.WriteString(sheet.Name)
		buf.WriteString("\n")
		for _, row := range sheet.Rows {
			cells := make([]string, 0, len(row))
			for _, cell := range row {
				cells = append(cells, spreadsheetString(cell))
			}
			buf.WriteString(strings.Join(cells, "\t"))
			buf.WriteString("\n")
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o640); err != nil {
		return "", err
	}
	return spreadsheetPreview(workbook), nil
}

func writeSpreadsheetMarkdown(path string, workbook *spreadsheetWorkbook) (string, error) {
	var buf bytes.Buffer
	for idx, sheet := range workbook.Sheets {
		if idx > 0 {
			buf.WriteString("\n\n")
		}
		buf.WriteString("## ")
		buf.WriteString(sheet.Name)
		buf.WriteString("\n\n")
		if len(sheet.Rows) == 0 {
			buf.WriteString("_No rows_\n")
			continue
		}
		headers := sheet.Rows[0]
		buf.WriteString("| ")
		for _, cell := range headers {
			buf.WriteString(markdownEscape(spreadsheetString(cell)))
			buf.WriteString(" | ")
		}
		buf.WriteString("\n| ")
		for range headers {
			buf.WriteString("--- | ")
		}
		buf.WriteString("\n")
		for _, row := range sheet.Rows[1:] {
			buf.WriteString("| ")
			for idx := range headers {
				var cell interface{}
				if idx < len(row) {
					cell = row[idx]
				}
				buf.WriteString(markdownEscape(spreadsheetString(cell)))
				buf.WriteString(" | ")
			}
			buf.WriteString("\n")
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o640); err != nil {
		return "", err
	}
	return spreadsheetPreview(workbook), nil
}

func spreadsheetString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(v)
	}
}

func markdownEscape(value string) string {
	replacer := strings.NewReplacer("|", "\\|", "\n", "<br>")
	return replacer.Replace(value)
}

func spreadsheetPreview(workbook *spreadsheetWorkbook) string {
	if workbook == nil || len(workbook.Sheets) == 0 {
		return "Spreadsheet converted"
	}
	parts := make([]string, 0, len(workbook.Sheets))
	for _, sheet := range workbook.Sheets {
		rowCount := sheet.RowCount
		if rowCount > 0 {
			rowCount--
		}
		parts = append(parts, fmt.Sprintf("%s (%d data rows)", sheet.Name, rowCount))
	}
	return "Converted spreadsheet with sheets: " + strings.Join(parts, ", ")
}
