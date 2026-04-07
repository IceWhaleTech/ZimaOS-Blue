package bootstrap

import (
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

var (
	defaultHarnessArtifactRoots = map[string]struct{}{
		filepath.Clean("./data/harness/artifacts"): {},
		filepath.Clean("data/harness/artifacts"):   {},
	}
	defaultHarnessStorePaths = map[string]struct{}{
		filepath.Clean("./data/blue.db"):    {},
		filepath.Clean("data/blue.db"):      {},
		filepath.Clean("./data/harness.db"): {},
		filepath.Clean("data/harness.db"):   {},
	}
)

// ApplyRuntimeDataDirDefaults aligns config defaults that still use legacy
// cwd-relative data paths with the runtime's resolved data directory.
// Explicit custom paths are preserved.
func ApplyRuntimeDataDirDefaults(cfg *config.Config, dataDir string) {
	if cfg == nil {
		return
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return
	}

	if shouldUseRuntimeDefaultHarnessArtifactRoot(cfg.Harness.ArtifactRoot) {
		cfg.Harness.ArtifactRoot = filepath.Join(dataDir, "harness", "artifacts")
	}
	if shouldUseRuntimeDefaultHarnessStorePath(cfg.Harness.StorePath) {
		cfg.Harness.StorePath = filepath.Join(dataDir, "blue.db")
	}
}

func shouldUseRuntimeDefaultHarnessArtifactRoot(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	_, ok := defaultHarnessArtifactRoots[filepath.Clean(trimmed)]
	return ok
}

func shouldUseRuntimeDefaultHarnessStorePath(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return true
	}
	_, ok := defaultHarnessStorePaths[filepath.Clean(trimmed)]
	return ok
}
