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
	info           pdfextract.DocumentInfo
	extract        pdfextract.ExtractResult
	lastInfoPath   string
	lastExtractReq pdfextract.ExtractRequest
	infoPaths      []string
	extractReqs    []pdfextract.ExtractRequest
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
