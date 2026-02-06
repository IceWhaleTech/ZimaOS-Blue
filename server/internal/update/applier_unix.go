// +build !windows

package update

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
)

// exec performs hot update using syscall.Exec (Linux/macOS)
func (a *Applier) exec() error {
	env := append(os.Environ(), fmt.Sprintf("ECHO_START_TIME=%d", a.startTime.Unix()))
	return syscall.Exec(a.binaryPath, os.Args, env)
}

// GetStartTime returns the service start time (preserved across hot updates)
func GetStartTime() time.Time {
	if s := os.Getenv("ECHO_START_TIME"); s != "" {
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
