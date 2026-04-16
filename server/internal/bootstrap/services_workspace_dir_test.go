package bootstrap

import (
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceSkillsDir_PrefersAgentsSkillsRoot(t *testing.T) {
	dataDir := t.TempDir()

	got := ResolveWorkspaceSkillsDir(dataDir, nil)
	want := filepath.Join(dataDir, "workspace", ".agents", "skills")

	if got != want {
		t.Fatalf("ResolveWorkspaceSkillsDir() = %q, want %q", got, want)
	}
}
