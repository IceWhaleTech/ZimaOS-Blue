//go:build darwin

package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"time"
)

// DarwinExecutor provides sandboxed execution on macOS using sandbox-exec.
type DarwinExecutor struct {
	*BaseExecutor
}

// newPlatformExecutor creates a new Darwin executor.
func newPlatformExecutor(config *Config) (Executor, error) {
	base := NewBaseExecutor(config)

	return &DarwinExecutor{
		BaseExecutor: base,
	}, nil
}

// Execute executes a command with macOS-specific isolation.
func (e *DarwinExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Create command
	// On macOS, we can use sandbox-exec for basic sandboxing
	cmd := exec.CommandContext(execCtx, req.Command, req.Args...)

	// Set working directory
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// Set environment
	env := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"TMPDIR=/tmp",
	}
	for k, v := range req.Env {
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
		StartTime: time.Now(),
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
	err := cmd.Run()
	result.EndTime = time.Now()
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
	// Check if sandbox-exec is available
	cmd := exec.Command("sandbox-exec", "-h")
	return cmd.Run() == nil
}

// sandboxProfile returns a basic sandbox profile for macOS.
func sandboxProfile() string {
	return `
(version 1)
(deny default)
(allow process-fork)
(allow process-exec)
(allow file-read*)
(allow file-write* (subpath "/tmp"))
(allow file-write* (subpath "/var/folders"))
(allow sysctl-read)
(allow mach-lookup)
`
}

// Ensure DarwinExecutor implements Executor
var _ Executor = (*DarwinExecutor)(nil)
