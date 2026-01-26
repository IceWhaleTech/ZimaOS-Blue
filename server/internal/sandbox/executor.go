package sandbox

import (
	"bytes"
	"context"
	"os/exec"
	"sync"
	"time"
)

// BaseExecutor provides common functionality for all executors.
type BaseExecutor struct {
	config     *Config
	mu         sync.RWMutex
	executions map[string]*executionState
}

type executionState struct {
	result *ExecutionResult
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// NewBaseExecutor creates a new base executor.
func NewBaseExecutor(config *Config) *BaseExecutor {
	return &BaseExecutor{
		config:     config,
		executions: make(map[string]*executionState),
	}
}

// Execute executes a command with basic isolation.
func (e *BaseExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Create execution context with timeout
	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)

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
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

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

// GetStatus returns the status of an execution.
func (e *BaseExecutor) GetStatus(id string) (*ExecutionResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.executions[id]
	if !ok {
		return nil, ErrExecutionNotFound
	}

	return state.result, nil
}

// Kill kills a running execution.
func (e *BaseExecutor) Kill(id string) error {
	e.mu.RLock()
	state, ok := e.executions[id]
	e.mu.RUnlock()

	if !ok {
		return ErrExecutionNotFound
	}

	if state.cancel != nil {
		state.cancel()
	}

	if state.cmd != nil && state.cmd.Process != nil {
		return state.cmd.Process.Kill()
	}

	return nil
}

// Cleanup cleans up resources.
func (e *BaseExecutor) Cleanup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Kill all running executions
	for _, state := range e.executions {
		if state.cancel != nil {
			state.cancel()
		}
		if state.cmd != nil && state.cmd.Process != nil {
			_ = state.cmd.Process.Kill()
		}
	}

	e.executions = make(map[string]*executionState)
	return nil
}

// IsSupported returns true (base executor is always supported).
func (e *BaseExecutor) IsSupported() bool {
	return true
}

// truncateOutput truncates output to a maximum length.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
