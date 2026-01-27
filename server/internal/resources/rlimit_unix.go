//go:build unix && !darwin

package resources

import (
	"fmt"
	"syscall"
)

// applyRlimits applies OS-specific resource limits on Unix systems.
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
func GetRlimits() (map[string]Rlimit, error) {
	limits := make(map[string]Rlimit)

	// RLIMIT_NOFILE
	var nofile syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &nofile); err == nil {
		limits["nofile"] = Rlimit{Cur: nofile.Cur, Max: nofile.Max}
	}

	// RLIMIT_NPROC
	var nproc syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NPROC, &nproc); err == nil {
		limits["nproc"] = Rlimit{Cur: nproc.Cur, Max: nproc.Max}
	}

	// RLIMIT_AS (address space)
	var as syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_AS, &as); err == nil {
		limits["as"] = Rlimit{Cur: as.Cur, Max: as.Max}
	}

	return limits, nil
}

// Rlimit represents a resource limit.
type Rlimit struct {
	Cur uint64 `json:"cur"`
	Max uint64 `json:"max"`
}
