package convert

import (
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestParseSpreadsheetWorksheetXMLPreservesSparseCellsAndFormulaMetadata(t *testing.T) {
	worksheetXML := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
      <c r="D1" t="s"><v>2</v></c>
      <c r="E1" t="s"><v>3</v></c>
    </row>
    <row r="2">
      <c r="A2" s="1"><v>45292</v></c>
      <c r="B2"><v>1200</v></c>
      <c r="C2"><f t="shared" si="0" ref="C2:C3">B2*2</f><v>2400</v></c>
      <c r="D2"><f>B2+1</f></c>
      <c r="E2" t="inlineStr"><is><t>Finance</t></is></c>
    </row>
    <row r="3">
      <c r="A3" s="1"><v>45293</v></c>
      <c r="B3"><v>1500</v></c>
      <c r="C3"><f t="shared" si="0"/><v>3000</v></c>
      <c r="E3" t="inlineStr"><is><r><t>Ops</t></r><r><t> Team</t></r></is></c>
    </row>
  </sheetData>
</worksheet>`)
	sharedStrings := []string{"Date", "Revenue", "Projected", "Owner"}
	styles := map[int]spreadsheetStyleInfo{
		1: {
			Kind:         "date",
			NumberFormat: "yyyy-mm-dd",
			IsDate:       true,
		},
	}

	rows, err := parseSpreadsheetWorksheetXML(worksheetXML, sharedStrings, styles)
	if err != nil {
		t.Fatalf("parseSpreadsheetWorksheetXML failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("row count = %d, want 3", len(rows))
	}
	if len(rows[0]) != 5 {
		t.Fatalf("header width = %d, want 5", len(rows[0]))
	}
	if got := rows[0][3].Display; got != "Projected" {
		t.Fatalf("rows[0][3].Display = %q, want Projected", got)
	}
	if got := rows[1][0].Display; got != "2024-01-01" {
		t.Fatalf("rows[1][0].Display = %q, want 2024-01-01", got)
	}
	if got := rows[1][2].Formula; got != "B2*2" {
		t.Fatalf("rows[1][2].Formula = %q, want B2*2", got)
	}
	if got := rows[2][2].Formula; got != "B3*2" {
		t.Fatalf("rows[2][2].Formula = %q, want B3*2", got)
	}
	if got := rows[1][3].Formula; got != "B2+1" {
		t.Fatalf("rows[1][3].Formula = %q, want B2+1", got)
	}
	if got := rows[1][3].Display; got != "" {
		t.Fatalf("rows[1][3].Display = %q, want empty string for formula without cached value", got)
	}
	if got := rows[2][4].Display; got != "Ops Team" {
		t.Fatalf("rows[2][4].Display = %q, want Ops Team", got)
	}
	gotHeaders := uniqueSpreadsheetHeaders(rows[0])
	wantHeaders := []string{"Date", "Revenue", "Column_3", "Projected", "Owner"}
	if !reflect.DeepEqual(gotHeaders, wantHeaders) {
		t.Fatalf("headers = %#v, want %#v", gotHeaders, wantHeaders)
	}
}

func TestParseSpreadsheetWorksheetXMLDecodesEscapedText(t *testing.T) {
	worksheetXML := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="inlineStr"><is><t>R&amp;D &lt;Core&gt;</t></is></c>
      <c r="B1"><f>IF(A1=&quot;x&quot;,1,0)</f></c>
    </row>
  </sheetData>
</worksheet>`)

	rows, err := parseSpreadsheetWorksheetXML(worksheetXML, nil, nil)
	if err != nil {
		t.Fatalf("parseSpreadsheetWorksheetXML failed: %v", err)
	}
	if got := rows[0][0].Display; got != "R&D <Core>" {
		t.Fatalf("rows[0][0].Display = %q, want %q", got, "R&D <Core>")
	}
	if got := rows[0][1].Formula; got != `IF(A1="x",1,0)` {
		t.Fatalf("rows[0][1].Formula = %q, want %q", got, `IF(A1="x",1,0)`)
	}
}

func TestParseSpreadsheetSharedStringsXMLPreservesTextAndRuns(t *testing.T) {
	sharedStringsXML := strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4" uniqueCount="4">
  <si><t>Department &amp; Ops</t></si>
  <si><r><t>Ops</t></r><r><t> Team</t></r></si>
  <si><t xml:space="preserve">  padded text  </t></si>
  <si><r><t>North</t></r><r><t>-</t></r><r><t>East</t></r></si>
</sst>`)

	values, err := parseSpreadsheetSharedStringsXML(sharedStringsXML)
	if err != nil {
		t.Fatalf("parseSpreadsheetSharedStringsXML failed: %v", err)
	}
	want := []string{"Department & Ops", "Ops Team", "  padded text  ", "North-East"}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("values = %#v, want %#v", values, want)
	}
}

func TestDeriveSpreadsheetSheetDataBuildsRowsRecordsAndMetadata(t *testing.T) {
	rows := [][]spreadsheetCell{
		{
			{Display: "Date", Value: "Date"},
			{Display: "Revenue", Value: "Revenue"},
			{Display: "Projected", Value: "Projected"},
			{Display: "Owner", Value: "Owner"},
		},
		{
			{Display: "2024-01-01", Value: "2024-01-01", Kind: "date", IsDate: true},
			{Display: "1200", Value: int64(1200), Kind: "integer"},
			{Display: "2400", Value: int64(2400), Kind: "integer", Formula: "B2*2"},
			{Display: "", Value: nil},
		},
		{
			{Display: "2024-01-02", Value: "2024-01-02", Kind: "date", IsDate: true},
			{Display: "1500", Value: int64(1500), Kind: "integer"},
			{Display: "3000", Value: int64(3000), Kind: "integer", Formula: "B3*2"},
			{Display: "Alice", Value: "Alice", Kind: "text"},
		},
	}

	derived := deriveSpreadsheetSheetData(rows)

	if !reflect.DeepEqual(derived.headers, []string{"Date", "Revenue", "Projected", "Owner"}) {
		t.Fatalf("headers = %#v", derived.headers)
	}
	if got := derived.formulaCount; got != 2 {
		t.Fatalf("formulaCount = %d, want 2", got)
	}
	if !reflect.DeepEqual(derived.dateColumns, []string{"Date"}) {
		t.Fatalf("dateColumns = %#v, want [Date]", derived.dateColumns)
	}
	if got := derived.columnKinds["Revenue"]; got != "integer" {
		t.Fatalf("columnKinds[Revenue] = %q, want integer", got)
	}
	if got := derived.columnKinds["Owner"]; got != "text" {
		t.Fatalf("columnKinds[Owner] = %q, want text", got)
	}
	if len(derived.interfaceRows) != 3 {
		t.Fatalf("interface row count = %d, want 3", len(derived.interfaceRows))
	}
	if got, _ := derived.interfaceRows[1][1].(int64); got != 1200 {
		t.Fatalf("interfaceRows[1][1] = %#v, want 1200", derived.interfaceRows[1][1])
	}
	if len(derived.records) != 2 {
		t.Fatalf("record count = %d, want 2", len(derived.records))
	}
	if got, _ := derived.records[0]["Projected"].(int64); got != 2400 {
		t.Fatalf("records[0][Projected] = %#v, want 2400", derived.records[0]["Projected"])
	}
	if got := derived.records[0]["Owner"]; got != "" {
		t.Fatalf("records[0][Owner] = %#v, want empty string", got)
	}
}

func BenchmarkParseSpreadsheetWorksheetXML(b *testing.B) {
	worksheet := benchmarkWorksheetXML(900, 10)
	sharedStrings := make([]string, 10)
	for i := range sharedStrings {
		sharedStrings[i] = "Column " + spreadsheetCellRef(i, 1)
	}
	styles := map[int]spreadsheetStyleInfo{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parseSpreadsheetWorksheetXML(strings.NewReader(worksheet), sharedStrings, styles); err != nil {
			b.Fatalf("parseSpreadsheetWorksheetXML failed: %v", err)
		}
	}
}

func BenchmarkParseSpreadsheetSharedStringsXML(b *testing.B) {
	sharedStringsXML := benchmarkSharedStringsXML(6000)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := parseSpreadsheetSharedStringsXML(strings.NewReader(sharedStringsXML)); err != nil {
			b.Fatalf("parseSpreadsheetSharedStringsXML failed: %v", err)
		}
	}
}

func BenchmarkDeriveSpreadsheetSheetData(b *testing.B) {
	rows := benchmarkSpreadsheetCellRows(900, 10)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		derived := deriveSpreadsheetSheetData(rows)
		if len(derived.interfaceRows) != len(rows) {
			b.Fatalf("interface row count = %d, want %d", len(derived.interfaceRows), len(rows))
		}
	}
}

func BenchmarkLegacySpreadsheetSheetDerivation(b *testing.B) {
	rows := benchmarkSpreadsheetCellRows(900, 10)
	headers := uniqueSpreadsheetHeaders(rows[0])

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		interfaceRows := convertSpreadsheetRowsToInterfaces(rows)
		records := spreadsheetRecordsFromRows(rows, headers)
		formulaCount := spreadsheetFormulaCount(rows)
		dateColumns := spreadsheetDateColumns(headers, rows)
		columnKinds := spreadsheetColumnKinds(headers, rows)
		if len(interfaceRows) != len(rows) || len(records) == 0 || formulaCount == 0 || len(dateColumns) == 0 || len(columnKinds) == 0 {
			b.Fatal("legacy derivation produced unexpected empty outputs")
		}
	}
}

func benchmarkWorksheetXML(rows, cols int) string {
	var sheet strings.Builder
	sheet.Grow(rows * cols * 24)
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	sheet.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	sheet.WriteString(`<row r="1">`)
	for colIndex := 0; colIndex < cols; colIndex++ {
		sheet.WriteString(`<c r="`)
		sheet.WriteString(spreadsheetCellRef(colIndex, 1))
		sheet.WriteString(`" t="s"><v>`)
		sheet.WriteString(strconv.Itoa(colIndex))
		sheet.WriteString(`</v></c>`)
	}
	sheet.WriteString(`</row>`)
	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		sheet.WriteString(`<row r="`)
		sheet.WriteString(strconv.Itoa(rowIndex + 2))
		sheet.WriteString(`">`)
		for colIndex := 0; colIndex < cols; colIndex++ {
			sheet.WriteString(`<c r="`)
			sheet.WriteString(spreadsheetCellRef(colIndex, rowIndex+2))
			sheet.WriteString(`"><v>`)
			sheet.WriteString(strconv.Itoa((rowIndex + 1) * (colIndex + 3)))
			sheet.WriteString(`</v></c>`)
		}
		sheet.WriteString(`</row>`)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	return sheet.String()
}

func benchmarkSharedStringsXML(items int) string {
	var xmlBuilder strings.Builder
	xmlBuilder.Grow(items * 48)
	xmlBuilder.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	xmlBuilder.WriteString(`<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	for i := 0; i < items; i++ {
		xmlBuilder.WriteString(`<si><r><t>Shared</t></r><r><t> string </t></r><r><t>`)
		xmlBuilder.WriteString(strconv.Itoa(i + 1))
		xmlBuilder.WriteString(`</t></r></si>`)
	}
	xmlBuilder.WriteString(`</sst>`)
	return xmlBuilder.String()
}

func benchmarkSpreadsheetCellRows(rows, cols int) [][]spreadsheetCell {
	out := make([][]spreadsheetCell, 0, rows+1)
	header := make([]spreadsheetCell, 0, cols)
	for colIndex := 0; colIndex < cols; colIndex++ {
		header = append(header, spreadsheetCell{
			Display: "Column " + strconv.Itoa(colIndex+1),
			Value:   "Column " + strconv.Itoa(colIndex+1),
			Kind:    "text",
		})
	}
	out = append(out, header)
	for rowIndex := 0; rowIndex < rows; rowIndex++ {
		row := make([]spreadsheetCell, 0, cols)
		for colIndex := 0; colIndex < cols; colIndex++ {
			cell := spreadsheetCell{
				Display: strconv.Itoa((rowIndex + 1) * (colIndex + 3)),
				Value:   int64((rowIndex + 1) * (colIndex + 3)),
				Kind:    "integer",
			}
			if colIndex == 0 {
				cell.Display = "2024-01-01"
				cell.Value = "2024-01-01"
				cell.Kind = "date"
				cell.IsDate = true
			}
			if colIndex == 2 {
				cell.Formula = "B2*2"
			}
			row = append(row, cell)
		}
		out = append(out, row)
	}
	return out
}
