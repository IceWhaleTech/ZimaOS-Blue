package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
)

func TestDOCXToolCreateAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":     "create",
		"path":       "reports/ui_review.docx",
		"style_hint": "UI audit report",
		"content": `# UI Review Report
Polished narrative

## Key Findings
- Contrast hierarchy is strong
- Dense tables need more spacing
`,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_docx_ooxml" {
		t.Fatalf("engine = %v, want native_docx_ooxml", got)
	}
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	if _, ok := payload["validation"].(map[string]interface{}); !ok {
		t.Fatalf("expected validation payload, got %#v", payload["validation"])
	}
	validation := payload["validation"].(map[string]interface{})
	if got, ok := validation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("quality_ok = %#v, want true with validation=%#v", validation["quality_ok"], validation)
	}
	if got := asNativeToolInt(t, validation["quality_issue_count"]); got != 0 {
		t.Fatalf("quality_issue_count = %d, want 0 (%#v)", got, validation)
	}

	path := filepath.Join(tmpDir, "reports", "ui_review.docx")
	assertZipEntryExists(t, path, "word/document.xml")
	assertZipEntryExists(t, path, "word/styles.xml")

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if doc.ExtractedVia != "local_docx" {
		t.Fatalf("ExtractedVia = %q, want local_docx", doc.ExtractedVia)
	}
	if !containsSubstring(doc.Text, "UI Review Report") || !containsSubstring(doc.Text, "Contrast hierarchy is strong") {
		t.Fatalf("unexpected document text: %q", doc.Text)
	}

	readResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "read",
		"path":   "reports/ui_review.docx",
	})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readPayload := parseNativeDocumentPayload(t, readResult)
	if !containsSubstring(asNativeToolString(t, readPayload["text"]), "UI Review Report") {
		t.Fatalf("unexpected tool read text: %#v", readPayload["text"])
	}
	readValidation := readPayload["validation"].(map[string]interface{})
	if got, ok := readValidation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("read quality_ok = %#v, want true with validation=%#v", readValidation["quality_ok"], readValidation)
	}
	if got := asNativeToolInt(t, readValidation["quality_issue_count"]); got != 0 {
		t.Fatalf("read quality_issue_count = %d, want 0 (%#v)", got, readValidation)
	}
}

func TestDOCXToolCreate_FromSingleMarkdownInputDerivesOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	if err := os.MkdirAll(filepath.Join(tmpDir, "reports"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	seed := "# Seed Docx Title\n\nThis is a seeded DOCX.\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "reports", "seed.md"), []byte(seed), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	args := map[string]interface{}{
		"action": "create",
		"path":   "reports/seed.md",
		"theme":  "editorial",
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("create failed: %v (args=%#v)", err, args)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["path"]; got != "reports/seed.docx" {
		t.Fatalf("path = %v, want reports/seed.docx", got)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), filepath.Join(tmpDir, "reports", "seed.docx"))
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if !containsSubstring(doc.Text, "Seed Docx Title") {
		t.Fatalf("expected docx text to include title, got %q", doc.Text)
	}
}

func TestDOCXToolCreateWithStructuredTableColumnWidths(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics_structured_columns.docx",
		"title":  "Quarterly Metrics Structured Columns",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region", "width": 2},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "width": 1},
						map[string]interface{}{"header": "Status", "key": "status", "width": 1},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": "120", "status": "Good"},
						map[string]interface{}{"region": "South", "revenue": "98", "status": "Watch"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_docx_ooxml" {
		t.Fatalf("engine = %v, want native_docx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "metrics_structured_columns.docx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	documentXML := officeZipEntryText(t, data, "word/document.xml")
	for _, needle := range []string{
		`<w:tbl>`,
		`<w:gridCol w:w="4500"/>`,
		`<w:gridCol w:w="2250"/>`,
		`<w:tcW w:w="4500" w:type="dxa"/>`,
		`<w:tcW w:w="2250" w:type="dxa"/>`,
		`Region`,
		`North`,
		`120`,
		`Good`,
	} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("expected word/document.xml to include %q, got %s", needle, documentXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Structured Columns", "Performance snapshot", "Region", "North", "South", "120", "98"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected document text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestDOCXToolCreateWithStructuredTableColumnAlignmentHints(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics_table_alignment.docx",
		"title":  "Quarterly Metrics Table Alignment",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region", "align": "left"},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "kind": "integer"},
						map[string]interface{}{"header": "Status", "key": "status", "align": "center"},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": "120", "status": "Good"},
						map[string]interface{}{"region": "South", "revenue": "98", "status": "Watch"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_docx_ooxml" {
		t.Fatalf("engine = %v, want native_docx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "metrics_table_alignment.docx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	documentXML := officeZipEntryText(t, data, "word/document.xml")
	for _, needle := range []string{
		`<w:jc w:val="left"/>`,
		`<w:jc w:val="right"/>`,
		`<w:jc w:val="center"/>`,
		`Region`,
		`North`,
		`120`,
		`Good`,
	} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("expected word/document.xml to include %q, got %s", needle, documentXML)
		}
	}
}

func TestDOCXToolCreateWithStructuredTableColumnDisplayFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics_table_formats.docx",
		"title":  "Quarterly Metrics Table Formats",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region"},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "kind": "currency"},
						map[string]interface{}{"header": "Growth", "key": "growth", "kind": "percent"},
						map[string]interface{}{"header": "Score", "key": "score", "kind": "decimal"},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": 1250.5, "growth": 0.125, "score": 3.5},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_docx_ooxml" {
		t.Fatalf("engine = %v, want native_docx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "metrics_table_formats.docx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	documentXML := officeZipEntryText(t, data, "word/document.xml")
	for _, needle := range []string{
		`$1250.50`,
		`12.50%`,
		`3.50`,
	} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("expected word/document.xml to include %q, got %s", needle, documentXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Table Formats", "$1250.50", "12.50%", "3.50"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected document text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestDOCXToolCreateWithStructuredTableDateDisplayFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewDOCXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/metrics_table_dates.docx",
		"title":  "Quarterly Metrics Table Dates",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region"},
						map[string]interface{}{"header": "Closed On", "key": "closed_on", "kind": "date"},
						map[string]interface{}{"header": "Reviewed At", "key": "reviewed_at", "kind": "datetime"},
					},
					"rows": []interface{}{
						map[string]interface{}{
							"region":      "North",
							"closed_on":   "2024-02-03T09:45:00Z",
							"reviewed_at": "2024-03-01T12:30:00Z",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_docx_ooxml" {
		t.Fatalf("engine = %v, want native_docx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "reports", "metrics_table_dates.docx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	documentXML := officeZipEntryText(t, data, "word/document.xml")
	for _, needle := range []string{
		`2024-02-03`,
		`2024-03-01 12:30`,
	} {
		if !containsSubstring(documentXML, needle) {
			t.Fatalf("expected word/document.xml to include %q, got %s", needle, documentXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Table Dates", "2024-02-03", "2024-03-01 12:30"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected document text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestXLSXToolCreateAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "reports/ui_review.xlsx",
		"theme":    "ui_review",
		"title":    "UI Review Comparison",
		"subtitle": "Polished workbook",
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Findings",
				"columns": []interface{}{
					map[string]interface{}{"header": "Area", "kind": "text"},
					map[string]interface{}{"header": "Score", "kind": "number"},
				},
				"rows": []interface{}{
					[]interface{}{"Visual clarity", 92.0},
					[]interface{}{"Layout rhythm", 84.0},
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
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	validation := payload["validation"].(map[string]interface{})
	if got, ok := validation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("quality_ok = %#v, want true with validation=%#v", validation["quality_ok"], validation)
	}
	if got := asNativeToolInt(t, validation["quality_issue_count"]); got != 0 {
		t.Fatalf("quality_issue_count = %d, want 0 (%#v)", got, validation)
	}

	path := filepath.Join(tmpDir, "reports", "ui_review.xlsx")
	assertZipEntryExists(t, path, "xl/workbook.xml")
	assertZipEntryExists(t, path, "xl/styles.xml")

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if doc.ExtractedVia != "local_spreadsheet" {
		t.Fatalf("ExtractedVia = %q, want local_spreadsheet", doc.ExtractedVia)
	}
	if !containsSubstring(doc.Text, "UI Review Comparison") || !containsSubstring(doc.Text, "Visual clarity") {
		t.Fatalf("unexpected workbook text: %q", doc.Text)
	}

	readResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "read",
		"path":   "reports/ui_review.xlsx",
	})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readPayload := parseNativeDocumentPayload(t, readResult)
	if _, ok := readPayload["tabular_summary"].(map[string]interface{}); !ok {
		t.Fatalf("expected tabular_summary, got %#v", readPayload["tabular_summary"])
	}
	readValidation := readPayload["validation"].(map[string]interface{})
	if got, ok := readValidation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("read quality_ok = %#v, want true with validation=%#v", readValidation["quality_ok"], readValidation)
	}
	if got := asNativeToolInt(t, readValidation["quality_issue_count"]); got != 0 {
		t.Fatalf("read quality_issue_count = %d, want 0 (%#v)", got, readValidation)
	}
}

func TestXLSXToolCreate_FromSingleMarkdownInputDerivesOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewXLSXTool([]string{tmpDir}, nil, nil)

	if err := os.MkdirAll(filepath.Join(tmpDir, "reports"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	seed := `# Seed XLSX

| Area | Score |
| --- | ---: |
| Visual clarity | 92 |
| Layout rhythm | 84 |
`
	if err := os.WriteFile(filepath.Join(tmpDir, "reports", "seed.md"), []byte(seed), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/seed.md",
		"theme":  "analysis",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["path"]; got != "reports/seed.xlsx" {
		t.Fatalf("path = %v, want reports/seed.xlsx", got)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), filepath.Join(tmpDir, "reports", "seed.xlsx"))
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if !containsSubstring(doc.Text, "Seed XLSX") || !containsSubstring(doc.Text, "Visual clarity") {
		t.Fatalf("unexpected workbook text: %q", doc.Text)
	}
}

func TestPPTXToolCreateAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/launch.pptx",
		"title":    "Launch Plan",
		"subtitle": "Q2 roll-out",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Beta in April", "GA in June"},
			},
			map[string]interface{}{
				"heading": "Risks",
				"bullets": []interface{}{"Migration timing", "Support readiness"},
			},
		},
		"summary": "Focus on a clean 16:9 executive structure.",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 5 {
		t.Fatalf("slide_count = %d, want 5", got)
	}
	validation := payload["validation"].(map[string]interface{})
	if got, ok := validation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("quality_ok = %#v, want true with validation=%#v", validation["quality_ok"], validation)
	}
	if got := asNativeToolInt(t, validation["quality_issue_count"]); got != 0 {
		t.Fatalf("quality_issue_count = %d, want 0 (%#v)", got, validation)
	}

	path := filepath.Join(tmpDir, "decks", "launch.pptx")
	assertZipEntryExists(t, path, "ppt/presentation.xml")
	assertZipEntryExists(t, path, "ppt/slides/slide1.xml")

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if doc.ExtractedVia != "local_pptx" {
		t.Fatalf("ExtractedVia = %q, want local_pptx", doc.ExtractedVia)
	}
	if !containsSubstring(doc.Text, "Launch Plan") || !containsSubstring(doc.Text, "Beta in April") {
		t.Fatalf("unexpected deck text: %q", doc.Text)
	}
	if !containsSubstring(doc.Text, "Table of Contents") {
		t.Fatalf("expected deck text to include TOC slide, got %q", doc.Text)
	}
	if !containsSubstring(doc.Text, "Overview") || !containsSubstring(doc.Text, "Risks") || !containsSubstring(doc.Text, "Summary") {
		t.Fatalf("expected deck text to include planned slide structure, got %q", doc.Text)
	}

	readResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "read",
		"path":   "decks/launch.pptx",
	})
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	readPayload := parseNativeDocumentPayload(t, readResult)
	if !containsSubstring(asNativeToolString(t, readPayload["text"]), "Launch Plan") {
		t.Fatalf("unexpected tool read text: %#v", readPayload["text"])
	}
	readValidation := readPayload["validation"].(map[string]interface{})
	if got, ok := readValidation["quality_ok"].(bool); !ok || !got {
		t.Fatalf("read quality_ok = %#v, want true with validation=%#v", readValidation["quality_ok"], readValidation)
	}
	if got := asNativeToolInt(t, readValidation["quality_issue_count"]); got != 0 {
		t.Fatalf("read quality_issue_count = %d, want 0 (%#v)", got, readValidation)
	}
}

func TestPPTXToolCreate_FromSingleMarkdownInputDerivesOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	if err := os.MkdirAll(filepath.Join(tmpDir, "decks"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	seed := `# Seed Deck Title

## Overview
- Slide one bullet
- Slide one follow-up
`
	if err := os.WriteFile(filepath.Join(tmpDir, "decks", "seed.md"), []byte(seed), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/seed.md",
		"theme":  "editorial",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["path"]; got != "decks/seed.pptx" {
		t.Fatalf("path = %v, want decks/seed.pptx", got)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), filepath.Join(tmpDir, "decks", "seed.pptx"))
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if !containsSubstring(doc.Text, "Seed Deck Title") || !containsSubstring(doc.Text, "Slide one bullet") {
		t.Fatalf("unexpected deck text: %q", doc.Text)
	}
}

func TestPPTXToolCreateWithStructuredTable(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/metrics.pptx",
		"title":    "Quarterly Metrics",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"headers": []interface{}{"Region", "Revenue", "Status"},
					"rows": []interface{}{
						[]interface{}{"North", "120", "Good"},
						[]interface{}{"South", "98", "Watch"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "metrics.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	if !containsSubstring(slideXML, `<a:tbl>`) {
		t.Fatalf("expected slide3.xml to contain native table markup, got %s", slideXML)
	}
	if containsSubstring(slideXML, `<a:t>Region | Revenue | Status</a:t>`) {
		t.Fatalf("expected native table cells instead of flattened table header text, got %s", slideXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics", "Performance snapshot", "Region", "North", "South"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolValidateFlagsMarkdownAndPlaceholderQualityIssues(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/quality_validate.pptx",
		"title":    "Launch Plan",
		"subtitle": "Q2 roll-out",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Beta in April", "GA in June"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	_ = parseNativeDocumentPayload(t, result)

	path := filepath.Join(tmpDir, "decks", "quality_validate.pptx")
	changed, err := replaceArchiveEntries(path, func(name string) bool {
		return name == "ppt/slides/slide3.xml" ||
			name == "docProps/app.xml" ||
			name == "ppt/presentation.xml" ||
			name == "ppt/_rels/presentation.xml.rels"
	}, func(name string, data []byte) ([]byte, bool, error) {
		text := string(data)
		updated := text
		switch name {
		case "ppt/slides/slide3.xml":
			updated = strings.ReplaceAll(updated, ">Overview<", ">幻灯片 1：封面<")
			updated = strings.ReplaceAll(updated, ">Beta in April<", "># Broken heading<")
		case "docProps/app.xml":
			updated = strings.ReplaceAll(updated, ">Overview<", ">幻灯片 1：封面<")
		case "ppt/presentation.xml":
			updated = regexp.MustCompile(`<p:sldId[^>]+r:id="rId4"\s*/>`).ReplaceAllString(updated, "")
		case "ppt/_rels/presentation.xml.rels":
			updated = regexp.MustCompile(`<Relationship[^>]+Id="rId4"[^>]*/>`).ReplaceAllString(updated, "")
		}
		return []byte(updated), updated != text, nil
	})
	if err != nil {
		t.Fatalf("replaceArchiveEntries() error = %v", err)
	}
	if !changed {
		t.Fatal("expected test deck patching to change at least one archive entry")
	}

	validateResult, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "validate",
		"path":   "decks/quality_validate.pptx",
	})
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	validatePayload := parseNativeDocumentPayload(t, validateResult)
	validation := validatePayload["validation"].(map[string]interface{})
	if got, ok := validation["quality_ok"].(bool); !ok || got {
		t.Fatalf("quality_ok = %#v, want false with validation=%#v", validation["quality_ok"], validation)
	}
	if got := asNativeToolInt(t, validation["quality_issue_count"]); got < 3 {
		t.Fatalf("quality_issue_count = %d, want at least 3 (%#v)", got, validation)
	}
	issues := asNativeToolStringSlice(t, validation["quality_issues"])
	for _, issue := range []string{"raw_markdown_heading_detected", "placeholder_slide_title_detected", "orphan_slide_part_detected"} {
		if !hasStringValue(issues, issue) {
			t.Fatalf("quality issues = %v, want %q", issues, issue)
		}
	}
	if got := asNativeToolInt(t, validation["raw_markdown_heading_detected_count"]); got < 1 {
		t.Fatalf("raw_markdown_heading_detected_count = %d, want >= 1", got)
	}
	if got := asNativeToolInt(t, validation["placeholder_slide_title_detected_count"]); got < 1 {
		t.Fatalf("placeholder_slide_title_detected_count = %d, want >= 1", got)
	}
	if got := asNativeToolInt(t, validation["orphan_slide_part_detected_count"]); got < 1 {
		t.Fatalf("orphan_slide_part_detected_count = %d, want >= 1", got)
	}
}

func TestPPTXToolCreateWithStructuredTableColumnObjectsAndObjectRows(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/metrics_structured_columns.pptx",
		"title":    "Quarterly Metrics Structured Columns",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region", "width": 2},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "width": 1},
						map[string]interface{}{"header": "Status", "key": "status", "width": 1},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": "120", "status": "Good"},
						map[string]interface{}{"region": "South", "revenue": "98", "status": "Watch"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "metrics_structured_columns.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`<a:tbl>`,
		`<a:gridCol w="5429250"/>`,
		`<a:gridCol w="2714625"/>`,
		`<a:t>Region</a:t>`,
		`<a:t>Revenue</a:t>`,
		`<a:t>Status</a:t>`,
		`<a:t>North</a:t>`,
		`<a:t>120</a:t>`,
		`<a:t>Good</a:t>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Structured Columns", "Performance snapshot", "Region", "North", "South", "120", "98"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithStructuredTableColumnAlignmentHints(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/metrics_table_alignment.pptx",
		"title":    "Quarterly Metrics Table Alignment",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region", "align": "left"},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "kind": "integer"},
						map[string]interface{}{"header": "Status", "key": "status", "align": "center"},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": "120", "status": "Good"},
						map[string]interface{}{"region": "South", "revenue": "98", "status": "Watch"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "metrics_table_alignment.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`<a:pPr algn="l">`,
		`<a:pPr algn="r">`,
		`<a:pPr algn="ctr">`,
		`<a:t>North</a:t>`,
		`<a:t>120</a:t>`,
		`<a:t>Good</a:t>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}
}

func TestPPTXToolCreateWithStructuredTableColumnDisplayFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/metrics_table_formats.pptx",
		"title":    "Quarterly Metrics Table Formats",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region"},
						map[string]interface{}{"header": "Revenue", "key": "revenue", "kind": "currency"},
						map[string]interface{}{"header": "Growth", "key": "growth", "kind": "percent"},
						map[string]interface{}{"header": "Score", "key": "score", "kind": "decimal"},
					},
					"rows": []interface{}{
						map[string]interface{}{"region": "North", "revenue": 1250.5, "growth": 0.125, "score": 3.5},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "metrics_table_formats.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`<a:t>$1250.50</a:t>`,
		`<a:t>12.50%</a:t>`,
		`<a:t>3.50</a:t>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Table Formats", "$1250.50", "12.50%", "3.50"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithStructuredTableDateDisplayFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/metrics_table_dates.pptx",
		"title":    "Quarterly Metrics Table Dates",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Metrics",
				"paragraphs": []interface{}{"Performance snapshot"},
				"table": map[string]interface{}{
					"columns": []interface{}{
						map[string]interface{}{"header": "Region", "key": "region"},
						map[string]interface{}{"header": "Closed On", "key": "closed_on", "kind": "date"},
						map[string]interface{}{"header": "Reviewed At", "key": "reviewed_at", "kind": "datetime"},
					},
					"rows": []interface{}{
						map[string]interface{}{
							"region":      "North",
							"closed_on":   "2024-02-03T09:45:00Z",
							"reviewed_at": "2024-03-01T12:30:00Z",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "metrics_table_dates.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`<a:t>2024-02-03</a:t>`,
		`<a:t>2024-03-01 12:30</a:t>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Metrics Table Dates", "2024-02-03", "2024-03-01 12:30"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithNativeChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_chart.pptx",
		"title":    "Quarterly Revenue",
		"subtitle": "Q2 snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue Trend",
				"paragraphs": []interface{}{"Revenue by region"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"North", "South", "West"},
					"series": []interface{}{
						map[string]interface{}{
							"name":   "Revenue",
							"values": []interface{}{120, 98, 110},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_chart.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	if !containsSubstring(slideXML, `http://schemas.openxmlformats.org/drawingml/2006/chart`) {
		t.Fatalf("expected slide3.xml to contain native chart markup, got %s", slideXML)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:barChart>`) || !containsSubstring(chartXML, `<c:v>North</c:v>`) {
		t.Fatalf("expected chart1.xml to contain a native bar chart with categories, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Quarterly Revenue", "Revenue by region", "Revenue", "North", "South", "West"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithStackedBarChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/stacked_chart.pptx",
		"title":    "Pipeline Coverage",
		"subtitle": "Regional split",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Coverage",
				"paragraphs": []interface{}{"Actual versus target by region"},
				"chart": map[string]interface{}{
					"type":       "stacked_bar",
					"categories": []interface{}{"North", "South"},
					"series": []interface{}{
						map[string]interface{}{"name": "Actual", "values": []interface{}{120, 98}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 110}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "stacked_chart.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:barDir val="bar"/>`) || !containsSubstring(chartXML, `<c:grouping val="stacked"/>`) {
		t.Fatalf("expected chart1.xml to contain a native stacked bar chart, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Pipeline Coverage", "Actual versus target by region", "Actual", "Target", "North", "South"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithDonutChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/status_donut.pptx",
		"title":    "Adoption Mix",
		"subtitle": "Account status",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Status Mix",
				"paragraphs": []interface{}{"Current account distribution"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"categories": []interface{}{"Adoption", "Pending", "Blocked"},
					"series": []interface{}{
						map[string]interface{}{"name": "Status", "values": []interface{}{70, 20, 10}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "status_donut.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:doughnutChart>`) || !containsSubstring(chartXML, `<c:holeSize val="50"/>`) {
		t.Fatalf("expected chart1.xml to contain a native doughnut chart, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Adoption Mix", "Current account distribution", "Status", "Adoption", "Pending", "Blocked"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithComboChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo.pptx",
		"title":    "Revenue And Margin",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Revenue columns with margin line"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:barChart>`) || !containsSubstring(chartXML, `<c:lineChart>`) {
		t.Fatalf("expected chart1.xml to contain a native combo chart, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue And Margin", "Revenue columns with margin line", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithComboSecondaryAxisChart(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_secondary.pptx",
		"title":    "Revenue And Margin Axis",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Revenue columns with margin line on the secondary axis"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_secondary.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:barChart>`) || !containsSubstring(chartXML, `<c:lineChart>`) {
		t.Fatalf("expected chart1.xml to contain a native combo chart, got %s", chartXML)
	}
	if !containsSubstring(chartXML, `<c:axPos val="r"/>`) || strings.Count(chartXML, `<c:valAx>`) != 2 {
		t.Fatalf("expected chart1.xml to contain a secondary value axis, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue And Margin Axis", "Revenue columns with margin line on the secondary axis", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithAxisTitles(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_titles.pptx",
		"title":    "Revenue Axis Titles",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Axis titles describe both revenue and margin scales"},
				"chart": map[string]interface{}{
					"type":          "combo",
					"x_axis_title":  "Quarter",
					"y_axis_title":  "Revenue ($M)",
					"y2_axis_title": "Margin %",
					"categories":    []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_titles.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<a:t>Quarter</a:t>`,
		`<a:t>Revenue ($M)</a:t>`,
		`<a:t>Margin %</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Titles", "Axis titles describe both revenue and margin scales", "Quarter", "Revenue ($M)", "Margin %"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPrimaryDateCategoryAxis(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-02-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-02-01")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_primary_date_axis.pptx",
		"title":    "Revenue Primary Date Axis",
		"subtitle": "Monthly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date-based category axes preserve calendar spacing"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_primary_date_axis.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dateAx>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
		`<c:baseTimeUnit val="days"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected chart1.xml to switch away from catAx, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Primary Date Axis", "Date-based category axes preserve calendar spacing", "Trend", "2026-01-01", "2026-02-01"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPrimaryDateCategoryAxisShortDateFormatReadback(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_primary_date_axis_short_format.pptx",
		"title":    "Revenue Primary Date Axis Short Format",
		"subtitle": "Monthly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date-based category axes can preserve short date formatting"},
				"chart": map[string]interface{}{
					"type":          "line",
					"x_axis_type":   "date",
					"x_axis_format": "m/d/yyyy",
					"categories":    []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_primary_date_axis_short_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>m/d/yyyy</c:formatCode><c:ptCount val="2"/>`) {
		t.Fatalf("expected chart1.xml to keep short-date formatCode, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Primary Date Axis Short Format", "Date-based category axes can preserve short date formatting", "Trend", "1/1/2026", "2/1/2026"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
	for _, needle := range []string{"2026-01-01", "2026-02-01"} {
		if containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to avoid ISO date label %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPrimaryDateCategoryAxisEmbeddedWorkbookPackage(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-02-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-02-01")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_primary_date_axis_embedded_workbook.pptx",
		"title":    "Revenue Primary Date Axis Embedded Workbook",
		"subtitle": "Monthly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date-based category axes package workbook-backed chart data"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_primary_date_axis_embedded_workbook.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !officeZipHasEntry(t, data, "ppt/charts/_rels/chart1.xml.rels") {
		t.Fatal("expected ppt/charts/_rels/chart1.xml.rels")
	}
	if !officeZipHasEntry(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatal("expected embedded chart workbook entry")
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:val><c:numRef><c:f>Data!$B$2:$B$3</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>10</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>12</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	chartRelsXML := officeZipEntryText(t, data, "ppt/charts/_rels/chart1.xml.rels")
	if !containsSubstring(chartRelsXML, `Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"`) {
		t.Fatalf("expected chart1.xml.rels to target embedded workbook, got %s", chartRelsXML)
	}

	workbookBytes := officeZipEntryBytes(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx")
	workbookXML := officeZipEntryText(t, workbookBytes, "xl/workbook.xml")
	if !containsSubstring(workbookXML, `<sheet name="Data" sheetId="1" r:id="rId1"/>`) {
		t.Fatalf("expected embedded workbook.xml to include Data sheet, got %s", workbookXML)
	}
	sheetXML := officeZipEntryText(t, workbookBytes, "xl/worksheets/sheet1.xml")
	for _, needle := range []string{
		`<c r="A2"`,
		`<v>` + firstSerial + `</v>`,
		`<v>` + secondSerial + `</v>`,
		`<c r="B2"`,
		`<v>10</v>`,
		`<v>12</v>`,
	} {
		if !containsSubstring(sheetXML, needle) {
			t.Fatalf("expected embedded sheet1.xml to include %q, got %s", needle, sheetXML)
		}
	}
}

func TestPPTXToolCreateWithStringCategoryAxisEmbeddedWorkbookPackage(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_string_axis_embedded_workbook.pptx",
		"title":    "Revenue String Axis Embedded Workbook",
		"subtitle": "Regional snapshot",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Text category axes also package workbook-backed chart data"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"North", "South"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 98}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_string_axis_embedded_workbook.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !officeZipHasEntry(t, data, "ppt/charts/_rels/chart1.xml.rels") {
		t.Fatal("expected ppt/charts/_rels/chart1.xml.rels")
	}
	if !officeZipHasEntry(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx") {
		t.Fatal("expected embedded chart workbook entry")
	}

	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
		`<c:cat><c:strRef><c:f>Data!$A$2:$A$3</c:f><c:strCache><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>North</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>South</c:v></c:pt>`,
		`<c:val><c:numRef><c:f>Data!$B$2:$B$3</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>120</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>98</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	if containsSubstring(chartXML, `<c:strLit>`) {
		t.Fatalf("expected chart1.xml to avoid string literal categories when workbook-backed, got %s", chartXML)
	}

	chartRelsXML := officeZipEntryText(t, data, "ppt/charts/_rels/chart1.xml.rels")
	if !containsSubstring(chartRelsXML, `Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/package" Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"`) {
		t.Fatalf("expected chart1.xml.rels to target embedded workbook, got %s", chartRelsXML)
	}

	workbookBytes := officeZipEntryBytes(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx")
	workbookXML := officeZipEntryText(t, workbookBytes, "xl/workbook.xml")
	if !containsSubstring(workbookXML, `<sheet name="Data" sheetId="1" r:id="rId1"/>`) {
		t.Fatalf("expected embedded workbook.xml to include Data sheet, got %s", workbookXML)
	}
	sheetXML := officeZipEntryText(t, workbookBytes, "xl/worksheets/sheet1.xml")
	for _, needle := range []string{
		`<c r="A2"`,
		`<c r="A3"`,
		`<c r="B2"`,
		`<c r="B3"`,
		`<t xml:space="preserve">North</t>`,
		`<t xml:space="preserve">South</t>`,
		`<t xml:space="preserve">Revenue</t>`,
		`<v>120</v>`,
		`<v>98</v>`,
	} {
		if !containsSubstring(sheetXML, needle) {
			t.Fatalf("expected embedded sheet1.xml to include %q, got %s", needle, sheetXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue String Axis Embedded Workbook", "Text category axes also package workbook-backed chart data", "Revenue", "North", "South"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPrimaryDateTimeCategoryAxis(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01 09:30", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 09:30")
	}
	secondSerial, ok := officeExcelDateSerial("2026-01-01 15:45", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 15:45")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_primary_datetime_axis.pptx",
		"title":    "Revenue Primary DateTime Axis",
		"subtitle": "Intraday trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date-time category axes preserve intraday points"},
				"chart": map[string]interface{}{
					"type":        "line",
					"x_axis_type": "datetime",
					"categories":  []interface{}{"2026-01-01 09:30", "2026-01-01 15:45"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_primary_datetime_axis.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dateAx>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd hh:mm</c:formatCode><c:ptCount val="2"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Primary DateTime Axis", "Date-time category axes preserve intraday points", "Trend", "2026-01-01 09:30", "2026-01-01 15:45"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPrimaryDateTimeCategoryAxisTimeOnlyFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_primary_datetime_time_only_axis.pptx",
		"title":    "Revenue Primary Time Axis",
		"subtitle": "Intraday trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Date-time category axes can expose time-only labels"},
				"chart": map[string]interface{}{
					"type":          "line",
					"x_axis_type":   "datetime",
					"x_axis_format": "hh:mm",
					"categories":    []interface{}{"2026-01-01 09:30", "2026-01-01 15:45"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_primary_datetime_time_only_axis.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>hh:mm</c:formatCode><c:ptCount val="2"/>`) {
		t.Fatalf("expected chart1.xml to keep time-only formatCode, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Primary Time Axis", "Date-time category axes can expose time-only labels", "Trend", "09:30", "15:45"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
	for _, needle := range []string{"2026-01-01 09:30", "2026-01-01 15:45"} {
		if containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to avoid full date-time label %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithComboDateCategoryAxes(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	thirdSerial, ok := officeExcelDateSerial("2026-03-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-03-01")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_date_axis.pptx",
		"title":    "Revenue Combo Date Axis",
		"subtitle": "Monthly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can share mirrored date-based category axes"},
				"chart": map[string]interface{}{
					"type":          "combo",
					"x_axis_type":   "date",
					"x2_axis_title": "Month (Top)",
					"categories":    []interface{}{"2026-01-01", "2026-02-01", "2026-03-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:dateAx>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 date axes, got %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected chart1.xml to avoid catAx, got %s", chartXML)
	}
	for _, needle := range []string{
		`<a:t>Month (Top)</a:t>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>` + thirdSerial + `</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Combo Date Axis", "Combo charts can share mirrored date-based category axes", "Month (Top)", "Revenue", "Margin", "2026-01-01", "2026-03-01"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithComboDateCategoryAxisEmbeddedWorkbookPackage(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-01-01")
	}
	secondSerial, ok := officeExcelDateSerial("2026-02-01", false)
	if !ok {
		t.Fatal("expected date serial for 2026-02-01")
	}

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/revenue_combo_date_axis_embedded_workbook.pptx",
		"title":  "Revenue Combo Date Axis Embedded Workbook",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "Revenue",
				"chart": map[string]interface{}{
					"type":        "combo",
					"x_axis_type": "date",
					"categories":  []interface{}{"2026-01-01", "2026-02-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis_embedded_workbook.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	_ = officeZipEntryBytes(t, data, "ppt/charts/_rels/chart1.xml.rels")
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$3</c:f><c:numCache><c:formatCode>yyyy-mm-dd</c:formatCode>`,
		`<c:val><c:numRef><c:f>Data!$B$2:$B$3</c:f><c:numCache><c:formatCode>General</c:formatCode>`,
		`<c:val><c:numRef><c:f>Data!$C$2:$C$3</c:f><c:numCache><c:formatCode>General</c:formatCode>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="1"><c:v>` + secondSerial + `</c:v></c:pt>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
	chartRelsXML := officeZipEntryText(t, data, "ppt/charts/_rels/chart1.xml.rels")
	if !containsSubstring(chartRelsXML, `Target="../embeddings/Microsoft_Excel_Worksheet1.xlsx"`) {
		t.Fatalf("expected chart1.xml.rels to point at embedded workbook, got %s", chartRelsXML)
	}

	embeddedWorkbook := officeZipEntryBytes(t, data, "ppt/embeddings/Microsoft_Excel_Worksheet1.xlsx")
	workbookXML := officeZipEntryText(t, embeddedWorkbook, "xl/workbook.xml")
	if !containsSubstring(workbookXML, `name="Data"`) {
		t.Fatalf("expected embedded workbook to include Data sheet, got %s", workbookXML)
	}
	sheetXML := officeZipEntryText(t, embeddedWorkbook, "xl/worksheets/sheet1.xml")
	for _, needle := range []string{
		`<v>` + firstSerial + `</v>`,
		`<v>` + secondSerial + `</v>`,
		`<v>120</v>`,
		`<v>132</v>`,
		`<v>28</v>`,
		`<v>31</v>`,
	} {
		if !containsSubstring(sheetXML, needle) {
			t.Fatalf("expected embedded sheet1.xml to include %q, got %s", needle, sheetXML)
		}
	}
}

func TestPPTXToolCreateWithComboDateTimeCategoryAxes(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	firstSerial, ok := officeExcelDateSerial("2026-01-01 09:30", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 09:30")
	}
	thirdSerial, ok := officeExcelDateSerial("2026-01-01 18:15", true)
	if !ok {
		t.Fatal("expected date-time serial for 2026-01-01 18:15")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_datetime_axis.pptx",
		"title":    "Revenue Combo DateTime Axis",
		"subtitle": "Intraday trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can share mirrored date-time category axes"},
				"chart": map[string]interface{}{
					"type":          "combo",
					"x_axis_type":   "datetime",
					"x2_axis_title": "Time (Top)",
					"categories":    []interface{}{"2026-01-01 09:30", "2026-01-01 13:00", "2026-01-01 18:15"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_datetime_axis.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:dateAx>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 date axes, got %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:catAx>`) {
		t.Fatalf("expected chart1.xml to avoid catAx, got %s", chartXML)
	}
	for _, needle := range []string{
		`<a:t>Time (Top)</a:t>`,
		`<c:cat><c:numRef><c:f>Data!$A$2:$A$4</c:f><c:numCache><c:formatCode>yyyy-mm-dd hh:mm</c:formatCode><c:ptCount val="3"/>`,
		`<c:pt idx="0"><c:v>` + firstSerial + `</c:v></c:pt>`,
		`<c:pt idx="2"><c:v>` + thirdSerial + `</c:v></c:pt>`,
		`<c:externalData r:id="rId1"><c:autoUpdate val="0"/></c:externalData>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Combo DateTime Axis", "Combo charts can share mirrored date-time category axes", "Time (Top)", "Revenue", "Margin", "2026-01-01 09:30", "2026-01-01 18:15"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithDateCategoryAxisBaseTimeUnit(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_date_axis_years.pptx",
		"title":    "Revenue Combo Date Axis Years",
		"subtitle": "Yearly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can override the date-axis base time unit"},
				"chart": map[string]interface{}{
					"type":                  "combo",
					"x_axis_type":           "date",
					"x_axis_base_time_unit": "years",
					"categories":            []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis_years.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:baseTimeUnit val="years"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 year baseTimeUnit nodes, got %s", chartXML)
	}
	if containsSubstring(chartXML, `<c:baseTimeUnit val="days"/>`) {
		t.Fatalf("expected chart1.xml to override default days baseTimeUnit, got %s", chartXML)
	}
}

func TestPPTXToolCreateWithDateCategoryAxisMajorMinorTimeUnits(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_date_axis_time_units.pptx",
		"title":    "Revenue Combo Date Axis Time Units",
		"subtitle": "Yearly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can override the date-axis major and minor time units"},
				"chart": map[string]interface{}{
					"type":                   "combo",
					"x_axis_type":            "date",
					"x_axis_major_time_unit": "years",
					"x_axis_minor_time_unit": "months",
					"categories":             []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis_time_units.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:majorTimeUnit val="years"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 year majorTimeUnit nodes, got %s", chartXML)
	}
	if strings.Count(chartXML, `<c:minorTimeUnit val="months"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 month minorTimeUnit nodes, got %s", chartXML)
	}
}

func TestPPTXToolCreateWithDateCategoryAxisMajorMinorUnits(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_date_axis_units.pptx",
		"title":    "Revenue Combo Date Axis Units",
		"subtitle": "Yearly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can override the date-axis numeric major and minor units"},
				"chart": map[string]interface{}{
					"type":              "combo",
					"x_axis_type":       "date",
					"x_axis_major_unit": 12,
					"x_axis_minor_unit": 1,
					"categories":        []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis_units.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:majorUnit val="12"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 majorUnit nodes, got %s", chartXML)
	}
	if strings.Count(chartXML, `<c:minorUnit val="1"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 minorUnit nodes, got %s", chartXML)
	}
}

func TestPPTXToolCreateWithDateCategoryAxisBounds(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	minSerial, ok := officeExcelDateSerial("2025-12-01", false)
	if !ok {
		t.Fatal("expected date serial for 2025-12-01")
	}
	maxSerial, ok := officeExcelDateSerial("2028-12-31", false)
	if !ok {
		t.Fatal("expected date serial for 2028-12-31")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_date_axis_bounds.pptx",
		"title":    "Revenue Combo Date Axis Bounds",
		"subtitle": "Yearly trend",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Combo charts can override the date-axis min and max bounds"},
				"chart": map[string]interface{}{
					"type":        "combo",
					"x_axis_type": "date",
					"x_axis_min":  "2025-12-01",
					"x_axis_max":  "2028-12-31",
					"categories":  []interface{}{"2026-01-01", "2027-01-01", "2028-01-01"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_date_axis_bounds.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:min val="`+minSerial+`"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 min bounds, got %s", chartXML)
	}
	if strings.Count(chartXML, `<c:max val="`+maxSerial+`"/>`) != 2 {
		t.Fatalf("expected chart1.xml to include 2 max bounds, got %s", chartXML)
	}
}

func TestPPTXToolCreateWithSecondaryCategoryAxisTitle(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_secondary_category_axis_title.pptx",
		"title":    "Revenue Secondary Category Axis Title",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Top category axis title helps label the mirrored category scale"},
				"chart": map[string]interface{}{
					"type":          "combo",
					"x2_axis_title": "Quarter (Top)",
					"categories":    []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_secondary_category_axis_title.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:axPos val="t"/>`,
		`<a:t>Quarter (Top)</a:t>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Secondary Category Axis Title", "Top category axis title helps label the mirrored category scale", "Quarter (Top)", "Revenue", "Margin"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_formats.pptx",
		"title":    "Revenue Axis Formats",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit currency and percentage formats"},
				"chart": map[string]interface{}{
					"type":           "combo",
					"y_axis_format":  "$#,##0",
					"y2_axis_format": "0.0%",
					"categories":     []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_formats.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:numFmt formatCode="$#,##0" sourceLinked="0"/>`,
		`<c:numFmt formatCode="0.0%" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Formats", "Value axes use explicit currency and percentage formats", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisBounds(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_bounds.pptx",
		"title":    "Revenue Axis Bounds",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit lower and upper bounds"},
				"chart": map[string]interface{}{
					"type":        "combo",
					"y_axis_min":  0,
					"y_axis_max":  200,
					"y2_axis_min": 0,
					"y2_axis_max": 40,
					"categories":  []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_bounds.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:min val="0"/>`,
		`<c:max val="200"/>`,
		`<c:max val="40"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Bounds", "Value axes use explicit lower and upper bounds", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisUnits(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_units.pptx",
		"title":    "Revenue Axis Units",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit major and minor units"},
				"chart": map[string]interface{}{
					"type":               "combo",
					"y_axis_major_unit":  25,
					"y_axis_minor_unit":  5,
					"y2_axis_major_unit": 10,
					"y2_axis_minor_unit": 2,
					"categories":         []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_units.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:majorUnit val="25"/>`,
		`<c:minorUnit val="5"/>`,
		`<c:majorUnit val="10"/>`,
		`<c:minorUnit val="2"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Units", "Value axes use explicit major and minor units", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisTickGridControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_tick_grid.pptx",
		"title":    "Revenue Axis Tick Grid",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit gridline and tick mark controls"},
				"chart": map[string]interface{}{
					"type":                    "combo",
					"y_axis_major_gridlines":  false,
					"y_axis_major_tick_mark":  "none",
					"y_axis_minor_tick_mark":  "none",
					"y2_axis_major_gridlines": true,
					"y2_axis_major_tick_mark": "cross",
					"y2_axis_minor_tick_mark": "in",
					"categories":              []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_tick_grid.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Tick Grid", "Value axes use explicit gridline and tick mark controls", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisLabelPositions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_label_positions.pptx",
		"title":    "Revenue Axis Label Positions",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit tick label positions"},
				"chart": map[string]interface{}{
					"type":                   "combo",
					"y_axis_label_position":  "high",
					"y2_axis_label_position": "low",
					"categories":             []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_label_positions.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:tickLblPos val="high"/>`,
		`<c:tickLblPos val="low"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Label Positions", "Value axes use explicit tick label positions", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisMinorGridlines(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_minor_gridlines.pptx",
		"title":    "Revenue Axis Minor Gridlines",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit minor gridline visibility"},
				"chart": map[string]interface{}{
					"type":                    "combo",
					"y_axis_major_gridlines":  false,
					"y_axis_minor_gridlines":  true,
					"y2_axis_minor_gridlines": true,
					"categories":              []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_minor_gridlines.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if count := strings.Count(chartXML, `<c:minorGridlines>`); count != 2 {
		t.Fatalf("expected chart1.xml to include 2 minor gridline nodes, got %d in %s", count, chartXML)
	}
	valueAxisBlocks := regexp.MustCompile(`<c:valAx>.*?</c:valAx>`).FindAllString(chartXML, -1)
	if len(valueAxisBlocks) != 2 {
		t.Fatalf("expected chart1.xml to include 2 value-axis blocks, got %d in %s", len(valueAxisBlocks), chartXML)
	}
	if containsSubstring(valueAxisBlocks[0], `<c:majorGridlines`) {
		t.Fatalf("expected primary axis to keep major gridlines hidden in %s", valueAxisBlocks[0])
	}
	if !containsSubstring(valueAxisBlocks[0], `<c:minorGridlines`) {
		t.Fatalf("expected primary axis to include minor gridlines in %s", valueAxisBlocks[0])
	}
	if containsSubstring(valueAxisBlocks[1], `<c:majorGridlines`) {
		t.Fatalf("expected secondary axis to keep major gridlines independently hidden in %s", valueAxisBlocks[1])
	}
	if !containsSubstring(valueAxisBlocks[1], `<c:minorGridlines`) {
		t.Fatalf("expected secondary axis to include minor gridlines in %s", valueAxisBlocks[1])
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Minor Gridlines", "Value axes use explicit minor gridline visibility", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisCrosses(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_crosses.pptx",
		"title":    "Revenue Axis Crosses",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit crossing modes"},
				"chart": map[string]interface{}{
					"type":            "combo",
					"y_axis_crosses":  "max",
					"y2_axis_crosses": "min",
					"categories":      []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_crosses.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:crosses val="max"/>`,
		`<c:crosses val="min"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Crosses", "Value axes use explicit crossing modes", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisCrossBetween(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_cross_between.pptx",
		"title":    "Revenue Axis Cross Between",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use explicit crossBetween modes"},
				"chart": map[string]interface{}{
					"type":                  "combo",
					"y_axis_cross_between":  "mid_cat",
					"y2_axis_cross_between": "mid_cat",
					"categories":            []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_cross_between.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if count := strings.Count(chartXML, `<c:crossBetween val="midCat"/>`); count != 2 {
		t.Fatalf("expected chart1.xml to include 2 midCat crossBetween nodes, got %d in %s", count, chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Cross Between", "Value axes use explicit crossBetween modes", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithValueAxisReverseOrder(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_axis_reverse_order.pptx",
		"title":    "Revenue Axis Reverse Order",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Value axes use reverse-order scaling"},
				"chart": map[string]interface{}{
					"type":                  "combo",
					"y_axis_reverse_order":  true,
					"y2_axis_reverse_order": true,
					"categories":            []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_axis_reverse_order.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if count := strings.Count(chartXML, `<c:orientation val="maxMin"/>`); count != 2 {
		t.Fatalf("expected chart1.xml to include 2 reverse-order value-axis orientations, got %d in %s", count, chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Axis Reverse Order", "Value axes use reverse-order scaling", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisLabelPositions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_label_positions.pptx",
		"title":    "Revenue Category Axis Label Positions",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use explicit tick label positions"},
				"chart": map[string]interface{}{
					"type":                   "combo",
					"x_axis_label_position":  "high",
					"x2_axis_label_position": "low",
					"categories":             []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_label_positions.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:tickLblPos val="high"/>`,
		`<c:tickLblPos val="low"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Label Positions", "Category axes use explicit tick label positions", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisReverseOrder(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_reverse_order.pptx",
		"title":    "Revenue Category Axis Reverse Order",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use reverse-order scaling"},
				"chart": map[string]interface{}{
					"type":                  "combo",
					"x_axis_reverse_order":  true,
					"x2_axis_reverse_order": true,
					"categories":            []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_reverse_order.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if count := strings.Count(chartXML, `<c:orientation val="maxMin"/>`); count != 2 {
		t.Fatalf("expected chart1.xml to include 2 reverse-order category-axis orientations, got %d in %s", count, chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Reverse Order", "Category axes use reverse-order scaling", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisCrosses(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_crosses.pptx",
		"title":    "Revenue Category Axis Crosses",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use explicit crosses overrides"},
				"chart": map[string]interface{}{
					"type":            "combo",
					"x_axis_crosses":  "max",
					"x2_axis_crosses": "min",
					"categories":      []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_crosses.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:crosses val="max"/>`,
		`<c:crosses val="min"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Crosses", "Category axes use explicit crosses overrides", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisTickMarks(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_tick_marks.pptx",
		"title":    "Revenue Category Axis Tick Marks",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use explicit tick-mark overrides"},
				"chart": map[string]interface{}{
					"type":                    "combo",
					"x_axis_major_tick_mark":  "cross",
					"x_axis_minor_tick_mark":  "in",
					"x2_axis_major_tick_mark": "out",
					"x2_axis_minor_tick_mark": "cross",
					"categories":              []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_tick_marks.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:majorTickMark val="cross"/>`,
		`<c:minorTickMark val="in"/>`,
		`<c:majorTickMark val="out"/>`,
		`<c:minorTickMark val="cross"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Tick Marks", "Category axes use explicit tick-mark overrides", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisLabelLayout(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_label_layout.pptx",
		"title":    "Revenue Category Axis Label Layout",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use explicit label alignment and offset"},
				"chart": map[string]interface{}{
					"type":                    "combo",
					"x_axis_label_alignment":  "left",
					"x_axis_label_offset":     250,
					"x2_axis_label_alignment": "right",
					"x2_axis_label_offset":    500,
					"categories":              []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_label_layout.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:lblAlgn val="l"/>`,
		`<c:lblOffset val="250"/>`,
		`<c:lblAlgn val="r"/>`,
		`<c:lblOffset val="500"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Label Layout", "Category axes use explicit label alignment and offset", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisMultiLevelLabelsToggle(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_multilevel_labels.pptx",
		"title":    "Revenue Category Axis Multi-Level Labels",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes can disable multi-level labels"},
				"chart": map[string]interface{}{
					"type":                       "combo",
					"x_axis_multi_level_labels":  false,
					"x2_axis_multi_level_labels": false,
					"categories":                 []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_multilevel_labels.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if count := strings.Count(chartXML, `<c:noMultiLvlLbl val="1"/>`); count != 2 {
		t.Fatalf("expected chart1.xml to include 2 disabled multi-level-label markers, got %d in %s", count, chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Multi-Level Labels", "Category axes can disable multi-level labels", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_visibility.pptx",
		"title":    "Revenue Category Axis Visibility",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes can toggle visibility"},
				"chart": map[string]interface{}{
					"type":            "combo",
					"x_axis_visible":  false,
					"x2_axis_visible": true,
					"categories":      []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_visibility.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:delete val="1"/>`) {
		t.Fatalf("expected primary category axis to be hidden in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:delete val="0"/>`) {
		t.Fatalf("expected secondary category axis to be visible in %s", categoryAxisBlocks[1])
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Visibility", "Category axes can toggle visibility", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisFormats(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_formats.pptx",
		"title":    "Revenue Category Axis Formats",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes use explicit number formats"},
				"chart": map[string]interface{}{
					"type":           "combo",
					"x_axis_format":  "m/d/yyyy",
					"x2_axis_format": "[$-409]mmm-yy",
					"categories":     []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_formats.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:numFmt formatCode="m/d/yyyy" sourceLinked="0"/>`,
		`<c:numFmt formatCode="[$-409]mmm-yy" sourceLinked="0"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Formats", "Category axes use explicit number formats", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCategoryAxisAuto(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_category_axis_auto.pptx",
		"title":    "Revenue Category Axis Auto",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Category axes can disable automatic behavior"},
				"chart": map[string]interface{}{
					"type":         "combo",
					"x_axis_auto":  false,
					"x2_axis_auto": false,
					"categories":   []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "axis": "secondary", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_category_axis_auto.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	categoryAxisBlocks := regexp.MustCompile(`<c:catAx>.*?</c:catAx>`).FindAllString(chartXML, -1)
	if len(categoryAxisBlocks) != 2 {
		t.Fatalf("expected 2 category-axis blocks in %s", chartXML)
	}
	if !containsSubstring(categoryAxisBlocks[0], `<c:auto val="0"/>`) {
		t.Fatalf("expected primary category axis auto to be disabled in %s", categoryAxisBlocks[0])
	}
	if !containsSubstring(categoryAxisBlocks[1], `<c:auto val="0"/>`) {
		t.Fatalf("expected secondary category axis auto to be disabled in %s", categoryAxisBlocks[1])
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Category Axis Auto", "Category axes can disable automatic behavior", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSeriesColors(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_combo_color.pptx",
		"title":    "Revenue And Margin Colors",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Revenue columns with explicit brand colors"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "color": "#D97706", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "color": "2563EB", "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_combo_color.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{`<a:srgbClr val="D97706"/>`, `<a:srgbClr val="2563EB"/>`} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue And Margin Colors", "Revenue columns with explicit brand colors", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSeriesLineStyling(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/trend_styled_line.pptx",
		"title":    "Styled Trend Line",
		"subtitle": "Monthly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Trend",
				"paragraphs": []interface{}{"Line chart with custom width, dash, and marker"},
				"chart": map[string]interface{}{
					"type":       "line",
					"categories": []interface{}{"Jan", "Feb", "Mar"},
					"series": []interface{}{
						map[string]interface{}{
							"name":       "Trend",
							"color":      "#2563EB",
							"line_width": 3,
							"dash":       "dash",
							"marker":     "diamond",
							"values":     []interface{}{10, 12, 15},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "trend_styled_line.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{`<c:marker><c:symbol val="diamond"/></c:marker>`, `<a:ln w="38100">`, `<a:prstDash val="dash"/>`, `<a:srgbClr val="2563EB"/>`} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Styled Trend Line", "Line chart with custom width, dash, and marker", "Trend", "Jan", "Feb", "Mar"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithLineSmoothing(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/trend_smooth_line.pptx",
		"title":    "Smooth Trend Line",
		"subtitle": "Monthly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Trend",
				"paragraphs": []interface{}{"Line chart uses smoothing for a softer trend curve"},
				"chart": map[string]interface{}{
					"type":       "line",
					"smooth":     true,
					"categories": []interface{}{"Jan", "Feb", "Mar"},
					"series": []interface{}{
						map[string]interface{}{"name": "Trend", "values": []interface{}{10, 12, 15}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "trend_smooth_line.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:smooth val="1"/>`) {
		t.Fatalf("expected chart1.xml to include line smoothing, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Smooth Trend Line", "Line chart uses smoothing for a softer trend curve", "Trend", "Jan", "Feb", "Mar"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithPointColors(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/trend_point_colors.pptx",
		"title":    "Point Colored Trend",
		"subtitle": "Monthly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Trend",
				"paragraphs": []interface{}{"Line points use explicit milestone colors"},
				"chart": map[string]interface{}{
					"type":       "line",
					"categories": []interface{}{"Jan", "Feb", "Mar"},
					"series": []interface{}{
						map[string]interface{}{
							"name":         "Trend",
							"point_colors": []interface{}{"#2563EB", "10B981", "F59E0B"},
							"values":       []interface{}{10, 12, 15},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "trend_point_colors.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dPt><c:idx val="0"/><c:spPr><a:solidFill><a:srgbClr val="2563EB"/></a:solidFill>`,
		`<c:dPt><c:idx val="1"/><c:spPr><a:solidFill><a:srgbClr val="10B981"/></a:solidFill>`,
		`<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="F59E0B"/></a:solidFill>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Point Colored Trend", "Line points use explicit milestone colors", "Trend", "Jan", "Feb", "Mar"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceExplosions(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/status_exploded_donut.pptx",
		"title":    "Exploded Status Donut",
		"subtitle": "Account status",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Status Mix",
				"paragraphs": []interface{}{"Pending and blocked slices are pulled out for emphasis"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"categories": []interface{}{"Adoption", "Pending", "Blocked"},
					"series": []interface{}{
						map[string]interface{}{
							"name":             "Status",
							"slice_explosions": []interface{}{0, 22, 34},
							"values":           []interface{}{70, 20, 10},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "status_exploded_donut.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dPt><c:idx val="1"/><c:explosion val="22"/>`,
		`<c:dPt><c:idx val="2"/><c:explosion val="34"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Exploded Status Donut", "Pending and blocked slices are pulled out for emphasis", "Status", "Adoption", "Pending", "Blocked"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithLegendControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_legend_control.pptx",
		"title":    "Revenue Legend Control",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Legend is explicitly pinned to the top"},
				"chart": map[string]interface{}{
					"type":            "bar",
					"show_legend":     true,
					"legend_position": "top",
					"categories":      []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_legend_control.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:legend><c:legendPos val="t"/><c:layout/>`,
		`<c:txPr>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Legend Control", "Legend is explicitly pinned to the top", "Revenue", "Target", "Q1", "Q2"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithCircularPlotControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/status_plot_controls.pptx",
		"title":    "Status Plot Controls",
		"subtitle": "Account status",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Status Mix",
				"paragraphs": []interface{}{"Donut plot controls tune colors, start angle, and hole size"},
				"chart": map[string]interface{}{
					"type":        "donut",
					"vary_colors": false,
					"start_angle": 120,
					"hole_size":   64,
					"categories":  []interface{}{"Adoption", "Pending", "Blocked"},
					"series": []interface{}{
						map[string]interface{}{"name": "Status", "values": []interface{}{70, 20, 10}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "status_plot_controls.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:varyColors val="0"/>`,
		`<c:firstSliceAng val="120"/>`,
		`<c:holeSize val="64"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Status Plot Controls", "Donut plot controls tune colors, start angle, and hole size", "Status", "Adoption", "Pending", "Blocked"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithAxisVaryColors(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_vary_colors.pptx",
		"title":    "Revenue Vary Colors",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Bar chart varies colors across series groups"},
				"chart": map[string]interface{}{
					"type":        "bar",
					"vary_colors": true,
					"categories":  []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_vary_colors.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/><c:varyColors val="1"/>`) {
		t.Fatalf("expected chart1.xml to include axis varyColors override, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Vary Colors", "Bar chart varies colors across series groups", "Revenue", "Target", "Q1", "Q2"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithBarLayoutControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_bar_layout.pptx",
		"title":    "Revenue Bar Layout",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Revenue",
				"paragraphs": []interface{}{"Bar layout uses explicit gap width and overlap"},
				"chart": map[string]interface{}{
					"type":       "bar",
					"gap_width":  60,
					"overlap":    -20,
					"categories": []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Target", "values": []interface{}{140, 145}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_bar_layout.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:gapWidth val="60"/>`,
		`<c:overlap val="-20"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Bar Layout", "Bar layout uses explicit gap width and overlap", "Revenue", "Target", "Q1", "Q2"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithChartDataLabels(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_labels.pptx",
		"title":    "Revenue Labels",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Columns stay unlabeled while the margin line shows values"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{"name": "Margin", "type": "line", "labels": true, "values": []interface{}{28, 31, 34}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_labels.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if strings.Count(chartXML, `<c:dLbls>`) != 1 || !containsSubstring(chartXML, `<c:showVal val="1"/>`) {
		t.Fatalf("expected chart1.xml to include one data-label block for the labeled line series, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Labels", "Columns stay unlabeled while the margin line shows values", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithLabelPositionAndFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_label_format.pptx",
		"title":    "Revenue Label Format",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Margin labels use a custom position and number format"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{
							"name":           "Margin",
							"type":           "line",
							"labels":         true,
							"label_position": "above",
							"label_format":   "0.0",
							"values":         []interface{}{28, 31, 34},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_label_format.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{`<c:dLblPos val="t"/>`, `<c:numFmt formatCode="0.0" sourceLinked="0"/>`} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Label Format", "Margin labels use a custom position and number format", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithLabelContentControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_label_content.pptx",
		"title":    "Revenue Label Content",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Margin labels show the series name instead of the numeric value"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{
							"name":             "Margin",
							"type":             "line",
							"labels":           true,
							"show_value":       false,
							"show_series_name": true,
							"values":           []interface{}{28, 31, 34},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_label_content.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{`<c:showVal val="0"/>`, `<c:showSerName val="1"/>`, `<c:showPercent val="0"/>`} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Label Content", "Margin labels show the series name instead of the numeric value", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithLegendKeyAndBubbleSizeLabelControls(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/revenue_label_detail.pptx",
		"title":    "Revenue Label Detail",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Combo",
				"paragraphs": []interface{}{"Margin labels expose legend key and bubble-size flags"},
				"chart": map[string]interface{}{
					"type":       "combo",
					"categories": []interface{}{"Q1", "Q2", "Q3"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "type": "bar", "values": []interface{}{120, 132, 140}},
						map[string]interface{}{
							"name":             "Margin",
							"type":             "line",
							"labels":           true,
							"show_legend_key":  true,
							"show_bubble_size": true,
							"values":           []interface{}{28, 31, 34},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "revenue_label_detail.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{`<c:showLegendKey val="1"/>`, `<c:showBubbleSize val="1"/>`} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Revenue Label Detail", "Margin labels expose legend key and bubble-size flags", "Revenue", "Margin", "Q1", "Q2", "Q3"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithChartLabelSeparator(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/pie_label_separator.pptx",
		"title":    "Pie Label Separator",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Pie labels use a custom separator"},
				"chart": map[string]interface{}{
					"type":            "pie",
					"labels":          true,
					"label_separator": " / ",
					"categories":      []interface{}{"Won", "Lost"},
					"series": []interface{}{
						map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 45}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "pie_label_separator.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:separator>/</c:separator>`) {
		t.Fatalf("expected chart1.xml to include label separator, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Pie Label Separator", "Pie labels use a custom separator", "Pipeline", "Won", "Lost"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithDonutLeaderLines(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_leader_lines.pptx",
		"title":    "Donut Leader Lines",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels request leader lines"},
				"chart": map[string]interface{}{
					"type":              "donut",
					"labels":            true,
					"show_leader_lines": true,
					"categories":        []interface{}{"Won", "Lost"},
					"series": []interface{}{
						map[string]interface{}{"name": "Pipeline", "values": []interface{}{55, 45}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_leader_lines.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	if !containsSubstring(chartXML, `<c:showLeaderLines val="1"/>`) {
		t.Fatalf("expected chart1.xml to include leader lines, got %s", chartXML)
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Leader Lines", "Donut labels request leader lines", "Pipeline", "Won", "Lost"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceLabelVisibilityOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_label_visibility.pptx",
		"title":    "Donut Slice Label Visibility",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels hide the middle slice label"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":              "Pipeline",
							"slice_show_labels": []interface{}{true, false, true},
							"values":            []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_label_visibility.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:delete val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:delete val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:delete val="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Label Visibility", "Donut labels hide the middle slice label", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceLabelPositionOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_label_positions.pptx",
		"title":    "Donut Slice Label Positions",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels pin each slice to a specific label position"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":                  "Pipeline",
							"slice_label_positions": []interface{}{"outside_end", "center", "best_fit"},
							"values":                []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_label_positions.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:dLblPos val="outEnd"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:dLblPos val="ctr"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:dLblPos val="bestFit"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Label Positions", "Donut labels pin each slice to a specific label position", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceLabelFormatOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_label_formats.pptx",
		"title":    "Donut Slice Label Formats",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels give each slice a specific number format"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":                "Pipeline",
							"slice_label_formats": []interface{}{"0.0%", "$#,##0", "0.0"},
							"values":              []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_label_formats.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:numFmt formatCode="0.0%" sourceLinked="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:numFmt formatCode="$#,##0" sourceLinked="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:numFmt formatCode="0.0" sourceLinked="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Label Formats", "Donut labels give each slice a specific number format", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceLabelSeparatorOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_label_separators.pptx",
		"title":    "Donut Slice Label Separators",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels give each slice a specific separator"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":                   "Pipeline",
							"slice_label_separators": []interface{}{" / ", " | ", " - "},
							"values":                 []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_label_separators.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:separator>/</c:separator></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:separator>|</c:separator></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:separator>-</c:separator></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Label Separators", "Donut labels give each slice a specific separator", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSliceLabelContentOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_label_content.pptx",
		"title":    "Donut Slice Label Content",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels give each slice a specific value/category mix"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":                  "Pipeline",
							"slice_show_values":     []interface{}{true, false, true},
							"slice_show_categories": []interface{}{false, true, false},
							"values":                []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_label_content.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:showVal val="1"/><c:showCatName val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:showVal val="0"/><c:showCatName val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:showVal val="1"/><c:showCatName val="0"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Label Content", "Donut labels give each slice a specific value/category mix", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPPTXToolCreateWithSlicePercentAndSeriesNameOverrides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/donut_slice_percent_series_name.pptx",
		"title":    "Donut Slice Percent And Series Name",
		"subtitle": "Quarterly view",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Pipeline",
				"paragraphs": []interface{}{"Donut labels give each slice a specific percent/series-name mix"},
				"chart": map[string]interface{}{
					"type":       "donut",
					"labels":     true,
					"categories": []interface{}{"Won", "Lost", "Open"},
					"series": []interface{}{
						map[string]interface{}{
							"name":                    "Pipeline",
							"slice_show_percents":     []interface{}{true, false, true},
							"slice_show_series_names": []interface{}{false, true, false},
							"values":                  []interface{}{55, 25, 20},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pptx_ooxml" {
		t.Fatalf("engine = %v, want native_pptx_ooxml", got)
	}
	if got := asNativeToolInt(t, payload["slide_count"]); got != 3 {
		t.Fatalf("slide_count = %d, want 3", got)
	}

	path := filepath.Join(tmpDir, "decks", "donut_slice_percent_series_name.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:dLbl><c:idx val="0"/><c:showSerName val="0"/><c:showPercent val="1"/></c:dLbl>`,
		`<c:dLbl><c:idx val="1"/><c:showSerName val="1"/><c:showPercent val="0"/></c:dLbl>`,
		`<c:dLbl><c:idx val="2"/><c:showSerName val="0"/><c:showPercent val="1"/></c:dLbl>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	for _, needle := range []string{"Donut Slice Percent And Series Name", "Donut labels give each slice a specific percent/series-name mix", "Pipeline", "Won", "Lost", "Open"} {
		if !containsSubstring(doc.Text, needle) {
			t.Fatalf("expected deck text to include %q, got %q", needle, doc.Text)
		}
	}
}

func TestPDFToolCreate(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "reports/launch_brief.pdf",
		"title":    "Launch Brief",
		"subtitle": "Q2 roll-out",
		"summary":  "Native-first document tools should default to fast local rendering.",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "Overview",
				"paragraphs": []interface{}{"Ship docx/xlsx/pptx now.", "Add pdf creation as the first writer increment."},
			},
			map[string]interface{}{
				"heading": "Risks",
				"bullets": []interface{}{"Layout polish can improve later.", "Form filling remains a follow-up slice."},
			},
		},
		"notes": []interface{}{"Prepared by Blue."},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pdf_ir" {
		t.Fatalf("engine = %v, want native_pdf_ir", got)
	}
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected validation payload, got %#v", payload["validation"])
	}
	if got := asNativeToolInt(t, validation["page_count"]); got < 1 {
		t.Fatalf("page_count = %d, want >= 1", got)
	}

	path := filepath.Join(tmpDir, "reports", "launch_brief.pdf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || string(data[:5]) != "%PDF-" {
		t.Fatalf("unexpected PDF header: %q", string(data))
	}
	assertPDFExtractedTextContainsAll(t, path, []string{
		"Launch Brief",
		"Q2 roll-out",
		"Overview",
		"Ship docx/xlsx/pptx now.",
		"Form filling remains a follow-up slice.",
		"Prepared by Blue.",
	})
}

func TestPDFToolCreate_UsesMarkdownAlias(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "reports/markdown_alias.pdf",
		"markdown": `# Product Update

## Highlights

- Direct markdown input should render without a temp markdown file.
- Invalid outputs should fail loudly instead of reporting success.

Prepared for the native PDF path.`,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pdf_ir" {
		t.Fatalf("engine = %v, want native_pdf_ir", got)
	}
	if got := payload["path"]; got != "reports/markdown_alias.pdf" {
		t.Fatalf("path = %v, want reports/markdown_alias.pdf", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected validation payload, got %#v", payload["validation"])
	}
	if got := asNativeToolInt(t, validation["page_count"]); got < 1 {
		t.Fatalf("page_count = %d, want >= 1", got)
	}

	path := filepath.Join(tmpDir, "reports", "markdown_alias.pdf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || string(data[:5]) != "%PDF-" {
		t.Fatalf("unexpected PDF header: %q", string(data))
	}
	assertPDFExtractedTextContainsAll(t, path, []string{
		"Product Update",
		"Highlights",
		"Direct markdown input should render without a temp markdown file.",
		"Invalid outputs should fail loudly instead of reporting success.",
		"Prepared for the native PDF path.",
	})
}

func TestPDFToolCreate_UsesOutputPathAliasWhenPathEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "create",
		"path":        "",
		"output_path": "reports/output_path_alias.pdf",
		"summary":     "This should render to PDF.",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["path"]; got != "reports/output_path_alias.pdf" {
		t.Fatalf("path = %v, want reports/output_path_alias.pdf", got)
	}

	path := filepath.Join(tmpDir, "reports", "output_path_alias.pdf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || string(data[:5]) != "%PDF-" {
		t.Fatalf("unexpected PDF header: %q", string(data))
	}
}

func TestPDFToolCreate_FromSingleMarkdownInputDerivesOutputPath(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	if err := os.MkdirAll(filepath.Join(tmpDir, "reports"), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	seed := "# Seed PDF Title\n\nThis is a seeded PDF.\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "reports", "seed.md"), []byte(seed), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	args := map[string]interface{}{
		"action": "create",
		"path":   "reports/seed.md",
		"theme":  "editorial",
	}
	result, err := tool.Execute(context.Background(), args)
	if err != nil {
		t.Fatalf("create failed: %v (args=%#v)", err, args)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["path"]; got != "reports/seed.pdf" {
		t.Fatalf("path = %v, want reports/seed.pdf", got)
	}

	path := filepath.Join(tmpDir, "reports", "seed.pdf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || string(data[:5]) != "%PDF-" {
		t.Fatalf("unexpected PDF header: %q", string(data))
	}
}

func TestExecuteCreateLikeDocumentWrite_RejectsInvalidPDFBytes(t *testing.T) {
	tmpDir := t.TempDir()
	scope := newFSToolScope([]string{tmpDir})

	_, _, err := executeCreateLikeDocumentWrite(
		context.Background(),
		"pdf",
		scope,
		"reports/invalid.pdf",
		true,
		[]byte("not a pdf"),
	)
	if err == nil {
		t.Fatal("expected invalid pdf bytes to fail validation")
	}

	if _, statErr := os.Stat(filepath.Join(tmpDir, "reports", "invalid.pdf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected invalid pdf output to be removed, stat err=%v", statErr)
	}
}

func TestPDFToolReformat(t *testing.T) {
	tmpDir := t.TempDir()
	sourcePath := filepath.Join(tmpDir, "source.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{extract: pdfextract.ExtractResult{
		Document: pdfextract.DocumentInfo{
			FileName: "source.pdf",
			Engine:   "pdfium/webassembly",
		},
		Markdown: "# Executive Summary\n\nBlue can now reformat PDFs.\n\n## Risks\n- Keep layout simple.\n",
		Text:     "Executive Summary\nBlue can now reformat PDFs.\nRisks\nKeep layout simple.",
	}}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{tmpDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "reformat",
		"path":        "source.pdf",
		"output_path": "exports/source_reformatted.pdf",
		"title":       "Reformatted Brief",
	})
	if err != nil {
		t.Fatalf("reformat failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "native_pdf_ir" {
		t.Fatalf("engine = %v, want native_pdf_ir", got)
	}
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	engineChain, ok := payload["engine_chain"].([]interface{})
	if !ok {
		t.Fatalf("engine_chain type = %T, want array", payload["engine_chain"])
	}
	if len(engineChain) != 2 || engineChain[0] != "pdfium/webassembly" || engineChain[1] != "native_pdf_ir" {
		t.Fatalf("engine_chain = %#v, want [pdfium/webassembly native_pdf_ir]", engineChain)
	}
	if got := payload["path"]; got != "exports/source_reformatted.pdf" {
		t.Fatalf("path = %v, want exports/source_reformatted.pdf", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected validation payload, got %#v", payload["validation"])
	}
	if got := asNativeToolInt(t, validation["page_count"]); got < 1 {
		t.Fatalf("page_count = %d, want >= 1", got)
	}
	if svc.lastExtractReq.Path != sourcePath {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, sourcePath)
	}
	if !svc.lastExtractReq.IncludeMarkdown {
		t.Fatal("expected reformat to request markdown extraction")
	}

	outputPath := filepath.Join(tmpDir, "exports", "source_reformatted.pdf")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(data) == 0 || string(data[:5]) != "%PDF-" {
		t.Fatalf("unexpected PDF header: %q", string(data))
	}
	assertPDFExtractedTextContainsAll(t, outputPath, []string{
		"Reformatted Brief",
		"Executive Summary",
		"Blue can now reformat PDFs.",
		"Keep layout simple.",
	})
}

func assertPDFExtractedTextContainsAll(t *testing.T, path string, needles []string) {
	t.Helper()

	svc := pdfextract.NewService(nil, nil, pdfextract.ServiceConfig{
		RuntimeDir:   t.TempDir(),
		AutoDownload: false,
	})
	t.Cleanup(func() { _ = svc.Close() })

	result, err := svc.Extract(context.Background(), pdfextract.ExtractRequest{
		Path:         path,
		IncludePages: true,
	})
	if err != nil {
		t.Fatalf("Extract(%q) error = %v", path, err)
	}
	text := result.Text
	for _, needle := range needles {
		if !containsSubstring(text, needle) {
			t.Fatalf("extracted PDF text = %q, want fragment %q", text, needle)
		}
	}
}

func TestNativeCreateTools_IncludeThemePreviewMetadata(t *testing.T) {
	forestTheme, err := officeThemeByName("forest")
	if err != nil {
		t.Fatalf("officeThemeByName(forest) error = %v", err)
	}

	testCases := []struct {
		name           string
		run            func(tmpDir string) (interface{}, error)
		wantTheme      string
		wantMood       string
		wantDisplay    string
		wantBody       string
		wantHTMLNeedle string
	}{
		{
			name: "docx",
			run: func(tmpDir string) (interface{}, error) {
				tool := NewDOCXTool([]string{tmpDir}, nil, nil)
				return tool.Execute(context.Background(), map[string]interface{}{
					"action":  "create",
					"path":    "reports/board_update.docx",
					"theme":   "midnight",
					"title":   "Board Update",
					"summary": "A concise executive summary.",
				})
			},
			wantTheme:      "midnight",
			wantMood:       "trustworthy",
			wantDisplay:    "Helvetica Neue",
			wantBody:       "Helvetica",
			wantHTMLNeedle: "#242C38",
		},
		{
			name: "xlsx",
			run: func(tmpDir string) (interface{}, error) {
				tool := NewXLSXTool([]string{tmpDir}, nil, nil)
				return tool.Execute(context.Background(), map[string]interface{}{
					"action":     "create",
					"path":       "reports/growth.xlsx",
					"style_hint": "startup launch metrics",
					"title":      "Growth Dashboard",
					"sheets": []interface{}{
						map[string]interface{}{
							"name": "Metrics",
							"columns": []interface{}{
								map[string]interface{}{"header": "Metric"},
								map[string]interface{}{"header": "Value"},
							},
							"rows": []interface{}{
								[]interface{}{"Activation", "42%"},
							},
						},
					},
				})
			},
			wantTheme:      "coral",
			wantMood:       "energetic",
			wantDisplay:    "Poppins",
			wantBody:       "Nunito",
			wantHTMLNeedle: "#FF6B6B",
		},
		{
			name: "pdf",
			run: func(tmpDir string) (interface{}, error) {
				tool := NewPDFTool(nil)
				tool.scope = newFSToolScope([]string{tmpDir})
				return tool.Execute(context.Background(), map[string]interface{}{
					"action":  "create",
					"path":    "reports/sustainability.pdf",
					"theme":   "forest",
					"title":   "Sustainability Brief",
					"summary": "A grounded update for stakeholders.",
				})
			},
			wantTheme:      "forest",
			wantMood:       "natural",
			wantDisplay:    "Times New Roman",
			wantBody:       "Open Sans",
			wantHTMLNeedle: forestTheme.Primary,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			result, err := tc.run(tmpDir)
			if err != nil {
				t.Fatalf("create failed: %v", err)
			}

			payload := parseNativeDocumentPayload(t, result)
			if got := payload["theme"]; got != tc.wantTheme {
				t.Fatalf("theme = %v, want %s", got, tc.wantTheme)
			}
			preview, ok := payload["theme_preview"].(map[string]interface{})
			if !ok {
				t.Fatalf("theme_preview = %#v, want object", payload["theme_preview"])
			}
			if got := preview["mood"]; got != tc.wantMood {
				t.Fatalf("theme_preview.mood = %v, want %s", got, tc.wantMood)
			}
			fonts, ok := preview["fonts"].(map[string]interface{})
			if !ok {
				t.Fatalf("theme_preview.fonts = %#v, want object", preview["fonts"])
			}
			if fonts["display"] != tc.wantDisplay || fonts["body"] != tc.wantBody {
				t.Fatalf("theme_preview.fonts = %#v, want %s/%s", fonts, tc.wantDisplay, tc.wantBody)
			}
			html := asNativeToolString(t, preview["html"])
			if !containsSubstring(html, tc.wantHTMLNeedle) {
				t.Fatalf("theme_preview.html = %q, want %q", html, tc.wantHTMLNeedle)
			}
		})
	}
}

func TestPPTXToolCreate_UsesExplicitThemeMetadataAndThemeXML(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/board_update.pptx",
		"theme":    "midnight",
		"title":    "Board Update",
		"subtitle": "Q2 review",
		"sections": []interface{}{map[string]interface{}{"heading": "Overview", "paragraphs": []interface{}{"Margins improved"}}},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}
	preview, ok := payload["theme_preview"].(map[string]interface{})
	if !ok {
		t.Fatalf("theme_preview = %#v, want object", payload["theme_preview"])
	}
	if got := preview["mood"]; got != "trustworthy" {
		t.Fatalf("theme_preview.mood = %v, want trustworthy", got)
	}

	path := filepath.Join(tmpDir, "decks", "board_update.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	themeXML := officeZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Midnight Theme"`,
		`val="242C38"`,
		`val="4D7CFE"`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
	} {
		if !containsSubstring(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}

	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`name="Theme Background"`,
		`name="Theme Canvas"`,
		`name="Theme Header Rule"`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="F5F3EE"`,
		`val="D8DCE5"`,
		`val="4D7CFE"`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}
	for _, unwanted := range []string{
		`name="Theme Header Panel"`,
		`name="Theme Accent Pill"`,
		`name="Theme Footer Rule"`,
	} {
		if containsSubstring(slideXML, unwanted) {
			t.Fatalf("expected slide3.xml to avoid layered theme patch %q, got %s", unwanted, slideXML)
		}
	}
}

func TestPPTXToolCreate_UsesEditorialThemeMetadataAndThemeXML(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "decks/editorial_signal.pptx",
		"theme":    "editorial",
		"title":    "Signal Systems",
		"subtitle": "High-Contrast Story Deck",
		"sections": []interface{}{map[string]interface{}{"heading": "Core Frames", "paragraphs": []interface{}{"Bold typographic pacing", "High-contrast editorial rhythm"}}},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "editorial" {
		t.Fatalf("theme = %v, want editorial", got)
	}
	preview, ok := payload["theme_preview"].(map[string]interface{})
	if !ok {
		t.Fatalf("theme_preview = %#v, want object", payload["theme_preview"])
	}
	if got := preview["mood"]; got != "bold" {
		t.Fatalf("theme_preview.mood = %v, want bold", got)
	}

	path := filepath.Join(tmpDir, "decks", "editorial_signal.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	themeXML := officeZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Editorial Theme"`,
		`val="0A0D14"`,
		`val="FFB700"`,
		`val="00E5FF"`,
		`typeface="Arial Black"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
	} {
		if !containsSubstring(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}

	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`name="Theme Background"`,
		`name="Theme Canvas"`,
		`name="Theme Header Rule"`,
		`typeface="Arial Black"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="11131A"`,
		`val="727A8F"`,
		`val="FFB700"`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}
}

func TestPDFToolCreate_UsesMagazineThemeAliasMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPDFTool(nil)
	tool.scope = newFSToolScope([]string{tmpDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":   "create",
		"path":     "reports/magazine_alias.pdf",
		"theme":    "magazine",
		"title":    "Magazine Alias",
		"subtitle": "Explicit alias should map to editorial",
		"content":  "# Cover\n\nA magazine-style report should keep the editorial theme.",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "editorial" {
		t.Fatalf("theme = %v, want editorial", got)
	}
	preview, ok := payload["theme_preview"].(map[string]interface{})
	if !ok {
		t.Fatalf("theme_preview = %#v, want object", payload["theme_preview"])
	}
	if got := preview["mood"]; got != "bold" {
		t.Fatalf("theme_preview.mood = %v, want bold", got)
	}
}

func TestPPTXToolCreate_DefaultsGenericDecksToMidnightThemeWithSafeFonts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/qwen3_intro_default_theme.pptx",
		"title":  "Qwen3 最新优化介绍",
		"sections": []interface{}{
			map[string]interface{}{
				"heading":    "核心更新",
				"paragraphs": []interface{}{"混合思考模式升级", "性能与多语言能力继续提升"},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}

	path := filepath.Join(tmpDir, "decks", "qwen3_intro_default_theme.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	themeXML := officeZipEntryText(t, data, "ppt/theme/theme1.xml")
	for _, needle := range []string{
		`name="Midnight Theme"`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`script="Hans" typeface="Hiragino Sans GB"`,
	} {
		if !containsSubstring(themeXML, needle) {
			t.Fatalf("expected theme1.xml to include %q, got %s", needle, themeXML)
		}
	}

	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`name="Theme Background"`,
		`name="Theme Canvas"`,
		`name="Theme Header Rule"`,
		`<a:latin typeface="Helvetica Neue"/>`,
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:cs typeface="Helvetica"/>`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}
	for _, unwanted := range []string{
		`name="Theme Header Panel"`,
		`name="Theme Accent Pill"`,
		`name="Theme Footer Rule"`,
	} {
		if containsSubstring(slideXML, unwanted) {
			t.Fatalf("expected slide3.xml to avoid layered theme patch %q, got %s", unwanted, slideXML)
		}
	}
}

func TestPPTXToolCreate_UsesThemePaletteForCharts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/midnight_chart_theme.pptx",
		"theme":  "midnight",
		"title":  "Board Metrics",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "Revenue Mix",
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Margin", "values": []interface{}{28, 31}},
						map[string]interface{}{"name": "Pipeline", "values": []interface{}{80, 96}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}

	path := filepath.Join(tmpDir, "decks", "midnight_chart_theme.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:v>Revenue</c:v>`,
		`<c:v>Margin</c:v>`,
		`<c:v>Pipeline</c:v>`,
		`<a:srgbClr val="242C38"/>`,
		`<a:srgbClr val="4D7CFE"/>`,
		`<a:srgbClr val="8A93A2"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
}

func TestPPTXToolCreate_UsesThemeTypographyForChartText(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/midnight_chart_type.pptx",
		"theme":  "midnight",
		"title":  "Board Metrics",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "Revenue Mix",
				"chart": map[string]interface{}{
					"type":         "bar",
					"title":        "Board Metrics",
					"x_axis_title": "Quarter",
					"y_axis_title": "Revenue ($M)",
					"categories":   []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Margin", "values": []interface{}{28, 31}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}

	path := filepath.Join(tmpDir, "decks", "midnight_chart_type.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<a:latin typeface="Helvetica"/>`,
		`<a:ea typeface="Hiragino Sans GB"/>`,
		`<a:srgbClr val="F5F3EE"/>`,
		`<a:srgbClr val="D8DCE5"/>`,
		`<c:legend><c:legendPos val="r"/><c:layout/><c:txPr>`,
		`<c:autoTitleDeleted val="1"/>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
}

func TestPPTXToolCreate_UsesThemeAxisChromeForCharts(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/midnight_chart_axis_theme.pptx",
		"theme":  "midnight",
		"title":  "Board Metrics",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "Revenue Mix",
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
						map[string]interface{}{"name": "Margin", "values": []interface{}{28, 31}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}

	path := filepath.Join(tmpDir, "decks", "midnight_chart_axis_theme.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	chartXML := officeZipEntryText(t, data, "ppt/charts/chart1.xml")
	for _, needle := range []string{
		`<c:majorGridlines><c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="11151B"/></a:solidFill></a:ln></c:spPr></c:majorGridlines>`,
		`<c:txPr><a:bodyPr/><a:lstStyle/><a:p><a:pPr><a:defRPr lang="en-US" sz="1000"><a:latin typeface="Helvetica"/><a:ea typeface="Hiragino Sans GB"/><a:cs typeface="Helvetica"/><a:solidFill><a:srgbClr val="D8DCE5"/></a:solidFill></a:defRPr></a:pPr>`,
		`<c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="2B313C"/></a:solidFill></a:ln></c:spPr>`,
	} {
		if !containsSubstring(chartXML, needle) {
			t.Fatalf("expected chart1.xml to include %q, got %s", needle, chartXML)
		}
	}
}

func TestPPTXToolCreate_UsesThemeCalloutRailForChartSlides(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewPPTXTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "create",
		"path":   "decks/midnight_chart_callout_theme.pptx",
		"theme":  "midnight",
		"title":  "Board Metrics",
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "Revenue Mix",
				"bullets": []interface{}{
					"Revenue: $12.4M",
					"Watch conversion quality",
				},
				"chart": map[string]interface{}{
					"type":       "bar",
					"categories": []interface{}{"Q1", "Q2"},
					"series": []interface{}{
						map[string]interface{}{"name": "Revenue", "values": []interface{}{120, 132}},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	payload := parseNativeDocumentPayload(t, result)
	if got := payload["theme"]; got != "midnight" {
		t.Fatalf("theme = %v, want midnight", got)
	}

	path := filepath.Join(tmpDir, "decks", "midnight_chart_callout_theme.pptx")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	slideXML := officeZipEntryText(t, data, "ppt/slides/slide3.xml")
	for _, needle := range []string{
		`name="Chart Callout Rail"`,
		`name="Chart Callout 1"`,
		`name="Chart Callout 2"`,
		`<a:t>Revenue</a:t>`,
		`<a:t>$12.4M</a:t>`,
		`<a:t>Watch conversion quality</a:t>`,
		`typeface="Helvetica Neue"`,
		`typeface="Helvetica"`,
		`typeface="Hiragino Sans GB"`,
		`val="4D7CFE"`,
		`cx="8031480"`,
	} {
		if !containsSubstring(slideXML, needle) {
			t.Fatalf("expected slide3.xml to include %q, got %s", needle, slideXML)
		}
	}
	if containsSubstring(slideXML, `name="Chart Callout Accent"`) {
		t.Fatalf("expected slide3.xml to avoid the extra chart accent patch, got %s", slideXML)
	}
	if count := strings.Count(slideXML, "Watch conversion quality"); count != 1 {
		t.Fatalf("expected callout text to appear once in slide3.xml, got %d in %s", count, slideXML)
	}
	if count := strings.Count(slideXML, "$12.4M"); count != 1 {
		t.Fatalf("expected stat value to appear once in slide3.xml, got %d in %s", count, slideXML)
	}
}

func TestStructuredWorkspaceArtifactWorkflowToolNames_UsesNativeDocumentTools(t *testing.T) {
	docxNames := StructuredWorkspaceArtifactWorkflowToolNames("Read findings.md and save the polished report to ui_review.docx.")
	if !hasStringValue(docxNames, "docx") {
		t.Fatalf("expected docx workflow tool, got %v", docxNames)
	}
	if hasStringValue(docxNames, "office") {
		t.Fatalf("did not expect legacy office workflow tool, got %v", docxNames)
	}

	xlsxNames := StructuredWorkspaceArtifactWorkflowToolNames("Read findings.md and save the scorecard to ui_review.xlsx.")
	if !hasStringValue(xlsxNames, "xlsx") {
		t.Fatalf("expected xlsx workflow tool, got %v", xlsxNames)
	}

	pptxNames := StructuredWorkspaceArtifactWorkflowToolNames("Read findings.md and save the deck to launch_plan.pptx.")
	if !hasStringValue(pptxNames, "pptx") {
		t.Fatalf("expected pptx workflow tool, got %v", pptxNames)
	}
}

func parseNativeDocumentPayload(t *testing.T, result interface{}) map[string]interface{} {
	t.Helper()
	text, ok := result.(string)
	if !ok {
		t.Fatalf("result type = %T, want string", result)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, content=%q", err, text)
	}
	return payload
}

func asNativeToolString(t *testing.T, value interface{}) string {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("value type = %T, want string", value)
	}
	return text
}

func asNativeToolInt(t *testing.T, value interface{}) int {
	t.Helper()
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		t.Fatalf("value type = %T, want numeric", value)
		return 0
	}
}

func asNativeToolStringSlice(t *testing.T, value interface{}) []string {
	t.Helper()
	items, ok := value.([]interface{})
	if !ok {
		t.Fatalf("value type = %T, want []interface{}", value)
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			t.Fatalf("slice item type = %T, want string", item)
		}
		out = append(out, text)
	}
	return out
}

func hasStringValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
