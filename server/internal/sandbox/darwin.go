//go:build darwin

package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// DarwinExecutorMode specifies which executor to use on macOS.
type DarwinExecutorMode string

const (
	// DarwinExecutorModeSandboxExec uses the traditional sandbox-exec approach
	DarwinExecutorModeSandboxExec DarwinExecutorMode = "sandbox-exec"
	// DarwinExecutorModeHypervisor uses Hypervisor.framework for stronger isolation
	DarwinExecutorModeHypervisor DarwinExecutorMode = "hypervisor"
	// DarwinExecutorModeAuto automatically selects the best available executor
	DarwinExecutorModeAuto DarwinExecutorMode = "auto"
)

// DarwinExecutor provides sandboxed execution on macOS using sandbox-exec.
type DarwinExecutor struct {
	*BaseExecutor
}

// newPlatformExecutor creates a new Darwin executor.
// BLUE_SANDBOX_MODE (or config.DarwinExecutorMode) controls selection:
// - "hypervisor": fail closed until a true VM-backed executor is implemented
// - "sandbox-exec": use the traditional sandbox-exec path
// - "auto" or unset: use sandbox-exec when available, otherwise report unsupported
func newPlatformExecutor(config *Config) (Executor, error) {
	mode := resolveDarwinExecutorMode(config)

	switch mode {
	case DarwinExecutorModeHypervisor:
		return newUnsupportedExecutor("macOS hypervisor sandbox backend is not implemented yet"), nil

	case DarwinExecutorModeSandboxExec:
		if !sandboxExecAvailable() {
			return newUnsupportedExecutor("sandbox-exec is not available on this system"), nil
		}
		return &DarwinExecutor{BaseExecutor: NewBaseExecutor(config)}, nil

	case DarwinExecutorModeAuto, "":
		if !sandboxExecAvailable() {
			return newUnsupportedExecutor("sandbox-exec is not available on this system"), nil
		}
		return &DarwinExecutor{BaseExecutor: NewBaseExecutor(config)}, nil

	default:
		if !sandboxExecAvailable() {
			return newUnsupportedExecutor(fmt.Sprintf("unknown macOS sandbox mode %q and sandbox-exec is unavailable", mode)), nil
		}
		return &DarwinExecutor{BaseExecutor: NewBaseExecutor(config)}, nil
	}
}

func newStrongPlatformExecutor(config *Config) (Executor, error) {
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		return newUnsupportedExecutor(err.Error()), nil
	}
	if !executor.IsSupported() {
		_ = executor.Cleanup()
		return newUnsupportedExecutor("macOS hypervisor sandbox backend is not available on this system"), nil
	}
	return executor, nil
}

// Execute executes a command with macOS-specific isolation.
func (e *DarwinExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	if !sandboxExecAvailable() {
		return nil, fmt.Errorf("%w: sandbox-exec is not available on this system", ErrSandboxNotSupported)
	}

	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	profileFile, err := os.CreateTemp("", "blue-sandbox-*.sb")
	if err != nil {
		return nil, fmt.Errorf("failed to create sandbox profile: %w", err)
	}
	profilePath := profileFile.Name()
	if err := os.WriteFile(profilePath, []byte(generateDarwinSandboxProfile(e.config, req)), 0644); err != nil {
		_ = profileFile.Close()
		_ = os.Remove(profilePath)
		return nil, fmt.Errorf("failed to write sandbox profile: %w", err)
	}
	_ = profileFile.Close()
	defer os.Remove(profilePath)

	args := []string{"-f", profilePath, req.Command}
	args = append(args, req.Args...)
	cmd := exec.CommandContext(execCtx, "sandbox-exec", args...)

	// Set working directory
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// Set environment — start with sandbox defaults, then merge request env.
	// Request env overrides defaults (e.g. PATH from exec tool includes blue's dir).
	defaults := map[string]string{
		"PATH":   "/usr/local/bin:/usr/bin:/bin",
		"HOME":   "/tmp",
		"TMPDIR": "/tmp",
	}
	for k, v := range req.Env {
		if k == "PATH" {
			// Prepend request PATH to default PATH so blue binary is found.
			defaults["PATH"] = v + ":" + defaults["PATH"]
		} else {
			defaults[k] = v
		}
	}
	env := make([]string, 0, len(defaults))
	for k, v := range defaults {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	// Set stdin
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set up macOS-specific process attributes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create process in a new process group
		Setpgid: true,
	}

	// Store execution state
	result := &ExecutionResult{
		ID:        req.ID,
		Status:    StatusRunning,
		StartTime: timeutil.NowTime(),
	}

	state := &executionState{
		result: result,
		cmd:    cmd,
		cancel: cancel,
	}

	e.mu.Lock()
	e.executions[req.ID] = state
	e.mu.Unlock()

	// Run command
	err = cmd.Run()
	result.EndTime = timeutil.NowTime()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = truncateOutput(stdout.String(), 1024*1024)
	result.Stderr = truncateOutput(stderr.String(), 1024*1024)

	// Get resource usage
	if cmd.ProcessState != nil {
		rusage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
		result.ResourceUsage = &ResourceUsage{
			CPUTime:    rusage.Utime.Nano() + rusage.Stime.Nano(),
			MemoryPeak: rusage.Maxrss, // Already in bytes on macOS
			IORead:     rusage.Inblock * 512,
			IOWrite:    rusage.Oublock * 512,
		}
	}

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
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.Status = StatusCompleted
		result.ExitCode = 0
	}

	return result, nil
}

// IsSupported returns true if macOS sandboxing is supported.
func (e *DarwinExecutor) IsSupported() bool {
	return sandboxExecAvailable()
}

func (e *DarwinExecutor) SupportsNetworkEnabled() bool {
	return true
}

func resolveDarwinExecutorMode(config *Config) DarwinExecutorMode {
	mode := strings.TrimSpace(os.Getenv("BLUE_SANDBOX_MODE"))
	if mode == "" && config != nil {
		mode = strings.TrimSpace(config.DarwinExecutorMode)
	}
	if mode == "" {
		return DarwinExecutorModeAuto
	}
	return DarwinExecutorMode(mode)
}

func sandboxExecAvailable() bool {
	_, err := exec.LookPath("sandbox-exec")
	return err == nil
}

// generateDarwinSandboxProfile returns a sandbox-exec profile for macOS.
func generateDarwinSandboxProfile(config *Config, req *ExecutionRequest) string {
	if config == nil {
		config = DefaultConfig()
	}

	profile := `(version 1)
(deny default)

; Allow basic process operations
(allow process-fork)
(allow process-exec)

; Allow reading system files needed for execution
(allow file-read*)

; Allow writing to temp directories
(allow file-write* (subpath "/tmp"))
(allow file-write* (subpath "/private/tmp"))
(allow file-write* (subpath "/var/folders"))
(allow file-write* (subpath "/private/var/folders"))
`

	if req != nil && req.WorkDir != "" && req.WorkDir != "/tmp" {
		profile += fmt.Sprintf("(allow file-write* (subpath %q))\n", req.WorkDir)
	}

	for _, path := range config.AllowedPaths {
		if path != "/tmp" && path != "/tmp/sandbox" {
			profile += fmt.Sprintf("(allow file-write* (subpath %q))\n", path)
		}
	}

	profile += `
; Allow basic system operations
(allow sysctl-read)
(allow mach-lookup)
(allow signal (target self))

; Allow IPC for basic functionality
(allow ipc-posix-shm-read-data)
(allow ipc-posix-shm-write-data)
`

	if config.NetworkEnabled {
		profile += `
; Allow network access
(allow network-outbound)
(allow network-inbound)
(allow system-socket)
`
	} else {
		profile += `
; Deny network access
(deny network-outbound)
(deny network-inbound)
(deny system-socket)
`
	}

	return profile
}

// sandboxProfile returns a basic sandbox profile for macOS.
func sandboxProfile() string {
	return generateDarwinSandboxProfile(DefaultConfig(), nil)
}

// Ensure DarwinExecutor implements Executor
var _ Executor = (*DarwinExecutor)(nil)
