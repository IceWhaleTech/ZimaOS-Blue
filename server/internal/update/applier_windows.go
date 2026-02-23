//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// exec performs update on Windows using a helper bat script.
// The running binary is locked by the OS, so we can't rename it directly.
// Instead we write a bat script that waits for this process to exit,
// then replaces the binary and relaunches it.
func (a *Applier) exec() error {
	// The new binary was already placed at a.binaryPath by replace().
	// But on Windows, replace() may fail because the running exe is locked.
	// In that case, PrepareAndReplace stores the new binary next to the exe
	// and we use a bat script to do the swap after exit.

	// Try direct rename first (works if not locked, e.g. sidecar mode)
	newPath := a.binaryPath + ".new"
	if _, err := os.Stat(newPath); err != nil {
		// No .new file — replace() already succeeded, just relaunch
		return a.relaunch()
	}

	// Write a bat script to perform the swap
	batPath := filepath.Join(filepath.Dir(a.binaryPath), "_update.bat")
	pid := os.Getpid()
	args := strings.Join(os.Args[1:], " ")
	startTimeEnv := fmt.Sprintf("BLUE_START_TIME=%d", a.startTime.Unix())

	script := fmt.Sprintf(`@echo off
setlocal
:: Wait for the old process to exit
:wait
tasklist /FI "PID eq %d" 2>NUL | find /I "%d" >NUL
if not errorlevel 1 (
    timeout /t 1 /nobreak >NUL
    goto wait
)
:: Replace the binary
del /f "%s" 2>NUL
move /y "%s" "%s"
:: Relaunch
set %s
start "" "%s" %s
:: Self-delete
del /f "%%~f0"
`, pid, pid, a.binaryPath, newPath, a.binaryPath, startTimeEnv, a.binaryPath, args)

	if err := os.WriteFile(batPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("write update script: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/C", batPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start update script: %w", err)
	}

	os.Exit(0)
	return nil
}

// relaunch starts a new instance and exits (used when rename succeeded directly).
func (a *Applier) relaunch() error {
	cmd := exec.Command(a.binaryPath, os.Args[1:]...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("BLUE_START_TIME=%d", a.startTime.Unix()))
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

// GetStartTime returns the service start time
func GetStartTime() time.Time {
	if s := os.Getenv("BLUE_START_TIME"); s != "" {
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	}
	return timeutil.NowTime()
}

// GetUptime returns the service uptime
func GetUptime() time.Duration {
	return timeutil.SinceTime(GetStartTime())
}

// CleanupOldBinary removes leftover files from previous updates.
func CleanupOldBinary(binaryPath string) {
	os.Remove(binaryPath + ".old")
	os.Remove(binaryPath + ".new")
	os.Remove(filepath.Join(filepath.Dir(binaryPath), "_update.bat"))
}
