//go:build windows

package resources

// applyRlimits is a no-op on Windows as rlimits are Unix-specific.
func (l *Limiter) applyRlimits() error {
	// Windows does not support rlimits
	return nil
}

// Rlimit represents a resource limit.
type Rlimit struct {
	Cur uint64 `json:"cur"`
	Max uint64 `json:"max"`
}

// GetRlimits returns current rlimit values.
// On Windows, this returns an empty map as rlimits are not supported.
func GetRlimits() (map[string]Rlimit, error) {
	return make(map[string]Rlimit), nil
}
