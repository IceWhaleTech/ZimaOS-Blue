// +build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"
)

// exec performs update on Windows (rename + new process)
func (a *Applier) exec() error {
	oldPath := a.binaryPath + ".old"
	os.Remove(oldPath)
	if err := os.Rename(a.binaryPath, oldPath); err != nil {
		return err
	}

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
	return time.Now()
}

// GetUptime returns the service uptime
func GetUptime() time.Duration {
	return time.Since(GetStartTime())
}

// CleanupOldBinary removes old binary after Windows update
func CleanupOldBinary(binaryPath string) {
	os.Remove(binaryPath + ".old")
}
