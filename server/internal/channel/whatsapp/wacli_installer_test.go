package whatsapp

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestChannel_ResolveCLIPath_InstallsOnDemand(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	ch := New(DefaultConfig(), zap.NewNop())
	expected := filepath.Join(t.TempDir(), wacliBinaryName())
	called := false
	ch.ensureCLI = func(ctx context.Context) (string, error) {
		called = true
		return expected, nil
	}

	resolved, err := ch.resolveCLIPath(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("expected installer to be called")
	}
	if resolved != expected {
		t.Fatalf("resolved path = %q, want %q", resolved, expected)
	}
}

func TestChannel_ResolveCLIPath_UsesCachedBinaryBeforeInstall(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	cached, err := cachedWACLIPath()
	if err != nil {
		t.Fatalf("cachedWACLIPath error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(cached, []byte("cached"), 0o755); err != nil {
		t.Fatalf("write cached binary: %v", err)
	}

	ch := New(DefaultConfig(), zap.NewNop())
	called := false
	ch.ensureCLI = func(ctx context.Context) (string, error) {
		called = true
		return "", nil
	}

	resolved, err := ch.resolveCLIPath(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if called {
		t.Fatal("expected installer to be skipped when cached binary exists")
	}
	if resolved != cached {
		t.Fatalf("resolved path = %q, want %q", resolved, cached)
	}
}

func TestChannel_ResolveCLIPath_ExplicitPathDoesNotAutoInstall(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CLIPath = filepath.Join(t.TempDir(), "missing-wacli")
	ch := New(cfg, zap.NewNop())
	called := false
	ch.ensureCLI = func(ctx context.Context) (string, error) {
		called = true
		return "", nil
	}

	_, err := ch.resolveCLIPath(context.Background())
	if err == nil {
		t.Fatal("expected explicit missing cli_path to fail")
	}
	if called {
		t.Fatal("expected installer not to run for explicit cli_path")
	}
}

func TestValidator_Validate_AllowsAutoInstallWhenCLIPathUnset(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	v := NewValidator()
	result := v.Validate(context.Background(), map[string]string{
		"phone_number": "+1807890",
		"session_path": t.TempDir(),
	})
	if !result.Success {
		t.Fatalf("expected success when cli_path is unset, got %#v", result)
	}
}

func TestExtractTarGZBinary(t *testing.T) {
	archive, err := buildTarGZArchive(map[string][]byte{
		"README.md":                  []byte("readme"),
		"wacli/" + wacliBinaryName(): []byte("tar-binary"),
	})
	if err != nil {
		t.Fatalf("build tar.gz archive: %v", err)
	}

	dest := filepath.Join(t.TempDir(), wacliBinaryName())
	if err := extractTarGZBinary(bytes.NewReader(archive), dest, wacliBinaryName()); err != nil {
		t.Fatalf("extractTarGZBinary error = %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "tar-binary" {
		t.Fatalf("extracted data = %q, want %q", string(data), "tar-binary")
	}
}

func TestExtractZipBinary(t *testing.T) {
	archive, err := buildZipArchive(map[string][]byte{
		"README.md":                  []byte("readme"),
		"wacli/" + wacliBinaryName(): []byte("zip-binary"),
	})
	if err != nil {
		t.Fatalf("build zip archive: %v", err)
	}

	dest := filepath.Join(t.TempDir(), wacliBinaryName())
	if err := extractZipBinary(bytes.NewReader(archive), dest, wacliBinaryName()); err != nil {
		t.Fatalf("extractZipBinary error = %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read extracted binary: %v", err)
	}
	if string(data) != "zip-binary" {
		t.Fatalf("extracted data = %q, want %q", string(data), "zip-binary")
	}
}

func buildTarGZArchive(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildZipArchive(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		hdr := &zip.FileHeader{Name: name, Method: zip.Deflate}
		hdr.SetMode(0o755)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
