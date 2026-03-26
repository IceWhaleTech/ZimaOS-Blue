package browser

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLightpandaBinaryManagerUsesThreeNonJsdelivrSources(t *testing.T) {
	manager := NewLightpandaBinaryManager(DefaultConfig())
	for _, release := range []string{lightpandaReleaseNightly, lightpandaReleaseLatest} {
		sources := manager.downloadSources(release)
		if len(sources) != 3 {
			t.Fatalf("downloadSources(%q) = %d sources, want 3", release, len(sources))
		}
		for _, source := range sources {
			if strings.Contains(strings.ToLower(source), "jsdelivr") {
				t.Fatalf("download source %q should not use jsdelivr", source)
			}
		}
	}
}

func TestLightpandaAssetsForPlatformDarwinArm64PrefersNightlyBinary(t *testing.T) {
	assets, err := lightpandaAssetsForPlatform("darwin", "arm64")
	if err != nil {
		t.Fatalf("lightpandaAssetsForPlatform() error = %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("lightpandaAssetsForPlatform() = %d assets, want 2", len(assets))
	}
	if assets[0].Name != "lightpanda-aarch64-macos" || assets[0].Release != lightpandaReleaseNightly {
		t.Fatalf("first asset = %+v, want darwin nightly binary", assets[0])
	}
	if assets[1].Name != "lightpanda-darwin-arm64.tar.gz" || assets[1].Release != lightpandaReleaseLatest {
		t.Fatalf("second asset = %+v, want darwin legacy archive", assets[1])
	}
}

func TestLightpandaAssetsForPlatformLinuxAmd64PrefersNightlyBinary(t *testing.T) {
	assets, err := lightpandaAssetsForPlatform("linux", "amd64")
	if err != nil {
		t.Fatalf("lightpandaAssetsForPlatform() error = %v", err)
	}
	if len(assets) != 2 {
		t.Fatalf("lightpandaAssetsForPlatform() = %d assets, want 2", len(assets))
	}
	if assets[0].Name != "lightpanda-x86_64-linux" || assets[0].Release != lightpandaReleaseNightly {
		t.Fatalf("first asset = %+v, want linux nightly binary", assets[0])
	}
	if assets[1].Name != "lightpanda-linux-amd64.tar.gz" || assets[1].Release != lightpandaReleaseLatest {
		t.Fatalf("second asset = %+v, want linux legacy archive", assets[1])
	}
}

func TestDownloadTempPathPreservesArchiveSuffix(t *testing.T) {
	got := downloadTempPath(t.TempDir(), "lightpanda-darwin-arm64.tar.gz")
	if !strings.HasSuffix(got, ".tar.gz") {
		t.Fatalf("downloadTempPath() = %q, want suffix .tar.gz", got)
	}
}

func TestLightpandaBinaryManagerReadyPathUsesExplicitBinary(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Lightpanda.Enabled = true
	binaryPath := filepath.Join(t.TempDir(), "lightpanda")
	if err := os.WriteFile(binaryPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", binaryPath, err)
	}
	cfg.Lightpanda.BinaryPath = binaryPath

	manager := NewLightpandaBinaryManager(cfg)
	got, ok, err := manager.ReadyPath()
	if err != nil {
		t.Fatalf("ReadyPath() error = %v", err)
	}
	if !ok {
		t.Fatal("ReadyPath() reported binary unavailable, want ready")
	}
	if got != binaryPath {
		t.Fatalf("ReadyPath() = %q, want %q", got, binaryPath)
	}
}

func TestLightpandaServiceWarmBinaryUsesReadyPathWithoutDownload(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Lightpanda.Enabled = true
	binaryPath := filepath.Join(t.TempDir(), "lightpanda")
	if err := os.WriteFile(binaryPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", binaryPath, err)
	}
	cfg.Lightpanda.BinaryPath = binaryPath

	service := NewLightpandaService(cfg)
	got, err := service.WarmBinary(context.Background())
	if err != nil {
		t.Fatalf("WarmBinary() error = %v", err)
	}
	if got != binaryPath {
		t.Fatalf("WarmBinary() = %q, want %q", got, binaryPath)
	}
}

func TestLightpandaServiceWarmBinarySkipsDownloadWhenAutoDownloadDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Lightpanda.Enabled = true
	autoDownload := false
	cfg.Lightpanda.AutoDownload = &autoDownload
	// Use an explicit missing path so the test does not accidentally pick up a
	// developer's globally cached Lightpanda binary.
	cfg.Lightpanda.BinaryPath = filepath.Join(t.TempDir(), "missing-lightpanda")

	service := NewLightpandaService(cfg)
	got, err := service.WarmBinary(context.Background())
	if err != nil {
		t.Fatalf("WarmBinary() error = %v", err)
	}
	if got != "" {
		t.Fatalf("WarmBinary() = %q, want empty path when auto-download is disabled", got)
	}
}
