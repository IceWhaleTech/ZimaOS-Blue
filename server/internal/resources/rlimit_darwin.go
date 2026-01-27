//go:build darwin

package resources

import (
	"fmt"
	"syscall"
)

// applyRlimits applies OS-specific resource limits on macOS.
func (l *Limiter) applyRlimits() error {
	// Set max open files
	if l.config.MaxOpenFiles > 0 {
		var rLimit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			return fmt.Errorf("failed to get RLIMIT_NOFILE: %w", err)
		}

		rLimit.Cur = l.config.MaxOpenFiles
		if rLimit.Cur > rLimit.Max {
			rLimit.Cur = rLimit.Max
		}

		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			return fmt.Errorf("failed to set RLIMIT_NOFILE: %w", err)
		}
	}

	return nil
}

// GetRlimits returns current rlimit values.
func GetRlimits() (map[string]syscall.Rlimit, error) {
	limits := make(map[string]syscall.Rlimit)

	// RLIMIT_NOFILE
	var nofile syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &nofile); err == nil {
		limits["nofile"] = nofile
	}

	// Note: RLIMIT_NPROC is not available on macOS

	return limits, nil
}
