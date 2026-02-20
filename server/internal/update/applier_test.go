package update

import (
	"os"
	"path/filepath"
	"testing"
)

// mockExecutor records Exec calls without actually replacing the process.
type mockExecutor struct {
	called     bool
	binaryPath string
	args       []string
	env        []string
}

func (m *mockExecutor) Exec(binaryPath string, args []string, env []string) error {
	m.called = true
	m.binaryPath = binaryPath
	m.args = args
	m.env = env
	return nil
}

func TestApplier_PrepareAndReplace(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	if err := os.WriteFile(binaryPath, []byte("old-binary"), 0755); err != nil {
		t.Fatal(err)
	}
	updatePath := filepath.Join(dir, "update.bin")
	if err := os.WriteFile(updatePath, []byte("new-binary"), 0755); err != nil {
		t.Fatal(err)
	}

	a := NewApplier(binaryPath, dir, 3)
	if err := a.PrepareAndReplace(updatePath); err != nil {
		t.Fatalf("PrepareAndReplace failed: %v", err)
	}

	// Binary should be replaced
	data, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(data) != "new-binary" {
		t.Errorf("binary content = %q, want %q", string(data), "new-binary")
	}

	// Backup should exist
	entries, err := os.ReadDir(filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatalf("read backups dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no backup created")
	}

	// Backup should contain old binary
	backupData, _ := os.ReadFile(filepath.Join(dir, "backups", entries[0].Name()))
	if string(backupData) != "old-binary" {
		t.Errorf("backup content = %q, want %q", string(backupData), "old-binary")
	}
}

func TestApplier_Restart_WithMockExecutor(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	os.WriteFile(binaryPath, []byte("binary"), 0755)

	mock := &mockExecutor{}
	a := NewApplier(binaryPath, dir, 3)
	a.SetExecutor(mock)

	if err := a.Restart(); err != nil {
		t.Fatalf("Restart failed: %v", err)
	}

	if !mock.called {
		t.Error("executor was not called")
	}
	if mock.binaryPath != binaryPath {
		t.Errorf("binaryPath = %q, want %q", mock.binaryPath, binaryPath)
	}

	// Check BLUE_START_TIME is in env
	found := false
	for _, e := range mock.env {
		if len(e) > 16 && e[:16] == "BLUE_START_TIME=" {
			found = true
			break
		}
	}
	if !found {
		t.Error("BLUE_START_TIME not found in env")
	}
}

func TestApplier_Rollback(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	os.WriteFile(binaryPath, []byte("old-binary"), 0755)

	// Create a backup manually
	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "echo.1000.bak"), []byte("backup-binary"), 0755)

	// Replace binary with something else
	os.WriteFile(binaryPath, []byte("broken-binary"), 0755)

	a := NewApplier(binaryPath, dir, 3)
	if err := a.Rollback(); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	data, _ := os.ReadFile(binaryPath)
	if string(data) != "backup-binary" {
		t.Errorf("after rollback, binary = %q, want %q", string(data), "backup-binary")
	}
}

func TestApplier_Rollback_NoBackup(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	os.WriteFile(binaryPath, []byte("binary"), 0755)

	a := NewApplier(binaryPath, dir, 3)
	err := a.Rollback()
	if err == nil {
		t.Error("expected error when no backup available")
	}
}

func TestApplier_Apply_FullFlow(t *testing.T) {
	dir := t.TempDir()
	binaryPath := filepath.Join(dir, "blue")
	os.WriteFile(binaryPath, []byte("old"), 0755)
	updatePath := filepath.Join(dir, "update.bin")
	os.WriteFile(updatePath, []byte("new"), 0755)

	mock := &mockExecutor{}
	a := NewApplier(binaryPath, dir, 3)
	a.SetExecutor(mock)

	if err := a.Apply(updatePath); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Binary replaced
	data, _ := os.ReadFile(binaryPath)
	if string(data) != "new" {
		t.Errorf("binary = %q, want %q", string(data), "new")
	}

	// Executor called
	if !mock.called {
		t.Error("executor not called after Apply")
	}
}
