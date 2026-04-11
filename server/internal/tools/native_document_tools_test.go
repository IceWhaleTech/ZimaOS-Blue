package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
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
	text := string(data)
	for _, needle := range []string{
		"Launch Brief",
		"Q2 roll-out",
		"Overview",
		"Ship docx/xlsx/pptx now.",
		"Form filling remains a follow-up slice.",
		"Prepared by Blue.",
	} {
		if !containsSubstring(text, needle) {
			t.Fatalf("expected generated PDF bytes to contain %q", needle)
		}
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
	text := string(data)
	for _, needle := range []string{
		"Reformatted Brief",
		"Executive Summary",
		"Blue can now reformat PDFs.",
		"Keep layout simple.",
	} {
		if !containsSubstring(text, needle) {
			t.Fatalf("expected reformatted PDF bytes to contain %q", needle)
		}
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

func hasStringValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
