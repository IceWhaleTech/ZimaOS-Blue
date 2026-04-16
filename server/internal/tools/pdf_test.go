package tools

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
)

type stubPDFService struct {
	info            pdfextract.DocumentInfo
	extract         pdfextract.ExtractResult
	inspect         pdfextract.FormInspectResult
	fill            pdfextract.FillFormResult
	lastInfoPath    string
	lastExtractReq  pdfextract.ExtractRequest
	lastInspectPath string
	lastFillReq     pdfextract.FillFormRequest
	infoPaths       []string
	extractReqs     []pdfextract.ExtractRequest
	inspectPaths    []string
	fillReqs        []pdfextract.FillFormRequest
}

func (s *stubPDFService) Info(ctx context.Context, path string) (pdfextract.DocumentInfo, error) {
	_ = ctx
	s.lastInfoPath = path
	s.infoPaths = append(s.infoPaths, path)
	if s.info.Path == "" {
		s.info.Path = path
	}
	return s.info, nil
}

func (s *stubPDFService) Extract(ctx context.Context, req pdfextract.ExtractRequest) (pdfextract.ExtractResult, error) {
	_ = ctx
	s.lastExtractReq = req
	s.extractReqs = append(s.extractReqs, req)
	if s.extract.Document.Path == "" {
		s.extract.Document.Path = req.Path
	}
	return s.extract, nil
}

func (s *stubPDFService) InspectForm(ctx context.Context, path string) (pdfextract.FormInspectResult, error) {
	_ = ctx
	s.lastInspectPath = path
	s.inspectPaths = append(s.inspectPaths, path)
	if s.inspect.Document.Path == "" {
		s.inspect.Document.Path = path
	}
	return s.inspect, nil
}

func (s *stubPDFService) FillForm(ctx context.Context, req pdfextract.FillFormRequest) (pdfextract.FillFormResult, error) {
	_ = ctx
	s.lastFillReq = req
	s.fillReqs = append(s.fillReqs, req)
	if s.fill.Document.Path == "" {
		s.fill.Document.Path = req.Path
	}
	return s.fill, nil
}

func writeTestPDF(t *testing.T, name string, size int) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if size < len("%PDF-1.4\n") {
		size = len("%PDF-1.4\n")
	}
	body := "%PDF-1.4\n" + strings.Repeat("x", size-len("%PDF-1.4\n"))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write test pdf: %v", err)
	}
	return path
}

func testIntValue(v interface{}) int {
	switch typed := v.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func TestPDFToolInfoExecute(t *testing.T) {
	now := time.Now().UTC()
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{info: pdfextract.DocumentInfo{Path: path, FileName: "report.pdf", PageCount: 5, ModifiedAt: now, Engine: "pdfium/webassembly"}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "info", "path": path})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	doc := payload["document"].(pdfextract.DocumentInfo)
	if doc.PageCount != 5 {
		t.Fatalf("page_count = %d, want 5", doc.PageCount)
	}
	if svc.lastInfoPath != path {
		t.Fatalf("info path = %q, want %q", svc.lastInfoPath, path)
	}
}

func TestPDFToolDefinitionIncludesMarkdownAliases(t *testing.T) {
	tool := NewPDFTool(nil)
	def := tool.Definition()

	properties, ok := def.Parameters["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("properties type = %T, want object", def.Parameters["properties"])
	}

	for _, key := range []string{"content", "markdown", "body", "text"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("expected pdf schema to expose %q", key)
		}
	}
}

func TestPDFToolFillInspectExecute(t *testing.T) {
	path := writeTestPDF(t, "form.pdf", 256)
	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "form.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber:    1,
					Name:          "full_name",
					AlternateName: "Full Name",
					Type:          "text",
					Value:         "Alice",
				},
			},
		},
	}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "fill", "path": path})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if got := payload["mode"]; got != "inspect" {
		t.Fatalf("mode = %v, want inspect", got)
	}
	if got := payload["form_type"]; got != "acro_form" {
		t.Fatalf("form_type = %v, want acro_form", got)
	}
	if got := testIntValue(payload["field_count"]); got != 1 {
		t.Fatalf("field_count = %d, want 1", got)
	}
	if got := payload["degraded"]; got != false {
		t.Fatalf("degraded = %v, want false", got)
	}
	if _, ok := payload["fallback_reason"]; ok {
		t.Fatalf("fallback_reason = %#v, want absent", payload["fallback_reason"])
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", payload["validation"])
	}
	if got := validation["fillable"]; got != true {
		t.Fatalf("fillable = %v, want true", got)
	}
	supportedTypes, ok := validation["supported_fill_types"].([]string)
	if !ok {
		t.Fatalf("supported_fill_types type = %T, want []string", validation["supported_fill_types"])
	}
	if !reflect.DeepEqual(supportedTypes, []string{"text", "combo", "list", "checkbox", "radio"}) {
		t.Fatalf("supported_fill_types = %#v, want native supported fill types", supportedTypes)
	}
	unsupportedTypes, ok := validation["unsupported_field_types"].([]string)
	if !ok {
		t.Fatalf("unsupported_field_types type = %T, want []string", validation["unsupported_field_types"])
	}
	if len(unsupportedTypes) != 0 {
		t.Fatalf("unsupported_field_types = %#v, want none", unsupportedTypes)
	}
	fields, ok := payload["fields"].([]pdfextract.FormField)
	if !ok {
		t.Fatalf("fields type = %T, want []pdf.FormField", payload["fields"])
	}
	if len(fields) != 1 || fields[0].Name != "full_name" {
		t.Fatalf("fields = %#v, want full_name field", fields)
	}
	if svc.lastInspectPath != path {
		t.Fatalf("inspect path = %q, want %q", svc.lastInspectPath, path)
	}
}

func TestPDFToolFillInspectExecuteReportsUnsupportedFieldTypes(t *testing.T) {
	path := writeTestPDF(t, "signature.pdf", 256)
	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "signature.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "approval",
					Type:       "signature",
				},
			},
		},
	}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "fill", "path": path})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if got := payload["degraded"]; got != true {
		t.Fatalf("degraded = %v, want true", got)
	}
	if got := payload["fallback_reason"]; got != "native_fill_unsupported_field_types" {
		t.Fatalf("fallback_reason = %v, want native_fill_unsupported_field_types", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", payload["validation"])
	}
	if got := validation["fillable"]; got != false {
		t.Fatalf("fillable = %v, want false", got)
	}
	unsupportedTypes, ok := validation["unsupported_field_types"].([]string)
	if !ok {
		t.Fatalf("unsupported_field_types type = %T, want []string", validation["unsupported_field_types"])
	}
	if !reflect.DeepEqual(unsupportedTypes, []string{"signature"}) {
		t.Fatalf("unsupported_field_types = %#v, want [signature]", unsupportedTypes)
	}
}

func TestPDFToolFillInspectExecuteReportsUnsupportedFormType(t *testing.T) {
	path := writeTestPDF(t, "xfa.pdf", 256)
	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "xfa.pdf", Engine: "pdfium/webassembly"},
			FormType:   "xfa_full",
			FieldCount: 0,
		},
	}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{"action": "fill", "path": path})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if got := payload["degraded"]; got != true {
		t.Fatalf("degraded = %v, want true", got)
	}
	if got := payload["fallback_reason"]; got != "native_fill_unsupported_form_type" {
		t.Fatalf("fallback_reason = %v, want native_fill_unsupported_form_type", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("validation type = %T, want object", payload["validation"])
	}
	if got := validation["fillable"]; got != false {
		t.Fatalf("fillable = %v, want false", got)
	}
}

func TestPDFToolFillWriteRejectsUnsupportedFormTypeBeforeNativeWrite(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "xfa.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "xfa.pdf", Engine: "pdfium/webassembly"},
			FormType:   "xfa_full",
			FieldCount: 0,
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "xfa.pdf",
		"output_path": "exports/xfa-filled.pdf",
		"fields":      map[string]interface{}{"full_name": "Bob"},
	})
	if err == nil {
		t.Fatal("expected unsupported form type write to fail")
	}
	if !strings.Contains(err.Error(), "xfa_full") {
		t.Fatalf("error = %v, want form type in error", err)
	}
	if len(svc.fillReqs) != 0 {
		t.Fatalf("fill requests = %#v, want none", svc.fillReqs)
	}
}

func TestPDFToolFillWriteRejectsUnsupportedRequestedFieldTypeBeforeNativeWrite(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "signature.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "signature.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "approval",
					Type:       "signature",
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "signature.pdf",
		"output_path": "exports/signature-filled.pdf",
		"fields":      map[string]interface{}{"approval": "signed"},
	})
	if err == nil {
		t.Fatal("expected unsupported requested field type write to fail")
	}
	if !strings.Contains(err.Error(), "approval") || !strings.Contains(err.Error(), "signature") {
		t.Fatalf("error = %v, want field name and type", err)
	}
	if len(svc.fillReqs) != 0 {
		t.Fatalf("fill requests = %#v, want none", svc.fillReqs)
	}
}

func TestPDFToolFillWriteRejectsDocumentWithoutInteractiveFormBeforeNativeWrite(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "plain.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "plain.pdf", Engine: "pdfium/webassembly"},
			FormType:   "none",
			FieldCount: 0,
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "plain.pdf",
		"output_path": "exports/plain-filled.pdf",
		"fields":      map[string]interface{}{"full_name": "Bob"},
	})
	if err == nil {
		t.Fatal("expected plain pdf write to fail")
	}
	if !strings.Contains(err.Error(), "does not contain an interactive form") {
		t.Fatalf("error = %v, want no-form guidance", err)
	}
	if len(svc.fillReqs) != 0 {
		t.Fatalf("fill requests = %#v, want none", svc.fillReqs)
	}
}

func TestPDFToolFillWriteExecute(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "form.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "form.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"full_name"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "full_name",
					Type:       "text",
					Value:      "Bob",
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "form.pdf",
		"output_path": "exports/filled.pdf",
		"fields":      map[string]interface{}{"full_name": "Bob"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := parseNativeDocumentPayload(t, result)
	if got := payload["engine"]; got != "pdfium/webassembly" {
		t.Fatalf("engine = %v, want pdfium/webassembly", got)
	}
	if got := payload["path"]; got != "exports/filled.pdf" {
		t.Fatalf("path = %v, want exports/filled.pdf", got)
	}
	validation, ok := payload["validation"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected validation payload, got %#v", payload["validation"])
	}
	if got := validation["verified"]; got != true {
		t.Fatalf("verified = %v, want true", got)
	}
	if svc.lastFillReq.Path != sourcePath {
		t.Fatalf("fill path = %q, want %q", svc.lastFillReq.Path, sourcePath)
	}
	if !reflect.DeepEqual(svc.lastFillReq.Fields, map[string]string{"full_name": "Bob"}) {
		t.Fatalf("fill fields = %#v, want full_name=Bob", svc.lastFillReq.Fields)
	}
	outputPath := filepath.Join(workspaceDir, "exports", "filled.pdf")
	if svc.lastInspectPath != outputPath {
		t.Fatalf("inspect path = %q, want %q", svc.lastInspectPath, outputPath)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "%PDF-1.4\nfilled" {
		t.Fatalf("output bytes = %q, want filled pdf bytes", string(data))
	}
}

func TestPDFToolFillRejectsWriteWithoutOutputPath(t *testing.T) {
	path := writeTestPDF(t, "form.pdf", 256)
	tool := NewPDFTool(&stubPDFService{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "fill",
		"path":   path,
		"fields": map[string]interface{}{"full_name": "Bob"},
	})
	if err == nil {
		t.Fatal("expected fill write without output_path to fail")
	}
	if !strings.Contains(err.Error(), "output_path is required") {
		t.Fatalf("error = %v, want output_path guidance", err)
	}
}

func TestPDFToolFillRejectsVerificationMismatch(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "form.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "form.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"full_name"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "full_name",
					Type:       "text",
					Value:      "Alice",
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "form.pdf",
		"output_path": "exports/filled.pdf",
		"fields":      map[string]interface{}{"full_name": "Bob"},
	})
	if err == nil {
		t.Fatal("expected verification mismatch to fail")
	}
	if !strings.Contains(err.Error(), "verification") {
		t.Fatalf("error = %v, want verification failure", err)
	}
}

func TestPDFToolFillWriteExecuteForComboOptionLabel(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "combo.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "combo.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"favorite_color"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "combo-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "favorite_color",
					Type:       "combo",
					Value:      "Green",
					Options: []pdfextract.FormFieldOption{
						{Index: 0, Label: "Red"},
						{Index: 1, Label: "Green", Selected: true},
						{Index: 2, Label: "Blue"},
					},
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "combo.pdf",
		"output_path": "exports/combo-filled.pdf",
		"fields":      map[string]interface{}{"favorite_color": "Green"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !reflect.DeepEqual(svc.lastFillReq.Fields, map[string]string{"favorite_color": "Green"}) {
		t.Fatalf("fill fields = %#v, want favorite_color=Green", svc.lastFillReq.Fields)
	}
}

func TestPDFToolFillWriteExecuteForComboOptionIndex(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "combo.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "combo.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"favorite_color"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "combo-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "favorite_color",
					Type:       "combo",
					Value:      "Green",
					Options: []pdfextract.FormFieldOption{
						{Index: 0, Label: "Red"},
						{Index: 1, Label: "Green", Selected: true},
						{Index: 2, Label: "Blue"},
					},
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "combo.pdf",
		"output_path": "exports/combo-filled.pdf",
		"fields":      map[string]interface{}{"favorite_color": "1"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillWriteExecuteForCheckboxTrue(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "checkbox.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "checkbox.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"subscribe"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "checkbox-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber:  1,
					Name:        "subscribe",
					Type:        "checkbox",
					Checked:     true,
					ExportValue: "Yes",
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "checkbox.pdf",
		"output_path": "exports/checkbox-filled.pdf",
		"fields":      map[string]interface{}{"subscribe": "true"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillWriteExecuteForCheckboxFalseAlias(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "checkbox.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "checkbox.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"subscribe"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "checkbox-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber:  1,
					Name:        "subscribe",
					Type:        "checkbox",
					Checked:     false,
					ExportValue: "Yes",
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "checkbox.pdf",
		"output_path": "exports/checkbox-filled.pdf",
		"fields":      map[string]interface{}{"subscribe": "off"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillWriteExecuteForListOptionLabel(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "list.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "list.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"priority"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "list-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "priority",
					Type:       "list",
					Value:      "High",
					Options: []pdfextract.FormFieldOption{
						{Index: 0, Label: "Low"},
						{Index: 1, Label: "Medium"},
						{Index: 2, Label: "High", Selected: true},
					},
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "list.pdf",
		"output_path": "exports/list-filled.pdf",
		"fields":      map[string]interface{}{"priority": "High"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillWriteExecuteForListOptionIndex(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "list.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "list.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"priority"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "list-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 1,
			Fields: []pdfextract.FormField{
				{
					PageNumber: 1,
					Name:       "priority",
					Type:       "list",
					Value:      "High",
					Options: []pdfextract.FormFieldOption{
						{Index: 0, Label: "Low"},
						{Index: 1, Label: "Medium"},
						{Index: 2, Label: "High", Selected: true},
					},
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "list.pdf",
		"output_path": "exports/list-filled.pdf",
		"fields":      map[string]interface{}{"priority": "2"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillWriteExecuteForRadioExportValue(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "radio.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "radio.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"contact_method"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "radio-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 2,
			Fields: []pdfextract.FormField{
				{
					PageNumber:  1,
					Name:        "contact_method",
					Type:        "radio",
					ExportValue: "Email",
					Checked:     false,
				},
				{
					PageNumber:  1,
					Name:        "contact_method",
					Type:        "radio",
					ExportValue: "Phone",
					Checked:     true,
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "radio.pdf",
		"output_path": "exports/radio-filled.pdf",
		"fields":      map[string]interface{}{"contact_method": "Phone"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
}

func TestPDFToolFillRejectsRadioVerificationMismatch(t *testing.T) {
	workspaceDir := t.TempDir()
	sourcePath := filepath.Join(workspaceDir, "radio.pdf")
	if err := os.WriteFile(sourcePath, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write source pdf: %v", err)
	}

	svc := &stubPDFService{
		fill: pdfextract.FillFormResult{
			Document:      pdfextract.DocumentInfo{FileName: "radio.pdf", Engine: "pdfium/webassembly"},
			UpdatedFields: []string{"contact_method"},
			Bytes:         []byte("%PDF-1.4\nfilled"),
		},
		inspect: pdfextract.FormInspectResult{
			Document:   pdfextract.DocumentInfo{FileName: "radio-filled.pdf", Engine: "pdfium/webassembly"},
			FormType:   "acro_form",
			FieldCount: 2,
			Fields: []pdfextract.FormField{
				{
					PageNumber:  1,
					Name:        "contact_method",
					Type:        "radio",
					ExportValue: "Email",
					Checked:     true,
				},
				{
					PageNumber:  1,
					Name:        "contact_method",
					Type:        "radio",
					ExportValue: "Phone",
					Checked:     false,
				},
			},
		},
	}
	tool := NewPDFTool(svc)
	tool.scope = newFSToolScope([]string{workspaceDir})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"action":      "fill",
		"path":        "radio.pdf",
		"output_path": "exports/radio-filled.pdf",
		"fields":      map[string]interface{}{"contact_method": "Phone"},
	})
	if err == nil {
		t.Fatal("expected radio verification mismatch to fail")
	}
	if !strings.Contains(err.Error(), "verification") {
		t.Fatalf("error = %v, want verification failure", err)
	}
}

func TestNewPDFToolUsesLongerDefaultDownloadTimeout(t *testing.T) {
	tool := NewPDFTool(&stubPDFService{})
	if tool.httpClient == nil {
		t.Fatal("expected http client")
	}
	if tool.httpClient.Timeout != 5*time.Minute {
		t.Fatalf("http timeout = %v, want %v", tool.httpClient.Timeout, 5*time.Minute)
	}
}

func TestRegisterPDFToolLeavesToolEnabled(t *testing.T) {
	registry := NewRegistry()
	RegisterPDFTool(registry, &stubPDFService{})
	if registry.IsDisabled("pdf") {
		t.Fatal("expected pdf tool to remain enabled")
	}
	if _, ok := registry.LookupDefinitionForRoute("pdf", ToolRouteKindChat); !ok {
		t.Fatal("expected pdf tool definition to be visible for chat route")
	}
}

func TestRegisterPDFToolInheritsFileReadAllowedPaths(t *testing.T) {
	workspaceDir := t.TempDir()
	path := filepath.Join(workspaceDir, "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	registry := NewRegistry()
	registry.Register(NewFileReadTool([]string{workspaceDir}, 0))
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	RegisterPDFTool(registry, svc)

	rawTool := registry.Get("pdf")
	if rawTool == nil {
		t.Fatal("expected registered pdf tool")
	}

	result, err := rawTool.Execute(context.Background(), map[string]interface{}{"path": "report.pdf"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "hello" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello")
	}
	if svc.lastExtractReq.Path != path {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, path)
	}
}

func TestPDFToolInfoExecuteSupportsMultiplePDFs(t *testing.T) {
	now := time.Now().UTC()
	path1 := writeTestPDF(t, "one.pdf", 256)
	path2 := writeTestPDF(t, "two.pdf", 256)
	svc := &stubPDFService{info: pdfextract.DocumentInfo{PageCount: 5, ModifiedAt: now, Engine: "pdfium/webassembly"}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"action": "info",
		"pdf":    path1,
		"pdfs":   []interface{}{path2, path1},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "multi" {
		t.Fatalf("mode = %v, want multi", payload["mode"])
	}
	if payload["count"] != 2 {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
	documents, ok := payload["documents"].([]pdfextract.DocumentInfo)
	if !ok || len(documents) != 2 {
		t.Fatalf("documents = %#v", payload["documents"])
	}
	wantPaths := []string{path1, path2}
	if !reflect.DeepEqual(svc.infoPaths, wantPaths) {
		t.Fatalf("info paths = %#v, want %#v", svc.infoPaths, wantPaths)
	}
}

func TestPDFToolReadExecuteSupportsNestedCamelCaseArgs(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello world", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"input": map[string]interface{}{
			"filePath":      path,
			"page":          2,
			"maxChars":      5,
			"includePages":  true,
			"disableOcr":    true,
			"disableVision": true,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "hello world" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello world")
	}
	if svc.lastExtractReq.Path != path {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, path)
	}
	if !reflect.DeepEqual(svc.lastExtractReq.Pages, []int{2}) {
		t.Fatalf("pages = %#v, want %#v", svc.lastExtractReq.Pages, []int{2})
	}
	if svc.lastExtractReq.MaxChars != 5 {
		t.Fatalf("max chars = %d, want %d", svc.lastExtractReq.MaxChars, 5)
	}
	if !svc.lastExtractReq.IncludePages {
		t.Fatal("expected include pages to be true")
	}
	if !svc.lastExtractReq.DisableOCR {
		t.Fatal("expected disable OCR to be true")
	}
	if !svc.lastExtractReq.DisableVision {
		t.Fatal("expected disable vision to be true")
	}
}

func TestPDFToolReadExecuteSupportsGroupedIncludeAndFallbackMode(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello world", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewPDFTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":          path,
		"fallback_mode": "text_only",
		"include": map[string]interface{}{
			"pages":           true,
			"layout":          true,
			"headers_footers": true,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !svc.lastExtractReq.IncludePages {
		t.Fatal("expected include pages to be true")
	}
	if !svc.lastExtractReq.IncludeLayout {
		t.Fatal("expected include layout to be true")
	}
	if !svc.lastExtractReq.IncludeHeadersFooters {
		t.Fatal("expected include headers/footers to be true")
	}
	if !svc.lastExtractReq.DisableOCR {
		t.Fatal("expected disable OCR to be true for text_only mode")
	}
	if !svc.lastExtractReq.DisableVision {
		t.Fatal("expected disable vision to be true for text_only mode")
	}
}

func TestPDFToolReadExecuteParsesPages(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello", SelectedPages: []int{1, 3, 4}}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":          path,
		"pages":         "1,3-4",
		"max_pages":     10,
		"max_chars":     4096,
		"include_pages": true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "hello" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello")
	}
	wantPages := []int{1, 3, 4}
	if !reflect.DeepEqual(svc.lastExtractReq.Pages, wantPages) {
		t.Fatalf("pages = %#v, want %#v", svc.lastExtractReq.Pages, wantPages)
	}
	if svc.lastExtractReq.MaxPages != 10 {
		t.Fatalf("max_pages = %d, want 10", svc.lastExtractReq.MaxPages)
	}
	if svc.lastExtractReq.MaxChars != 4096 {
		t.Fatalf("max_chars = %d, want 4096", svc.lastExtractReq.MaxChars)
	}
	if !svc.lastExtractReq.IncludePages {
		t.Fatal("expected include_pages=true")
	}
}

func TestPDFToolReadExecuteResolvesRelativeWorkspacePath(t *testing.T) {
	workspaceDir := t.TempDir()
	path := filepath.Join(workspaceDir, "report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewPDFTool(svc)
	ctx := WithFSRootOverride(context.Background(), []string{workspaceDir}, map[string]string{"workspace": workspaceDir})

	result, err := tool.Execute(ctx, map[string]interface{}{"path": "report.pdf"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "hello" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello")
	}
	if svc.lastExtractReq.Path != path {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, path)
	}
}

func TestPDFToolReadExecuteResolvesRelativeWorkspacePathFromAdditionalScope(t *testing.T) {
	workspaceDir := t.TempDir()
	path := filepath.Join(workspaceDir, "openclaw_report.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\nstub"), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}

	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello", Document: pdfextract.DocumentInfo{FileName: "openclaw_report.pdf"}}}
	tool := NewPDFTool(svc)
	ctx := WithFSScope(context.Background(), []string{workspaceDir}, map[string]string{"workspace": workspaceDir})

	result, err := tool.Execute(ctx, map[string]interface{}{"path": "openclaw_report.pdf"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "hello" {
		t.Fatalf("text = %q, want %q", payload.Text, "hello")
	}
	if svc.lastExtractReq.Path != path {
		t.Fatalf("extract path = %q, want %q", svc.lastExtractReq.Path, path)
	}
}

func TestPDFToolReadExecuteCombinesPageAndPages(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":  path,
		"page":  2,
		"pages": []interface{}{4, "6-7", 2},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	wantPages := []int{2, 4, 6, 7}
	if !reflect.DeepEqual(svc.lastExtractReq.Pages, wantPages) {
		t.Fatalf("pages = %#v, want %#v", svc.lastExtractReq.Pages, wantPages)
	}
}

func TestPDFToolReadExecuteDisablesOCRAndVision(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":           path,
		"disable_ocr":    true,
		"disable_vision": true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !svc.lastExtractReq.DisableOCR {
		t.Fatal("expected DisableOCR=true")
	}
	if !svc.lastExtractReq.DisableVision {
		t.Fatal("expected DisableVision=true")
	}
}

func TestPDFToolReadExecuteUsesStructuredDefaults(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": path})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !svc.lastExtractReq.IncludeMarkdown {
		t.Fatal("expected IncludeMarkdown=true by default")
	}
	if !svc.lastExtractReq.IncludeOutline {
		t.Fatal("expected IncludeOutline=true by default")
	}
	if svc.lastExtractReq.IncludeLayout {
		t.Fatal("expected IncludeLayout=false by default")
	}
	if svc.lastExtractReq.IncludeHeadersFooters {
		t.Fatal("expected IncludeHeadersFooters=false by default")
	}
}

func TestPDFToolReadExecuteSupportsStructuredFlags(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":                    path,
		"include_markdown":        false,
		"include_outline":         false,
		"include_layout":          true,
		"include_headers_footers": true,
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if svc.lastExtractReq.IncludeMarkdown {
		t.Fatal("expected IncludeMarkdown=false")
	}
	if svc.lastExtractReq.IncludeOutline {
		t.Fatal("expected IncludeOutline=false")
	}
	if !svc.lastExtractReq.IncludeLayout {
		t.Fatal("expected IncludeLayout=true")
	}
	if !svc.lastExtractReq.IncludeHeadersFooters {
		t.Fatal("expected IncludeHeadersFooters=true")
	}
}

func TestPDFToolReadExecuteSupportsMultiPDFInput(t *testing.T) {
	path1 := writeTestPDF(t, "one.pdf", 256)
	path2 := writeTestPDF(t, "two.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "hello from pdf", Document: pdfextract.DocumentInfo{FileName: "report.pdf"}}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pdf":  path1,
		"pdfs": []interface{}{"\t" + path2 + " ", path1},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	if payload["mode"] != "multi" {
		t.Fatalf("mode = %v, want multi", payload["mode"])
	}
	if payload["count"] != 2 {
		t.Fatalf("count = %v, want 2", payload["count"])
	}
	results, ok := payload["results"].([]pdfextract.ExtractResult)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %#v", payload["results"])
	}
	text, _ := payload["text"].(string)
	if !strings.Contains(text, "[PDF 1]") || !strings.Contains(text, "[PDF 2]") {
		t.Fatalf("text = %q", text)
	}
	wantPaths := []string{path1, path2}
	gotPaths := []string{svc.extractReqs[0].Path, svc.extractReqs[1].Path}
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("extract paths = %#v, want %#v", gotPaths, wantPaths)
	}
	selected, ok := payload["selected_pdfs"].([]string)
	if !ok || !reflect.DeepEqual(selected, wantPaths) {
		t.Fatalf("selected_pdfs = %#v, want %#v", payload["selected_pdfs"], wantPaths)
	}
}

func TestPDFToolReadExecuteSupportsMultiPDFMarkdownAggregation(t *testing.T) {
	path1 := writeTestPDF(t, "one.pdf", 256)
	path2 := writeTestPDF(t, "two.pdf", 256)
	svc := &stubPDFService{extract: pdfextract.ExtractResult{
		Text:     "hello from pdf",
		Markdown: "# Summary\n\n- bullet",
		Document: pdfextract.DocumentInfo{FileName: "report.pdf"},
	}}
	tool := NewPDFTool(svc)

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pdf":  path1,
		"pdfs": []interface{}{path2},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(map[string]interface{})
	markdown, _ := payload["markdown"].(string)
	for _, needle := range []string{"[PDF 1] one.pdf", "[PDF 2] two.pdf", "# Summary"} {
		if !strings.Contains(markdown, needle) {
			t.Fatalf("expected combined markdown to contain %q, got=%q", needle, markdown)
		}
	}
}

func TestPDFToolReadExecuteDownloadsRemotePDFURL(t *testing.T) {
	svc := &stubPDFService{extract: pdfextract.ExtractResult{Text: "remote pdf"}}
	tool := NewPDFTool(svc)
	tool.SetHTTPClient(&http.Client{Transport: routingRoundTripper(func(req *http.Request) (*http.Response, error) {
		if req.URL.String() != "https://example.com/blob?id=1" {
			t.Fatalf("url = %q", req.URL.String())
		}
		return staticRoundTripper{contentType: "application/pdf", body: []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")}.RoundTrip(req)
	})})

	result, err := tool.Execute(context.Background(), map[string]interface{}{"pdf": "https://example.com/blob?id=1"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	payload := result.(pdfextract.ExtractResult)
	if payload.Text != "remote pdf" {
		t.Fatalf("text = %q, want remote pdf", payload.Text)
	}
	if svc.lastExtractReq.Path == "https://example.com/blob?id=1" {
		t.Fatalf("extract path should be temp file, got %q", svc.lastExtractReq.Path)
	}
	if filepath.Ext(svc.lastExtractReq.Path) != ".pdf" {
		t.Fatalf("extract path extension = %q, want .pdf", filepath.Ext(svc.lastExtractReq.Path))
	}
}

func TestPDFToolReadExecuteRejectsPrivateRemoteURL(t *testing.T) {
	tool := NewPDFTool(&stubPDFService{})
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"pdf": "http://127.0.0.1:8080/private.pdf",
	})
	if err == nil {
		t.Fatal("expected private host error")
	}
	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "blocked") && !strings.Contains(msg, "private") {
		t.Fatalf("error = %v", err)
	}
}

func TestPDFToolReadExecuteRejectsTooManyPDFs(t *testing.T) {
	tool := NewPDFTool(&stubPDFService{})
	inputs := make([]interface{}, 0, maxPDFInputs+1)
	for i := 0; i < maxPDFInputs+1; i++ {
		inputs = append(inputs, fmt.Sprintf("/tmp/doc-%d.pdf", i))
	}
	_, err := tool.Execute(context.Background(), map[string]interface{}{"pdfs": inputs})
	if err == nil {
		t.Fatal("expected too many pdfs error")
	}
	if !strings.Contains(err.Error(), "too many pdf inputs") {
		t.Fatalf("error = %v", err)
	}
}

func TestPDFToolReadExecuteRejectsInvalidPages(t *testing.T) {
	path := writeTestPDF(t, "report.pdf", 256)
	tool := NewPDFTool(&stubPDFService{})

	_, err := tool.Execute(context.Background(), map[string]interface{}{"path": path, "pages": "3-1"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestPDFToolReadExecuteRejectsRemotePDFOverMaxBytes(t *testing.T) {
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)
	tool.SetHTTPClient(&http.Client{Transport: routingRoundTripper(func(req *http.Request) (*http.Response, error) {
		return staticRoundTripper{contentType: "application/pdf", body: []byte("%PDF-1.4\n" + strings.Repeat("x", 2<<20))}.RoundTrip(req)
	})})

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"pdf":          "https://example.com/large.pdf",
		"max_bytes_mb": 1,
	})
	if err == nil {
		t.Fatal("expected max bytes error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
	if len(svc.extractReqs) != 0 {
		t.Fatalf("extract should not run, got %d calls", len(svc.extractReqs))
	}
}

func TestPDFToolReadExecuteRejectsLocalPDFOverMaxBytes(t *testing.T) {
	svc := &stubPDFService{}
	tool := NewPDFTool(svc)
	path := writeTestPDF(t, "large.pdf", 2<<20)

	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"pdf":          path,
		"max_bytes_mb": 1,
	})
	if err == nil {
		t.Fatal("expected max bytes error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("error = %v", err)
	}
	if len(svc.extractReqs) != 0 {
		t.Fatalf("extract should not run, got %d calls", len(svc.extractReqs))
	}
}
