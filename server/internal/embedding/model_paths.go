package embedding

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveVectorStoreDBPath keeps vector-store DB path resolution consistent
// anywhere we need colocated embedding model caches.
func ResolveVectorStoreDBPath(dataDir, configuredPath string) string {
	trimmed := strings.TrimSpace(configuredPath)
	if trimmed == "" {
		return filepath.Join(dataDir, "memory.db")
	}
	// Preserve special sqlite DSNs and explicit absolute paths.
	if trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:") || filepath.IsAbs(trimmed) {
		return trimmed
	}
	// Keep backward compatibility with legacy default "./data/memory.db" while anchoring to app data dir.
	if filepath.Clean(trimmed) == filepath.Join("data", "memory.db") {
		return filepath.Join(dataDir, "memory.db")
	}
	return trimmed
}

// ResolveModelsDir returns the shared local-model cache directory used by
// cybertron-backed embedding features.
func ResolveModelsDir(dataDir, configuredVectorStoreDBPath string) string {
	dbPath := ResolveVectorStoreDBPath(dataDir, configuredVectorStoreDBPath)
	return filepath.Join(filepath.Dir(dbPath), "models")
}

// PrepareSharedModelCache returns the unified embedding model cache directory
// and best-effort migrates the legacy skill-market cache into that location.
func PrepareSharedModelCache(dataDir, configuredVectorStoreDBPath, model string) string {
	modelsDir := ResolveModelsDir(dataDir, configuredVectorStoreDBPath)
	adoptLegacySkillMarketModelDir(dataDir, modelsDir, model)
	return modelsDir
}

func adoptLegacySkillMarketModelDir(dataDir, sharedModelsDir, model string) {
	if strings.TrimSpace(dataDir) == "" {
		return
	}
	model = strings.TrimSpace(model)
	if model == "" {
		model = DefaultCybertronModel
	}

	legacyModelDir := filepath.Join(dataDir, "models", "skillmarket", filepath.FromSlash(model))
	sharedModelDir := filepath.Join(sharedModelsDir, filepath.FromSlash(model))
	if filepath.Clean(legacyModelDir) == filepath.Clean(sharedModelDir) {
		return
	}
	if _, err := os.Stat(sharedModelDir); err == nil {
		return
	}
	if _, err := os.Stat(legacyModelDir); err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(sharedModelDir), 0o755); err != nil {
		return
	}
	if err := os.Rename(legacyModelDir, sharedModelDir); err != nil {
		return
	}

	// Best-effort cleanup for empty legacy cache parents.
	_ = os.Remove(filepath.Dir(legacyModelDir))
	_ = os.Remove(filepath.Join(dataDir, "models", "skillmarket"))
}
