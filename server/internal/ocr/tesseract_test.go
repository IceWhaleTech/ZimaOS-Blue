//go:build !darwin

package ocr

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danlock/gogosseract"
)

type fakePool struct {
	text   string
	err    error
	closed bool
}

func (p *fakePool) ParseImage(ctx context.Context, img io.Reader, opts gogosseract.ParseImageOptions) (string, error) {
	_, _ = ctx, opts
	_, _ = io.ReadAll(img)
	return p.text, p.err
}

func (p *fakePool) Close() { p.closed = true }

func TestTesseractServiceExtractPrefersBetterText(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"eng.traineddata", "chi_sim.traineddata", tesseractRuntimeFileName} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("model"), 0o644); err != nil {
			t.Fatalf("write model: %v", err)
		}
	}
	svc := NewTesseractService(nil, Config{ModelDir: dir, AutoDownload: false, PreferredModels: []string{"eng", "chi_sim"}})
	pools := map[string]*fakePool{
		"eng":     {text: "hello world 123"},
		"chi_sim": {text: "! ? -"},
	}
	svc.newPool = func(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error) {
		_ = ctx
		_ = count
		return pools[cfg.Language], nil
	}

	result, err := svc.Extract(context.Background(), []byte("png-bytes"))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if result.Model != "eng" {
		t.Fatalf("model = %q, want %q", result.Model, "eng")
	}
	if result.Text != "hello world 123" {
		t.Fatalf("text = %q, want %q", result.Text, "hello world 123")
	}
}

func TestTesseractServiceExtractAutoDownloadsMissingModel(t *testing.T) {
	dir := t.TempDir()
	svc := NewTesseractService(nil, Config{ModelDir: dir, AutoDownload: true, PreferredModels: []string{"eng"}})
	downloaded := false
	svc.download = func(ctx context.Context, url, path string) error {
		_ = ctx
		_ = url
		downloaded = true
		return os.WriteFile(path, []byte("model"), 0o644)
	}
	svc.newPool = func(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error) {
		_ = ctx
		_ = count
		if !bytes.Equal(cfg.TrainingDataBytes, []byte("model")) {
			t.Fatalf("training data bytes = %q, want %q", string(cfg.TrainingDataBytes), "model")
		}
		return &fakePool{text: "downloaded works"}, nil
	}

	result, err := svc.Extract(context.Background(), []byte("png-bytes"))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !downloaded {
		t.Fatal("expected download to run")
	}
	if len(result.AutoDownloaded) != 1 || result.AutoDownloaded[0] != "eng" {
		t.Fatalf("auto_downloaded = %#v, want [eng]", result.AutoDownloaded)
	}
}

func TestTesseractRuntimeURLCandidates(t *testing.T) {
	got := tesseractRuntimeURLCandidates()
	want := []string{
		"https://raw.githubusercontent.com/danlock/gogosseract/0ad342167d77c5393aac6369e1f4bb36fec77482/internal/wasm/tesseract-core.wasm",
		"https://raw.gitmirror.com/danlock/gogosseract/0ad342167d77c5393aac6369e1f4bb36fec77482/internal/wasm/tesseract-core.wasm",
		"https://cdn.jsdelivr.net/gh/danlock/gogosseract@0ad342167d77c5393aac6369e1f4bb36fec77482/internal/wasm/tesseract-core.wasm",
		"https://ghproxy.com/https://raw.githubusercontent.com/danlock/gogosseract/0ad342167d77c5393aac6369e1f4bb36fec77482/internal/wasm/tesseract-core.wasm",
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

func TestTesseractServiceExtractRequiresRuntimeWhenAutoDownloadDisabled(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "eng.traineddata"), []byte("model"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}
	svc := NewTesseractService(nil, Config{ModelDir: dir, AutoDownload: false, PreferredModels: []string{"eng"}})
	svc.newPool = func(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error) {
		_ = ctx
		_ = count
		_ = cfg
		t.Fatal("expected missing runtime wasm to fail before pool initialization")
		return nil, nil
	}

	_, err := svc.Extract(context.Background(), []byte("png-bytes"))
	if err == nil {
		t.Fatal("expected Extract to fail when runtime wasm is missing")
	}
	if !strings.Contains(err.Error(), "missing OCR runtime") {
		t.Fatalf("error = %v, want missing OCR runtime", err)
	}
}

func TestTesseractServiceExtractAutoDownloadsMissingRuntime(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "eng.traineddata"), []byte("model"), 0o644); err != nil {
		t.Fatalf("write model: %v", err)
	}
	svc := NewTesseractService(nil, Config{ModelDir: dir, AutoDownload: true, PreferredModels: []string{"eng"}})
	downloaded := false
	svc.download = func(ctx context.Context, url, path string) error {
		_ = ctx
		if !strings.HasSuffix(path, "tesseract-core.wasm") {
			t.Fatalf("download path = %q, want runtime wasm path", path)
		}
		if !strings.Contains(url, "tesseract-core.wasm") {
			t.Fatalf("download url = %q, want runtime wasm source", url)
		}
		downloaded = true
		return os.WriteFile(path, []byte("runtime-wasm"), 0o644)
	}
	svc.newPool = func(ctx context.Context, count uint, cfg gogosseract.PoolConfig) (parsePool, error) {
		_ = ctx
		_ = count
		if !bytes.Equal(cfg.TrainingDataBytes, []byte("model")) {
			t.Fatalf("training data bytes = %q, want %q", string(cfg.TrainingDataBytes), "model")
		}
		if !bytes.Equal(cfg.WASMBytes, []byte("runtime-wasm")) {
			t.Fatalf("wasm bytes = %q, want %q", string(cfg.WASMBytes), "runtime-wasm")
		}
		return &fakePool{text: "downloaded runtime works"}, nil
	}

	result, err := svc.Extract(context.Background(), []byte("png-bytes"))
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if !downloaded {
		t.Fatal("expected runtime download to run")
	}
	if result.Model != "eng" {
		t.Fatalf("model = %q, want %q", result.Model, "eng")
	}
}
