package tools

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type staticRipgrepResolver struct {
	binary *ripgrepBinary
	err    error
}

func (r staticRipgrepResolver) Resolve(context.Context) (*ripgrepBinary, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.binary, nil
}

func TestRipgrepManagerDownloadURLs_DefaultMirrors(t *testing.T) {
	manager, err := NewRipgrepManager(config.ToolCallingRipgrepConfig{Enabled: true}, t.TempDir())
	if err != nil {
		t.Fatalf("NewRipgrepManager failed: %v", err)
	}

	entry := manager.manifest.Platforms["darwin/arm64"]
	urls := manager.downloadURLs(entry)
	if len(urls) != 4 {
		t.Fatalf("download url count = %d, want 4 (%v)", len(urls), urls)
	}

	githubURL := manager.githubReleaseURL(entry)
	if got := urls[0]; got != "https://mirror.ghproxy.com/"+githubURL {
		t.Fatalf("url[0] = %q, want mirror.ghproxy", got)
	}
	if got := urls[1]; got != "https://gh-proxy.com/"+githubURL {
		t.Fatalf("url[1] = %q, want gh-proxy", got)
	}
	if got := urls[2]; got != "https://downloads.sourceforge.net/project/ripgrep.mirror/"+manager.manifest.Version+"/"+entry.Asset {
		t.Fatalf("url[2] = %q, want sourceforge mirror", got)
	}
	if got := urls[3]; got != githubURL {
		t.Fatalf("url[3] = %q, want direct github", got)
	}
}

func TestRipgrepManagerResolvePrefersCachedBinary(t *testing.T) {
	manager, entry := newTestRipgrepManager(t, config.ToolCallingRipgrepConfig{
		Enabled:           true,
		AutoDownload:      false,
		AllowSystemBinary: false,
		CacheDir:          t.TempDir(),
	})

	binaryPath := manager.managedBinaryPath("darwin/arm64", entry)
	if err := os.MkdirAll(filepath.Dir(binaryPath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}
	if err := os.WriteFile(binaryPath, []byte("cached rg"), 0o755); err != nil {
		t.Fatalf("write cached rg: %v", err)
	}

	manager.execCommand = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		if binPath != binaryPath {
			t.Fatalf("validate path = %q, want %q", binPath, binaryPath)
		}
		return []byte("ripgrep 15.1.0\n"), 0, nil
	}

	bin, err := manager.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if bin.Source != "managed_cache" {
		t.Fatalf("source = %q, want managed_cache", bin.Source)
	}
	if bin.Path != binaryPath {
		t.Fatalf("path = %q, want %q", bin.Path, binaryPath)
	}
}

func TestRipgrepManagerResolveUsesSystemBinaryBeforeDownload(t *testing.T) {
	manager, _ := newTestRipgrepManager(t, config.ToolCallingRipgrepConfig{
		Enabled:           true,
		AutoDownload:      true,
		AllowSystemBinary: true,
		CacheDir:          t.TempDir(),
	})

	var hits atomic.Int32
	manager.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			hits.Add(1)
			return nil, errors.New("unexpected download")
		}),
	}
	manager.lookPath = func(name string) (string, error) {
		if name != "rg" {
			t.Fatalf("lookup name = %q, want rg", name)
		}
		return filepath.Join(t.TempDir(), "system-rg"), nil
	}
	manager.execCommand = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return []byte("ripgrep 14.1.0\n"), 0, nil
	}

	bin, err := manager.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if bin.Source != "system_path" {
		t.Fatalf("source = %q, want system_path", bin.Source)
	}
	if hits.Load() != 0 {
		t.Fatalf("download hits = %d, want 0", hits.Load())
	}
}

func TestRipgrepManagerResolveDownloadsAndCachesBinary(t *testing.T) {
	cacheDir := t.TempDir()
	manager, entry := newTestRipgrepManager(t, config.ToolCallingRipgrepConfig{
		Enabled:           true,
		AutoDownload:      true,
		AllowSystemBinary: false,
		CacheDir:          cacheDir,
	})

	archiveBytes := buildZipArchive(t, entry.BinaryPath, []byte("downloaded rg"))
	entry.Asset = "ripgrep-test.zip"
	entry.ArchiveType = "zip"
	entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(archiveBytes))
	manager.manifest.Platforms["darwin/arm64"] = entry

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/assets/"+entry.Asset {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(archiveBytes)
	}))
	defer server.Close()

	manager.cfg.MirrorBaseURLs = []string{server.URL + "/assets/{asset}"}
	manager.httpClient = server.Client()
	manager.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	manager.execCommand = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return []byte("ripgrep 15.1.0\n"), 0, nil
	}

	bin, err := manager.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}
	if bin.Source != "managed_download" {
		t.Fatalf("source = %q, want managed_download", bin.Source)
	}
	if hits.Load() != 1 {
		t.Fatalf("download hits = %d, want 1", hits.Load())
	}

	cached, err := manager.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve cached failed: %v", err)
	}
	if cached.Source != "managed_cache" {
		t.Fatalf("cached source = %q, want managed_cache", cached.Source)
	}
	if hits.Load() != 1 {
		t.Fatalf("download hits after cache = %d, want 1", hits.Load())
	}
}

func TestRipgrepManagerConcurrentDownloadDedupes(t *testing.T) {
	cacheDir := t.TempDir()
	manager, entry := newTestRipgrepManager(t, config.ToolCallingRipgrepConfig{
		Enabled:           true,
		AutoDownload:      true,
		AllowSystemBinary: false,
		CacheDir:          cacheDir,
	})

	archiveBytes := buildZipArchive(t, entry.BinaryPath, []byte("downloaded rg"))
	entry.Asset = "ripgrep-test.zip"
	entry.ArchiveType = "zip"
	entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(archiveBytes))
	manager.manifest.Platforms["darwin/arm64"] = entry

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(archiveBytes)
	}))
	defer server.Close()

	manager.cfg.MirrorBaseURLs = []string{server.URL + "/{asset}"}
	manager.httpClient = server.Client()
	manager.lookPath = func(string) (string, error) { return "", errors.New("missing") }
	manager.execCommand = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return []byte("ripgrep 15.1.0\n"), 0, nil
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.Resolve(context.Background())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Resolve failed: %v", err)
		}
	}
	if hits.Load() != 1 {
		t.Fatalf("download hits = %d, want 1", hits.Load())
	}
}

func TestGrepToolUsesRipgrepBackendWhenAvailable(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello there\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewGrepToolWithRipgrep([]string{tmpDir}, 0, staticRipgrepResolver{
		binary: &ripgrepBinary{Path: "/fake/rg", Source: "system_path"},
	})
	tool.ripgrepExec = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return []byte(`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hello there\n"},"line_number":1,"submatches":[{"start":0}]}}` + "\n"), 0, nil
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "hello",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if got, _ := payload["backend"].(string); got != "ripgrep" {
		t.Fatalf("backend = %q, want ripgrep", got)
	}
	if got, _ := payload["backend_source"].(string); got != "system_path" {
		t.Fatalf("backend_source = %q, want system_path", got)
	}
}

func TestGrepToolFallsBackToBuiltinWhenRipgrepFails(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("hello there\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewGrepToolWithRipgrep([]string{tmpDir}, 0, staticRipgrepResolver{
		binary: &ripgrepBinary{Path: "/fake/rg", Source: "system_path"},
	})
	tool.ripgrepExec = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return nil, 2, errors.New("boom")
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"pattern": "hello",
		"path":    ".",
	})
	if err != nil {
		t.Fatalf("grep failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if got, _ := payload["backend"].(string); got != "builtin" {
		t.Fatalf("backend = %q, want builtin", got)
	}
	if got, _ := payload["fallback_reason"].(string); !strings.Contains(got, "ripgrep execution failed") {
		t.Fatalf("fallback_reason = %q, want ripgrep failure", got)
	}
}

func TestFindToolUsesRipgrepBackendForFileSearch(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "sub", "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	tool := NewFindToolWithRipgrep([]string{tmpDir}, staticRipgrepResolver{
		binary: &ripgrepBinary{Path: "/fake/rg", Source: "system_path"},
	})
	tool.ripgrepExec = func(ctx context.Context, binPath string, args []string, dir string) ([]byte, int, error) {
		return []byte("sub/note.txt\x00"), 0, nil
	}

	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"path":      ".",
		"pattern":   "*.txt",
		"type":      "file",
		"max_depth": 5,
	})
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(result.(string)), &payload); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if got, _ := payload["backend"].(string); got != "ripgrep" {
		t.Fatalf("backend = %q, want ripgrep", got)
	}
	if got, _ := payload["backend_source"].(string); got != "system_path" {
		t.Fatalf("backend_source = %q, want system_path", got)
	}
}

func newTestRipgrepManager(t *testing.T, cfg config.ToolCallingRipgrepConfig) (*RipgrepManager, ripgrepPlatformManifest) {
	t.Helper()

	manager, err := NewRipgrepManager(cfg, t.TempDir())
	if err != nil {
		t.Fatalf("NewRipgrepManager failed: %v", err)
	}
	entry := ripgrepPlatformManifest{
		Asset:       "ripgrep-test.zip",
		ArchiveType: "zip",
		SHA256:      "",
		BinaryPath:  "ripgrep-test/rg",
	}
	manager.manifest = ripgrepManifest{
		Version: "test-version",
		Platforms: map[string]ripgrepPlatformManifest{
			"darwin/arm64": entry,
		},
	}
	manager.platformKey = func() string { return "darwin/arm64" }
	return manager, entry
}

func buildZipArchive(t *testing.T, name string, content []byte) []byte {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), "ripgrep-test.zip")
	file, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	if _, err := entry.Write(content); err != nil {
		t.Fatalf("write zip entry: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip writer: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return data
}
