package convert

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const workbookRelNS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

type spreadsheetWorkbook struct {
	Source     string                  `json:"source"`
	SheetCount int                     `json:"sheet_count"`
	Summary    *TabularWorkbookSummary `json:"summary,omitempty"`
	Sheets     []spreadsheetSheet      `json:"sheets"`
}

type spreadsheetSheet struct {
	Name         string                   `json:"name"`
	Headers      []string                 `json:"headers,omitempty"`
	Summary      *TabularSummary          `json:"summary,omitempty"`
	Rows         [][]interface{}          `json:"rows"`
	Records      []map[string]interface{} `json:"records,omitempty"`
	RowCount     int                      `json:"row_count"`
	FormulaCount int                      `json:"formula_count,omitempty"`
	DateColumns  []string                 `json:"date_columns,omitempty"`
	ColumnKinds  map[string]string        `json:"column_kinds,omitempty"`
}

type spreadsheetCell struct {
	Display      string
	Value        interface{}
	RawValue     string
	Formula      string
	StyleID      int
	Kind         string
	NumberFormat string
	IsDate       bool
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

type xlsxStylesXML struct {
	NumFmts xlsxNumFmtsXML `xml:"numFmts"`
	CellXfs xlsxCellXfsXML `xml:"cellXfs"`
}

type xlsxNumFmtsXML struct {
	Items []xlsxNumFmtXML `xml:"numFmt"`
}

type xlsxNumFmtXML struct {
	ID   int    `xml:"numFmtId,attr"`
	Code string `xml:"formatCode,attr"`
}

type xlsxCellXfsXML struct {
	Items []xlsxCellXFXML `xml:"xf"`
}

type xlsxCellXFXML struct {
	NumFmtID int `xml:"numFmtId,attr"`
}

type spreadsheetStyleInfo struct {
	Kind         string
	NumberFormat string
	IsDate       bool
}

type spreadsheetSharedFormula struct {
	CellRef string
	Text    string
}

type derivedSpreadsheetSheetData struct {
	headers       []string
	interfaceRows [][]interface{}
	records       []map[string]interface{}
	formulaCount  int
	dateColumns   []string
	columnKinds   map[string]string
}

var spreadsheetCellRefPattern = regexp.MustCompile(`(?i)(\$?)([A-Z]{1,3})(\$?)([0-9]+)`)

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
	if sourceExt != "xlsx" && sourceExt != "csv" && sourceExt != "tsv" {
		return "", "", false, nil
	}
	switch target {
	case "json", "csv", "txt", "md":
	default:
		return "", "", false, nil
	}
	if sourceExt != "xlsx" && target == "csv" {
		return "", "", false, nil
	}

	var (
		workbook *spreadsheetWorkbook
		err      error
	)
	switch sourceExt {
	case "xlsx":
		workbook, err = loadSpreadsheetWorkbook(source.Path)
	case "csv", "tsv":
		workbook, err = loadDelimitedWorkbook(source.Path)
	}
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

func loadDelimitedWorkbook(sourcePath string) (*spreadsheetWorkbook, error) {
	file, err := os.Open(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("open delimited file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	if docExt(sourcePath) == "tsv" {
		reader.Comma = '\t'
	}
	rawRows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read delimited file: %w", err)
	}

	maxCols := 0
	for _, rawRow := range rawRows {
		if len(rawRow) > maxCols {
			maxCols = len(rawRow)
		}
	}
	rows := make([][]spreadsheetCell, 0, len(rawRows))
	for _, rawRow := range rawRows {
		row := make([]spreadsheetCell, maxCols)
		for idx, value := range rawRow {
			row[idx] = decodeDelimitedSpreadsheetCell(value)
		}
		rows = append(rows, row)
	}

	sheetName := trimExt(filepath.Base(sourcePath))
	if strings.TrimSpace(sheetName) == "" {
		sheetName = "Sheet1"
	}
	sheet := spreadsheetSheet{
		Name:     sheetName,
		Rows:     convertSpreadsheetRowsToInterfaces(rows),
		RowCount: len(rows),
	}
	if len(rows) > 0 {
		sheet.Headers = uniqueSpreadsheetHeaders(rows[0])
		sheet.Records = spreadsheetRecordsFromRows(rows, sheet.Headers)
		sheet.Summary = summarizeTabularRecords(sheet.Name, sheet.Headers, sheet.Records)
	}

	workbook := &spreadsheetWorkbook{
		Source: filepath.Base(sourcePath),
		Sheets: []spreadsheetSheet{sheet},
	}
	workbook.SheetCount = len(workbook.Sheets)
	workbook.Summary = summarizeWorkbookSheets(workbook.Sheets)
	return workbook, nil
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
		parsed, err := parseSpreadsheetSharedStrings(sharedFile)
		if err != nil {
			return nil, fmt.Errorf("decode sharedStrings.xml: %w", err)
		}
		sharedStrings = parsed
	}

	styles := make(map[int]spreadsheetStyleInfo)
	if stylesFile, ok := files["xl/styles.xml"]; ok {
		parsed, err := decodeSpreadsheetStyles(stylesFile)
		if err != nil {
			return nil, fmt.Errorf("decode styles.xml: %w", err)
		}
		styles = parsed
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
		rows, err := parseSpreadsheetWorksheet(sheetFile, sharedStrings, styles)
		if err != nil {
			return nil, fmt.Errorf("parse worksheet %q: %w", sheetMeta.Name, err)
		}
		derived := deriveSpreadsheetSheetData(rows)
		sheet := spreadsheetSheet{
			Name:     sheetMeta.Name,
			Rows:     derived.interfaceRows,
			RowCount: len(rows),
		}
		if len(rows) > 0 {
			sheet.Headers = derived.headers
			sheet.Records = derived.records
			sheet.Summary = summarizeTabularRecords(sheet.Name, sheet.Headers, sheet.Records)
			sheet.FormulaCount = derived.formulaCount
			sheet.DateColumns = derived.dateColumns
			sheet.ColumnKinds = derived.columnKinds
			if sheet.Summary != nil {
				sheet.Summary.FormulaCount = sheet.FormulaCount
				sheet.Summary.DateColumns = append([]string(nil), sheet.DateColumns...)
				sheet.Summary.ColumnKinds = cloneSpreadsheetStringMap(sheet.ColumnKinds)
			}
		}
		workbook.Sheets = append(workbook.Sheets, sheet)
	}
	workbook.SheetCount = len(workbook.Sheets)
	workbook.Summary = summarizeWorkbookSheets(workbook.Sheets)
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

func decodeSpreadsheetStyles(file *zip.File) (map[int]spreadsheetStyleInfo, error) {
	var stylesXML xlsxStylesXML
	if err := decodeZipXML(file, &stylesXML); err != nil {
		return nil, err
	}

	numFmtCodes := make(map[int]string, len(stylesXML.NumFmts.Items))
	for _, item := range stylesXML.NumFmts.Items {
		numFmtCodes[item.ID] = strings.TrimSpace(item.Code)
	}

	out := make(map[int]spreadsheetStyleInfo, len(stylesXML.CellXfs.Items))
	for idx, xf := range stylesXML.CellXfs.Items {
		kind, isDate, format := classifySpreadsheetNumberFormat(xf.NumFmtID, numFmtCodes[xf.NumFmtID])
		out[idx] = spreadsheetStyleInfo{
			Kind:         kind,
			NumberFormat: format,
			IsDate:       isDate,
		}
	}
	return out, nil
}

func parseSpreadsheetSharedStrings(file *zip.File) ([]string, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := readSpreadsheetBytes(rc, int(file.UncompressedSize64))
	if err != nil {
		return nil, err
	}
	if values, err := parseSpreadsheetSharedStringsBytes(data); err == nil {
		return values, nil
	}
	return parseSpreadsheetSharedStringsXMLDecoder(bytes.NewReader(data))
}

func parseSpreadsheetSharedStringsXML(r io.Reader) ([]string, error) {
	data, err := readSpreadsheetBytes(r, 0)
	if err != nil {
		return nil, err
	}
	if values, err := parseSpreadsheetSharedStringsBytes(data); err == nil {
		return values, nil
	}
	return parseSpreadsheetSharedStringsXMLDecoder(bytes.NewReader(data))
}

func parseSpreadsheetSharedStringsXMLDecoder(r io.Reader) ([]string, error) {
	decoder := xml.NewDecoder(r)
	values := make([]string, 0, 128)
	var (
		inItem bool
		inText bool
		buf    bytes.Buffer
	)

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return values, nil
			}
			return nil, err
		}

		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "si":
				inItem = true
				inText = false
				buf.Reset()
			case "t":
				if inItem {
					inText = true
				}
			}
		case xml.EndElement:
			switch typed.Name.Local {
			case "t":
				inText = false
			case "si":
				if inItem {
					values = append(values, buf.String())
					buf.Reset()
				}
				inItem = false
				inText = false
			}
		case xml.CharData:
			if inItem && inText {
				_, _ = buf.Write(typed)
			}
		}
	}
}

func parseSpreadsheetWorksheet(file *zip.File, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := readSpreadsheetBytes(rc, int(file.UncompressedSize64))
	if err != nil {
		return nil, err
	}
	return parseSpreadsheetWorksheetBytesWithFallback(data, sharedStrings, styles)
}

func parseSpreadsheetWorksheetXML(r io.Reader, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	data, err := readSpreadsheetBytes(r, 0)
	if err != nil {
		return nil, err
	}
	return parseSpreadsheetWorksheetBytesWithFallback(data, sharedStrings, styles)
}

func parseSpreadsheetWorksheetBytesWithFallback(data []byte, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	if rows, err := parseSpreadsheetWorksheetBytes(data, sharedStrings, styles); err == nil {
		return rows, nil
	}
	return parseSpreadsheetWorksheetXMLDecoder(bytes.NewReader(data), sharedStrings, styles)
}

func parseSpreadsheetWorksheetXMLDecoder(r io.Reader, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	decoder := xml.NewDecoder(r)
	rows := make([][]spreadsheetCell, 0, 128)
	sharedFormulae := make(map[int]spreadsheetSharedFormula)
	inSheetData := false
	rowWidthHint := 0

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return rows, nil
			}
			return nil, err
		}

		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "sheetData":
				inSheetData = true
			case "row":
				if !inSheetData {
					continue
				}
				row, err := parseSpreadsheetRow(decoder, typed, len(rows)+1, rowWidthHint, sharedStrings, styles, sharedFormulae)
				if err != nil {
					return nil, err
				}
				if row == nil {
					row = make([]spreadsheetCell, 0)
				}
				if len(row) > rowWidthHint {
					rowWidthHint = len(row)
				}
				rows = append(rows, row)
			}
		case xml.EndElement:
			if typed.Name.Local == "sheetData" {
				inSheetData = false
			}
		}
	}
}

func parseSpreadsheetWorksheetBytes(data []byte, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	sheetData, err := spreadsheetElementInnerXML(data, "sheetData")
	if err != nil {
		return nil, err
	}
	if bytes.Contains(sheetData, []byte(":row")) || bytes.Contains(sheetData, []byte(":c")) {
		return nil, fmt.Errorf("worksheet uses prefixed row/c elements")
	}

	rows := make([][]spreadsheetCell, 0, 128)
	sharedFormulae := make(map[int]spreadsheetSharedFormula)
	rowWidthHint := 0
	offset := 0
	for offset < len(sheetData) {
		rowStart := bytes.Index(sheetData[offset:], []byte("<row"))
		if rowStart < 0 {
			break
		}
		rowStart += offset
		row, nextOffset, err := parseSpreadsheetRowBytes(sheetData, rowStart, len(rows)+1, rowWidthHint, sharedStrings, styles, sharedFormulae)
		if err != nil {
			return nil, err
		}
		if len(row) > rowWidthHint {
			rowWidthHint = len(row)
		}
		rows = append(rows, row)
		offset = nextOffset
	}
	return rows, nil
}

func parseSpreadsheetRowBytes(data []byte, rowStart, fallbackRowIndex, widthHint int, sharedStrings []string, styles map[int]spreadsheetStyleInfo, sharedFormulae map[int]spreadsheetSharedFormula) ([]spreadsheetCell, int, error) {
	rowOpenEnd, rowSelfClosing, err := spreadsheetFindTagEnd(data, rowStart)
	if err != nil {
		return nil, 0, err
	}
	rowAttrs := data[rowStart+len("<row") : rowOpenEnd]
	rowIndex := spreadsheetParseIntAttrBytes(rowAttrs, "r", fallbackRowIndex)
	if rowIndex <= 0 {
		rowIndex = fallbackRowIndex
	}
	if rowSelfClosing {
		return make([]spreadsheetCell, 0), rowOpenEnd + 1, nil
	}

	rowCloseStart := bytes.Index(data[rowOpenEnd+1:], []byte("</row>"))
	if rowCloseStart < 0 {
		return nil, 0, fmt.Errorf("row close tag missing")
	}
	rowCloseStart += rowOpenEnd + 1
	rowContent := data[rowOpenEnd+1 : rowCloseStart]

	var (
		row      []spreadsheetCell
		rowWidth int
		nextCol  int
	)
	if widthHint > 0 {
		row = make([]spreadsheetCell, widthHint)
	}

	offset := 0
	for offset < len(rowContent) {
		cellStart := bytes.Index(rowContent[offset:], []byte("<c"))
		if cellStart < 0 {
			break
		}
		cellStart += offset
		colIndex, cell, nextOffset, err := parseSpreadsheetCellBytes(rowContent, cellStart, rowIndex, nextCol, sharedStrings, styles, sharedFormulae)
		if err != nil {
			return nil, 0, err
		}
		if colIndex >= len(row) {
			size := len(row)
			if size == 0 {
				size = 4
			}
			for size <= colIndex {
				size *= 2
			}
			grown := make([]spreadsheetCell, size)
			copy(grown, row)
			row = grown
		}
		row[colIndex] = cell
		if colIndex+1 > rowWidth {
			rowWidth = colIndex + 1
		}
		nextCol = colIndex + 1
		offset = nextOffset
	}

	if row == nil {
		row = make([]spreadsheetCell, 0)
	} else {
		row = row[:rowWidth]
	}
	return row, rowCloseStart + len("</row>"), nil
}

func parseSpreadsheetCellBytes(data []byte, cellStart, rowIndex, nextCol int, sharedStrings []string, styles map[int]spreadsheetStyleInfo, sharedFormulae map[int]spreadsheetSharedFormula) (int, spreadsheetCell, int, error) {
	cellOpenEnd, cellSelfClosing, err := spreadsheetFindTagEnd(data, cellStart)
	if err != nil {
		return 0, spreadsheetCell{}, 0, err
	}
	cellAttrs := data[cellStart+len("<c") : cellOpenEnd]
	cellRefBytes, _ := spreadsheetAttrValueBytes(cellAttrs, "r")
	cellTypeBytes, _ := spreadsheetAttrValueBytes(cellAttrs, "t")
	styleID := spreadsheetParseIntAttrBytes(cellAttrs, "s", 0)

	cellRef := string(cellRefBytes)
	colIndex := nextCol
	if parsed, ok := spreadsheetColumnIndex(cellRef); ok {
		colIndex = parsed
	}

	if cellSelfClosing {
		if strings.TrimSpace(cellRef) == "" {
			cellRef = spreadsheetCellRef(colIndex, rowIndex)
		}
		cell, err := decodeSpreadsheetCellValue("", "", "", "", 0, "", styleID, cellRef, styles[styleID], sharedStrings, sharedFormulae)
		if err != nil {
			return 0, spreadsheetCell{}, 0, err
		}
		return colIndex, cell, cellOpenEnd + 1, nil
	}

	cellCloseStart := bytes.Index(data[cellOpenEnd+1:], []byte("</c>"))
	if cellCloseStart < 0 {
		return 0, spreadsheetCell{}, 0, fmt.Errorf("cell close tag missing")
	}
	cellCloseStart += cellOpenEnd + 1
	cellContent := data[cellOpenEnd+1 : cellCloseStart]

	formulaType := ""
	formulaSharedIndex := 0
	formulaText := ""
	if fStart := bytes.Index(cellContent, []byte("<f")); fStart >= 0 {
		fOpenEnd, fSelfClosing, err := spreadsheetFindTagEnd(cellContent, fStart)
		if err != nil {
			return 0, spreadsheetCell{}, 0, err
		}
		fAttrs := cellContent[fStart+len("<f") : fOpenEnd]
		if value, ok := spreadsheetAttrValueBytes(fAttrs, "t"); ok {
			formulaType = string(value)
		}
		formulaSharedIndex = spreadsheetParseIntAttrBytes(fAttrs, "si", 0)
		if !fSelfClosing {
			fCloseStart := bytes.Index(cellContent[fOpenEnd+1:], []byte("</f>"))
			if fCloseStart < 0 {
				return 0, spreadsheetCell{}, 0, fmt.Errorf("formula close tag missing")
			}
			fCloseStart += fOpenEnd + 1
			formulaText = spreadsheetXMLText(cellContent[fOpenEnd+1 : fCloseStart])
		}
	}

	value := ""
	if vStart := bytes.Index(cellContent, []byte("<v>")); vStart >= 0 {
		vEnd := bytes.Index(cellContent[vStart+len("<v>"):], []byte("</v>"))
		if vEnd < 0 {
			return 0, spreadsheetCell{}, 0, fmt.Errorf("value close tag missing")
		}
		vEnd += vStart + len("<v>")
		value = spreadsheetXMLText(cellContent[vStart+len("<v>") : vEnd])
	}

	inlineText := ""
	if isStart := bytes.Index(cellContent, []byte("<is")); isStart >= 0 {
		isOpenEnd, isSelfClosing, err := spreadsheetFindTagEnd(cellContent, isStart)
		if err != nil {
			return 0, spreadsheetCell{}, 0, err
		}
		if !isSelfClosing {
			isCloseStart := bytes.Index(cellContent[isOpenEnd+1:], []byte("</is>"))
			if isCloseStart < 0 {
				return 0, spreadsheetCell{}, 0, fmt.Errorf("inline string close tag missing")
			}
			isCloseStart += isOpenEnd + 1
			inlineText, err = spreadsheetInlineStringTextBytes(cellContent[isOpenEnd+1 : isCloseStart])
			if err != nil {
				return 0, spreadsheetCell{}, 0, err
			}
		}
	}

	if strings.TrimSpace(cellRef) == "" {
		cellRef = spreadsheetCellRef(colIndex, rowIndex)
	}
	cell, err := decodeSpreadsheetCellValue(
		string(cellTypeBytes),
		value,
		inlineText,
		formulaType,
		formulaSharedIndex,
		formulaText,
		styleID,
		cellRef,
		styles[styleID],
		sharedStrings,
		sharedFormulae,
	)
	if err != nil {
		return 0, spreadsheetCell{}, 0, err
	}
	return colIndex, cell, cellCloseStart + len("</c>"), nil
}

func parseSpreadsheetSharedStringsBytes(data []byte) ([]string, error) {
	if bytes.Contains(data, []byte(":si")) || bytes.Contains(data, []byte(":t")) {
		return nil, fmt.Errorf("shared strings use prefixed si/t elements")
	}
	values := make([]string, 0, 128)
	offset := 0
	for offset < len(data) {
		siStart := bytes.Index(data[offset:], []byte("<si"))
		if siStart < 0 {
			break
		}
		siStart += offset
		siOpenEnd, siSelfClosing, err := spreadsheetFindTagEnd(data, siStart)
		if err != nil {
			return nil, err
		}
		if siSelfClosing {
			values = append(values, "")
			offset = siOpenEnd + 1
			continue
		}
		siCloseStart := bytes.Index(data[siOpenEnd+1:], []byte("</si>"))
		if siCloseStart < 0 {
			return nil, fmt.Errorf("shared string close tag missing")
		}
		siCloseStart += siOpenEnd + 1
		text, err := spreadsheetInlineStringTextBytes(data[siOpenEnd+1 : siCloseStart])
		if err != nil {
			return nil, err
		}
		values = append(values, text)
		offset = siCloseStart + len("</si>")
	}
	return values, nil
}

func readSpreadsheetBytes(r io.Reader, sizeHint int) ([]byte, error) {
	var buf bytes.Buffer
	if sizeHint > 0 {
		buf.Grow(sizeHint)
	}
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func spreadsheetElementInnerXML(data []byte, name string) ([]byte, error) {
	open := []byte("<" + name)
	start := bytes.Index(data, open)
	if start < 0 {
		return nil, fmt.Errorf("%s start tag missing", name)
	}
	openEnd, selfClosing, err := spreadsheetFindTagEnd(data, start)
	if err != nil {
		return nil, err
	}
	if selfClosing {
		return nil, nil
	}
	closeTag := []byte("</" + name + ">")
	closeStart := bytes.Index(data[openEnd+1:], closeTag)
	if closeStart < 0 {
		return nil, fmt.Errorf("%s close tag missing", name)
	}
	closeStart += openEnd + 1
	return data[openEnd+1 : closeStart], nil
}

func spreadsheetFindTagEnd(data []byte, start int) (int, bool, error) {
	inQuote := byte(0)
	for idx := start + 1; idx < len(data); idx++ {
		switch ch := data[idx]; ch {
		case '"', '\'':
			if inQuote == 0 {
				inQuote = ch
			} else if inQuote == ch {
				inQuote = 0
			}
		case '>':
			if inQuote != 0 {
				continue
			}
			prev := idx - 1
			for prev > start && isSpreadsheetXMLSpace(data[prev]) {
				prev--
			}
			return idx, prev > start && data[prev] == '/', nil
		}
	}
	return 0, false, fmt.Errorf("tag end missing")
}

func spreadsheetAttrValueBytes(tag []byte, name string) ([]byte, bool) {
	offset := 0
	for offset < len(tag) {
		for offset < len(tag) && isSpreadsheetXMLSpace(tag[offset]) {
			offset++
		}
		if offset >= len(tag) {
			return nil, false
		}
		nameStart := offset
		for offset < len(tag) && !isSpreadsheetXMLSpace(tag[offset]) && tag[offset] != '=' && tag[offset] != '/' {
			offset++
		}
		attrName := bytes.TrimSpace(tag[nameStart:offset])
		for offset < len(tag) && isSpreadsheetXMLSpace(tag[offset]) {
			offset++
		}
		if offset >= len(tag) || tag[offset] != '=' {
			for offset < len(tag) && tag[offset] != ' ' && tag[offset] != '\t' && tag[offset] != '\n' && tag[offset] != '\r' {
				offset++
			}
			continue
		}
		offset++
		for offset < len(tag) && isSpreadsheetXMLSpace(tag[offset]) {
			offset++
		}
		if offset >= len(tag) {
			return nil, false
		}
		quote := tag[offset]
		if quote != '"' && quote != '\'' {
			return nil, false
		}
		offset++
		valueStart := offset
		for offset < len(tag) && tag[offset] != quote {
			offset++
		}
		if offset >= len(tag) {
			return nil, false
		}
		if bytes.Equal(attrName, []byte(name)) {
			return tag[valueStart:offset], true
		}
		offset++
	}
	return nil, false
}

func spreadsheetParseIntAttrBytes(tag []byte, name string, fallback int) int {
	value, ok := spreadsheetAttrValueBytes(tag, name)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(string(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func spreadsheetInlineStringTextBytes(data []byte) (string, error) {
	var buf strings.Builder
	offset := 0
	for offset < len(data) {
		tStart := bytes.Index(data[offset:], []byte("<t"))
		if tStart < 0 {
			break
		}
		tStart += offset
		tOpenEnd, tSelfClosing, err := spreadsheetFindTagEnd(data, tStart)
		if err != nil {
			return "", err
		}
		if tSelfClosing {
			offset = tOpenEnd + 1
			continue
		}
		tCloseStart := bytes.Index(data[tOpenEnd+1:], []byte("</t>"))
		if tCloseStart < 0 {
			return "", fmt.Errorf("inline text close tag missing")
		}
		tCloseStart += tOpenEnd + 1
		buf.WriteString(spreadsheetXMLText(data[tOpenEnd+1 : tCloseStart]))
		offset = tCloseStart + len("</t>")
	}
	return buf.String(), nil
}

func spreadsheetXMLText(raw []byte) string {
	if bytes.IndexByte(raw, '&') < 0 {
		return string(raw)
	}
	var buf strings.Builder
	buf.Grow(len(raw))
	for idx := 0; idx < len(raw); idx++ {
		if raw[idx] != '&' {
			buf.WriteByte(raw[idx])
			continue
		}
		semi := bytes.IndexByte(raw[idx:], ';')
		if semi <= 0 {
			buf.WriteByte(raw[idx])
			continue
		}
		semi += idx
		if text, ok := spreadsheetXMLEntity(raw[idx+1 : semi]); ok {
			buf.WriteString(text)
			idx = semi
			continue
		}
		buf.Write(raw[idx : semi+1])
		idx = semi
	}
	return buf.String()
}

func spreadsheetXMLEntity(entity []byte) (string, bool) {
	switch string(entity) {
	case "amp":
		return "&", true
	case "lt":
		return "<", true
	case "gt":
		return ">", true
	case "quot":
		return `"`, true
	case "apos":
		return `'`, true
	}
	if len(entity) >= 2 && entity[0] == '#' {
		base := 10
		digits := entity[1:]
		if len(entity) >= 3 && (entity[1] == 'x' || entity[1] == 'X') {
			base = 16
			digits = entity[2:]
		}
		value, err := strconv.ParseInt(string(digits), base, 32)
		if err == nil {
			return string(rune(value)), true
		}
	}
	return "", false
}

func isSpreadsheetXMLSpace(ch byte) bool {
	switch ch {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func parseSpreadsheetRow(decoder *xml.Decoder, start xml.StartElement, fallbackRowIndex int, widthHint int, sharedStrings []string, styles map[int]spreadsheetStyleInfo, sharedFormulae map[int]spreadsheetSharedFormula) ([]spreadsheetCell, error) {
	rowIndex := parseSpreadsheetIntAttr(start.Attr, "r", fallbackRowIndex)
	if rowIndex <= 0 {
		rowIndex = fallbackRowIndex
	}

	var (
		row      []spreadsheetCell
		rowWidth int
		nextCol  int
	)
	if widthHint > 0 {
		row = make([]spreadsheetCell, widthHint)
	}

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return nil, io.ErrUnexpectedEOF
			}
			return nil, err
		}

		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local != "c" {
				if err := skipSpreadsheetElement(decoder, typed.Name.Local); err != nil {
					return nil, err
				}
				continue
			}
			colIndex, cell, err := parseSpreadsheetCellElement(decoder, typed, rowIndex, nextCol, sharedStrings, styles, sharedFormulae)
			if err != nil {
				return nil, err
			}
			if colIndex >= len(row) {
				size := len(row)
				if size == 0 {
					size = 4
				}
				for size <= colIndex {
					size *= 2
				}
				grown := make([]spreadsheetCell, size)
				copy(grown, row)
				row = grown
			}
			row[colIndex] = cell
			if colIndex+1 > rowWidth {
				rowWidth = colIndex + 1
			}
			nextCol = colIndex + 1
		case xml.EndElement:
			if typed.Name.Local == "row" {
				if row == nil {
					return make([]spreadsheetCell, 0), nil
				}
				return row[:rowWidth], nil
			}
		}
	}
}

func parseSpreadsheetCellElement(decoder *xml.Decoder, start xml.StartElement, rowIndex int, nextCol int, sharedStrings []string, styles map[int]spreadsheetStyleInfo, sharedFormulae map[int]spreadsheetSharedFormula) (int, spreadsheetCell, error) {
	cellRef := ""
	cellType := ""
	styleID := 0
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "r":
			cellRef = attr.Value
		case "t":
			cellType = attr.Value
		case "s":
			if parsed, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err == nil {
				styleID = parsed
			}
		}
	}

	colIndex := nextCol
	if parsed, ok := spreadsheetColumnIndex(cellRef); ok {
		colIndex = parsed
	}

	var (
		value              string
		inlineText         string
		formulaType        string
		formulaSharedIndex int
		formulaText        string
	)

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return 0, spreadsheetCell{}, io.ErrUnexpectedEOF
			}
			return 0, spreadsheetCell{}, err
		}

		switch typed := token.(type) {
		case xml.StartElement:
			switch typed.Name.Local {
			case "f":
				parsedFormulaType, parsedSharedIndex, parsedFormulaText, err := parseSpreadsheetFormulaElement(decoder, typed)
				if err != nil {
					return 0, spreadsheetCell{}, err
				}
				formulaType = parsedFormulaType
				formulaSharedIndex = parsedSharedIndex
				formulaText = parsedFormulaText
			case "v":
				text, err := parseSpreadsheetTextElement(decoder, "v")
				if err != nil {
					return 0, spreadsheetCell{}, err
				}
				value = text
			case "is":
				text, err := parseSpreadsheetInlineString(decoder)
				if err != nil {
					return 0, spreadsheetCell{}, err
				}
				inlineText = text
			default:
				if err := skipSpreadsheetElement(decoder, typed.Name.Local); err != nil {
					return 0, spreadsheetCell{}, err
				}
			}
		case xml.EndElement:
			if typed.Name.Local != "c" {
				continue
			}
			if strings.TrimSpace(cellRef) == "" {
				cellRef = spreadsheetCellRef(colIndex, rowIndex)
			}
			cell, err := decodeSpreadsheetCellValue(
				cellType,
				value,
				inlineText,
				formulaType,
				formulaSharedIndex,
				formulaText,
				styleID,
				cellRef,
				styles[styleID],
				sharedStrings,
				sharedFormulae,
			)
			if err != nil {
				return 0, spreadsheetCell{}, err
			}
			return colIndex, cell, nil
		}
	}
}

func parseSpreadsheetFormulaElement(decoder *xml.Decoder, start xml.StartElement) (string, int, string, error) {
	formulaType := ""
	formulaSharedIndex := 0
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "t":
			formulaType = attr.Value
		case "si":
			if parsed, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err == nil {
				formulaSharedIndex = parsed
			}
		}
	}
	text, err := parseSpreadsheetTextElement(decoder, "f")
	if err != nil {
		return "", 0, "", err
	}
	return formulaType, formulaSharedIndex, text, nil
}

func parseSpreadsheetInlineString(decoder *xml.Decoder) (string, error) {
	var (
		text        string
		builder     strings.Builder
		captureText bool
	)

	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}

		switch typed := token.(type) {
		case xml.StartElement:
			if typed.Name.Local == "t" {
				captureText = true
			}
		case xml.EndElement:
			switch typed.Name.Local {
			case "t":
				captureText = false
			case "is":
				return finishSpreadsheetAccumulatedText(text, &builder), nil
			}
		case xml.CharData:
			if captureText {
				text = appendSpreadsheetTextChunk(text, &builder, typed)
			}
		}
	}
}

func parseSpreadsheetTextElement(decoder *xml.Decoder, endElement string) (string, error) {
	var (
		text    string
		builder strings.Builder
	)
	for {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return "", io.ErrUnexpectedEOF
			}
			return "", err
		}
		switch typed := token.(type) {
		case xml.CharData:
			text = appendSpreadsheetTextChunk(text, &builder, typed)
		case xml.EndElement:
			if typed.Name.Local == endElement {
				return finishSpreadsheetAccumulatedText(text, &builder), nil
			}
		case xml.StartElement:
			if err := skipSpreadsheetElement(decoder, typed.Name.Local); err != nil {
				return "", err
			}
		}
	}
}

func appendSpreadsheetTextChunk(current string, builder *strings.Builder, chunk []byte) string {
	if len(chunk) == 0 {
		return current
	}
	if builder.Len() == 0 && current == "" {
		return string(chunk)
	}
	if builder.Len() == 0 {
		builder.Grow(len(current) + len(chunk))
		builder.WriteString(current)
	}
	builder.Write(chunk)
	return current
}

func finishSpreadsheetAccumulatedText(current string, builder *strings.Builder) string {
	if builder.Len() > 0 {
		return builder.String()
	}
	return current
}

func skipSpreadsheetElement(decoder *xml.Decoder, endElement string) error {
	depth := 1
	for depth > 0 {
		token, err := decoder.RawToken()
		if err != nil {
			if err == io.EOF {
				return io.ErrUnexpectedEOF
			}
			return err
		}
		switch typed := token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			if typed.Name.Local == endElement {
				depth--
			} else {
				depth--
			}
		}
	}
	return nil
}

func parseSpreadsheetIntAttr(attrs []xml.Attr, name string, fallback int) int {
	for _, attr := range attrs {
		if attr.Name.Local != name {
			continue
		}
		if parsed, err := strconv.Atoi(strings.TrimSpace(attr.Value)); err == nil {
			return parsed
		}
		break
	}
	return fallback
}

func buildSpreadsheetRows(xmlRows []xlsxRowXML, sharedStrings []string, styles map[int]spreadsheetStyleInfo) ([][]spreadsheetCell, error) {
	rows := make([][]spreadsheetCell, 0, len(xmlRows))
	sharedFormulae := make(map[int]spreadsheetSharedFormula)
	for _, rowXML := range xmlRows {
		rowIndex := rowXML.Index
		if rowIndex <= 0 {
			rowIndex = len(rows) + 1
		}
		maxCol := 0
		nextCol := 0
		colPositions := make([]int, len(rowXML.Cells))
		for idx, cellXML := range rowXML.Cells {
			col := nextCol
			if parsed, ok := spreadsheetColumnIndex(cellXML.Ref); ok {
				col = parsed
			}
			colPositions[idx] = col
			if col+1 > maxCol {
				maxCol = col + 1
			}
			nextCol = col + 1
		}
		row := make([]spreadsheetCell, maxCol)
		for idx, cellXML := range rowXML.Cells {
			col := colPositions[idx]
			ref := strings.TrimSpace(cellXML.Ref)
			if ref == "" {
				ref = spreadsheetCellRef(col, rowIndex)
			}
			style := styles[cellXML.Style]
			cell, err := decodeSpreadsheetCell(cellXML, sharedStrings, style, ref, sharedFormulae)
			if err != nil {
				return nil, err
			}
			row[col] = cell
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func decodeSpreadsheetCell(cell xlsxCellXML, sharedStrings []string, style spreadsheetStyleInfo, cellRef string, sharedFormulae map[int]spreadsheetSharedFormula) (spreadsheetCell, error) {
	inlineText := ""
	if cell.InlineStr != nil {
		inlineText = cell.InlineStr.fullText()
	}
	formulaType := ""
	formulaSharedIndex := 0
	formulaText := ""
	if cell.Formula != nil {
		formulaType = cell.Formula.Type
		formulaSharedIndex = cell.Formula.SharedIndex
		formulaText = cell.Formula.Text
	}
	return decodeSpreadsheetCellValue(cell.Type, cell.Value, inlineText, formulaType, formulaSharedIndex, formulaText, cell.Style, cellRef, style, sharedStrings, sharedFormulae)
}

func decodeDelimitedSpreadsheetCell(value string) spreadsheetCell {
	text := strings.TrimSpace(value)
	if text == "" {
		return spreadsheetCell{}
	}
	if i, err := strconv.ParseInt(text, 10, 64); err == nil {
		return spreadsheetCell{Display: text, Value: i, RawValue: text, Kind: "integer"}
	}
	if f, err := strconv.ParseFloat(text, 64); err == nil {
		return spreadsheetCell{Display: text, Value: f, RawValue: text, Kind: "decimal"}
	}
	return spreadsheetCell{Display: value, Value: value, RawValue: value, Kind: "text"}
}

func spreadsheetCellText(cell xlsxCellXML, sharedStrings []string) string {
	inlineText := ""
	if cell.InlineStr != nil {
		inlineText = cell.InlineStr.fullText()
	}
	return spreadsheetCellTextFromParts(cell.Type, cell.Value, inlineText, sharedStrings)
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

func resolveSpreadsheetFormula(cell xlsxCellXML, cellRef string, sharedFormulae map[int]spreadsheetSharedFormula) string {
	if cell.Formula == nil {
		return ""
	}
	return resolveSpreadsheetFormulaParts(cell.Formula.Type, cell.Formula.SharedIndex, cell.Formula.Text, cellRef, sharedFormulae)
}

func decodeSpreadsheetCellValue(cellType, rawValue, inlineText, formulaType string, formulaSharedIndex int, formulaText string, styleID int, cellRef string, style spreadsheetStyleInfo, sharedStrings []string, sharedFormulae map[int]spreadsheetSharedFormula) (spreadsheetCell, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(cellType))
	trimmedValue := strings.TrimSpace(rawValue)
	text := spreadsheetCellTextFromParts(normalizedType, rawValue, inlineText, sharedStrings)
	formula := resolveSpreadsheetFormulaParts(formulaType, formulaSharedIndex, formulaText, cellRef, sharedFormulae)
	result := spreadsheetCell{
		RawValue:     trimmedValue,
		Formula:      formula,
		StyleID:      styleID,
		Kind:         style.Kind,
		NumberFormat: style.NumberFormat,
		IsDate:       style.IsDate,
	}

	if trimmedValue == "" && strings.TrimSpace(text) == "" && formula != "" {
		return result, nil
	}

	switch normalizedType {
	case "b":
		value := trimmedValue == "1"
		if value {
			result.Display = "true"
			result.Value = true
			if result.Kind == "" {
				result.Kind = "boolean"
			}
			return result, nil
		}
		result.Display = "false"
		result.Value = false
		if result.Kind == "" {
			result.Kind = "boolean"
		}
		return result, nil
	case "n", "":
		if trimmedValue == "" {
			if formula != "" {
				return result, nil
			}
			return spreadsheetCell{}, nil
		}
		if style.IsDate {
			parsed, err := strconv.ParseFloat(trimmedValue, 64)
			if err == nil {
				display, kind := formatSpreadsheetDateValue(parsed, style.NumberFormat)
				result.Display = display
				result.Value = display
				result.Kind = kind
				result.IsDate = true
				return result, nil
			}
		}
		if i, err := strconv.ParseInt(trimmedValue, 10, 64); err == nil {
			result.Display = trimmedValue
			result.Value = i
			if result.Kind == "" {
				result.Kind = "integer"
			}
			return result, nil
		}
		if f, err := strconv.ParseFloat(trimmedValue, 64); err == nil {
			result.Display = trimmedValue
			result.Value = f
			if result.Kind == "" {
				result.Kind = "decimal"
			}
			return result, nil
		}
	case "inlinestr", "s", "str", "e":
		result.Display = text
		result.Value = text
		if result.Kind == "" {
			result.Kind = "text"
		}
		return result, nil
	default:
		if text != "" {
			result.Display = text
			result.Value = text
			if result.Kind == "" {
				result.Kind = "text"
			}
			return result, nil
		}
	}
	if text == "" {
		if formula != "" {
			return result, nil
		}
		return spreadsheetCell{}, nil
	}
	result.Display = text
	result.Value = text
	if result.Kind == "" {
		result.Kind = "text"
	}
	return result, nil
}

func spreadsheetCellTextFromParts(cellType, rawValue, inlineText string, sharedStrings []string) string {
	switch strings.ToLower(strings.TrimSpace(cellType)) {
	case "inlinestr":
		if inlineText != "" {
			return inlineText
		}
	case "s":
		idx, err := strconv.Atoi(strings.TrimSpace(rawValue))
		if err == nil && idx >= 0 && idx < len(sharedStrings) {
			return sharedStrings[idx]
		}
	case "str", "e":
		return strings.TrimSpace(rawValue)
	}
	if inlineText != "" {
		return inlineText
	}
	return strings.TrimSpace(rawValue)
}

func resolveSpreadsheetFormulaParts(formulaType string, sharedIndex int, formulaText, cellRef string, sharedFormulae map[int]spreadsheetSharedFormula) string {
	formulaText = strings.TrimSpace(formulaText)
	if strings.EqualFold(strings.TrimSpace(formulaType), "shared") {
		if formulaText != "" {
			sharedFormulae[sharedIndex] = spreadsheetSharedFormula{
				CellRef: cellRef,
				Text:    formulaText,
			}
			return formulaText
		}
		base, ok := sharedFormulae[sharedIndex]
		if !ok {
			return ""
		}
		return translateSpreadsheetSharedFormula(base.Text, base.CellRef, cellRef)
	}
	return formulaText
}

func translateSpreadsheetSharedFormula(formula, baseRef, targetRef string) string {
	baseCol, baseRow, ok := spreadsheetParseCellRef(baseRef)
	if !ok {
		return formula
	}
	targetCol, targetRow, ok := spreadsheetParseCellRef(targetRef)
	if !ok {
		return formula
	}
	deltaCol := targetCol - baseCol
	deltaRow := targetRow - baseRow
	return spreadsheetCellRefPattern.ReplaceAllStringFunc(formula, func(match string) string {
		parts := spreadsheetCellRefPattern.FindStringSubmatch(match)
		if len(parts) != 5 {
			return match
		}
		colAbs := parts[1] == "$"
		rowAbs := parts[3] == "$"
		col := spreadsheetLettersToIndex(parts[2])
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
		replacement += spreadsheetIndexToLetters(col)
		if rowAbs {
			replacement += "$"
		}
		replacement += strconv.Itoa(row)
		return replacement
	})
}

func classifySpreadsheetNumberFormat(numFmtID int, formatCode string) (string, bool, string) {
	formatCode = strings.TrimSpace(formatCode)
	if formatCode == "" {
		formatCode = builtinSpreadsheetNumberFormat(numFmtID)
	}
	if formatCode == "" {
		switch numFmtID {
		case 49:
			return "text", false, ""
		default:
			return "general", false, ""
		}
	}

	if kind, isDate := spreadsheetDateFormatKind(formatCode); isDate {
		return kind, true, formatCode
	}

	lower := strings.ToLower(formatCode)
	switch {
	case numFmtID == 49 || strings.Contains(lower, "@"):
		return "text", false, formatCode
	case numFmtID == 9 || numFmtID == 10 || strings.Contains(lower, "%"):
		return "percent", false, formatCode
	case strings.Contains(lower, "[$") || strings.ContainsAny(lower, "$¥€£"):
		return "currency", false, formatCode
	case strings.Contains(lower, "."):
		return "decimal", false, formatCode
	case lower == "general":
		return "general", false, formatCode
	default:
		return "integer", false, formatCode
	}
}

func spreadsheetDateFormatKind(formatCode string) (string, bool) {
	cleaned := strings.ToLower(strings.TrimSpace(formatCode))
	cleaned = regexp.MustCompile(`"[^"]*"`).ReplaceAllString(cleaned, "")
	cleaned = regexp.MustCompile(`\[[^\]]*\]`).ReplaceAllString(cleaned, "")
	cleaned = strings.ReplaceAll(cleaned, `\`, "")
	cleaned = strings.ReplaceAll(cleaned, "_", "")
	hasDate := strings.Contains(cleaned, "y") || strings.Contains(cleaned, "d") || strings.Contains(cleaned, "m/")
	if !hasDate && strings.Contains(cleaned, "m") && strings.Contains(cleaned, "d") {
		hasDate = true
	}
	hasTime := strings.Contains(cleaned, "h") || strings.Contains(cleaned, "s") || strings.Contains(cleaned, "am/pm")
	if !hasDate && !hasTime {
		return "", false
	}
	if hasTime {
		return "datetime", true
	}
	return "date", true
}

func builtinSpreadsheetNumberFormat(numFmtID int) string {
	switch numFmtID {
	case 0:
		return "General"
	case 1:
		return "0"
	case 2:
		return "0.00"
	case 3:
		return "#,##0"
	case 4:
		return "#,##0.00"
	case 9:
		return "0%"
	case 10:
		return "0.00%"
	case 11:
		return "0.00E+00"
	case 12:
		return "# ?/?"
	case 13:
		return "# ??/??"
	case 14:
		return "mm-dd-yy"
	case 15:
		return "d-mmm-yy"
	case 16:
		return "d-mmm"
	case 17:
		return "mmm-yy"
	case 18:
		return "h:mm AM/PM"
	case 19:
		return "h:mm:ss AM/PM"
	case 20:
		return "h:mm"
	case 21:
		return "h:mm:ss"
	case 22:
		return "m/d/yy h:mm"
	case 37:
		return "#,##0 ;(#,##0)"
	case 38:
		return "#,##0 ;[Red](#,##0)"
	case 39:
		return "#,##0.00;(#,##0.00)"
	case 40:
		return "#,##0.00;[Red](#,##0.00)"
	case 45:
		return "mm:ss"
	case 46:
		return "[h]:mm:ss"
	case 47:
		return "mmss.0"
	case 48:
		return "##0.0E+0"
	case 49:
		return "@"
	default:
		return ""
	}
}

func formatSpreadsheetDateValue(serial float64, numberFormat string) (string, string) {
	whole := int64(serial)
	frac := serial - float64(whole)
	base := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
	timestamp := base.AddDate(0, 0, int(whole)).Add(time.Duration(frac * float64(24*time.Hour)))
	kind, isDate := spreadsheetDateFormatKind(numberFormat)
	if !isDate || kind == "" {
		kind = "date"
	}
	if kind == "datetime" {
		return timestamp.Format("2006-01-02 15:04:05"), kind
	}
	return timestamp.Format("2006-01-02"), "date"
}

func spreadsheetFormulaCount(rows [][]spreadsheetCell) int {
	count := 0
	for _, row := range rows {
		for _, cell := range row {
			if strings.TrimSpace(cell.Formula) != "" {
				count++
			}
		}
	}
	return count
}

func spreadsheetDateColumns(headers []string, rows [][]spreadsheetCell) []string {
	if len(rows) == 0 {
		return nil
	}
	startRow := 0
	if len(headers) > 0 {
		startRow = 1
	}
	seen := make([]bool, len(headers))
	out := make([]string, 0, len(headers))
	for rowIndex := startRow; rowIndex < len(rows); rowIndex++ {
		for colIndex, cell := range rows[rowIndex] {
			if colIndex >= len(headers) || seen[colIndex] {
				continue
			}
			if cell.IsDate || cell.Kind == "date" || cell.Kind == "datetime" {
				seen[colIndex] = true
				out = append(out, headers[colIndex])
			}
		}
	}
	sort.Strings(out)
	return out
}

func spreadsheetColumnKinds(headers []string, rows [][]spreadsheetCell) map[string]string {
	if len(headers) == 0 || len(rows) == 0 {
		return nil
	}
	startRow := 0
	if len(rows) > 0 {
		startRow = 1
	}
	out := make(map[string]string, len(headers))
	for colIndex, header := range headers {
		kinds := make(map[string]struct{})
		for rowIndex := startRow; rowIndex < len(rows); rowIndex++ {
			if colIndex >= len(rows[rowIndex]) {
				continue
			}
			kind := spreadsheetCellKind(rows[rowIndex][colIndex])
			if kind == "" {
				continue
			}
			kinds[kind] = struct{}{}
		}
		out[header] = spreadsheetCollapsedKind(kinds)
	}
	return out
}

func spreadsheetCellKind(cell spreadsheetCell) string {
	if cell.Kind != "" && cell.Kind != "general" {
		return cell.Kind
	}
	switch cell.Value.(type) {
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
		return ""
	}
}

func spreadsheetCollapsedKind(kinds map[string]struct{}) string {
	if len(kinds) == 0 {
		return ""
	}
	if len(kinds) == 1 {
		for kind := range kinds {
			return kind
		}
	}
	if len(kinds) == 2 {
		if _, ok := kinds["date"]; ok {
			if _, ok := kinds["datetime"]; ok {
				return "datetime"
			}
		}
	}
	return "mixed"
}

func cloneSpreadsheetStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
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

func spreadsheetParseCellRef(ref string) (int, int, bool) {
	col, ok := spreadsheetColumnIndex(ref)
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

func spreadsheetLettersToIndex(letters string) int {
	col := 0
	for _, r := range strings.ToUpper(strings.TrimSpace(letters)) {
		if r < 'A' || r > 'Z' {
			break
		}
		col = col*26 + int(r-'A'+1)
	}
	return col - 1
}

func spreadsheetIndexToLetters(index int) string {
	index++
	name := ""
	for index > 0 {
		index--
		name = string(rune('A'+index%26)) + name
		index /= 26
	}
	return name
}

func spreadsheetCellRef(colIndex, rowIndex int) string {
	return spreadsheetIndexToLetters(colIndex) + strconv.Itoa(rowIndex)
}

func deriveSpreadsheetSheetData(rows [][]spreadsheetCell) derivedSpreadsheetSheetData {
	derived := derivedSpreadsheetSheetData{
		interfaceRows: make([][]interface{}, 0, len(rows)),
	}
	if len(rows) == 0 {
		return derived
	}

	derived.headers = uniqueSpreadsheetHeaders(rows[0])
	derived.records = make([]map[string]interface{}, 0, maxInt(len(rows)-1, 0))
	kindMasks := make([]uint8, len(derived.headers))
	dateSeen := make([]bool, len(derived.headers))

	for rowIndex, row := range rows {
		values := make([]interface{}, 0, len(row))
		for colIndex, cell := range row {
			if strings.TrimSpace(cell.Formula) != "" {
				derived.formulaCount++
			}
			if cell.Value == nil {
				values = append(values, "")
			} else {
				values = append(values, cell.Value)
			}
			if rowIndex == 0 || colIndex >= len(derived.headers) {
				continue
			}
			if !dateSeen[colIndex] && (cell.IsDate || cell.Kind == "date" || cell.Kind == "datetime") {
				dateSeen[colIndex] = true
			}
			kindMasks[colIndex] |= spreadsheetKindMask(spreadsheetCellKind(cell))
		}
		derived.interfaceRows = append(derived.interfaceRows, values)

		if rowIndex == 0 {
			continue
		}
		if record := buildSpreadsheetRecord(row, derived.headers); record != nil {
			derived.records = append(derived.records, record)
		}
	}

	derived.dateColumns = make([]string, 0, len(derived.headers))
	derived.columnKinds = make(map[string]string, len(derived.headers))
	for idx, header := range derived.headers {
		if dateSeen[idx] {
			derived.dateColumns = append(derived.dateColumns, header)
		}
		derived.columnKinds[header] = spreadsheetCollapsedKindMask(kindMasks[idx])
	}
	sort.Strings(derived.dateColumns)
	return derived
}

func buildSpreadsheetRecord(row []spreadsheetCell, headers []string) map[string]interface{} {
	var record map[string]interface{}
	for idx, header := range headers {
		value := interface{}("")
		if idx < len(row) && row[idx].Value != nil && row[idx].Display != "" {
			value = row[idx].Value
		}
		if record == nil {
			if value == "" {
				continue
			}
			record = make(map[string]interface{}, len(headers))
			for prior := 0; prior < idx; prior++ {
				record[headers[prior]] = ""
			}
		}
		record[header] = value
	}
	return record
}

func spreadsheetKindMask(kind string) uint8 {
	switch kind {
	case "boolean":
		return 1 << 0
	case "integer":
		return 1 << 1
	case "decimal":
		return 1 << 2
	case "text":
		return 1 << 3
	case "date":
		return 1 << 4
	case "datetime":
		return 1 << 5
	default:
		return 0
	}
}

func spreadsheetCollapsedKindMask(mask uint8) string {
	switch mask {
	case 0:
		return ""
	case 1 << 0:
		return "boolean"
	case 1 << 1:
		return "integer"
	case 1 << 2:
		return "decimal"
	case 1 << 3:
		return "text"
	case 1 << 4:
		return "date"
	case 1 << 5:
		return "datetime"
	case (1 << 4) | (1 << 5):
		return "datetime"
	default:
		return "mixed"
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
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

func spreadsheetRecordsFromRows(rows [][]spreadsheetCell, headers []string) []map[string]interface{} {
	if len(rows) <= 1 || len(headers) == 0 {
		return nil
	}
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
	writeSpreadsheetTextContent(&buf, workbook)
	if err := os.WriteFile(path, buf.Bytes(), 0o640); err != nil {
		return "", err
	}
	return spreadsheetPreview(workbook), nil
}

func spreadsheetTextContent(workbook *spreadsheetWorkbook) string {
	var buf bytes.Buffer
	writeSpreadsheetTextContent(&buf, workbook)
	return buf.String()
}

func writeSpreadsheetTextContent(buf *bytes.Buffer, workbook *spreadsheetWorkbook) {
	if buf == nil || workbook == nil {
		return
	}
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
	preview := "Converted spreadsheet with sheets: " + strings.Join(parts, ", ")
	if workbook.Summary == nil || len(workbook.Summary.Highlights) == 0 {
		return preview
	}
	highlights := workbook.Summary.Highlights
	if len(highlights) > 3 {
		highlights = highlights[:3]
	}
	return preview + ". Highlights: " + strings.Join(highlights, "; ")
}
