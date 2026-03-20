package convert

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadSpreadsheetWorkbookHandlesInlineStringsAndHeaderDeduplication(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "company_expenses.xlsx")
	writeTestSpreadsheetXLSX(t, sourcePath)

	workbook, err := loadSpreadsheetWorkbook(sourcePath)
	if err != nil {
		t.Fatalf("loadSpreadsheetWorkbook failed: %v", err)
	}
	if workbook.SheetCount != 1 {
		t.Fatalf("sheet count = %d, want 1", workbook.SheetCount)
	}

	sheet := workbook.Sheets[0]
	if sheet.Name != "Budget" {
		t.Fatalf("sheet name = %q, want Budget", sheet.Name)
	}
	wantHeaders := []string{"Department", "Department_2", "Column_3", "Owner"}
	if !reflect.DeepEqual(sheet.Headers, wantHeaders) {
		t.Fatalf("headers = %v, want %v", sheet.Headers, wantHeaders)
	}
	if sheet.RowCount != 2 {
		t.Fatalf("row_count = %d, want 2", sheet.RowCount)
	}
	if len(sheet.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(sheet.Records))
	}
	if sheet.Summary == nil {
		t.Fatal("expected sheet summary")
	}
	if got := sheet.Summary.NumericTotals["Column_3"]; got != 1200 {
		t.Fatalf("summary total Column_3 = %v, want 1200", got)
	}
	if workbook.Summary == nil || len(workbook.Summary.SheetSummaries) != 1 {
		t.Fatalf("workbook summary = %#v, want one sheet summary", workbook.Summary)
	}

	record := sheet.Records[0]
	if got, _ := record["Department"].(string); got != "Finance" {
		t.Fatalf("Department = %v, want Finance", record["Department"])
	}
	if got, _ := record["Department_2"].(string); got != "Platform" {
		t.Fatalf("Department_2 = %v, want Platform", record["Department_2"])
	}
	if got, _ := record["Column_3"].(int64); got != 1200 {
		t.Fatalf("Column_3 = %v, want 1200", record["Column_3"])
	}
	if got, _ := record["Owner"].(string); got != "Alice" {
		t.Fatalf("Owner = %v, want Alice", record["Owner"])
	}
}

func TestConvertDocumentUsesLocalSpreadsheetConverterForJSON(t *testing.T) {
	svc := setupConvertTestService(t)
	sourcePath := filepath.Join(t.TempDir(), "company_expenses.xlsx")
	writeTestSpreadsheetXLSX(t, sourcePath)

	task := &ConvertTask{ID: "task-local-spreadsheet-json"}
	source := ResolvedSource{
		Name:     filepath.Base(sourcePath),
		Path:     sourcePath,
		Category: "document",
	}
	outputs, message, err := svc.convertDocument(context.Background(), task, source, "json")
	if err != nil {
		t.Fatalf("convertDocument failed: %v", err)
	}
	if message != "Spreadsheet converted" {
		t.Fatalf("message = %q, want %q", message, "Spreadsheet converted")
	}
	if len(outputs) != 1 {
		t.Fatalf("outputs = %d, want 1", len(outputs))
	}
	if outputs[0].PreviewKind != PreviewText {
		t.Fatalf("preview kind = %q, want %q", outputs[0].PreviewKind, PreviewText)
	}
	if !strings.Contains(outputs[0].PreviewText, "Budget (1 data rows)") {
		t.Fatalf("preview text = %q, want Budget row summary", outputs[0].PreviewText)
	}
	data, err := os.ReadFile(outputs[0].Path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), `"sheet_count": 1`) {
		t.Fatalf("output json missing sheet_count, got=%s", string(data))
	}
	if !strings.Contains(string(data), `"summary": {`) {
		t.Fatalf("output json missing summary block, got=%s", string(data))
	}
	if !strings.Contains(string(data), `"Department_2": "Platform"`) {
		t.Fatalf("output json missing deduplicated record field, got=%s", string(data))
	}
}

func TestSummarizeDelimitedFileComputesDeterministicTotalsAndTopGroups(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "quarterly_sales.csv")
	content := strings.Join([]string{
		"Date,Region,Product,Units_Sold,Revenue,Cost",
		"2024-01-01,East,Widget B,100,3000,1800",
		"2024-01-02,West,Widget A,50,1250,750",
		"2024-01-03,East,Widget B,60,1800,1080",
	}, "\n")
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write csv: %v", err)
	}

	summary, err := SummarizeDelimitedFile(sourcePath)
	if err != nil {
		t.Fatalf("SummarizeDelimitedFile failed: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary")
	}
	if got := summary.NumericTotals["Revenue"]; got != 6050 {
		t.Fatalf("Revenue total = %v, want 6050", got)
	}
	if got := summary.NumericTotals["Cost"]; got != 3630 {
		t.Fatalf("Cost total = %v, want 3630", got)
	}
	if got := summary.NumericTotals["Profit"]; got != 2420 {
		t.Fatalf("Profit total = %v, want 2420", got)
	}
	if got := summary.NumericTotals["Units_Sold"]; got != 210 {
		t.Fatalf("Units_Sold total = %v, want 210", got)
	}
	topRegion := summary.TopByMetric["Revenue"]["Region"]
	if topRegion.Value != "East" || topRegion.Total != 4800 {
		t.Fatalf("top revenue region = %#v, want East / 4800", topRegion)
	}
	topProduct := summary.TopByMetric["Revenue"]["Product"]
	if topProduct.Value != "Widget B" || topProduct.Total != 4800 {
		t.Fatalf("top revenue product = %#v, want Widget B / 4800", topProduct)
	}
	highlights := strings.Join(summary.Highlights, " | ")
	if !strings.Contains(highlights, "Total Profit: 2,420") {
		t.Fatalf("highlights = %q, want total profit line", highlights)
	}
	if !strings.Contains(highlights, "Top Revenue by Region: East (4,800)") {
		t.Fatalf("highlights = %q, want top region line", highlights)
	}
}

func writeTestSpreadsheetXLSX(t *testing.T, path string) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create xlsx: %v", err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	writeZipEntry := func(name, content string) {
		t.Helper()
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}

	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <sheets>
    <sheet name="Budget" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`)
	writeZipEntry("xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="3" uniqueCount="3">
  <si><t>Department</t></si>
  <si><t>Department</t></si>
  <si><t>Owner</t></si>
</sst>`)
	writeZipEntry("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="s"><v>0</v></c>
      <c r="B1" t="s"><v>1</v></c>
      <c r="D1" t="s"><v>2</v></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t>Finance</t></is></c>
      <c r="B2" t="inlineStr"><is><t>Platform</t></is></c>
      <c r="C2"><v>1200</v></c>
      <c r="D2" t="inlineStr"><is><t>Alice</t></is></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
}
