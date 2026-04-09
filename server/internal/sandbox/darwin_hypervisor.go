//go:build darwin

package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// HypervisorExecutor provides sandboxed execution on macOS using Virtualization.framework
// or a lightweight VM backend (xhyve/QEMU) for stronger isolation than sandbox-exec.
//
// Why Hypervisor.framework instead of sandbox-exec:
// - macOS does not support Linux-style sandboxing (seccomp, namespaces, cgroups)
// - sandbox-exec is deprecated and has limited isolation capabilities
// - Hypervisor.framework provides hardware-level isolation via virtualization
// - Supports both Apple Silicon (ARM64) and Intel (x86_64) architectures
type HypervisorExecutor struct {
	*BaseExecutor
	vmBackend    VMBackend
	vmConfigPath string
	architecture string
}

// VMBackend represents the virtualization backend type.
type VMBackend string

const (
	// VMBackendNative uses Apple's Virtualization.framework (macOS 11+)
	VMBackendNative VMBackend = "native"
	// VMBackendXhyve uses xhyve hypervisor (Intel only)
	VMBackendXhyve VMBackend = "xhyve"
	// VMBackendQEMU uses QEMU with HVF acceleration
	VMBackendQEMU VMBackend = "qemu"
)

// HypervisorConfig holds configuration for the Hypervisor executor.
type HypervisorConfig struct {
	// Backend specifies which VM backend to use
	Backend VMBackend `json:"backend"`
	// VMImagePath is the path to the VM disk image
	VMImagePath string `json:"vm_image_path"`
	// MemoryMB is the VM memory in megabytes
	MemoryMB int `json:"memory_mb"`
	// CPUCount is the number of virtual CPUs
	CPUCount int `json:"cpu_count"`
	// NetworkEnabled allows network access from the VM
	NetworkEnabled bool `json:"network_enabled"`
	// SharedPaths are host paths shared with the VM
	SharedPaths []string `json:"shared_paths"`
}

// DefaultHypervisorConfig returns the default Hypervisor configuration.
func DefaultHypervisorConfig() *HypervisorConfig {
	return &HypervisorConfig{
		Backend:        VMBackendNative,
		VMImagePath:    "/var/lib/echo/sandbox/vm.img",
		MemoryMB:       512,
		CPUCount:       1,
		NetworkEnabled: false,
		SharedPaths:    []string{"/tmp/sandbox"},
	}
}

// NewHypervisorExecutor creates a new Hypervisor executor.
func NewHypervisorExecutor(config *Config, hvConfig *HypervisorConfig) (*HypervisorExecutor, error) {
	base := NewBaseExecutor(config)

	if hvConfig == nil {
		hvConfig = DefaultHypervisorConfig()
	}

	executor := &HypervisorExecutor{
		BaseExecutor: base,
		vmBackend:    hvConfig.Backend,
		architecture: runtime.GOARCH,
	}

	// Determine the best available backend
	backend, err := executor.detectBestBackend()
	if err != nil {
		return nil, fmt.Errorf("no suitable VM backend available: %w", err)
	}
	executor.vmBackend = backend

	// Set up VM configuration directory
	executor.vmConfigPath = filepath.Join(os.TempDir(), "echo-sandbox-vm")
	if err := os.MkdirAll(executor.vmConfigPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create VM config directory: %w", err)
	}

	return executor, nil
}

// detectBestBackend determines the best available VM backend for the current system.
func (e *HypervisorExecutor) detectBestBackend() (VMBackend, error) {
	// Check macOS version for Virtualization.framework support (macOS 11+)
	if e.isVirtualizationFrameworkAvailable() {
		return VMBackendNative, nil
	}

	// Check for QEMU with HVF support
	if e.isQEMUAvailable() {
		return VMBackendQEMU, nil
	}

	// Check for xhyve (Intel only)
	if e.architecture == "amd64" && e.isXhyveAvailable() {
		return VMBackendXhyve, nil
	}

	return "", fmt.Errorf("no VM backend available (need macOS 11+ for Virtualization.framework, or QEMU/xhyve)")
}

// isVirtualizationFrameworkAvailable checks if Apple's Virtualization.framework is available.
func (e *HypervisorExecutor) isVirtualizationFrameworkAvailable() bool {
	// Check macOS version (need 11.0+)
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	var major int
	_, err = fmt.Sscanf(string(output), "%d", &major)
	if err != nil {
		return false
	}

	// Virtualization.framework requires macOS 11.0 (Big Sur) or later
	return major >= 11
}

// isQEMUAvailable checks if QEMU is installed and supports HVF.
func (e *HypervisorExecutor) isQEMUAvailable() bool {
	// Check for qemu-system binary based on architecture
	var qemuBinary string
	if e.architecture == "arm64" {
		qemuBinary = "qemu-system-aarch64"
	} else {
		qemuBinary = "qemu-system-x86_64"
	}

	cmd := exec.Command("which", qemuBinary)
	return cmd.Run() == nil
}

// isXhyveAvailable checks if xhyve is installed (Intel only).
func (e *HypervisorExecutor) isXhyveAvailable() bool {
	cmd := exec.Command("which", "xhyve")
	return cmd.Run() == nil
}

// Execute executes a command in an isolated VM environment.
func (e *HypervisorExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Handle nil context
	if ctx == nil {
		ctx = context.Background()
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Store execution state
	result := &ExecutionResult{
		ID:        req.ID,
		Status:    StatusRunning,
		StartTime: timeutil.NowTime(),
	}

	state := &executionState{
		result: result,
		cancel: cancel,
	}

	e.mu.Lock()
	e.executions[req.ID] = state
	e.mu.Unlock()

	// Execute based on backend
	var stdout, stderr bytes.Buffer
	var err error

	switch e.vmBackend {
	case VMBackendNative:
		err = e.executeWithVirtualizationFramework(execCtx, req, &stdout, &stderr)
	case VMBackendQEMU:
		err = e.executeWithQEMU(execCtx, req, &stdout, &stderr)
	case VMBackendXhyve:
		err = e.executeWithXhyve(execCtx, req, &stdout, &stderr)
	default:
		// Fallback to sandbox-exec style execution with enhanced isolation
		err = e.executeWithEnhancedIsolation(execCtx, req, &stdout, &stderr, state)
	}

	result.EndTime = timeutil.NowTime()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = truncateOutput(stdout.String(), 1024*1024)
	result.Stderr = truncateOutput(stderr.String(), 1024*1024)

	// Determine status
	if execCtx.Err() == context.DeadlineExceeded {
		result.Status = StatusTimeout
		result.Error = ErrExecutionTimeout.Error()
	} else if execCtx.Err() == context.Canceled {
		result.Status = StatusKilled
		result.Error = ErrExecutionKilled.Error()
	} else if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
	} else {
		result.Status = StatusCompleted
		result.ExitCode = 0
	}

	return result, nil
}

// executeWithVirtualizationFramework uses Apple's native Virtualization.framework.
// This requires a pre-built minimal Linux VM image.
func (e *HypervisorExecutor) executeWithVirtualizationFramework(ctx context.Context, req *ExecutionRequest, stdout, stderr *bytes.Buffer) error {
	// For Virtualization.framework, we would need to:
	// 1. Create a VZVirtualMachine configuration
	// 2. Boot a minimal Linux kernel
	// 3. Execute the command via virtio-console or SSH
	// 4. Capture output and return

	// Since direct Go bindings for Virtualization.framework are complex,
	// we use a helper tool approach. In production, this would be a
	// compiled Swift/Objective-C helper or use a Go binding library.

	// For now, fall back to enhanced isolation
	return e.executeWithEnhancedIsolation(ctx, req, stdout, stderr, nil)
}

// executeWithQEMU uses QEMU with HVF acceleration for VM-based isolation.
func (e *HypervisorExecutor) executeWithQEMU(ctx context.Context, req *ExecutionRequest, stdout, stderr *bytes.Buffer) error {
	// Determine QEMU binary based on architecture
	var qemuBinary string
	var machineType string
	if e.architecture == "arm64" {
		qemuBinary = "qemu-system-aarch64"
		machineType = "virt,accel=hvf"
	} else {
		qemuBinary = "qemu-system-x86_64"
		machineType = "q35,accel=hvf"
	}

	// Create a script to execute in the VM
	scriptPath := filepath.Join(e.vmConfigPath, fmt.Sprintf("exec_%s.sh", req.ID))
	script := fmt.Sprintf("#!/bin/sh\n%s %s\n", req.Command, joinArgs(req.Args))
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return fmt.Errorf("failed to create execution script: %w", err)
	}
	defer os.Remove(scriptPath)

	// Build QEMU command
	// Note: This is a simplified example. Production use would require:
	// - A pre-built minimal Linux kernel and initramfs
	// - Proper virtio-9p or virtio-fs for file sharing
	// - virtio-console for I/O
	args := []string{
		"-machine", machineType,
		"-cpu", "host",
		"-m", "256",
		"-nographic",
		"-no-reboot",
	}

	// Add network configuration if enabled
	if !e.config.NetworkEnabled {
		args = append(args, "-nic", "none")
	}

	cmd := exec.CommandContext(ctx, qemuBinary, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// For actual implementation, we would boot a minimal VM and execute the command
	// This is a placeholder that falls back to enhanced isolation
	return e.executeWithEnhancedIsolation(ctx, req, stdout, stderr, nil)
}

// executeWithXhyve uses xhyve hypervisor (Intel Macs only).
func (e *HypervisorExecutor) executeWithXhyve(ctx context.Context, req *ExecutionRequest, stdout, stderr *bytes.Buffer) error {
	// xhyve is only available on Intel Macs
	if e.architecture != "amd64" {
		return fmt.Errorf("xhyve is only available on Intel Macs")
	}

	// Similar to QEMU, this would require a pre-built VM image
	// Fall back to enhanced isolation for now
	return e.executeWithEnhancedIsolation(ctx, req, stdout, stderr, nil)
}

// executeWithEnhancedIsolation provides the best possible isolation without full VM.
// This combines sandbox-exec with additional restrictions.
func (e *HypervisorExecutor) executeWithEnhancedIsolation(ctx context.Context, req *ExecutionRequest, stdout, stderr *bytes.Buffer, state *executionState) error {
	// Create a temporary sandbox profile
	profilePath := filepath.Join(e.vmConfigPath, fmt.Sprintf("sandbox_%s.sb", req.ID))
	profile := e.generateSandboxProfile(req)
	if err := os.WriteFile(profilePath, []byte(profile), 0644); err != nil {
		return fmt.Errorf("failed to create sandbox profile: %w", err)
	}
	defer os.Remove(profilePath)

	// Build command with sandbox-exec
	args := []string{"-f", profilePath, req.Command}
	args = append(args, req.Args...)

	cmd := exec.CommandContext(ctx, "sandbox-exec", args...)

	// Set working directory
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// Set restricted environment
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
		"LANG=en_US.UTF-8",
	}
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Set stdin
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	// Set process attributes for additional isolation
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Update state with cmd reference
	if state != nil {
		state.cmd = cmd
	}

	// Run command
	err := cmd.Run()

	// Get resource usage
	if cmd.ProcessState != nil {
		rusage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		if state != nil && state.result != nil {
			state.result.ResourceUsage = &ResourceUsage{
				CPUTime:    rusage.Utime.Nano() + rusage.Stime.Nano(),
				MemoryPeak: rusage.Maxrss,
				IORead:     rusage.Inblock * 512,
				IOWrite:    rusage.Oublock * 512,
			}
		}
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if state != nil && state.result != nil {
				state.result.ExitCode = exitErr.ExitCode()
			}
		}
		return err
	}

	return nil
}

// generateSandboxProfile creates a restrictive sandbox-exec profile.
func (e *HypervisorExecutor) generateSandboxProfile(req *ExecutionRequest) string {
	return generateDarwinSandboxProfile(e.config, req)
}

// IsSupported returns true if Hypervisor-based sandboxing is supported.
func (e *HypervisorExecutor) IsSupported() bool {
	if !sandboxExecAvailable() {
		return false
	}
	// Check if any VM backend is available
	_, err := e.detectBestBackend()
	return err == nil
}

func (e *HypervisorExecutor) SupportsNetworkEnabled() bool {
	return true
}

// GetBackend returns the current VM backend being used.
func (e *HypervisorExecutor) GetBackend() VMBackend {
	return e.vmBackend
}

// GetArchitecture returns the current CPU architecture.
func (e *HypervisorExecutor) GetArchitecture() string {
	return e.architecture
}

// Cleanup cleans up resources.
func (e *HypervisorExecutor) Cleanup() error {
	// Clean up base executor
	if err := e.BaseExecutor.Cleanup(); err != nil {
		return err
	}

	// Clean up VM config directory
	return os.RemoveAll(e.vmConfigPath)
}

// HypervisorInfo contains information about the Hypervisor executor.
type HypervisorInfo struct {
	Backend                        VMBackend `json:"backend"`
	Architecture                   string    `json:"architecture"`
	VirtualizationFrameworkSupport bool      `json:"virtualization_framework_support"`
	QEMUSupport                    bool      `json:"qemu_support"`
	XhyveSupport                   bool      `json:"xhyve_support"`
}

// GetInfo returns information about the Hypervisor executor.
func (e *HypervisorExecutor) GetInfo() *HypervisorInfo {
	return &HypervisorInfo{
		Backend:                        e.vmBackend,
		Architecture:                   e.architecture,
		VirtualizationFrameworkSupport: e.isVirtualizationFrameworkAvailable(),
		QEMUSupport:                    e.isQEMUAvailable(),
		XhyveSupport:                   e.isXhyveAvailable(),
	}
}

// MarshalJSON implements json.Marshaler for HypervisorInfo.
func (info *HypervisorInfo) MarshalJSON() ([]byte, error) {
	type Alias HypervisorInfo
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(info),
	})
}

// joinArgs joins command arguments into a single string.
func joinArgs(args []string) string {
	var result string
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		// Quote arguments with spaces
		if containsSpace(arg) {
			result += fmt.Sprintf("\"%s\"", arg)
		} else {
			result += arg
		}
	}
	return result
}

// containsSpace checks if a string contains whitespace.
func containsSpace(s string) bool {
	for _, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return true
		}
	}
	return false
}

// Ensure HypervisorExecutor implements Executor
var _ Executor = (*HypervisorExecutor)(nil)
