//go:build linux

package cgroup

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// IsCgroupV2Available checks if cgroup v2 is available on the system.
func IsCgroupV2Available(cgroupRoot string) bool {
	if cgroupRoot == "" {
		cgroupRoot = DefaultCgroupRoot
	}

	// Check if cgroup v2 is mounted
	// cgroup v2 has a "cgroup.controllers" file at the root
	controllersPath := filepath.Join(cgroupRoot, "cgroup.controllers")
	_, err := os.Stat(controllersPath)
	return err == nil
}

// GetAvailableControllers returns the list of available cgroup controllers.
func GetAvailableControllers(cgroupRoot string) ([]string, error) {
	if cgroupRoot == "" {
		cgroupRoot = DefaultCgroupRoot
	}

	controllersPath := filepath.Join(cgroupRoot, "cgroup.controllers")
	data, err := os.ReadFile(controllersPath)
	if err != nil {
		return nil, err
	}

	controllers := strings.Fields(string(data))
	return controllers, nil
}

// getRootDevice returns the major:minor number of the root filesystem device.
func getRootDevice() (string, error) {
	// Read /proc/self/mountinfo to find root device
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		// mountinfo format: ID parent major:minor root mount-point ...
		if len(fields) < 5 {
			continue
		}

		mountPoint := fields[4]
		if mountPoint == "/" {
			// Found root mount, get major:minor
			majorMinor := fields[2]
			return majorMinor, nil
		}
	}

	return "", nil
}

// EnableControllers enables the specified controllers in the cgroup.
func EnableControllers(cgroupDir string, controllers []string) error {
	subtreeControlPath := filepath.Join(cgroupDir, "cgroup.subtree_control")

	// Build content: +controller1 +controller2 ...
	var parts []string
	for _, c := range controllers {
		parts = append(parts, "+"+c)
	}
	content := strings.Join(parts, " ")

	return os.WriteFile(subtreeControlPath, []byte(content), 0644)
}

// GetCgroupPath returns the cgroup path for the current process.
func GetCgroupPath() (string, error) {
	// Read /proc/self/cgroup
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}

	// cgroup v2 format: 0::/path
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "0::") {
			return strings.TrimPrefix(line, "0::"), nil
		}
	}

	return "", nil
}

// IsInCgroup checks if the current process is in the specified cgroup.
func IsInCgroup(cgroupName string) (bool, error) {
	path, err := GetCgroupPath()
	if err != nil {
		return false, err
	}

	return strings.Contains(path, cgroupName), nil
}
