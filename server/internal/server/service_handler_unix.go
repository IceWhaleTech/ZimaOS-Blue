//go:build !windows

package server

import (
	"fmt"
)

// getWindowsServiceInfo is a stub for non-Windows platforms
func (h *ServiceHandler) getWindowsServiceInfo(info *ServiceInfo) {
	info.Installed = false
	info.Status = "unsupported"
}

// getWindowsStatus is a stub for non-Windows platforms
func (h *ServiceHandler) getWindowsStatus() (status string, running, installed, enabled bool) {
	return "unsupported", false, false, false
}

// installWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) installWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service installation not supported on this platform")
}

// uninstallWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) uninstallWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service uninstallation not supported on this platform")
}

// startWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) startWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service start not supported on this platform")
}

// stopWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) stopWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service stop not supported on this platform")
}

// enableWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) enableWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service enable not supported on this platform")
}

// disableWindowsService is a stub for non-Windows platforms
func (h *ServiceHandler) disableWindowsService() (string, error) {
	return "", fmt.Errorf("Windows service disable not supported on this platform")
}
