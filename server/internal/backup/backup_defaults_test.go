package backup

import (
	"path/filepath"
	"testing"
)

func TestNewManager_DefaultsSkillsPathToAgentsRoot(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	manager, err := NewManager(Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}, dataDir, configDir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	want := filepath.Join(dataDir, "workspace", ".agents", "skills")
	if manager.skillsDir != want {
		t.Fatalf("manager.skillsDir = %q, want %q", manager.skillsDir, want)
	}
}
