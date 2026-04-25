//go:build !darwin

package pdf

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klippa-app/go-pdfium"
	"go.uber.org/zap"
)

func TestServiceInfoAndExtractTextPDF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.pdf")
	if err := os.WriteFile(path, buildTextPDF("Hello PDF"), 0o644); err != nil {
		t.Fatalf("write pdf fixture: %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   "testdata",
		AutoDownload: false,
	})
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
	if info.Engine != nativePDFEngineName() {
		t.Fatalf("engine = %q, want %q", info.Engine, nativePDFEngineName())
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
	if result.Document.Engine != nativePDFEngineName() {
		t.Fatalf("result engine = %q, want %q", result.Document.Engine, nativePDFEngineName())
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

type fakePDFiumPool struct{}

func (p *fakePDFiumPool) GetInstance(timeout time.Duration) (pdfium.Pdfium, error) {
	_ = timeout
	return nil, nil
}

func (p *fakePDFiumPool) Close() error { return nil }

func TestPDFiumRuntimeURLCandidates(t *testing.T) {
	got := pdfiumRuntimeURLCandidates()
	want := []string{
		"https://raw.githubusercontent.com/klippa-app/go-pdfium/852818152bff9c1e366b8737a0802481f3a80e0f/webassembly/pdfium.wasm",
		"https://raw.gitmirror.com/klippa-app/go-pdfium/852818152bff9c1e366b8737a0802481f3a80e0f/webassembly/pdfium.wasm",
		"https://cdn.jsdelivr.net/gh/klippa-app/go-pdfium@852818152bff9c1e366b8737a0802481f3a80e0f/webassembly/pdfium.wasm",
		"https://ghproxy.com/https://raw.githubusercontent.com/klippa-app/go-pdfium/852818152bff9c1e366b8737a0802481f3a80e0f/webassembly/pdfium.wasm",
	}
	if len(got) != len(want) {
		t.Fatalf("len(urls) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("urls[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestServiceEnsureReadyRequiresRuntimeWhenAutoDownloadDisabled(t *testing.T) {
	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   t.TempDir(),
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		_ = cfg
		t.Fatal("expected missing runtime to fail before pdfium init")
		return nil, nil
	}

	err := svc.ensureReady(context.Background())
	if err == nil {
		t.Fatal("expected ensureReady to fail when runtime wasm is missing")
	}
	if !strings.Contains(err.Error(), "missing PDF runtime") {
		t.Fatalf("error = %v, want missing PDF runtime", err)
	}
}

func TestServiceEnsureReadyResolvesRelativeRuntimeDirAtConstruction(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   "testdata",
		AutoDownload: false,
	})
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		if len(cfg.WASM) == 0 {
			t.Fatal("expected runtime wasm bytes to be loaded")
		}
		return &fakePDFiumPool{}, nil
	}

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	if err := svc.ensureReady(context.Background()); err != nil {
		t.Fatalf("ensureReady returned error after cwd change: %v", err)
	}
}

func TestServiceEnsureReadyAutoDownloadsMissingRuntime(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(zap.NewNop(), nil, ServiceConfig{
		RuntimeDir:   dir,
		AutoDownload: true,
	})
	downloaded := false
	svc.download = func(ctx context.Context, url, path string) error {
		_ = ctx
		downloaded = true
		if !strings.HasSuffix(path, pdfiumRuntimeFileName) {
			t.Fatalf("download path = %q, want runtime wasm path", path)
		}
		if !strings.Contains(url, "/webassembly/pdfium.wasm") {
			t.Fatalf("download url = %q, want pdfium runtime source", url)
		}
		return os.WriteFile(path, []byte("pdfium-runtime"), 0o644)
	}
	pool := &fakePDFiumPool{}
	svc.initPool = func(cfg pdfRuntimeConfig) (any, error) {
		if !bytes.Equal(cfg.WASM, []byte("pdfium-runtime")) {
			t.Fatalf("cfg.WASM = %q, want %q", string(cfg.WASM), "pdfium-runtime")
		}
		return pool, nil
	}

	if err := svc.ensureReady(context.Background()); err != nil {
		t.Fatalf("ensureReady returned error: %v", err)
	}
	if !downloaded {
		t.Fatal("expected runtime download to run")
	}
	if svc.pool != pool {
		t.Fatalf("pool = %#v, want %#v", svc.pool, pool)
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
