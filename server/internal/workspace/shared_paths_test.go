package workspace

import (
	"path/filepath"
	"testing"
)

func TestSharedScratchpadDir(t *testing.T) {
	workspaceDir := t.TempDir()

	got := SharedScratchpadDir(workspaceDir)
	want := filepath.Join(workspaceDir, ".blue", "scratchpad", "shared")
	if got != want {
		t.Fatalf("SharedScratchpadDir() = %q, want %q", got, want)
	}
}

func TestSharedScratchpadRelPath(t *testing.T) {
	if got := SharedScratchpadRelPath(); got != ".blue/scratchpad/shared" {
		t.Fatalf("SharedScratchpadRelPath() = %q, want %q", got, ".blue/scratchpad/shared")
	}
}
