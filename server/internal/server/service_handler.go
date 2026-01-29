package server

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/labstack/echo/v4"
)

// ServiceHandler handles service management API endpoints
type ServiceHandler struct {
	serviceName string
	execPath    string
}

// NewServiceHandler creates a new service handler
func NewServiceHandler() *ServiceHandler {
	execPath, _ := os.Executable()
	return &ServiceHandler{
		serviceName: "ZimaOS-Echo",
		execPath:    execPath,
	}
}

// RegisterRoutes registers service management routes
func (h *ServiceHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/service/status", h.GetStatus)
	g.GET("/service/info", h.GetInfo)
	g.POST("/service/install", h.Install)
	g.POST("/service/uninstall", h.Uninstall)
	g.POST("/service/start", h.Start)
	g.POST("/service/stop", h.Stop)
	g.POST("/service/restart", h.Restart)
	g.POST("/service/enable", h.Enable)
	g.POST("/service/disable", h.Disable)
}

// ServiceInfo represents service information
type ServiceInfo struct {
	Platform       string `json:"platform"`
	ServiceName    string `json:"service_name"`
	ServiceType    string `json:"service_type"`
	ExecutablePath string `json:"executable_path"`
	Installed      bool   `json:"installed"`
	Running        bool   `json:"running"`
	Enabled        bool   `json:"enabled"`
	Status         string `json:"status"`
	StartType      string `json:"start_type"`
	Description    string `json:"description,omitempty"`
	InstallPath    string `json:"install_path,omitempty"`
	ConfigPath     string `json:"config_path,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
}

// ServiceResponse represents a service operation response
type ServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

// GetInfo returns service information for the current platform
func (h *ServiceHandler) GetInfo(c echo.Context) error {
	info := ServiceInfo{
		Platform:       runtime.GOOS,
		ServiceName:    h.serviceName,
		ExecutablePath: h.execPath,
	}

	switch runtime.GOOS {
	case "windows":
		info.ServiceType = "windows_service"
		h.getWindowsServiceInfo(&info)
	case "darwin":
		info.ServiceType = "launchd"
		h.getLaunchdServiceInfo(&info)
	case "linux":
		info.ServiceType = "systemd"
		h.getSystemdServiceInfo(&info)
	default:
		info.ServiceType = "unknown"
		info.Status = "unsupported"
	}

	return c.JSON(http.StatusOK, info)
}

// GetStatus returns the current service status
func (h *ServiceHandler) GetStatus(c echo.Context) error {
	var status string
	var running bool
	var installed bool
	var enabled bool

	switch runtime.GOOS {
	case "windows":
		status, running, installed, enabled = h.getWindowsStatus()
	case "darwin":
		status, running, installed, enabled = h.getLaunchdStatus()
	case "linux":
		status, running, installed, enabled = h.getSystemdStatus()
	default:
		status = "unsupported"
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"platform":  runtime.GOOS,
		"status":    status,
		"running":   running,
		"installed": installed,
		"enabled":   enabled,
	})
}

// Install installs the service
func (h *ServiceHandler) Install(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.installWindowsService()
	case "darwin":
		output, err = h.installLaunchdService()
	case "linux":
		output, err = h.installSystemdService()
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service installed successfully",
		Output:  output,
	})
}

// Uninstall removes the service
func (h *ServiceHandler) Uninstall(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.uninstallWindowsService()
	case "darwin":
		output, err = h.uninstallLaunchdService()
	case "linux":
		output, err = h.uninstallSystemdService()
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service uninstalled successfully",
		Output:  output,
	})
}

// Start starts the service
func (h *ServiceHandler) Start(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.runCommand("sc", "start", h.serviceName)
	case "darwin":
		output, err = h.runCommand("launchctl", "start", h.getLaunchdLabel())
	case "linux":
		output, err = h.runCommand("systemctl", "start", h.getSystemdUnit())
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service started successfully",
		Output:  output,
	})
}

// Stop stops the service
func (h *ServiceHandler) Stop(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.runCommand("sc", "stop", h.serviceName)
	case "darwin":
		output, err = h.runCommand("launchctl", "stop", h.getLaunchdLabel())
	case "linux":
		output, err = h.runCommand("systemctl", "stop", h.getSystemdUnit())
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service stopped successfully",
		Output:  output,
	})
}

// Restart restarts the service
func (h *ServiceHandler) Restart(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		// Windows doesn't have a native restart, so stop then start
		h.runCommand("sc", "stop", h.serviceName)
		output, err = h.runCommand("sc", "start", h.serviceName)
	case "darwin":
		output, err = h.runCommand("launchctl", "kickstart", "-k", "gui/"+h.getLaunchdLabel())
		if err != nil {
			// Fallback to stop/start
			h.runCommand("launchctl", "stop", h.getLaunchdLabel())
			output, err = h.runCommand("launchctl", "start", h.getLaunchdLabel())
		}
	case "linux":
		output, err = h.runCommand("systemctl", "restart", h.getSystemdUnit())
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service restarted successfully",
		Output:  output,
	})
}

// Enable enables the service to start on boot
func (h *ServiceHandler) Enable(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.runCommand("sc", "config", h.serviceName, "start=", "auto")
	case "darwin":
		// launchd services with RunAtLoad are automatically enabled
		output = "launchd services are enabled by default when installed"
	case "linux":
		output, err = h.runCommand("systemctl", "enable", h.getSystemdUnit())
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service enabled successfully",
		Output:  output,
	})
}

// Disable disables the service from starting on boot
func (h *ServiceHandler) Disable(c echo.Context) error {
	var output string
	var err error

	switch runtime.GOOS {
	case "windows":
		output, err = h.runCommand("sc", "config", h.serviceName, "start=", "demand")
	case "darwin":
		output, err = h.runCommand("launchctl", "disable", "gui/"+h.getLaunchdLabel())
	case "linux":
		output, err = h.runCommand("systemctl", "disable", h.getSystemdUnit())
	default:
		return c.JSON(http.StatusBadRequest, ServiceResponse{
			Success: false,
			Message: fmt.Sprintf("Unsupported platform: %s", runtime.GOOS),
		})
	}

	if err != nil {
		return c.JSON(http.StatusInternalServerError, ServiceResponse{
			Success: false,
			Message: err.Error(),
			Output:  output,
		})
	}

	return c.JSON(http.StatusOK, ServiceResponse{
		Success: true,
		Message: "Service disabled successfully",
		Output:  output,
	})
}

// Helper functions

func (h *ServiceHandler) runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func (h *ServiceHandler) getLaunchdLabel() string {
	return "com.icewhale.zimaos-echo"
}

func (h *ServiceHandler) getSystemdUnit() string {
	return "zimaos-echo.service"
}

// Windows-specific functions

func (h *ServiceHandler) getWindowsServiceInfo(info *ServiceInfo) {
	output, err := h.runCommand("sc", "query", h.serviceName)
	if err != nil {
		info.Installed = false
		info.Status = "not_installed"
		return
	}

	info.Installed = true
	info.Status = "installed"

	if strings.Contains(output, "RUNNING") {
		info.Running = true
		info.Status = "running"
	} else if strings.Contains(output, "STOPPED") {
		info.Running = false
		info.Status = "stopped"
	}

	// Check start type
	configOutput, _ := h.runCommand("sc", "qc", h.serviceName)
	if strings.Contains(configOutput, "AUTO_START") {
		info.Enabled = true
		info.StartType = "automatic"
	} else if strings.Contains(configOutput, "DEMAND_START") {
		info.Enabled = false
		info.StartType = "manual"
	} else if strings.Contains(configOutput, "DISABLED") {
		info.Enabled = false
		info.StartType = "disabled"
	}

	info.InstallPath = filepath.Dir(h.execPath)
	info.ConfigPath = filepath.Join(info.InstallPath, "config", "config.yaml")
}

func (h *ServiceHandler) getWindowsStatus() (status string, running, installed, enabled bool) {
	output, err := h.runCommand("sc", "query", h.serviceName)
	if err != nil {
		return "not_installed", false, false, false
	}

	installed = true
	if strings.Contains(output, "RUNNING") {
		status = "running"
		running = true
	} else if strings.Contains(output, "STOPPED") {
		status = "stopped"
		running = false
	} else {
		status = "unknown"
	}

	configOutput, _ := h.runCommand("sc", "qc", h.serviceName)
	enabled = strings.Contains(configOutput, "AUTO_START")

	return
}

func (h *ServiceHandler) installWindowsService() (string, error) {
	// Use the executable's install command
	return h.runCommand(h.execPath, "install")
}

func (h *ServiceHandler) uninstallWindowsService() (string, error) {
	// Stop the service first
	h.runCommand("sc", "stop", h.serviceName)
	// Use the executable's uninstall command
	return h.runCommand(h.execPath, "uninstall")
}

// macOS launchd-specific functions

func (h *ServiceHandler) getLaunchdServiceInfo(info *ServiceInfo) {
	label := h.getLaunchdLabel()
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")

	// Check if plist exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		// Check system-wide location
		plistPath = filepath.Join("/Library/LaunchDaemons", label+".plist")
		if _, err := os.Stat(plistPath); os.IsNotExist(err) {
			info.Installed = false
			info.Status = "not_installed"
			return
		}
	}

	info.Installed = true
	info.ConfigPath = plistPath

	// Check if running
	output, _ := h.runCommand("launchctl", "list")
	if strings.Contains(output, label) {
		info.Running = true
		info.Status = "running"
	} else {
		info.Running = false
		info.Status = "stopped"
	}

	// launchd services with RunAtLoad are enabled by default
	info.Enabled = true
	info.StartType = "automatic"
	info.InstallPath = filepath.Dir(h.execPath)
}

func (h *ServiceHandler) getLaunchdStatus() (status string, running, installed, enabled bool) {
	label := h.getLaunchdLabel()
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")

	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		plistPath = filepath.Join("/Library/LaunchDaemons", label+".plist")
		if _, err := os.Stat(plistPath); os.IsNotExist(err) {
			return "not_installed", false, false, false
		}
	}

	installed = true
	enabled = true // launchd services are enabled by default

	output, _ := h.runCommand("launchctl", "list")
	if strings.Contains(output, label) {
		return "running", true, true, true
	}

	return "stopped", false, true, true
}

func (h *ServiceHandler) installLaunchdService() (string, error) {
	label := h.getLaunchdLabel()
	installDir := filepath.Dir(h.execPath)
	configPath := filepath.Join(installDir, "config", "config.yaml")
	logPath := filepath.Join(installDir, "logs", "echo.log")
	errLogPath := filepath.Join(installDir, "logs", "echo-error.log")

	// Create logs directory
	os.MkdirAll(filepath.Join(installDir, "logs"), 0755)

	// Create plist content
	plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--config</string>
        <string>%s</string>
    </array>
    <key>WorkingDirectory</key>
    <string>%s</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
        <key>Crashed</key>
        <true/>
    </dict>
    <key>ThrottleInterval</key>
    <integer>5</integer>
    <key>StandardOutPath</key>
    <string>%s</string>
    <key>StandardErrorPath</key>
    <string>%s</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
`, label, h.execPath, configPath, installDir, logPath, errLogPath)

	// Write plist to LaunchAgents
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")
	os.MkdirAll(filepath.Dir(plistPath), 0755)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write plist: %w", err)
	}

	// Load the service
	output, err := h.runCommand("launchctl", "load", plistPath)
	if err != nil {
		return output, fmt.Errorf("failed to load service: %w", err)
	}

	return "Service installed and loaded successfully", nil
}

func (h *ServiceHandler) uninstallLaunchdService() (string, error) {
	label := h.getLaunchdLabel()
	plistPath := filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents", label+".plist")

	// Check if exists in user location
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		plistPath = filepath.Join("/Library/LaunchDaemons", label+".plist")
	}

	// Unload the service
	h.runCommand("launchctl", "unload", plistPath)

	// Remove the plist
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove plist: %w", err)
	}

	return "Service uninstalled successfully", nil
}

// Linux systemd-specific functions

func (h *ServiceHandler) getSystemdServiceInfo(info *ServiceInfo) {
	unit := h.getSystemdUnit()

	// Check if service file exists
	servicePath := filepath.Join("/etc/systemd/system", unit)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		servicePath = filepath.Join("/usr/lib/systemd/system", unit)
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			info.Installed = false
			info.Status = "not_installed"
			return
		}
	}

	info.Installed = true
	info.ConfigPath = servicePath

	// Check status
	output, _ := h.runCommand("systemctl", "is-active", unit)
	if strings.TrimSpace(output) == "active" {
		info.Running = true
		info.Status = "running"
	} else {
		info.Running = false
		info.Status = "stopped"
	}

	// Check if enabled
	enabledOutput, _ := h.runCommand("systemctl", "is-enabled", unit)
	info.Enabled = strings.TrimSpace(enabledOutput) == "enabled"
	if info.Enabled {
		info.StartType = "automatic"
	} else {
		info.StartType = "manual"
	}

	info.InstallPath = filepath.Dir(h.execPath)
	info.LogPath = "/var/log/zimaos-echo"
}

func (h *ServiceHandler) getSystemdStatus() (status string, running, installed, enabled bool) {
	unit := h.getSystemdUnit()

	// Check if service file exists
	servicePath := filepath.Join("/etc/systemd/system", unit)
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		servicePath = filepath.Join("/usr/lib/systemd/system", unit)
		if _, err := os.Stat(servicePath); os.IsNotExist(err) {
			return "not_installed", false, false, false
		}
	}

	installed = true

	// Check status
	output, _ := h.runCommand("systemctl", "is-active", unit)
	if strings.TrimSpace(output) == "active" {
		status = "running"
		running = true
	} else {
		status = "stopped"
		running = false
	}

	// Check if enabled
	enabledOutput, _ := h.runCommand("systemctl", "is-enabled", unit)
	enabled = strings.TrimSpace(enabledOutput) == "enabled"

	return
}

func (h *ServiceHandler) installSystemdService() (string, error) {
	unit := h.getSystemdUnit()
	installDir := filepath.Dir(h.execPath)
	configPath := filepath.Join(installDir, "config", "config.yaml")

	// Create systemd service content
	serviceContent := fmt.Sprintf(`[Unit]
Description=ZimaOS Echo - NAS-Native Agent Runtime
Documentation=https://github.com/IceWhaleTech/ZimaOS-Echo
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s --config %s
WorkingDirectory=%s
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

# Security hardening
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=read-only
PrivateTmp=true
ReadWritePaths=%s/data %s/logs

# Resource limits
MemoryMax=512M
CPUQuota=100%%

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=zimaos-echo

[Install]
WantedBy=multi-user.target
`, h.execPath, configPath, installDir, installDir, installDir)

	// Write service file
	servicePath := filepath.Join("/etc/systemd/system", unit)
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return "", fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if output, err := h.runCommand("systemctl", "daemon-reload"); err != nil {
		return output, fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable the service
	if output, err := h.runCommand("systemctl", "enable", unit); err != nil {
		return output, fmt.Errorf("failed to enable service: %w", err)
	}

	return "Service installed and enabled successfully", nil
}

func (h *ServiceHandler) uninstallSystemdService() (string, error) {
	unit := h.getSystemdUnit()

	// Stop the service
	h.runCommand("systemctl", "stop", unit)

	// Disable the service
	h.runCommand("systemctl", "disable", unit)

	// Remove service file
	servicePath := filepath.Join("/etc/systemd/system", unit)
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	h.runCommand("systemctl", "daemon-reload")

	return "Service uninstalled successfully", nil
}
