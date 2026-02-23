package update

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Executor abstracts the process-replacement step so it can be mocked in tests.
type Executor interface {
	Exec(binaryPath string, args []string, env []string) error
}

// Applier handles applying updates
type Applier struct {
	binaryPath  string
	storagePath string
	backupCount int
	startTime   time.Time
	executor    Executor // nil → uses platform default
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

// SetExecutor overrides the default platform executor (for testing).
func (a *Applier) SetExecutor(e Executor) { a.executor = e }

// Apply applies the downloaded update (backup + replace + exec).
// Note: on success the process is replaced and this never returns.
func (a *Applier) Apply(downloadedPath string) error {
	if err := a.PrepareAndReplace(downloadedPath); err != nil {
		return err
	}
	return a.Restart()
}

// PrepareAndReplace backs up the current binary and replaces it with the new one.
// Does NOT restart — call Restart() separately when ready.
// On Windows, if the binary is locked (running process), the new binary is saved
// as .new and the bat-script-based exec() will handle the swap after exit.
func (a *Applier) PrepareAndReplace(downloadedPath string) error {
	if err := a.backup(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	if err := a.replace(downloadedPath); err != nil {
		// On Windows, the running exe is locked — save as .new for deferred swap
		if runtime.GOOS == "windows" {
			newPath := a.binaryPath + ".new"
			if err2 := os.Rename(downloadedPath, newPath); err2 != nil {
				a.Rollback()
				return fmt.Errorf("replace failed (deferred also failed): %w", err2)
			}
			return nil
		}
		a.Rollback()
		return fmt.Errorf("replace failed: %w", err)
	}
	return nil
}

// Restart replaces the current process with the new binary.
// On Unix this calls syscall.Exec (never returns on success).
// On Windows this spawns a new process and exits.
func (a *Applier) Restart() error {
	if a.executor != nil {
		env := append(os.Environ(), fmt.Sprintf("BLUE_START_TIME=%d", a.startTime.Unix()))
		return a.executor.Exec(a.binaryPath, os.Args, env)
	}
	return a.exec()
}

func (a *Applier) backup() error {
	backupDir := filepath.Join(a.storagePath, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	backupPath := filepath.Join(backupDir, fmt.Sprintf("echo.%d.bak", timeutil.Now()))
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
