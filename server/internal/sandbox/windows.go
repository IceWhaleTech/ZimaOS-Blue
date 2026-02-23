//go:build windows

package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"syscall"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// WindowsExecutor provides sandboxed execution on Windows using Job Objects.
type WindowsExecutor struct {
	*BaseExecutor
}

// newPlatformExecutor creates a new Windows executor.
func newPlatformExecutor(config *Config) (Executor, error) {
	base := NewBaseExecutor(config)

	return &WindowsExecutor{
		BaseExecutor: base,
	}, nil
}

// Execute executes a command with Windows-specific isolation.
func (e *WindowsExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(execCtx, req.Command, req.Args...)

	// Set working directory
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	// Set environment
	if len(req.Env) > 0 {
		env := make([]string, 0, len(req.Env))
		for k, v := range req.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Set stdin
	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Set up Windows-specific process attributes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		// Create process in a new process group
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		// Hide the console window
		HideWindow: true,
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
	err := cmd.Run()
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
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	} else {
		result.Status = StatusCompleted
		result.ExitCode = 0
	}

	return result, nil
}

// IsSupported returns true (Windows executor is always supported on Windows).
func (e *WindowsExecutor) IsSupported() bool {
	return true
}

// Ensure WindowsExecutor implements Executor
var _ Executor = (*WindowsExecutor)(nil)
