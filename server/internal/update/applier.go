package update

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Applier handles applying updates
type Applier struct {
	binaryPath  string
	storagePath string
	backupCount int
	startTime   time.Time
}

// NewApplier creates a new update applier
func NewApplier(binaryPath, storagePath string, backupCount int) *Applier {
	return &Applier{
		binaryPath:  binaryPath,
		storagePath: storagePath,
		backupCount: backupCount,
		startTime:   GetStartTime(),
	}
}

// Apply applies the downloaded update
func (a *Applier) Apply(downloadedPath string) error {
	if err := a.backup(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	if err := a.replace(downloadedPath); err != nil {
		a.Rollback()
		return fmt.Errorf("replace failed: %w", err)
	}

	return a.exec()
}

func (a *Applier) backup() error {
	backupDir := filepath.Join(a.storagePath, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf("echo.%d.bak", time.Now().Unix()))
	return copyFile(a.binaryPath, backupPath)
}

func (a *Applier) replace(newBinary string) error {
	if err := os.Chmod(newBinary, 0755); err != nil {
		return err
	}
	return os.Rename(newBinary, a.binaryPath)
}

// Rollback restores the previous version
func (a *Applier) Rollback() error {
	backupDir := filepath.Join(a.storagePath, "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return fmt.Errorf("no backup available")
	}

	latest := entries[len(entries)-1]
	backupPath := filepath.Join(backupDir, latest.Name())
	return os.Rename(backupPath, a.binaryPath)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}
