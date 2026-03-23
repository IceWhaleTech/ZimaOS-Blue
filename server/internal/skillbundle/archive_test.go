package skillbundle

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindArchiveInstallRootSupportsClaudeAndCompatibilitySkill(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "bundle", "writer")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	claudePath := filepath.Join(skillDir, "CLAUDE.md")
	content := []byte("---\nid: writer\nname: Writer\n---\n")
	if err := os.WriteFile(claudePath, content, 0o644); err != nil {
		t.Fatalf("WriteFile(CLAUDE.md) error = %v", err)
	}

	installRoot, skillFile, err := FindArchiveInstallRoot(root, "writer")
	if err != nil {
		t.Fatalf("FindArchiveInstallRoot() error = %v", err)
	}
	if installRoot != skillDir {
		t.Fatalf("installRoot = %q, want %q", installRoot, skillDir)
	}
	if skillFile != claudePath {
		t.Fatalf("skillFile = %q, want original CLAUDE.md path", skillFile)
	}
	got, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("ReadFile(SKILL.md) error = %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("SKILL.md compatibility content mismatch: %q", string(got))
	}
}
