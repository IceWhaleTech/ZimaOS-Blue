package bootstrap

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
)

// NewSandboxManagerFromConfig builds a sandbox manager from the loaded app
// config. Returning nil, nil means sandbox is disabled by configuration.
func NewSandboxManagerFromConfig(cfg *config.Config) (*sandbox.Manager, error) {
	if cfg != nil && !cfg.Security.Sandbox.Enabled {
		return nil, nil
	}
	if cfg == nil {
		return sandbox.NewManager(nil)
	}

	runtimeCfg := sandbox.DefaultConfig()
	sandboxCfg := cfg.Security.Sandbox

	if sandboxCfg.DefaultTimeout > 0 {
		runtimeCfg.DefaultTimeout = sandboxCfg.DefaultTimeout
	}
	if sandboxCfg.MaxTimeout > 0 {
		runtimeCfg.MaxTimeout = sandboxCfg.MaxTimeout
	}
	if runtimeCfg.MaxTimeout > 0 && runtimeCfg.MaxTimeout < runtimeCfg.DefaultTimeout {
		runtimeCfg.MaxTimeout = runtimeCfg.DefaultTimeout
	}
	if raw := strings.TrimSpace(sandboxCfg.MemoryLimit); raw != "" {
		memoryBytes, err := parseSandboxMemoryLimitBytes(raw)
		if err != nil {
			return nil, err
		}
		runtimeCfg.MemoryLimit = memoryBytes
	}
	if sandboxCfg.CPULimit > 0 {
		runtimeCfg.CPULimit = sandboxCfg.CPULimit
	}
	if sandboxCfg.ProcessLimit > 0 {
		runtimeCfg.ProcessLimit = sandboxCfg.ProcessLimit
	}
	runtimeCfg.NetworkEnabled = sandboxCfg.NetworkEnabled
	if raw := strings.TrimSpace(sandboxCfg.DarwinExecutorMode); raw != "" {
		runtimeCfg.DarwinExecutorMode = raw
	}
	if raw := strings.TrimSpace(sandboxCfg.HypervisorVMImagePath); raw != "" {
		runtimeCfg.HypervisorVMImagePath = raw
	}
	if sandboxCfg.HypervisorMemoryMB > 0 {
		runtimeCfg.HypervisorMemoryMB = sandboxCfg.HypervisorMemoryMB
	}
	if sandboxCfg.HypervisorCPUCount > 0 {
		runtimeCfg.HypervisorCPUCount = sandboxCfg.HypervisorCPUCount
	}
	if raw := strings.TrimSpace(sandboxCfg.LinuxLXCExecutable); raw != "" {
		runtimeCfg.LinuxLXCExecutable = raw
	}
	if raw := strings.TrimSpace(sandboxCfg.LinuxLXCInstance); raw != "" {
		runtimeCfg.LinuxLXCInstance = raw
	}
	if raw := strings.TrimSpace(sandboxCfg.WindowsCodexExecutable); raw != "" {
		runtimeCfg.WindowsCodexExecutable = raw
	}
	runtimeCfg.WindowsCodexAutoDownload = sandboxCfg.WindowsCodexAutoDownload
	if sandboxCfg.WindowsCodexDownloadTimeout > 0 {
		runtimeCfg.WindowsCodexDownloadTimeout = sandboxCfg.WindowsCodexDownloadTimeout
	}

	return sandbox.NewManager(runtimeCfg)
}

func parseSandboxMemoryLimitBytes(raw string) (int64, error) {
	mb := parseScannerMemoryLimitMB(raw)
	if mb <= 0 {
		return 0, fmt.Errorf("invalid sandbox memory limit %q", raw)
	}
	return int64(mb) * 1024 * 1024, nil
}
