// Package sandbox provides a sandboxed execution environment for untrusted code.
package sandbox

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrExecutionTimeout is returned when execution times out.
	ErrExecutionTimeout = errors.New("execution timeout")
	// ErrExecutionKilled is returned when execution is killed.
	ErrExecutionKilled = errors.New("execution killed")
	// ErrResourceLimitExceeded is returned when resource limits are exceeded.
	ErrResourceLimitExceeded = errors.New("resource limit exceeded")
	// ErrSandboxNotSupported is returned when sandboxing is not supported on the platform.
	ErrSandboxNotSupported = errors.New("sandbox not supported on this platform")
	// ErrExecutionNotFound is returned when an execution is not found.
	ErrExecutionNotFound = errors.New("execution not found")
)

// Config holds sandbox configuration.
type Config struct {
	// DefaultTimeout is the default execution timeout.
	DefaultTimeout time.Duration
	// MaxTimeout is the maximum allowed timeout.
	MaxTimeout time.Duration
	// MemoryLimit is the memory limit in bytes.
	MemoryLimit int64
	// CPULimit is the CPU limit (1.0 = 1 core).
	CPULimit float64
	// ProcessLimit is the maximum number of processes.
	ProcessLimit int
	// NetworkEnabled allows network access.
	NetworkEnabled bool
	// WorkDir is the working directory for executions.
	WorkDir string
	// AllowedPaths are paths that can be accessed.
	AllowedPaths []string
	// DeniedPaths are paths that cannot be accessed.
	DeniedPaths []string
}

// DefaultConfig returns the default sandbox configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout: 30 * time.Second,
		MaxTimeout:     5 * time.Minute,
		MemoryLimit:    256 * 1024 * 1024, // 256 MB
		CPULimit:       1.0,
		ProcessLimit:   10,
		NetworkEnabled: false,
		WorkDir:        "/tmp/sandbox",
		AllowedPaths:   []string{"/tmp/sandbox"},
		DeniedPaths:    []string{"/etc", "/var", "/home", "/root"},
	}
}

// ExecutionRequest represents a request to execute code in the sandbox.
type ExecutionRequest struct {
	// ID is the unique execution identifier.
	ID string
	// Command is the command to execute.
	Command string
	// Args are the command arguments.
	Args []string
	// Env are environment variables.
	Env map[string]string
	// WorkDir is the working directory.
	WorkDir string
	// Timeout is the execution timeout.
	Timeout time.Duration
	// Stdin is the standard input.
	Stdin string
	// MemoryLimit overrides the default memory limit.
	MemoryLimit int64
	// CPULimit overrides the default CPU limit.
	CPULimit float64
}

// NewExecutionRequest creates a new execution request.
func NewExecutionRequest(command string, args ...string) *ExecutionRequest {
	return &ExecutionRequest{
		ID:      uuid.New().String(),
		Command: command,
		Args:    args,
		Env:     make(map[string]string),
	}
}

// ExecutionStatus represents the status of an execution.
type ExecutionStatus string

const (
	// StatusPending indicates the execution is pending.
	StatusPending ExecutionStatus = "pending"
	// StatusRunning indicates the execution is running.
	StatusRunning ExecutionStatus = "running"
	// StatusCompleted indicates the execution completed successfully.
	StatusCompleted ExecutionStatus = "completed"
	// StatusFailed indicates the execution failed.
	StatusFailed ExecutionStatus = "failed"
	// StatusTimeout indicates the execution timed out.
	StatusTimeout ExecutionStatus = "timeout"
	// StatusKilled indicates the execution was killed.
	StatusKilled ExecutionStatus = "killed"
)

// ExecutionResult represents the result of an execution.
type ExecutionResult struct {
	// ID is the execution identifier.
	ID string `json:"id"`
	// Status is the execution status.
	Status ExecutionStatus `json:"status"`
	// ExitCode is the process exit code.
	ExitCode int `json:"exit_code"`
	// Stdout is the standard output.
	Stdout string `json:"stdout"`
	// Stderr is the standard error.
	Stderr string `json:"stderr"`
	// StartTime is when the execution started.
	StartTime time.Time `json:"start_time"`
	// EndTime is when the execution ended.
	EndTime time.Time `json:"end_time"`
	// Duration is the execution duration.
	Duration time.Duration `json:"duration"`
	// ResourceUsage contains resource usage statistics.
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`
	// Error contains any error message.
	Error string `json:"error,omitempty"`
}

// ResourceUsage contains resource usage statistics.
type ResourceUsage struct {
	// CPUTime is the CPU time used in nanoseconds.
	CPUTime int64 `json:"cpu_time_ns"`
	// MemoryPeak is the peak memory usage in bytes.
	MemoryPeak int64 `json:"memory_peak_bytes"`
	// IORead is the number of bytes read.
	IORead int64 `json:"io_read_bytes"`
	// IOWrite is the number of bytes written.
	IOWrite int64 `json:"io_write_bytes"`
}

// Executor is the interface for sandbox executors.
type Executor interface {
	// Execute executes a command in the sandbox.
	Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error)
	// GetStatus returns the status of an execution.
	GetStatus(id string) (*ExecutionResult, error)
	// Kill kills a running execution.
	Kill(id string) error
	// Cleanup cleans up resources.
	Cleanup() error
	// IsSupported returns true if the executor is supported on this platform.
	IsSupported() bool
}

// Manager manages sandbox executions.
type Manager struct {
	config   *Config
	executor Executor
}

// NewManager creates a new sandbox manager.
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	executor, err := newPlatformExecutor(config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		config:   config,
		executor: executor,
	}, nil
}

// Execute executes a command in the sandbox.
func (m *Manager) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	// Apply defaults
	if req.Timeout == 0 {
		req.Timeout = m.config.DefaultTimeout
	}
	if req.Timeout > m.config.MaxTimeout {
		req.Timeout = m.config.MaxTimeout
	}
	if req.MemoryLimit == 0 {
		req.MemoryLimit = m.config.MemoryLimit
	}
	if req.CPULimit == 0 {
		req.CPULimit = m.config.CPULimit
	}
	if req.WorkDir == "" {
		req.WorkDir = m.config.WorkDir
	}

	return m.executor.Execute(ctx, req)
}

// GetStatus returns the status of an execution.
func (m *Manager) GetStatus(id string) (*ExecutionResult, error) {
	return m.executor.GetStatus(id)
}

// Kill kills a running execution.
func (m *Manager) Kill(id string) error {
	return m.executor.Kill(id)
}

// Cleanup cleans up resources.
func (m *Manager) Cleanup() error {
	return m.executor.Cleanup()
}

// IsSupported returns true if sandboxing is supported on this platform.
func (m *Manager) IsSupported() bool {
	return m.executor.IsSupported()
}

// GetConfig returns the sandbox configuration.
func (m *Manager) GetConfig() *Config {
	return m.config
}
