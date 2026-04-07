package smallmodel

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLlamaCppRuntimeArchiveNameDarwin(t *testing.T) {
	got, err := llamaCppRuntimeArchiveName("darwin", "arm64")
	if err != nil {
		t.Fatalf("llamaCppRuntimeArchiveName(darwin, arm64) error = %v", err)
	}
	if got != "llama-b8690-bin-macos-arm64.tar.gz" {
		t.Fatalf("archive name = %q, want macOS arm64 runtime archive", got)
	}

	got, err = llamaCppRuntimeArchiveName("darwin", "amd64")
	if err != nil {
		t.Fatalf("llamaCppRuntimeArchiveName(darwin, amd64) error = %v", err)
	}
	if got != "llama-b8690-bin-macos-x64.tar.gz" {
		t.Fatalf("archive name = %q, want macOS x64 runtime archive", got)
	}
}

func TestLlamaCppRuntimeDownloadURLsUseOfficialAndMirrors(t *testing.T) {
	urls := llamaCppRuntimeDownloadURLs("llama-b8690-bin-macos-arm64.tar.gz")
	if len(urls) != 3 {
		t.Fatalf("llamaCppRuntimeDownloadURLs() = %d urls, want 3", len(urls))
	}
	if !strings.Contains(urls[0], "github.com/ggml-org/llama.cpp/releases/download/b8690/") {
		t.Fatalf("url[0] = %q, want official GitHub release asset", urls[0])
	}
	if !strings.Contains(urls[1], "mirror.ghproxy.com/https://github.com/ggml-org/llama.cpp/releases/download/b8690/") {
		t.Fatalf("url[1] = %q, want ghproxy mirror", urls[1])
	}
	if !strings.Contains(urls[2], "kkgithub.com/ggml-org/llama.cpp/releases/download/b8690/") {
		t.Fatalf("url[2] = %q, want kkgithub mirror", urls[2])
	}
	for _, url := range urls {
		if strings.Contains(strings.ToLower(url), "jsdelivr") {
			t.Fatalf("download url %q should not use jsdelivr for release archives", url)
		}
	}
}

func TestExtractLlamaCppRuntimeArchive(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "llama-runtime.tar.gz")
	if err := writeTestLlamaCppRuntimeArchive(archivePath, map[string]string{
		"llama-b8690/llama-server":   "server",
		"llama-b8690/llama-cli":      "cli",
		"llama-b8690/libllama.dylib": "libllama",
		"llama-b8690/libmtmd.dylib":  "libmtmd",
		"llama-b8690/libggml.dylib":  "libggml",
		"llama-b8690/LICENSE":        "license",
	}); err != nil {
		t.Fatalf("writeTestLlamaCppRuntimeArchive() error = %v", err)
	}

	destDir := filepath.Join(tmpDir, "runtime")
	if err := extractLlamaCppRuntimeArchive(archivePath, destDir); err != nil {
		t.Fatalf("extractLlamaCppRuntimeArchive() error = %v", err)
	}

	for _, rel := range []string{
		"llama-server",
		"llama-cli",
		"libllama.dylib",
		"libmtmd.dylib",
		"libggml.dylib",
	} {
		if _, err := os.Stat(filepath.Join(destDir, rel)); err != nil {
			t.Fatalf("Stat(%q) error = %v", rel, err)
		}
	}
}

func TestLlamaCppRuntimeReadinessUsesDownloadedRuntimePackage(t *testing.T) {
	m := NewManager(t.TempDir())
	createReadyModelFiles(t, m)

	dataDir := filepath.Dir(filepath.Dir(m.ModelDir()))
	runtimeDir := llamaCppRuntimeInstallDir(dataDir, runtime.GOOS, runtime.GOARCH)
	if err := os.MkdirAll(runtimeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", runtimeDir, err)
	}

	serverName := "llama-server"
	cliName := "llama-cli"
	if runtime.GOOS == "windows" {
		serverName = "llama-server.exe"
		cliName = "llama-cli.exe"
	}
	for _, rel := range []string{
		serverName,
		cliName,
		libraryFilename(runtimeDir, "llama"),
	} {
		target := rel
		if filepath.IsAbs(rel) {
			target = rel
		} else {
			target = filepath.Join(runtimeDir, rel)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(target), err)
		}
		if err := os.WriteFile(target, []byte("ok"), 0o755); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", target, err)
		}
	}

	rt := NewLlamaCppRuntime(m)
	if got := rt.ReadinessReason(); got != "ready" {
		t.Fatalf("ReadinessReason() = %q, want ready when downloaded runtime package exists", got)
	}
	serverPath, err := rt.resolveServerBinary()
	if err != nil {
		t.Fatalf("resolveServerBinary() error = %v", err)
	}
	if serverPath != filepath.Join(runtimeDir, serverName) {
		t.Fatalf("resolveServerBinary() = %q, want %q", serverPath, filepath.Join(runtimeDir, serverName))
	}
}

func writeTestLlamaCppRuntimeArchive(path string, files map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o755,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}
