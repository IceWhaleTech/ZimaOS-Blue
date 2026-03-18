package embedding

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveVectorStoreDBPathAnchorsLegacyDefaultToDataDir(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data")

	got := ResolveVectorStoreDBPath(dataDir, "./data/memory.db")

	want := filepath.Join(dataDir, "memory.db")
	if got != want {
		t.Fatalf("ResolveVectorStoreDBPath() = %q, want %q", got, want)
	}
}

func TestPrepareSharedModelCacheMigratesLegacySkillMarketDir(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	model := DefaultCybertronModel

	legacyModelDir := filepath.Join(dataDir, "models", "skillmarket", filepath.FromSlash(model))
	if err := os.MkdirAll(legacyModelDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy model dir: %v", err)
	}
	legacyFile := filepath.Join(legacyModelDir, "config.json")
	if err := os.WriteFile(legacyFile, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("write legacy model file: %v", err)
	}

	modelsDir := PrepareSharedModelCache(dataDir, "./data/memory.db", model)
	sharedModelDir := filepath.Join(modelsDir, filepath.FromSlash(model))
	sharedFile := filepath.Join(sharedModelDir, "config.json")

	if _, err := os.Stat(sharedFile); err != nil {
		t.Fatalf("shared model file missing after migration: %v", err)
	}
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Fatalf("legacy model file still exists, err=%v", err)
	}
}
