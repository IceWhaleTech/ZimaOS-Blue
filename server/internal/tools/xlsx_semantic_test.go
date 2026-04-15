package tools

import (
	"archive/zip"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

func TestXLSXToolCreateWritesFormulaCellsAndReadMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/formulas.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Growth", "key": "Growth", "kind": "number"},
					map[string]interface{}{"header": "Total", "key": "Total", "kind": "number"},
					map[string]interface{}{"header": "ClosedOn", "key": "ClosedOn", "kind": "text"},
				},
				"rows": []interface{}{
					map[string]interface{}{
						"Region":   "East",
						"Revenue":  100.0,
						"Growth":   map[string]interface{}{"value": 0.25, "format": "percent"},
						"Total":    map[string]interface{}{"formula": "B2*(1+C2)", "value": 125.0, "format": "currency"},
						"ClosedOn": map[string]interface{}{"value": "2024-01-01", "format": "date"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_xlsx_ooxml" {
		t.Fatalf("engine = %v, want native_xlsx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "formulas.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheetXML, "<f>B2*(1+C2)</f>") {
		t.Fatalf("metrics worksheet missing formula cell: %s", sheetXML)
	}

	readResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "read",
		"path":   "reports/formulas.xlsx",
	})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	readPayload := parseNativeDocumentPayload(t, readResult)
	summary, ok := readPayload["tabular_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("tabular_summary type = %T, want object", readPayload["tabular_summary"])
	}
	sheetSummaries, ok := summary["sheet_summaries"].([]interface{})
	if !ok || len(sheetSummaries) == 0 {
		t.Fatalf("sheet_summaries = %#v, want at least one summary", summary["sheet_summaries"])
	}
	var sheetSummary map[string]interface{}
	for _, item := range sheetSummaries {
		candidate, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if candidate["source"] == "Metrics" {
			sheetSummary = candidate
			break
		}
	}
	if sheetSummary == nil {
		t.Fatalf("missing Metrics summary in %#v", sheetSummaries)
	}
	if got := asNativeToolInt(t, sheetSummary["formula_count"]); got != 1 {
		t.Fatalf("formula_count = %d, want 1", got)
	}
	dateColumns, ok := sheetSummary["date_columns"].([]interface{})
	if !ok || len(dateColumns) != 1 || dateColumns[0] != "ClosedOn" {
		t.Fatalf("date_columns = %#v, want [ClosedOn]", sheetSummary["date_columns"])
	}
	columnKinds, ok := sheetSummary["column_kinds"].(map[string]interface{})
	if !ok {
		t.Fatalf("column_kinds type = %T, want object", sheetSummary["column_kinds"])
	}
	if got := asNativeToolString(t, columnKinds["ClosedOn"]); got != "date" {
		t.Fatalf("column_kinds[ClosedOn] = %q, want date", got)
	}
}

func TestXLSXToolCreateAcceptsMarkdownInput(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/markdown.xlsx",
		"markdown": `# Launch Scorecard
Quarterly planning workbook

Summary: Capture the latest status in a spreadsheet-friendly format.

## Metrics
| Metric | Value |
| --- | --- |
| Launch | Ready |
| Risk | Low |

## Owners
- Platform
- Design
`,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_xlsx_ooxml" {
		t.Fatalf("engine = %v, want native_xlsx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "markdown.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	workbookXML := officeZipEntryText(t, data, "xl/workbook.xml")
	for _, needle := range []string{"Overview", "Metrics", "Owners"} {
		if !strings.Contains(workbookXML, needle) {
			t.Fatalf("workbook.xml missing %q: %s", needle, workbookXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Launch Scorecard", "Launch", "Ready", "Platform", "Design"} {
		if !strings.Contains(doc.Text, needle) {
			t.Fatalf("expected workbook text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestXLSXToolCreateComputesSupportedFormulaCachedValues(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/formula_cache.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Growth", "key": "Growth", "kind": "number"},
					map[string]interface{}{"header": "Total", "key": "Total", "kind": "number"},
				},
				"rows": []interface{}{
					map[string]interface{}{
						"Region":  "East",
						"Revenue": 100.0,
						"Growth":  map[string]interface{}{"value": 0.25, "format": "percent"},
						"Total":   map[string]interface{}{"formula": "B2*(1+C2)", "format": "currency"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	path := filepath.Join(tmpDir, "reports", "formula_cache.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheetXML, "<f>B2*(1+C2)</f><v>125</v>") {
		t.Fatalf("expected formula-only cell to receive a cached value, got %s", sheetXML)
	}
}

func TestXLSXToolCreateComputesSupportedSUMFormulaCachedValues(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/formula_sum.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Total", "key": "Total", "kind": "number"},
				},
				"rows": []interface{}{
					map[string]interface{}{"Region": "East", "Revenue": 100.0},
					map[string]interface{}{"Region": "West", "Revenue": 80.0},
					map[string]interface{}{
						"Region": "Total",
						"Total":  map[string]interface{}{"formula": "SUM(B2:B3)", "format": "currency"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	path := filepath.Join(tmpDir, "reports", "formula_sum.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheetXML, "<f>SUM(B2:B3)</f><v>180</v>") {
		t.Fatalf("expected SUM formula to receive a cached value, got %s", sheetXML)
	}
}

func TestXLSXToolUpdateCellsRecomputesSupportedFormulaCachedValues(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/formula_recalc.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Growth", "key": "Growth", "kind": "number"},
					map[string]interface{}{"header": "Total", "key": "Total", "kind": "number"},
				},
				"rows": []interface{}{
					map[string]interface{}{
						"Region":  "East",
						"Revenue": 100.0,
						"Growth":  map[string]interface{}{"value": 0.25, "format": "percent"},
						"Total":   map[string]interface{}{"formula": "B2*(1+C2)", "format": "currency"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	if _, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_cells",
		"path":   "reports/formula_recalc.xlsx",
		"sheet":  "Metrics",
		"cells": map[string]interface{}{
			"B2": 120.0,
			"C2": map[string]interface{}{"value": 0.50, "format": "percent"},
		},
	}); err != nil {
		t.Fatalf("update_cells failed: %v", err)
	}

	path := filepath.Join(tmpDir, "reports", "formula_recalc.xlsx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheetXML, "<f>B2*(1+C2)</f><v>180</v>") {
		t.Fatalf("expected supported formula cache to refresh after precedent edits, got %s", sheetXML)
	}
}

func TestXLSXToolSemanticActionsAndAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Metrics",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Margin", "key": "Margin", "kind": "number"},
				},
				"rows": []interface{}{
					[]interface{}{"East", 100.0, 0.25},
					[]interface{}{"West", 80.0, 0.20},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	for _, args := range []map[string]interface{}{
		{
			"action": "append_rows",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"rows":   []interface{}{[]interface{}{"South", 90.0, 0.15}},
		},
		{
			"action": "update_cells",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"cells": map[string]interface{}{
				"B2": 110.0,
				"C2": map[string]interface{}{"value": 0.30, "format": "percent"},
			},
		},
		{
			"action": "insert_rows",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"row":    4,
			"rows":   []interface{}{[]interface{}{"North", 95.0, 0.12}},
		},
		{
			"action": "delete_rows",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"row":    3,
			"count":  1,
		},
		{
			"action": "set_filter",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"range":  "A1:C4",
		},
		{
			"action": "set_freeze",
			"path":   "reports/metrics.xlsx",
			"sheet":  "Metrics",
			"freeze": "A2",
		},
		{
			"action":   "rename_sheet",
			"path":     "reports/metrics.xlsx",
			"sheet":    "Metrics",
			"new_name": "History",
		},
	} {
		if _, err := tool.Execute(context.Background(), args); err != nil {
			t.Fatalf("%s failed: %v", args["action"], err)
		}
	}

	workbookPath := filepath.Join(tmpDir, "reports", "metrics.xlsx")
	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), workbookPath)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if !containsSubstring(doc.Text, "History") || !containsSubstring(doc.Text, "North") || !containsSubstring(doc.Text, "South") {
		t.Fatalf("unexpected workbook text: %q", doc.Text)
	}
	if containsSubstring(doc.Text, "West") {
		t.Fatalf("expected deleted row to be absent, got text %q", doc.Text)
	}

	data, err := os.ReadFile(workbookPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	workbookXML := officeZipEntryText(t, data, "xl/workbook.xml")
	if !strings.Contains(workbookXML, `name="History"`) {
		t.Fatalf("workbook.xml missing renamed sheet: %s", workbookXML)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet2.xml")
	if !strings.Contains(sheetXML, `<autoFilter ref="A1:C4"/>`) {
		t.Fatalf("metrics worksheet missing autoFilter: %s", sheetXML)
	}
	if !strings.Contains(sheetXML, `topLeftCell="A2"`) {
		t.Fatalf("metrics worksheet missing freeze pane: %s", sheetXML)
	}

	profileResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "profile_sheet",
		"path":   "reports/metrics.xlsx",
		"sheet":  "History",
	})
	if err != nil {
		t.Fatalf("profile_sheet failed: %v", err)
	}
	profilePayload := parseNativeDocumentPayload(t, profileResult)
	profile, ok := profilePayload["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("profile result type = %T, want object", profilePayload["result"])
	}
	if got := asNativeToolInt(t, profile["row_count"]); got != 3 {
		t.Fatalf("profile row_count = %d, want 3", got)
	}

	groupResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "group_by",
		"path":     "reports/metrics.xlsx",
		"sheet":    "History",
		"group_by": "Region",
		"metric":   "Revenue",
	})
	if err != nil {
		t.Fatalf("group_by failed: %v", err)
	}
	groupPayload := parseNativeDocumentPayload(t, groupResult)
	groupData, ok := groupPayload["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("group result type = %T, want object", groupPayload["result"])
	}
	groups, ok := groupData["groups"].(map[string]interface{})
	if !ok {
		t.Fatalf("groups type = %T, want object", groupData["groups"])
	}
	if got := asNativeToolInt(t, groups["East"]); got != 110 {
		t.Fatalf("groups[East] = %d, want 110", got)
	}

	topResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "top_n",
		"path":   "reports/metrics.xlsx",
		"sheet":  "History",
		"metric": "Revenue",
		"n":      2,
	})
	if err != nil {
		t.Fatalf("top_n failed: %v", err)
	}
	topPayload := parseNativeDocumentPayload(t, topResult)
	topData, ok := topPayload["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("top result type = %T, want object", topPayload["result"])
	}
	topRows, ok := topData["rows"].([]interface{})
	if !ok || len(topRows) != 2 {
		t.Fatalf("top rows = %#v, want two rows", topData["rows"])
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics_compare.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "History",
				"columns": []interface{}{
					map[string]interface{}{"header": "Region", "key": "Region", "kind": "text"},
					map[string]interface{}{"header": "Revenue", "key": "Revenue", "kind": "number"},
					map[string]interface{}{"header": "Margin", "key": "Margin", "kind": "number"},
				},
				"rows": []interface{}{
					[]interface{}{"East", 120.0, 0.30},
					[]interface{}{"North", 95.0, 0.12},
					[]interface{}{"South", 90.0, 0.15},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create compare workbook failed: %v", err)
	}

	compareResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":       "sheet_compare",
		"path":         "reports/metrics.xlsx",
		"sheet":        "History",
		"compare_path": "reports/metrics_compare.xlsx",
	})
	if err != nil {
		t.Fatalf("sheet_compare failed: %v", err)
	}
	comparePayload := parseNativeDocumentPayload(t, compareResult)
	compareData, ok := comparePayload["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("compare result type = %T, want object", comparePayload["result"])
	}
	if got := asNativeToolInt(t, compareData["changed_cells"]); got != 1 {
		t.Fatalf("changed_cells = %d, want 1", got)
	}
}

func TestXLSXToolSemanticEditPreservesUntouchedSheetXML(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/multi_sheet.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Primary",
				"rows": []interface{}{
					[]interface{}{"Name", "Value"},
					[]interface{}{"Alpha", 1},
				},
			},
			map[string]interface{}{
				"name": "Archive",
				"rows": []interface{}{
					[]interface{}{"Name", "Value"},
					[]interface{}{"Beta", 2},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	path := filepath.Join(tmpDir, "reports", "multi_sheet.xlsx")
	_, err = replaceArchiveEntries(path, func(name string) bool {
		return name == "xl/worksheets/sheet3.xml"
	}, func(_ string, data []byte) ([]byte, bool, error) {
		updated := strings.Replace(string(data), `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`, `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><extLst><ext uri="untouched-marker"/></extLst>`, 1)
		return []byte(updated), true, nil
	})
	if err != nil {
		t.Fatalf("inject marker failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action": "append_rows",
		"path":   "reports/multi_sheet.xlsx",
		"sheet":  "Primary",
		"rows":   []interface{}{[]interface{}{"Gamma", 3}},
	})
	if err != nil {
		t.Fatalf("append_rows failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	archiveXML := officeZipEntryText(t, data, "xl/worksheets/sheet3.xml")
	if !strings.Contains(archiveXML, `uri="untouched-marker"`) {
		t.Fatalf("expected untouched sheet marker to remain, got %s", archiveXML)
	}
}

func TestXLSXToolRenameSheetPreservesWorkbookMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/rename.xlsx",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Original",
				"rows": []interface{}{
					[]interface{}{"Name"},
					[]interface{}{"Alpha"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_, err = tool.Execute(context.Background(), map[string]interface{}{
		"action":   "rename_sheet",
		"path":     "reports/rename.xlsx",
		"sheet":    "Original",
		"new_name": "Renamed",
	})
	if err != nil {
		t.Fatalf("rename_sheet failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "reports", "rename.xlsx"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	workbookXML := officeZipEntryText(t, data, "xl/workbook.xml")
	if !strings.Contains(workbookXML, `<bookViews>`) {
		t.Fatalf("expected workbook.xml to preserve bookViews, got %s", workbookXML)
	}
	if !strings.Contains(workbookXML, `name="Renamed"`) {
		t.Fatalf("expected workbook.xml to include renamed sheet, got %s", workbookXML)
	}
}

func TestXLSXToolSemanticEditAppendsStylesForExistingWorkbookFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	path := filepath.Join(tmpDir, "reports", "styled_existing.xlsx")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeExistingStyleWorkbookXLSX(t, path)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_cells",
		"path":   "reports/styled_existing.xlsx",
		"sheet":  "Data",
		"cells": map[string]interface{}{
			"B2": map[string]interface{}{"value": 1234.56, "format": "currency"},
			"C2": map[string]interface{}{"value": "2024-02-03", "format": "date"},
			"D2": map[string]interface{}{"value": 0.42, "format": "percent"},
		},
	})
	if err != nil {
		t.Fatalf("update_cells failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	stylesXML := officeZipEntryText(t, data, "xl/styles.xml")
	var styles struct {
		CellXfs struct {
			Items []struct {
				NumFmtID int `xml:"numFmtId,attr"`
			} `xml:"xf"`
		} `xml:"cellXfs"`
	}
	if err := xml.Unmarshal([]byte(stylesXML), &styles); err != nil {
		t.Fatalf("xml.Unmarshal(styles) error = %v", err)
	}
	if len(styles.CellXfs.Items) < 4 {
		t.Fatalf("expected appended style records, got %d in %s", len(styles.CellXfs.Items), stylesXML)
	}

	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet1.xml")
	styleIDs := extractSheetStyleIDs(t, sheetXML)
	if len(styleIDs) < 4 {
		t.Fatalf("expected style-bearing cells, got %v in %s", styleIDs, sheetXML)
	}
	maxStyle := len(styles.CellXfs.Items) - 1
	for _, styleID := range styleIDs {
		if styleID < 0 || styleID > maxStyle {
			t.Fatalf("style id %d out of range 0..%d in %s", styleID, maxStyle, sheetXML)
		}
	}
	if !strings.Contains(sheetXML, `<v>1234.56</v>`) {
		t.Fatalf("expected currency value to be written, got %s", sheetXML)
	}
	if !strings.Contains(sheetXML, `<v>45325</v>`) {
		t.Fatalf("expected date serial to be written, got %s", sheetXML)
	}
	if !strings.Contains(sheetXML, `<v>0.42</v>`) {
		t.Fatalf("expected percent value to be written, got %s", sheetXML)
	}
}

func TestXLSXToolSemanticEditPreservesColumnStylesAndInheritsThemForUpdatedCells(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	path := filepath.Join(tmpDir, "reports", "column_style.xlsx")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeColumnStyledWorkbookXLSX(t, path)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_cells",
		"path":   "reports/column_style.xlsx",
		"sheet":  "Data",
		"cells": map[string]interface{}{
			"B2": 123,
		},
	})
	if err != nil {
		t.Fatalf("update_cells failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet1.xml")
	if !strings.Contains(sheetXML, `style="1"`) {
		t.Fatalf("expected column style to be preserved, got %s", sheetXML)
	}
	if style := extractCellStyleByRef(t, sheetXML, "B2"); style != 1 {
		t.Fatalf("B2 style = %d, want inherited column style 1 in %s", style, sheetXML)
	}
	if !strings.Contains(sheetXML, `<v>123</v>`) {
		t.Fatalf("expected numeric value to be written, got %s", sheetXML)
	}
}

func TestXLSXToolSemanticEditAppendsAlignmentSpecificStyleWhenExistingFormatUsesDifferentAlignment(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	path := filepath.Join(tmpDir, "reports", "alignment_style.xlsx")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writeAlignmentSensitiveWorkbookXLSX(t, path)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "update_cells",
		"path":   "reports/alignment_style.xlsx",
		"sheet":  "Data",
		"cells": map[string]interface{}{
			"B2": map[string]interface{}{"value": 0.42, "format": "percent"},
		},
	})
	if err != nil {
		t.Fatalf("update_cells failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	stylesXML := officeZipEntryText(t, data, "xl/styles.xml")
	var styles struct {
		CellXfs struct {
			Items []struct {
				NumFmtID  int `xml:"numFmtId,attr"`
				Alignment struct {
					Horizontal string `xml:"horizontal,attr"`
				} `xml:"alignment"`
			} `xml:"xf"`
		} `xml:"cellXfs"`
	}
	if err := xml.Unmarshal([]byte(stylesXML), &styles); err != nil {
		t.Fatalf("xml.Unmarshal(styles) error = %v", err)
	}
	if len(styles.CellXfs.Items) != 4 {
		t.Fatalf("expected appended alignment-specific style, got %d xfs in %s", len(styles.CellXfs.Items), stylesXML)
	}
	last := styles.CellXfs.Items[3]
	if last.NumFmtID != 10 {
		t.Fatalf("appended xf numFmtId = %d, want 10 in %s", last.NumFmtID, stylesXML)
	}
	if last.Alignment.Horizontal != "center" {
		t.Fatalf("appended xf horizontal alignment = %q, want center in %s", last.Alignment.Horizontal, stylesXML)
	}

	sheetXML := officeZipEntryText(t, data, "xl/worksheets/sheet1.xml")
	if style := extractCellStyleByRef(t, sheetXML, "B2"); style != 3 {
		t.Fatalf("B2 style = %d, want newly appended style 3 in %s", style, sheetXML)
	}
}

func writeExistingStyleWorkbookXLSX(t *testing.T, path string) {
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

	writeZipEntry("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`)
	writeZipEntry("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)
	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <bookViews><workbookView xWindow="0" yWindow="0" windowWidth="10000" windowHeight="6000"/></bookViews>
  <sheets>
    <sheet name="Data" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
	writeZipEntry("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Aptos"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="1" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
  </cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`)
	writeZipEntry("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1">
      <c r="A1" t="inlineStr"><is><t>Name</t></is></c>
      <c r="B1" t="inlineStr"><is><t>Amount</t></is></c>
      <c r="C1" t="inlineStr"><is><t>ClosedOn</t></is></c>
      <c r="D1" t="inlineStr"><is><t>Ratio</t></is></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t>Alpha</t></is></c>
      <c r="B2" s="1"><v>100</v></c>
      <c r="C2"><v>0</v></c>
      <c r="D2"><v>0.1</v></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
}

func writeColumnStyledWorkbookXLSX(t *testing.T, path string) {
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

	writeZipEntry("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`)
	writeZipEntry("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)
	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <bookViews><workbookView xWindow="0" yWindow="0" windowWidth="10000" windowHeight="6000"/></bookViews>
  <sheets>
    <sheet name="Data" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
	writeZipEntry("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Aptos"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="2">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="1" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/>
  </cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`)
	writeZipEntry("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <cols>
    <col min="1" max="1" width="12" customWidth="1"/>
    <col min="2" max="2" width="14" style="1" customWidth="1" customFormat="1"/>
  </cols>
  <sheetData>
    <row r="1">
      <c r="A1" t="inlineStr"><is><t>Name</t></is></c>
      <c r="B1" t="inlineStr"><is><t>Amount</t></is></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t>Alpha</t></is></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
}

func writeAlignmentSensitiveWorkbookXLSX(t *testing.T, path string) {
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

	writeZipEntry("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
  <Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
  <Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>
</Types>`)
	writeZipEntry("_rels/.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`)
	writeZipEntry("xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <bookViews><workbookView xWindow="0" yWindow="0" windowWidth="10000" windowHeight="6000"/></bookViews>
  <sheets>
    <sheet name="Data" sheetId="1" r:id="rId1"/>
  </sheets>
</workbook>`)
	writeZipEntry("xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>
</Relationships>`)
	writeZipEntry("xl/styles.xml", `<?xml version="1.0" encoding="UTF-8"?>
<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <fonts count="1"><font><sz val="11"/><name val="Aptos"/></font></fonts>
  <fills count="2">
    <fill><patternFill patternType="none"/></fill>
    <fill><patternFill patternType="gray125"/></fill>
  </fills>
  <borders count="1">
    <border><left/><right/><top/><bottom/><diagonal/></border>
  </borders>
  <cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs>
  <cellXfs count="3">
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/>
    <xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0" applyAlignment="1"><alignment horizontal="center"/></xf>
    <xf numFmtId="10" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1" applyAlignment="1"><alignment horizontal="left"/></xf>
  </cellXfs>
  <cellStyles count="1"><cellStyle name="Normal" xfId="0" builtinId="0"/></cellStyles>
</styleSheet>`)
	writeZipEntry("xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <cols>
    <col min="1" max="1" width="12" customWidth="1"/>
    <col min="2" max="2" width="14" style="1" customWidth="1" customFormat="1"/>
  </cols>
  <sheetData>
    <row r="1">
      <c r="A1" t="inlineStr"><is><t>Name</t></is></c>
      <c r="B1" t="inlineStr"><is><t>Ratio</t></is></c>
    </row>
    <row r="2">
      <c r="A2" t="inlineStr"><is><t>Alpha</t></is></c>
    </row>
  </sheetData>
</worksheet>`)

	if err := zw.Close(); err != nil {
		t.Fatalf("close xlsx zip: %v", err)
	}
}

func extractSheetStyleIDs(t *testing.T, sheetXML string) []int {
	t.Helper()
	var worksheet struct {
		Rows []struct {
			Cells []struct {
				Style int `xml:"s,attr"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal([]byte(sheetXML), &worksheet); err != nil {
		t.Fatalf("xml.Unmarshal(sheet) error = %v", err)
	}
	var out []int
	for _, row := range worksheet.Rows {
		for _, cell := range row.Cells {
			out = append(out, cell.Style)
		}
	}
	return out
}

func extractCellStyleByRef(t *testing.T, sheetXML, ref string) int {
	t.Helper()
	var worksheet struct {
		Rows []struct {
			Cells []struct {
				Ref   string `xml:"r,attr"`
				Style int    `xml:"s,attr"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal([]byte(sheetXML), &worksheet); err != nil {
		t.Fatalf("xml.Unmarshal(sheet) error = %v", err)
	}
	for _, row := range worksheet.Rows {
		for _, cell := range row.Cells {
			if strings.EqualFold(strings.TrimSpace(cell.Ref), strings.TrimSpace(ref)) {
				return cell.Style
			}
		}
	}
	t.Fatalf("cell %s not found in %s", ref, sheetXML)
	return 0
}
