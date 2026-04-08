//go:build darwin

package pdf

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"go.uber.org/zap"
)

func TestNativePDFEngineNameUsesNativePDFKitOnDarwin(t *testing.T) {
	if got := nativePDFEngineName(); got != "pdfkit/native" {
		t.Fatalf("nativePDFEngineName() = %q, want %q", got, "pdfkit/native")
	}
}

func TestRunDarwinPDFKitExtractUsesInjectedNativeExtractor(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	called := false
	darwinPDFKitExtractFunc = func(ctx context.Context, path string) (*darwinPDFKitOutput, error) {
		called = true
		if path != "/tmp/sample.pdf" {
			t.Fatalf("path = %q, want %q", path, "/tmp/sample.pdf")
		}
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Pages: []darwinPDFKitPage{
				{Number: 1, Text: "native page"},
			},
		}, nil
	}

	output, ok, err := runDarwinPDFKitExtract(context.Background(), "/tmp/sample.pdf")
	if err != nil {
		t.Fatalf("runDarwinPDFKitExtract returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected native PDF extractor to be available on darwin")
	}
	if !called {
		t.Fatal("expected injected native extractor to be called")
	}
	if output == nil || output.PageCount != 1 {
		t.Fatalf("output = %#v, want page_count=1", output)
	}
	if len(output.Pages) != 1 || output.Pages[0].Text != "native page" {
		t.Fatalf("pages = %#v, want single native page", output.Pages)
	}
}

func TestTryNativePDFExtractBuildsLightweightStructuredOutput(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf fixture: %v", err)
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Pages: []darwinPDFKitPage{
				{
					Number: 1,
					Text:   "Executive Summary\n- First item\n- Second item",
				},
			},
		}, nil
	}

	result, ok, err := tryNativePDFExtract(context.Background(), ExtractRequest{
		Path:            path,
		IncludePages:    true,
		IncludeMarkdown: true,
		IncludeLayout:   true,
		IncludeOutline:  true,
	}, path, stat)
	if err != nil {
		t.Fatalf("tryNativePDFExtract returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected native PDF extraction to be available")
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
	if result.Markdown == "" {
		t.Fatal("expected markdown output")
	}
	if len(result.Pages) != 1 {
		t.Fatalf("pages len = %d, want 1", len(result.Pages))
	}
	if len(result.Pages[0].Blocks) == 0 {
		t.Fatal("expected lightweight blocks for native PDF extraction")
	}
	if len(result.Outline) == 0 || result.Outline[0].Title != "Executive Summary" {
		t.Fatalf("outline = %#v, want heading from synthetic presentation", result.Outline)
	}
}

func TestTryNativePDFInfoIncludesNativeMetadataOnDarwin(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf fixture: %v", err)
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Metadata: map[string]string{
				"Title":  "Native Title",
				"Author": "Native Author",
			},
		}, nil
	}

	info, ok, err := tryNativePDFInfo(context.Background(), path, stat)
	if err != nil {
		t.Fatalf("tryNativePDFInfo returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected native PDF info to be available")
	}
	if info.Metadata["Title"] != "Native Title" || info.Metadata["Author"] != "Native Author" {
		t.Fatalf("metadata = %#v, want native metadata map", info.Metadata)
	}
}

func TestTryNativePDFExtractPrefersNativeOutlineOnDarwin(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat pdf fixture: %v", err)
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Outline: []OutlineEntry{
				{Title: "Native Bookmark", Level: 1, PageNumber: 1},
			},
			Pages: []darwinPDFKitPage{
				{Number: 1, Text: "Executive Summary"},
			},
		}, nil
	}

	result, ok, err := tryNativePDFExtract(context.Background(), ExtractRequest{
		Path:           path,
		IncludeOutline: true,
	}, path, stat)
	if err != nil {
		t.Fatalf("tryNativePDFExtract returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected native PDF extraction to be available")
	}
	if len(result.Outline) != 1 || result.Outline[0].Title != "Native Bookmark" {
		t.Fatalf("outline = %#v, want native outline", result.Outline)
	}
}

func TestServiceExtractPrefersNativePDFKitForLightweightDarwinRequests(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, pdfiumRuntimeFileName)
	if err := os.WriteFile(runtimePath, []byte("pdfium-runtime"), 0o644); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}

	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   runtimeDir,
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected lightweight darwin requests to avoid pdfium initialization")
		return nil, nil
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Pages: []darwinPDFKitPage{
				{Number: 1, Text: "Executive Summary\n- Native only"},
			},
		}, nil
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:            path,
		IncludeMarkdown: true,
		IncludeOutline:  true,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
	if result.Markdown == "" {
		t.Fatal("expected markdown output from native darwin extraction")
	}
}

func TestServiceExtractKeepsEmptyNativeTextOnDarwinWithoutPDFiumFallback(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, pdfiumRuntimeFileName)
	if err := os.WriteFile(runtimePath, []byte("pdfium-runtime"), 0o644); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}

	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   runtimeDir,
		AutoDownload: false,
	})
	called := false
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		called = true
		return nil, fmt.Errorf("forced pdfium init failure")
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Pages: []darwinPDFKitPage{
				{Number: 1, Text: ""},
			},
		}, nil
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:            path,
		IncludeMarkdown: true,
		IncludeOutline:  true,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if called {
		t.Fatal("expected empty native text extraction to avoid pdfium fallback on darwin")
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
}

func TestServiceExtractPrefersNativePDFKitForLayoutDarwinRequests(t *testing.T) {
	original := darwinPDFKitExtractFunc
	t.Cleanup(func() {
		darwinPDFKitExtractFunc = original
	})

	path := filepath.Join(t.TempDir(), "layout.pdf")
	if err := os.WriteFile(path, buildTextPDF("Executive Summary"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, pdfiumRuntimeFileName)
	if err := os.WriteFile(runtimePath, []byte("pdfium-runtime"), 0o644); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   runtimeDir,
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected darwin layout extraction to avoid pdfium initialization")
		return nil, nil
	}

	darwinPDFKitExtractFunc = func(ctx context.Context, gotPath string) (*darwinPDFKitOutput, error) {
		if err := ctx.Err(); err != nil {
			t.Fatalf("context unexpectedly canceled: %v", err)
		}
		if gotPath != path {
			t.Fatalf("path = %q, want %q", gotPath, path)
		}
		return &darwinPDFKitOutput{
			PageCount: 1,
			Pages: []darwinPDFKitPage{
				{Number: 1, Text: "Executive Summary"},
			},
		}, nil
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:            path,
		IncludePages:    true,
		IncludeMarkdown: true,
		IncludeLayout:   true,
		IncludeOutline:  true,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
	if len(result.Pages) != 1 {
		t.Fatalf("pages len = %d, want 1", len(result.Pages))
	}
	if len(result.Pages[0].Blocks) == 0 {
		t.Fatal("expected native layout blocks")
	}
}

func TestServiceExtractUsesNativeRenderForOCRFallbackOnDarwin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image-only.pdf")
	if err := os.WriteFile(path, buildImageOnlyPDF(t), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}
	runtimeDir := t.TempDir()
	runtimePath := filepath.Join(runtimeDir, pdfiumRuntimeFileName)
	if err := os.WriteFile(runtimePath, []byte("pdfium-runtime"), 0o644); err != nil {
		t.Fatalf("write fake runtime: %v", err)
	}

	ocr := &recordingOCRService{
		result: ocrResultText("Native OCR fallback"),
	}
	svc := NewService(zap.NewNop(), ocr, ServiceConfig{
		RuntimeDir:   runtimeDir,
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected darwin OCR fallback to avoid pdfium initialization")
		return nil, nil
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:         path,
		IncludePages: true,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !result.OCRUsed {
		t.Fatal("expected OCRUsed=true for image-only PDF")
	}
	if !bytes.Contains([]byte(result.Text), []byte("Native OCR fallback")) {
		t.Fatalf("text = %q, want OCR fallback text", result.Text)
	}
	if len(ocr.images) != 1 {
		t.Fatalf("ocr images = %d, want 1", len(ocr.images))
	}
	if _, err := png.Decode(bytes.NewReader(ocr.images[0])); err != nil {
		t.Fatalf("expected OCR input to be valid PNG: %v", err)
	}
}

func TestRunDarwinPDFKitExtractGoogleCXXGuideFixtureShowsMalformedRawText(t *testing.T) {
	output, ok, err := runDarwinPDFKitExtract(context.Background(), filepath.Join("testdata", "Google_C++_Guide.pdf"))
	if err != nil {
		t.Fatalf("runDarwinPDFKitExtract returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected native PDF extractor to be available on darwin")
	}
	if output == nil || len(output.Pages) < 2 {
		t.Fatalf("output = %#v, want at least 2 pages", output)
	}

	pageText := output.Pages[1].Text
	if !strings.Contains(pageText, "智能挃针和其他 C++特性") {
		t.Fatalf("page text = %q, want malformed raw native phrase", pageText)
	}
	if !strings.Contains(pageText, "觃则乊例外") {
		t.Fatalf("page text = %q, want malformed raw native section title", pageText)
	}

	bodyPageText := output.Pages[19].Text
	if !strings.Contains(bodyPageText, "智能指针和其他 C++特性") {
		t.Fatalf("body page text = %q, want correct section heading on later native page", bodyPageText)
	}
	if !strings.Contains(bodyPageText, "如果确实需要使用智能挃针的话") {
		t.Fatalf("body page text = %q, want malformed body phrase showing mixed unicode mapping", bodyPageText)
	}
}

func TestServiceExtractGoogleCXXGuideChineseFixtureRepairsMalformedNativeText(t *testing.T) {
	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   t.TempDir(),
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected lightweight darwin extraction to use native PDFKit without pdfium initialization")
		return nil, nil
	}

	result, err := svc.Extract(context.Background(), ExtractRequest{
		Path:          filepath.Join("testdata", "Google_C++_Guide.pdf"),
		Pages:         []int{2},
		IncludePages:  true,
		DisableOCR:    true,
		DisableVision: true,
	})
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result.Document.Engine != "pdfkit/native" {
		t.Fatalf("engine = %q, want %q", result.Document.Engine, "pdfkit/native")
	}
	if len(result.Pages) != 1 {
		t.Fatalf("pages len = %d, want 1", len(result.Pages))
	}

	pageText := result.Pages[0].Text
	if !strings.Contains(pageText, "智能指针和其他 C++特性") {
		t.Fatalf("page text = %q, want repaired phrase from native extraction", pageText)
	}
	if strings.Contains(pageText, "智能挃针和其他 C++特性") {
		t.Fatalf("page text = %q, expected malformed PDF text fallback to repair current garbled phrase", pageText)
	}
	if !strings.Contains(pageText, "规则之例外") {
		t.Fatalf("page text = %q, want repaired section title from native extraction", pageText)
	}
	if strings.Contains(pageText, "觃则乊例外") {
		t.Fatalf("page text = %q, expected malformed PDF text fallback to repair current garbled section title", pageText)
	}
}

type recordingOCRService struct {
	images [][]byte
	result ocrruntime.Result
}

func (s *recordingOCRService) Extract(ctx context.Context, imagePNG []byte) (ocrruntime.Result, error) {
	if err := ctx.Err(); err != nil {
		return ocrruntime.Result{}, err
	}
	s.images = append(s.images, append([]byte(nil), imagePNG...))
	return s.result, nil
}

func ocrResultText(text string) ocrruntime.Result {
	return ocrruntime.Result{
		Text:   text,
		Engine: "vision/native",
		Model:  "test",
	}
}

func buildImageOnlyPDF(t *testing.T) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(24 * x), G: uint8(24 * y), B: 180, A: 255})
		}
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode image fixture: %v", err)
	}
	_ = pngBuf

	rawPixels := make([]byte, 0, 8*8*3)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			rawPixels = append(rawPixels, byte(r>>8), byte(g>>8), byte(b>>8))
		}
	}
	var imageStream bytes.Buffer
	zw := zlib.NewWriter(&imageStream)
	if _, err := zw.Write(rawPixels); err != nil {
		t.Fatalf("compress image fixture: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close image compressor: %v", err)
	}
	imageBytes := imageStream.Bytes()

	contents := "q\n144 0 0 144 0 0 cm\n/Im0 Do\nQ\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 144 144] /Resources << /XObject << /Im0 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(contents), contents),
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width 8 /Height 8 /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /FlateDecode /Length %d >>\nstream\n%sendstream", len(imageBytes), imageBytes),
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
