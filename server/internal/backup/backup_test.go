package backup

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	if m == nil {
		t.Fatal("expected non-nil manager")
	}

	// Check backup directory was created
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("backup directory was not created")
	}
}

func TestManagerCreate(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	// Create test data
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test data"), 0644)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("test: config"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	tests := []struct {
		name       string
		backupType BackupType
	}{
		{"full backup", BackupTypeFull},
		{"config backup", BackupTypeConfig},
		{"data backup", BackupTypeData},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := m.Create(context.Background(), tt.backupType)
			if err != nil {
				t.Fatalf("failed to create backup: %v", err)
			}

			if info.ID == "" {
				t.Error("expected non-empty ID")
			}
			if info.SizeBytes <= 0 {
				t.Error("expected positive size")
			}
			if info.Checksum == "" {
				t.Error("expected non-empty checksum")
			}
			if info.Type != string(tt.backupType) {
				t.Errorf("expected type %s, got %s", tt.backupType, info.Type)
			}

			// Verify backup file exists
			if _, err := os.Stat(info.Path); os.IsNotExist(err) {
				t.Error("backup file was not created")
			}
		})
	}
}

func TestManagerList(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create multiple backups
	for i := 0; i < 3; i++ {
		_, err := m.Create(context.Background(), BackupTypeData)
		if err != nil {
			t.Fatalf("failed to create backup: %v", err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	backups := m.List()
	if len(backups) != 3 {
		t.Errorf("expected 3 backups, got %d", len(backups))
	}

	// Check sorted by newest first
	for i := 1; i < len(backups); i++ {
		if backups[i].CreatedAt.After(backups[i-1].CreatedAt) {
			t.Error("backups not sorted by newest first")
		}
	}
}

func TestManagerGet(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Get existing backup
	got, err := m.Get(info.ID)
	if err != nil {
		t.Fatalf("failed to get backup: %v", err)
	}
	if got.ID != info.ID {
		t.Errorf("expected ID %s, got %s", info.ID, got.ID)
	}

	// Get non-existent backup
	_, err = m.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent backup")
	}
}

func TestManagerDelete(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Delete backup
	if err := m.Delete(info.ID); err != nil {
		t.Fatalf("failed to delete backup: %v", err)
	}

	// Verify backup is gone
	if _, err := m.Get(info.ID); err == nil {
		t.Error("expected error after deletion")
	}

	// Verify file is gone
	if _, err := os.Stat(info.Path); !os.IsNotExist(err) {
		t.Error("backup file was not deleted")
	}

	// Delete non-existent backup
	if err := m.Delete("non-existent"); err == nil {
		t.Error("expected error for non-existent backup")
	}
}

func TestManagerVerify(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Verify valid backup
	if err := m.Verify(info.ID); err != nil {
		t.Errorf("verification failed: %v", err)
	}

	// Corrupt the backup
	os.WriteFile(info.Path, []byte("corrupted"), 0644)

	// Verify corrupted backup
	if err := m.Verify(info.ID); err == nil {
		t.Error("expected verification to fail for corrupted backup")
	}
}

func TestManagerRestore(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")
	restoreDataDir := filepath.Join(tmpDir, "restore_data")
	restoreConfigDir := filepath.Join(tmpDir, "restore_config")

	// Create test data
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(configDir, 0755)
	testContent := []byte("test data content")
	os.WriteFile(filepath.Join(dataDir, "test.db"), testContent, 0644)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte("test: config"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeFull)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Create new manager for restore with different directories
	m2, err := NewManager(cfg, restoreDataDir, restoreConfigDir)
	if err != nil {
		t.Fatalf("failed to create restore manager: %v", err)
	}

	// Copy backup info to new manager
	m2.mu.Lock()
	m2.backups[info.ID] = info
	m2.mu.Unlock()

	// Restore
	result, err := m2.Restore(context.Background(), info.ID, DefaultRestoreOptions())
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}

	if !result.Success {
		t.Errorf("restore not successful: %v", result.Errors)
	}
	if result.FilesRestored == 0 {
		t.Error("no files were restored")
	}

	// Verify restored data
	restoredContent, err := os.ReadFile(filepath.Join(restoreDataDir, "test.db"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restoredContent) != string(testContent) {
		t.Error("restored content does not match original")
	}
}

func TestManagerRestoreDryRun(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Clear data directory
	os.RemoveAll(dataDir)
	os.MkdirAll(dataDir, 0755)

	// Dry run restore
	opts := DefaultRestoreOptions()
	opts.DryRun = true

	result, err := m.Restore(context.Background(), info.ID, opts)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if result.FilesRestored == 0 {
		t.Error("dry run should report files that would be restored")
	}

	// Verify no files were actually restored
	entries, _ := os.ReadDir(dataDir)
	if len(entries) > 0 {
		t.Error("dry run should not restore any files")
	}
}

func TestManagerListFiles(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dataDir, "other.txt"), []byte("other"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	files, err := m.ListFiles(info.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	if len(files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(files))
	}
}

func TestManagerLoadBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)
	os.WriteFile(filepath.Join(dataDir, "test.db"), []byte("test"), 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	// Create first manager and backup
	m1, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m1.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Create second manager - should load existing backups
	m2, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create second manager: %v", err)
	}

	// Verify backup was loaded
	loaded, err := m2.Get(info.ID)
	if err != nil {
		t.Fatalf("failed to get loaded backup: %v", err)
	}
	if loaded.ID != info.ID {
		t.Errorf("expected ID %s, got %s", info.ID, loaded.ID)
	}
}

// TestBackupWithLargeFile tests backup with a large file to ensure LimitReader works correctly
func TestBackupWithLargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)

	// Create a larger test file (1MB) - use .dat extension since .bin is excluded
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	os.WriteFile(filepath.Join(dataDir, "large.dat"), largeData, 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create backup - should not fail with "write too long" error
	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// Verify backup
	if err := m.Verify(info.ID); err != nil {
		t.Errorf("backup verification failed: %v", err)
	}

	// Restore and verify content
	restoreDir := filepath.Join(tmpDir, "restore")
	os.MkdirAll(restoreDir, 0755)

	m2, err := NewManager(cfg, restoreDir, configDir)
	if err != nil {
		t.Fatalf("failed to create restore manager: %v", err)
	}
	m2.mu.Lock()
	m2.backups[info.ID] = info
	m2.mu.Unlock()

	result, err := m2.Restore(context.Background(), info.ID, DefaultRestoreOptions())
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !result.Success {
		t.Errorf("restore not successful: %v", result.Errors)
	}
	t.Logf("Restore result: files=%d, skipped=%d, errors=%v", result.FilesRestored, result.FilesSkipped, result.Errors)

	// Verify restored content matches original
	restoredData, err := os.ReadFile(filepath.Join(restoreDir, "large.dat"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if len(restoredData) != len(largeData) {
		t.Errorf("restored file size mismatch: expected %d, got %d", len(largeData), len(restoredData))
	}
	for i := range largeData {
		if restoredData[i] != largeData[i] {
			t.Errorf("content mismatch at byte %d", i)
			break
		}
	}
}

func TestManagerCreateSkipsNoisyAndLargeArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(filepath.Join(dataDir, "logs"), 0755); err != nil {
		t.Fatalf("failed to create logs dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "models"), 0755); err != nil {
		t.Fatalf("failed to create models dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(dataDir, "logs", "app.log"), []byte("log"), 0644); err != nil {
		t.Fatalf("failed to write log file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "models", "tiny.gguf"), []byte("model"), 0644); err != nil {
		t.Fatalf("failed to write model file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "keep.txt"), []byte("keep"), 0644); err != nil {
		t.Fatalf("failed to write keep file: %v", err)
	}

	largeBlob := make([]byte, 11*1024*1024)
	for i := range largeBlob {
		largeBlob[i] = byte(i % 251)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "generated.dat"), largeBlob, 0644); err != nil {
		t.Fatalf("failed to write large generated file: %v", err)
	}
	// Keep large database files even if they exceed threshold.
	if err := os.WriteFile(filepath.Join(dataDir, "state.db"), largeBlob, 0644); err != nil {
		t.Fatalf("failed to write large db file: %v", err)
	}

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	files, err := m.ListFiles(info.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	hasEntry := func(name string) bool {
		for _, f := range files {
			if filepath.ToSlash(f) == name {
				return true
			}
		}
		return false
	}

	if !hasEntry("data/keep.txt") {
		t.Fatalf("expected keep file in backup, files=%v", files)
	}
	if !hasEntry("data/state.db") {
		t.Fatalf("expected large db file in backup, files=%v", files)
	}
	if hasEntry("data/generated.dat") {
		t.Fatalf("expected large generated file to be excluded, files=%v", files)
	}
	if hasEntry("data/logs/app.log") {
		t.Fatalf("expected log file to be excluded, files=%v", files)
	}
	if hasEntry("data/models/tiny.gguf") {
		t.Fatalf("expected model file to be excluded, files=%v", files)
	}
}

// TestBackupFileSizeConsistency tests that backup handles file size correctly
// This is a regression test for the "archive/tar: write too long" bug
func TestBackupFileSizeConsistency(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	os.MkdirAll(dataDir, 0755)

	// Create multiple files of varying sizes - avoid .bin extension which is excluded
	testFiles := map[string]int{
		"small.txt":  100,
		"medium.dat": 10 * 1024,
		"large.dat":  100 * 1024,
	}

	for name, size := range testFiles {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		if err := os.WriteFile(filepath.Join(dataDir, name), data, 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", name, err)
		}
	}

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	// Create backup
	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	// List files in backup
	files, err := m.ListFiles(info.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	// Verify all test files are in backup
	fileCount := 0
	for _, f := range files {
		for name := range testFiles {
			if filepath.Base(f) == name {
				fileCount++
				break
			}
		}
	}
	if fileCount != len(testFiles) {
		t.Errorf("expected %d test files in backup, found %d", len(testFiles), fileCount)
	}

	// Verify backup integrity
	if err := m.Verify(info.ID); err != nil {
		t.Errorf("backup verification failed: %v", err)
	}
}

func TestManagerCreateExcludesGitMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(filepath.Join(dataDir, "workspace", ".git"), 0o755); err != nil {
		t.Fatalf("mkdir repo .git dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "linked-worktree"), 0o755); err != nil {
		t.Fatalf("mkdir linked-worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "workspace", ".git", "config"), []byte("[core]\nrepositoryformatversion = 0\n"), 0o644); err != nil {
		t.Fatalf("write repo config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "linked-worktree", ".git"), []byte("gitdir: /tmp/example.git\n"), 0o644); err != nil {
		t.Fatalf("write worktree .git file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "linked-worktree", "USER.md"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	m, err := NewManager(Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	files, err := m.ListFiles(info.ID)
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}

	for _, name := range files {
		slashed := filepath.ToSlash(name)
		if strings.Contains(slashed, "/.git/") || strings.HasSuffix(slashed, "/.git") {
			t.Fatalf("expected git metadata to be excluded, got file %q in backup", slashed)
		}
	}

	foundWorkspaceFile := false
	for _, name := range files {
		if filepath.ToSlash(name) == "data/linked-worktree/USER.md" {
			foundWorkspaceFile = true
			break
		}
	}
	if !foundWorkspaceFile {
		t.Fatalf("expected regular workspace file to remain in backup, files=%v", files)
	}
}

func TestManagerBackupAndRestoreExternalSkillsDir(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")
	skillsDir := filepath.Join(tmpDir, "skills")

	os.MkdirAll(filepath.Join(dataDir, "nested"), 0755)
	os.MkdirAll(filepath.Join(skillsDir, "custom-skill"), 0755)
	os.WriteFile(filepath.Join(dataDir, "nested", "test.db"), []byte("test"), 0644)
	skillContent := []byte("name: custom-skill\n")
	os.WriteFile(filepath.Join(skillsDir, "custom-skill", "SKILL.md"), skillContent, 0644)

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
		SkillsPath:    skillsDir,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	info, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	foundSkillsEntry := false
	for _, f := range info.Files {
		if strings.HasPrefix(filepath.ToSlash(f), "skills/") {
			foundSkillsEntry = true
			break
		}
	}
	if !foundSkillsEntry {
		t.Fatalf("expected backup file list to include skills entries, got %v", info.Files)
	}

	restoreDataDir := filepath.Join(tmpDir, "restore-data")
	restoreConfigDir := filepath.Join(tmpDir, "restore-config")
	restoreSkillsDir := filepath.Join(tmpDir, "restore-skills")

	restoreCfg := cfg
	restoreCfg.SkillsPath = restoreSkillsDir
	m2, err := NewManager(restoreCfg, restoreDataDir, restoreConfigDir)
	if err != nil {
		t.Fatalf("failed to create restore manager: %v", err)
	}
	m2.mu.Lock()
	m2.backups[info.ID] = info
	m2.mu.Unlock()

	result, err := m2.Restore(context.Background(), info.ID, DefaultRestoreOptions())
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if !result.Success {
		t.Fatalf("restore not successful: %v", result.Errors)
	}

	restoredSkillPath := filepath.Join(restoreSkillsDir, "custom-skill", "SKILL.md")
	restoredSkill, err := os.ReadFile(restoredSkillPath)
	if err != nil {
		t.Fatalf("failed to read restored skill file: %v", err)
	}
	if string(restoredSkill) != string(skillContent) {
		t.Fatalf("restored skill content mismatch: got %q want %q", string(restoredSkill), string(skillContent))
	}
}

func TestManagerAutoBackupOnChange(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "initial.txt"), []byte("initial"), 0644); err != nil {
		t.Fatalf("failed to write initial data: %v", err)
	}

	cfg := Config{
		Enabled:            true,
		RetentionDays:      7,
		Path:               backupDir,
		AutoBackupOnChange: true,
		ChangePollInterval: 50 * time.Millisecond,
		ChangeDebounce:     0,
	}

	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.StartAutoBackup(ctx)
	defer m.StopAutoBackup()

	// Let watcher capture the initial snapshot.
	time.Sleep(120 * time.Millisecond)

	if err := os.WriteFile(filepath.Join(dataDir, "changed.txt"), []byte("changed"), 0644); err != nil {
		t.Fatalf("failed to write changed file: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(m.List()) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("expected auto-backup to create at least one backup after change")
}

func TestManagerAutoBackupKeepsOnlyLatestVersion(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("failed to write state file: %v", err)
	}

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}
	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	manual, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create manual backup: %v", err)
	}

	m.runAutoBackup(context.Background())
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v2"), 0644); err != nil {
		t.Fatalf("failed to update state file: %v", err)
	}
	m.runAutoBackup(context.Background())

	backups := m.List()
	autoCount := 0
	manualCount := 0
	for _, b := range backups {
		switch b.CreatedBy {
		case string(BackupSourceAuto):
			autoCount++
		case string(BackupSourceManual):
			manualCount++
		}
	}

	if autoCount != 1 {
		t.Fatalf("expected exactly 1 auto backup, got %d", autoCount)
	}
	if manualCount != 1 {
		t.Fatalf("expected manual backup to be retained, got %d", manualCount)
	}

	storedManual, err := m.Get(manual.ID)
	if err != nil {
		t.Fatalf("failed to get manual backup: %v", err)
	}
	if storedManual.CreatedBy != string(BackupSourceManual) {
		t.Fatalf("expected manual backup created_by=%q, got %q", BackupSourceManual, storedManual.CreatedBy)
	}
}

func TestManagerRestoreCreatesCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("failed to write v1: %v", err)
	}

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}
	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	backupV1, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create v1 backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v2"), 0644); err != nil {
		t.Fatalf("failed to write v2: %v", err)
	}

	result, err := m.Restore(context.Background(), backupV1.ID, DefaultRestoreOptions())
	if err != nil {
		t.Fatalf("restore failed: %v", err)
	}
	if result.CheckpointID == "" {
		t.Fatal("expected checkpoint id in restore result")
	}

	checkpointInfo, err := m.Get(result.CheckpointID)
	if err != nil {
		t.Fatalf("failed to load checkpoint backup info: %v", err)
	}
	if !checkpointInfo.IsCheckpoint {
		t.Fatal("expected checkpoint backup to be marked as checkpoint")
	}
	if checkpointInfo.CheckpointReason == "" {
		t.Fatal("expected checkpoint reason to be set")
	}
	if checkpointInfo.CreatedBy != string(BackupSourceCheckpoint) {
		t.Fatalf("expected checkpoint created_by=%q, got %q", BackupSourceCheckpoint, checkpointInfo.CreatedBy)
	}

	restored, err := os.ReadFile(filepath.Join(dataDir, "state.txt"))
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restored) != "v1" {
		t.Fatalf("expected restored data to be v1, got %q", string(restored))
	}
}

func TestManagerApplyPendingRestoreCreatesCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v1"), 0644); err != nil {
		t.Fatalf("failed to write v1: %v", err)
	}

	cfg := Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}
	m, err := NewManager(cfg, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	backupV1, err := m.Create(context.Background(), BackupTypeData)
	if err != nil {
		t.Fatalf("failed to create v1 backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "state.txt"), []byte("v2"), 0644); err != nil {
		t.Fatalf("failed to write v2: %v", err)
	}

	if _, err := m.StageRestore(context.Background(), backupV1.ID); err != nil {
		t.Fatalf("failed to stage restore: %v", err)
	}

	result, err := m.ApplyPendingRestore(context.Background())
	if err != nil {
		t.Fatalf("apply pending restore failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil restore result")
	}
	if result.CheckpointID == "" {
		t.Fatal("expected checkpoint id in pending restore result")
	}
	if m.HasPendingRestore() {
		t.Fatal("expected pending restore marker to be cleared")
	}

	restored, err := os.ReadFile(filepath.Join(dataDir, "state.txt"))
	if err != nil {
		t.Fatalf("failed to read restored data: %v", err)
	}
	if string(restored) != "v1" {
		t.Fatalf("expected restored data to be v1, got %q", string(restored))
	}
}

func TestDiscoverSQLiteDatabasePathsIncludesAdditionalDatabases(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "nested"), 0755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	files := []string{
		"blue.db",
		"session_audit.db",
		"memory.sqlite",
		"metrics.sqlite3",
		"notes.txt",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, name), []byte(name), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "nested", "ignored.db"), []byte("nested"), 0644); err != nil {
		t.Fatalf("failed to create nested db: %v", err)
	}

	paths, err := DiscoverSQLiteDatabasePaths(tmpDir)
	if err != nil {
		t.Fatalf("failed to discover database paths: %v", err)
	}

	want := []string{
		filepath.Join(tmpDir, "blue.db"),
		filepath.Join(tmpDir, "memory.sqlite"),
		filepath.Join(tmpDir, "metrics.sqlite3"),
		filepath.Join(tmpDir, "session_audit.db"),
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("unexpected database paths: got %v want %v", paths, want)
	}
}

func TestCheckAndAutoRecoverRestoresAdditionalDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	blueDB := filepath.Join(dataDir, "blue.db")
	auditDB := filepath.Join(dataDir, "session_audit.db")
	writeSQLiteValue(t, blueDB, "blue-v1")
	writeSQLiteValue(t, auditDB, "audit-v1")

	m, err := NewManager(Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	if _, err := m.Create(context.Background(), BackupTypeData); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	writeSQLiteValue(t, blueDB, "blue-v2")
	if err := os.WriteFile(auditDB, []byte("not a sqlite database"), 0644); err != nil {
		t.Fatalf("failed to corrupt audit db: %v", err)
	}

	dbPaths, err := DiscoverSQLiteDatabasePaths(dataDir)
	if err != nil {
		t.Fatalf("failed to discover database paths: %v", err)
	}

	result, err := m.CheckAndAutoRecover(context.Background(), dbPaths)
	if err != nil {
		t.Fatalf("auto recovery failed: %v", err)
	}
	if result == nil || !result.Recovered {
		t.Fatalf("expected recovery result, got %+v", result)
	}
	if !containsString(result.CorruptedDatabases, auditDB) {
		t.Fatalf("expected corrupted databases to include %s, got %v", auditDB, result.CorruptedDatabases)
	}

	if got := readSQLiteValue(t, blueDB); got != "blue-v1" {
		t.Fatalf("expected blue db to be restored to v1, got %q", got)
	}
	if got := readSQLiteValue(t, auditDB); got != "audit-v1" {
		t.Fatalf("expected audit db to be restored to v1, got %q", got)
	}
}

func TestCheckAndAutoRecoverPrefersRepairBeforeBackupRestore(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "backups")
	dataDir := filepath.Join(tmpDir, "data")
	configDir := filepath.Join(tmpDir, "config")

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}

	blueDB := filepath.Join(dataDir, "blue.db")
	auditDB := filepath.Join(dataDir, "session_audit.db")
	writeSQLiteValue(t, blueDB, "blue-v1")
	writeSQLiteValue(t, auditDB, "audit-v1")

	m, err := NewManager(Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          backupDir,
	}, dataDir, configDir)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	if _, err := m.Create(context.Background(), BackupTypeData); err != nil {
		t.Fatalf("failed to create backup: %v", err)
	}

	writeSQLiteValue(t, blueDB, "blue-v2")
	corruptSQLiteBytes(t, auditDB, 200, []byte("garbage"))

	dbPaths, err := DiscoverSQLiteDatabasePaths(dataDir)
	if err != nil {
		t.Fatalf("failed to discover database paths: %v", err)
	}

	result, err := m.CheckAndAutoRecover(context.Background(), dbPaths)
	if err != nil {
		t.Fatalf("auto recovery failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if result.Recovered {
		t.Fatalf("expected in-place repair instead of backup restore, got %+v", result)
	}
	if !containsString(result.RepairedDatabases, auditDB) {
		t.Fatalf("expected repaired databases to include %s, got %v", auditDB, result.RepairedDatabases)
	}
	if len(result.CorruptedDatabases) != 0 {
		t.Fatalf("expected no remaining corrupted databases, got %v", result.CorruptedDatabases)
	}

	if got := readSQLiteValue(t, blueDB); got != "blue-v2" {
		t.Fatalf("expected blue db to stay at v2, got %q", got)
	}
	if got := readSQLiteValue(t, auditDB); got != "audit-v1" {
		t.Fatalf("expected repaired audit db to retain data, got %q", got)
	}
}

func writeSQLiteValue(t *testing.T, path, value string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("failed to open sqlite db %s: %v", path, err)
	}
	defer db.Close()

	statements := []string{
		"DROP TABLE IF EXISTS entries",
		"CREATE TABLE entries (value TEXT NOT NULL)",
		"INSERT INTO entries(value) VALUES (?)",
	}
	if _, err := db.Exec(statements[0]); err != nil {
		t.Fatalf("failed to reset sqlite db %s: %v", path, err)
	}
	if _, err := db.Exec(statements[1]); err != nil {
		t.Fatalf("failed to create sqlite schema %s: %v", path, err)
	}
	if _, err := db.Exec(statements[2], value); err != nil {
		t.Fatalf("failed to insert sqlite value %s: %v", path, err)
	}
}

func readSQLiteValue(t *testing.T, path string) string {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("failed to open sqlite db %s: %v", path, err)
	}
	defer db.Close()

	var value string
	if err := db.QueryRow("SELECT value FROM entries LIMIT 1").Scan(&value); err != nil {
		t.Fatalf("failed to read sqlite value from %s: %v", path, err)
	}
	return value
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func corruptSQLiteBytes(t *testing.T, path string, offset int64, payload []byte) {
	t.Helper()

	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("failed to open sqlite db %s for corruption: %v", path, err)
	}
	defer f.Close()

	if _, err := f.WriteAt(payload, offset); err != nil {
		t.Fatalf("failed to corrupt sqlite db %s: %v", path, err)
	}
}
