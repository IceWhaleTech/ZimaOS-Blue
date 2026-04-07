package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestApplyRuntimeDataDirDefaults_RebasesDefaultHarnessPathsToDataDir(t *testing.T) {
	cfg := &config.Config{}
	cfg.Harness = *config.DefaultHarnessConfig()

	dataDir := filepath.Join(t.TempDir(), "data")
	ApplyRuntimeDataDirDefaults(cfg, dataDir)

	if got, want := filepath.Clean(cfg.Harness.ArtifactRoot), filepath.Join(dataDir, "harness", "artifacts"); got != want {
		t.Fatalf("artifact root = %q, want %q", got, want)
	}
	if got, want := filepath.Clean(cfg.Harness.StorePath), filepath.Join(dataDir, "blue.db"); got != want {
		t.Fatalf("store path = %q, want %q", got, want)
	}
}

func TestApplyRuntimeDataDirDefaults_PreservesExplicitHarnessPaths(t *testing.T) {
	cfg := &config.Config{}
	cfg.Harness = *config.DefaultHarnessConfig()
	cfg.Harness.ArtifactRoot = filepath.Join(t.TempDir(), "custom-artifacts")
	cfg.Harness.StorePath = filepath.Join(t.TempDir(), "custom.db")

	dataDir := filepath.Join(t.TempDir(), "data")
	wantArtifactRoot := cfg.Harness.ArtifactRoot
	wantStorePath := cfg.Harness.StorePath

	ApplyRuntimeDataDirDefaults(cfg, dataDir)

	if cfg.Harness.ArtifactRoot != wantArtifactRoot {
		t.Fatalf("artifact root = %q, want %q", cfg.Harness.ArtifactRoot, wantArtifactRoot)
	}
	if cfg.Harness.StorePath != wantStorePath {
		t.Fatalf("store path = %q, want %q", cfg.Harness.StorePath, wantStorePath)
	}
}

func TestApplyRuntimeDataDirDefaults_PreservesNonDefaultRelativeHarnessPaths(t *testing.T) {
	cfg := &config.Config{}
	cfg.Harness = *config.DefaultHarnessConfig()
	cfg.Harness.ArtifactRoot = "./custom-artifacts"
	cfg.Harness.StorePath = "./custom.db"

	ApplyRuntimeDataDirDefaults(cfg, filepath.Join(t.TempDir(), "data"))

	if cfg.Harness.ArtifactRoot != "./custom-artifacts" {
		t.Fatalf("artifact root = %q, want %q", cfg.Harness.ArtifactRoot, "./custom-artifacts")
	}
	if cfg.Harness.StorePath != "./custom.db" {
		t.Fatalf("store path = %q, want %q", cfg.Harness.StorePath, "./custom.db")
	}
}
