package sandbox

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexBinaryManagerDownloadSourcesPreferBlueHostedAssets(t *testing.T) {
	manager := NewCodexBinaryManager(DefaultConfig(), CodexBinaryManagerOptions{
		GOOS:   "windows",
		GOARCH: "amd64",
	})

	sources := manager.downloadSources()
	if len(sources) != 6 {
		t.Fatalf("downloadSources() = %d sources, want 6", len(sources))
	}

	wantPrefixes := []string{
		"https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download",
		"https://mirror.ghproxy.com/https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download",
		"https://kkgithub.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download",
		"https://github.com/openai/codex/releases/latest/download",
		"https://mirror.ghproxy.com/https://github.com/openai/codex/releases/latest/download",
		"https://kkgithub.com/openai/codex/releases/latest/download",
	}
	for i, want := range wantPrefixes {
		if sources[i] != want {
			t.Fatalf("downloadSources()[%d] = %q, want %q", i, sources[i], want)
		}
	}
}

func TestCodexBinaryAssetNameForWindowsTargets(t *testing.T) {
	tests := []struct {
		goarch string
		want   string
	}{
		{goarch: "amd64", want: "codex-x86_64-pc-windows-msvc.exe"},
		{goarch: "arm64", want: "codex-aarch64-pc-windows-msvc.exe"},
	}

	for _, tt := range tests {
		got, err := codexBinaryAssetName("windows", tt.goarch)
		if err != nil {
			t.Fatalf("codexBinaryAssetName(%q) error = %v", tt.goarch, err)
		}
		if got != tt.want {
			t.Fatalf("codexBinaryAssetName(%q) = %q, want %q", tt.goarch, got, tt.want)
		}
	}
}

func TestCodexBinaryManagerEnsureDownloadsToManagedCache(t *testing.T) {
	cacheDir := t.TempDir()
	payload := []byte("codex-binary")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/codex-x86_64-pc-windows-msvc.exe") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = true

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{
		GOOS:         "windows",
		GOARCH:       "amd64",
		UserCacheDir: func() (string, error) { return cacheDir, nil },
		LookPath:     func(string) (string, error) { return "", os.ErrNotExist },
		HTTPClient:   server.Client(),
		DownloadSources: []string{
			server.URL + "/blue",
		},
	})

	got, err := manager.Ensure(context.Background())
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if filepath.Base(got) != "codex.exe" {
		t.Fatalf("Ensure() path = %q, want managed codex.exe path", got)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", got, err)
	}
	if string(data) != string(payload) {
		t.Fatalf("downloaded payload = %q, want %q", string(data), string(payload))
	}
}

func TestCodexBinaryManagerReadyPathUsesManagedCacheBeforeDownload(t *testing.T) {
	cacheDir := t.TempDir()
	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = true

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{
		GOOS:         "windows",
		GOARCH:       "amd64",
		UserCacheDir: func() (string, error) { return cacheDir, nil },
		LookPath:     func(string) (string, error) { return "", os.ErrNotExist },
	})

	targetPath, err := manager.targetPathForCurrentPlatform()
	if err != nil {
		t.Fatalf("targetPathForCurrentPlatform() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(targetPath, []byte("cached"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, ok, err := manager.ReadyPath()
	if err != nil {
		t.Fatalf("ReadyPath() error = %v", err)
	}
	if !ok {
		t.Fatal("ReadyPath() = not ready, want cached managed binary")
	}
	if got != targetPath {
		t.Fatalf("ReadyPath() = %q, want %q", got, targetPath)
	}
}

func TestCodexBinaryManagerEnsureFailsWhenAutoDownloadDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowsCodexExecutable = ""
	cfg.WindowsCodexAutoDownload = false

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{
		GOOS:         "windows",
		GOARCH:       "amd64",
		UserCacheDir: func() (string, error) { return t.TempDir(), nil },
		LookPath:     func(string) (string, error) { return "", os.ErrNotExist },
	})

	_, err := manager.Ensure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "auto_download is disabled") {
		t.Fatalf("Ensure() error = %v, want auto_download disabled error", err)
	}
}

func TestCodexBinaryManagerDownloadTimeoutUsesConfiguredValue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowsCodexDownloadTimeout = 9 * time.Minute

	manager := NewCodexBinaryManager(cfg, CodexBinaryManagerOptions{})
	if got := manager.DownloadTimeout(); got != 9*time.Minute {
		t.Fatalf("DownloadTimeout() = %v, want %v", got, 9*time.Minute)
	}
}

func TestInstallDownloadedCodexBinaryOverwritesDestination(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "codex.exe")
	if err := os.WriteFile(dest, []byte("old"), 0644); err != nil {
		t.Fatalf("WriteFile(old) error = %v", err)
	}

	if err := installDownloadedCodexBinary(dest, io.NopCloser(strings.NewReader("new"))); err != nil {
		t.Fatalf("installDownloadedCodexBinary() error = %v", err)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", dest, err)
	}
	if string(data) != "new" {
		t.Fatalf("destination payload = %q, want %q", string(data), "new")
	}
}
