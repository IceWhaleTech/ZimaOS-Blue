//go:build !linux

package cgroup

// IsCgroupV2Available returns false on non-Linux systems.
func IsCgroupV2Available(cgroupRoot string) bool {
	return false
}

// GetAvailableControllers returns an empty list on non-Linux systems.
func GetAvailableControllers(cgroupRoot string) ([]string, error) {
	return nil, ErrCgroupNotSupported
}

// getRootDevice returns an error on non-Linux systems.
func getRootDevice() (string, error) {
	return "", ErrCgroupNotSupported
}

// EnableControllers returns an error on non-Linux systems.
func EnableControllers(cgroupDir string, controllers []string) error {
	return ErrCgroupNotSupported
}

// GetCgroupPath returns an error on non-Linux systems.
func GetCgroupPath() (string, error) {
	return "", ErrCgroupNotSupported
}

// IsInCgroup returns false on non-Linux systems.
func IsInCgroup(cgroupName string) (bool, error) {
	return false, ErrCgroupNotSupported
}
