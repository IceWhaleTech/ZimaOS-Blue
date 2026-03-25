package pdf

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.uber.org/zap"
)

func TestServiceInfoAndExtractTextPDF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.pdf")
	if err := os.WriteFile(path, buildTextPDF("Hello PDF"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil)
	defer func() {
		_ = svc.Close()
	}()

	info, err := svc.Info(context.Background(), path)
	if err != nil {
		t.Fatalf("Info returned error: %v", err)
	}
	if info.PageCount != 1 {
		t.Fatalf("page count = %d, want 1", info.PageCount)
	}
	if info.Engine != engineName {
		t.Fatalf("engine = %q, want %q", info.Engine, engineName)
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{Path: path, IncludePages: true})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !strings.Contains(result.Text, "Hello PDF") {
		t.Fatalf("text = %q, want Hello PDF", result.Text)
	}
	if !strings.Contains(result.RawText, "Hello PDF") {
		t.Fatalf("raw_text = %q, want Hello PDF", result.RawText)
	}
	if len(result.SelectedPages) != 1 || result.SelectedPages[0] != 1 {
		t.Fatalf("selected pages = %#v, want [1]", result.SelectedPages)
	}
	if result.OCRUsed {
		t.Fatal("expected OCRUsed=false for text PDF")
	}
	if result.VisionUsed {
		t.Fatal("expected VisionUsed=false for text PDF")
	}
	if len(result.Pages) != 1 || !strings.Contains(result.Pages[0].Text, "Hello PDF") {
		t.Fatalf("pages = %#v", result.Pages)
	}
}

func TestCanonicalizeExtractedPDFText_PreservesUsefulLayoutSignals(t *testing.T) {
	raw := "\u0000Top Categories\nAI & LLMs    287\nSearch & Research    253\n\nCollected:\u00a0February 7, 2026"

	got := canonicalizeExtractedPDFText(raw)
	want := strings.Join([]string{
		"Top Categories",
		"AI & LLMs    287",
		"Search & Research    253",
		"",
		"Collected: February 7, 2026",
	}, "\n")

	if got != want {
		t.Fatalf("canonicalizeExtractedPDFText() = %q, want %q", got, want)
	}
}

func TestNormalizeText_CleansWrappedListsAndParagraphs(t *testing.T) {
	raw := "\u0000Executive Summary\nThe gateway exposes a typed\nWebSocket API.\n\n• Scheduled daily brief-\ning + memory write-back\n1) Prompt-injection\ncontainment\n\u00a0\u00a0February 7, 2026\u00a0"

	got := normalizeText(raw)
	want := strings.Join([]string{
		"Executive Summary",
		"The gateway exposes a typed WebSocket API.",
		"",
		"- Scheduled daily briefing + memory write-back",
		"1. Prompt-injection containment",
		"February 7, 2026",
	}, "\n")

	if got != want {
		t.Fatalf("normalizeText() = %q, want %q", got, want)
	}
}

func buildTextPDF(text string) []byte {
	stream := fmt.Sprintf("BT\n/F1 24 Tf\n72 96 Td\n(%s) Tj\nET\n", escapePDFText(text))
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 144] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, obj := range objects {
		offsets[index+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", index+1, obj)
	}
	xrefStart := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects)+1)
	buf.WriteString("0000000000 65535 f \n")
	for index := 1; index <= len(objects); index++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[index])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Root 1 0 R /Size %d >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xrefStart)
	return buf.Bytes()
}

func escapePDFText(text string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "(", "\\(", ")", "\\)")
	return replacer.Replace(text)
}
