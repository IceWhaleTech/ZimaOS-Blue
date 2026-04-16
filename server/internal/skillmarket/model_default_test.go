package skillmarket

import (
	"path/filepath"
	"testing"
)

func TestDefaultConfig_DefaultsActiveSkillsDirToAgentsRoot(t *testing.T) {
	dataDir := t.TempDir()

	cfg := DefaultConfig(dataDir, "")
	want := filepath.Join(dataDir, "workspace", ".agents", "skills")

	if cfg.ActiveSkillsDir != want {
		t.Fatalf("cfg.ActiveSkillsDir = %q, want %q", cfg.ActiveSkillsDir, want)
	}
}
