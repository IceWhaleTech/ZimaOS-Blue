//go:build windows

package server

import (
	"fmt"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// getWindowsServiceInfo queries Windows service information using native API
func (h *ServiceHandler) getWindowsServiceInfo(info *ServiceInfo) {
	m, err := mgr.Connect()
	if err != nil {
		info.Installed = false
		info.Status = "not_installed"
		return
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		info.Installed = false
		info.Status = "not_installed"
		return
	}
	defer s.Close()

	info.Installed = true
	info.Status = "installed"

	// Query service status
	status, err := s.Query()
	if err == nil {
		switch status.State {
		case svc.Running:
			info.Running = true
			info.Status = "running"
		case svc.Stopped:
			info.Running = false
			info.Status = "stopped"
		case svc.StartPending:
			info.Running = false
			info.Status = "starting"
		case svc.StopPending:
			info.Running = true
			info.Status = "stopping"
		case svc.Paused:
			info.Running = false
			info.Status = "paused"
		}
	}

	// Query service configuration
	config, err := s.Config()
	if err == nil {
		switch config.StartType {
		case mgr.StartAutomatic:
			info.Enabled = true
			info.StartType = "automatic"
		case mgr.StartManual:
			info.Enabled = false
			info.StartType = "manual"
		case mgr.StartDisabled:
			info.Enabled = false
			info.StartType = "disabled"
		}
	}

	info.InstallPath = filepath.Dir(h.execPath)
	info.ConfigPath = filepath.Join(info.InstallPath, "config", "config.yaml")
}

// getWindowsStatus queries Windows service status using native API
func (h *ServiceHandler) getWindowsStatus() (status string, running, installed, enabled bool) {
	m, err := mgr.Connect()
	if err != nil {
		return "not_installed", false, false, false
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "not_installed", false, false, false
	}
	defer s.Close()

	installed = true

	// Query service status
	svcStatus, err := s.Query()
	if err == nil {
		switch svcStatus.State {
		case svc.Running:
			status = "running"
			running = true
		case svc.Stopped:
			status = "stopped"
			running = false
		case svc.StartPending:
			status = "starting"
			running = false
		case svc.StopPending:
			status = "stopping"
			running = true
		case svc.Paused:
			status = "paused"
			running = false
		default:
			status = "unknown"
		}
	}

	// Query service configuration
	config, err := s.Config()
	if err == nil {
		enabled = config.StartType == mgr.StartAutomatic
	}

	return
}

// installWindowsService installs the service using native API
func (h *ServiceHandler) installWindowsService() (string, error) {
	// Check if running in Tauri mode (embedded library)
	if h.isTauriMode() {
		return "", fmt.Errorf("service installation must be done through the GUI application's settings")
	}

	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	// Check if service already exists
	s, err := m.OpenService(h.serviceName)
	if err == nil {
		s.Close()
		return "", fmt.Errorf("service %s already exists", h.serviceName)
	}

	// Create the service
	svcConfig := mgr.Config{
		DisplayName: h.serviceName,
		Description: "ZimaOS Blue - A Local-first Agent Runtime",
		StartType:   mgr.StartAutomatic,
	}

	s, err = m.CreateService(h.serviceName, h.execPath, svcConfig, "--service")
	if err != nil {
		return "", fmt.Errorf("failed to create service: %w", err)
	}
	defer s.Close()

	return fmt.Sprintf("Service '%s' installed successfully", h.serviceName), nil
}

// uninstallWindowsService uninstalls the service using native API
func (h *ServiceHandler) uninstallWindowsService() (string, error) {
	// Check if running in Tauri mode (embedded library)
	if h.isTauriMode() {
		return "", fmt.Errorf("service uninstallation must be done through the GUI application's settings")
	}

	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "", fmt.Errorf("service %s not found: %w", h.serviceName, err)
	}
	defer s.Close()

	// Stop the service if running
	status, err := s.Query()
	if err == nil && status.State != svc.Stopped {
		_, err = s.Control(svc.Stop)
		if err != nil {
			return "", fmt.Errorf("failed to stop service: %w", err)
		}
		// Wait for service to stop
		for i := 0; i < 30; i++ {
			status, err = s.Query()
			if err != nil || status.State == svc.Stopped {
				break
			}
			time.Sleep(time.Second)
		}
	}

	// Delete the service
	err = s.Delete()
	if err != nil {
		return "", fmt.Errorf("failed to delete service: %w", err)
	}

	return fmt.Sprintf("Service '%s' uninstalled successfully", h.serviceName), nil
}

// startWindowsService starts the service using native API
func (h *ServiceHandler) startWindowsService() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "", fmt.Errorf("service %s not found: %w", h.serviceName, err)
	}
	defer s.Close()

	err = s.Start()
	if err != nil {
		return "", fmt.Errorf("failed to start service: %w", err)
	}

	return "Service started successfully", nil
}

// stopWindowsService stops the service using native API
func (h *ServiceHandler) stopWindowsService() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "", fmt.Errorf("service %s not found: %w", h.serviceName, err)
	}
	defer s.Close()

	status, err := s.Control(svc.Stop)
	if err != nil {
		return "", fmt.Errorf("failed to stop service: %w", err)
	}

	// Wait for service to stop
	timeout := time.Now().Add(30 * time.Second)
	for status.State != svc.Stopped {
		if time.Now().After(timeout) {
			return "", fmt.Errorf("timeout waiting for service to stop")
		}
		time.Sleep(500 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return "", fmt.Errorf("failed to query service status: %w", err)
		}
	}

	return "Service stopped successfully", nil
}

// enableWindowsService enables the service to start automatically
func (h *ServiceHandler) enableWindowsService() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "", fmt.Errorf("service %s not found: %w", h.serviceName, err)
	}
	defer s.Close()

	config, err := s.Config()
	if err != nil {
		return "", fmt.Errorf("failed to get service config: %w", err)
	}

	config.StartType = mgr.StartAutomatic
	err = s.UpdateConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to update service config: %w", err)
	}

	return "Service enabled successfully", nil
}

// disableWindowsService disables the service from starting automatically
func (h *ServiceHandler) disableWindowsService() (string, error) {
	m, err := mgr.Connect()
	if err != nil {
		return "", fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(h.serviceName)
	if err != nil {
		return "", fmt.Errorf("service %s not found: %w", h.serviceName, err)
	}
	defer s.Close()

	config, err := s.Config()
	if err != nil {
		return "", fmt.Errorf("failed to get service config: %w", err)
	}

	config.StartType = mgr.StartManual
	err = s.UpdateConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to update service config: %w", err)
	}

	return "Service disabled successfully", nil
}

