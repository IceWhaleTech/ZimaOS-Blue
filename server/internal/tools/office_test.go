package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	convertpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/convert"
)

func TestOfficeToolExecute_XLSXCreatesStyledWorkbook(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewOfficeTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":     "reports/ui_review.xlsx",
		"theme":    "ui_review",
		"title":    "UI Review Comparison",
		"subtitle": "PinchBench styled workbook",
		"summary": map[string]interface{}{
			"stats": []interface{}{
				map[string]interface{}{"label": "Pass Rate", "value": "86%", "tone": "success"},
				map[string]interface{}{"label": "Critical Issues", "value": "4", "tone": "danger"},
			},
			"notes": []interface{}{"Use the Overview sheet as the polished entry point."},
		},
		"sheets": []interface{}{
			map[string]interface{}{
				"name": "Findings",
				"columns": []interface{}{
					map[string]interface{}{"header": "Area", "kind": "text", "width": 18.0},
					map[string]interface{}{"header": "Score", "kind": "number", "width": 12.0},
					map[string]interface{}{"header": "Verdict", "kind": "tone", "width": 14.0},
				},
				"rows": []interface{}{
					[]interface{}{"Visual clarity", 92.0, "Good"},
					[]interface{}{"Navigation depth", 74.0, "Warning"},
					[]interface{}{"Form recovery", 58.0, "Critical"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	payload := parseOfficeToolPayload(t, result)
	if got := payload["format"]; got != "xlsx" {
		t.Fatalf("format = %v, want xlsx", got)
	}
	if got := payload["theme"]; got != "ui_review" {
		t.Fatalf("theme = %v, want ui_review", got)
	}

	path := filepath.Join(tmpDir, "reports", "ui_review.xlsx")
	assertZipEntryExists(t, path, "xl/styles.xml")
	assertZipEntryExists(t, path, "xl/worksheets/sheet1.xml")
	assertZipEntryExists(t, path, "xl/workbook.xml")

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if doc.ExtractedVia != "local_spreadsheet" {
		t.Fatalf("ExtractedVia = %q, want local_spreadsheet", doc.ExtractedVia)
	}
	if doc.Text == "" || !containsSubstring(doc.Text, "UI Review Comparison") || !containsSubstring(doc.Text, "Visual clarity") {
		t.Fatalf("unexpected workbook text: %q", doc.Text)
	}
}

func TestOfficeToolExecute_DOCXCreatesStyledDocument(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewOfficeTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":       "reports/ui_review.docx",
		"style_hint": "UI audit report",
		"content": `# UI Review Report
PinchBench styled narrative

Summary: The report should feel like a high-quality product review with clear hierarchy and readable callouts.

## Key Findings
- Contrast hierarchy is strong
- Dense tables need more spacing

## Metrics
| Area | Score | Verdict |
| --- | --- | --- |
| Visual clarity | 92 | Good |
| Layout rhythm | 84 | Warning |
`,
		"notes": []interface{}{
			"Prefer the executive summary first, then detailed findings.",
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	payload := parseOfficeToolPayload(t, result)
	if got := payload["format"]; got != "docx" {
		t.Fatalf("format = %v, want docx", got)
	}
	if got := payload["theme"]; got != "ui_review" {
		t.Fatalf("theme = %v, want ui_review", got)
	}

	path := filepath.Join(tmpDir, "reports", "ui_review.docx")
	assertZipEntryExists(t, path, "word/styles.xml")
	assertZipEntryExists(t, path, "word/document.xml")
	assertZipEntryExists(t, path, "word/fontTable.xml")

	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if doc.ExtractedVia != "local_docx" {
		t.Fatalf("ExtractedVia = %q, want local_docx", doc.ExtractedVia)
	}
	if doc.Text == "" || !containsSubstring(doc.Text, "UI Review Report") || !containsSubstring(doc.Text, "Contrast hierarchy is strong") {
		t.Fatalf("unexpected document text: %q", doc.Text)
	}
}

func TestOfficeToolExecute_DOCXCleansArtifactsAndDeduplicatesRepeatedParagraphs(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewOfficeTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    "reports/cleaned.docx",
		"title":   "【【质检】】报告",
		"summary": "这是【【重点】】摘要。\n\n这是【【重点】】摘要。",
		"paragraphs": []interface{}{
			"重复 段落。",
			"重复段落。",
			"另一段。【【】】",
		},
		"sections": []interface{}{
			map[string]interface{}{
				"heading": "结论【【列表】】",
				"bullets": []interface{}{
					"存在【【】】异常符号",
					"存在异常符号",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	path := filepath.Join(tmpDir, "reports", "cleaned.docx")
	reader := convertpkg.NewDocumentReader()
	doc, err := reader.ReadDocument(context.Background(), path)
	if err != nil {
		t.Fatalf("ReadDocument() error = %v", err)
	}
	if strings.Contains(doc.Text, "【【") || strings.Contains(doc.Text, "】】") {
		t.Fatalf("unexpected bracket artifacts in text: %q", doc.Text)
	}
	if strings.Count(doc.Text, "重复段落。") != 1 {
		t.Fatalf("expected deduplicated paragraph once, got %d: %q", strings.Count(doc.Text, "重复段落。"), doc.Text)
	}
	if strings.Count(doc.Text, "存在异常符号") != 1 {
		t.Fatalf("expected deduplicated bullet once, got %d: %q", strings.Count(doc.Text, "存在异常符号"), doc.Text)
	}
	if !containsSubstring(doc.Text, "【重点】摘要。") {
		t.Fatalf("expected cleaned summary in text: %q", doc.Text)
	}
}

func parseOfficeToolPayload(t *testing.T, result interface{}) map[string]interface{} {
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

func assertZipEntryExists(t *testing.T, path, entryName string) {
	t.Helper()
	reader, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("zip.OpenReader(%q) error = %v", path, err)
	}
	defer reader.Close()
	for _, file := range reader.File {
		if file.Name == entryName {
			return
		}
	}
	t.Fatalf("zip entry %q missing from %s", entryName, path)
}

func containsSubstring(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}

func TestRegisterBuiltinTools_DoesNotRegisterLegacyOfficeTool(t *testing.T) {
	registry := NewRegistry()
	RegisterBuiltinTools(registry)
	if registry.Get("office") != nil {
		t.Fatal("did not expect legacy office tool to be registered")
	}
}

func TestOfficeToolExecute_CreatesParentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewOfficeTool([]string{tmpDir}, nil, nil)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    "nested/reports/summary.docx",
		"summary": "A minimal docx payload.",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "nested", "reports", "summary.docx")); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestOfficeToolExecute_IncludesThemePreviewMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	tool := NewOfficeTool([]string{tmpDir}, nil, nil)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":    "reports/board_update.docx",
		"theme":   "midnight",
		"title":   "Board Update",
		"summary": "A concise executive summary.",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	payload := parseOfficeToolPayload(t, result)
	preview, ok := payload["theme_preview"].(map[string]interface{})
	if !ok {
		t.Fatalf("theme_preview = %#v, want object", payload["theme_preview"])
	}
	if got := preview["name"]; got != "midnight" {
		t.Fatalf("theme_preview.name = %v, want midnight", got)
	}
	if got := preview["mood"]; got != "trustworthy" {
		t.Fatalf("theme_preview.mood = %v, want trustworthy", got)
	}
	if got := preview["personality"]; got != "professional yet distinctive" {
		t.Fatalf("theme_preview.personality = %v", got)
	}
	fonts, ok := preview["fonts"].(map[string]interface{})
	if !ok {
		t.Fatalf("theme_preview.fonts = %#v, want object", preview["fonts"])
	}
	if fonts["display"] != "Helvetica Neue" || fonts["body"] != "Helvetica" {
		t.Fatalf("theme_preview.fonts = %#v, want Helvetica Neue/Helvetica", fonts)
	}
	html, ok := preview["html"].(string)
	if !ok || !containsSubstring(html, "#242C38") || !containsSubstring(html, "#4D7CFE") {
		t.Fatalf("theme_preview.html = %q, want theme colors", html)
	}
	swatchValues, ok := preview["swatches"].([]interface{})
	if !ok || len(swatchValues) < 3 {
		t.Fatalf("theme_preview.swatches = %#v, want at least 3 swatches", preview["swatches"])
	}
}
