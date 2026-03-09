package smallmodel

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/loader"
)

func TestResolveLlamaCppSharedLibFileFromEnv(t *testing.T) {
	tmpDir := t.TempDir()
	libName := "llama_probe_test"
	libPath := loader.LibraryFilename(tmpDir, libName)
	if err := os.MkdirAll(filepath.Dir(libPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(libPath), err)
	}
	if err := os.WriteFile(libPath, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write %s: %v", libPath, err)
	}

	t.Setenv(smallModelLlamaLibDirEnv, tmpDir)
	t.Setenv(loader.EnvLibPath, "")
	t.Setenv(smallModelLlamaCompatLibDirEnv, "")

	gotDir, gotFile, err := resolveLlamaCppSharedLibFile(libName)
	if err != nil {
		t.Fatalf("resolveLlamaCppSharedLibFile() error = %v", err)
	}
	wantDir, _ := filepath.Abs(tmpDir)
	wantFile := loader.LibraryFilename(wantDir, libName)
	if gotDir != wantDir {
		t.Fatalf("dir = %q, want %q", gotDir, wantDir)
	}
	if gotFile != wantFile {
		t.Fatalf("file = %q, want %q", gotFile, wantFile)
	}
}

func TestResolveLlamaCppSharedLibFileMissing(t *testing.T) {
	t.Setenv(smallModelLlamaLibDirEnv, "")
	t.Setenv(loader.EnvLibPath, "")
	t.Setenv(smallModelLlamaCompatLibDirEnv, "")

	_, _, err := resolveLlamaCppSharedLibFile("llama_missing_probe_test")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), smallModelLlamaLibDirEnv) {
		t.Fatalf("error = %q, want mention %s", err.Error(), smallModelLlamaLibDirEnv)
	}
}
