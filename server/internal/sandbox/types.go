// Package sandbox provides a sandboxed execution environment for untrusted code.
package sandbox

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// Tier identifies the sandbox isolation level requested by the caller.
type Tier string

const (
	// TierLight is the lightweight sandbox tier.
	TierLight Tier = "light"
	// TierStrong is the stronger isolation sandbox tier.
	TierStrong Tier = "strong"
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
	// LinuxLXCExecutable is the LXC CLI used for Linux strong isolation.
	LinuxLXCExecutable string
	// LinuxLXCInstance is the LXC instance name used for Linux strong isolation.
	LinuxLXCInstance string
	// WindowsCodexExecutable is the Codex CLI used for Windows strong isolation.
	WindowsCodexExecutable string
	// WindowsCodexAutoDownload controls whether Blue downloads a managed Codex
	// binary when no local executable is ready.
	WindowsCodexAutoDownload bool
	// WindowsCodexDownloadTimeout bounds background Windows Codex downloads.
	WindowsCodexDownloadTimeout time.Duration
	// WorkDir is the working directory for executions.
	WorkDir string
	// AllowedPaths are paths that can be accessed.
	AllowedPaths []string
	// DeniedPaths are paths that cannot be accessed.
	DeniedPaths []string

	// macOS Hypervisor.framework specific options
	// DarwinExecutorMode specifies which executor to use on macOS.
	// Valid values: "auto", "hypervisor", "sandbox-exec"
	// Default: "auto" (automatically selects the best available option)
	DarwinExecutorMode string
	// HypervisorVMImagePath is the path to the VM disk image for Hypervisor mode.
	HypervisorVMImagePath string
	// HypervisorMemoryMB is the VM memory in megabytes for Hypervisor mode.
	HypervisorMemoryMB int
	// HypervisorCPUCount is the number of virtual CPUs for Hypervisor mode.
	HypervisorCPUCount int
}

// DefaultConfig returns the default sandbox configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultTimeout:              5 * time.Minute,
		MaxTimeout:                  5 * time.Minute,
		MemoryLimit:                 256 * 1024 * 1024, // 256 MB
		CPULimit:                    1.0,
		ProcessLimit:                10,
		NetworkEnabled:              false,
		LinuxLXCExecutable:          "lxc",
		LinuxLXCInstance:            "blue-sandbox",
		WindowsCodexExecutable:      "codex",
		WindowsCodexAutoDownload:    true,
		WindowsCodexDownloadTimeout: 20 * time.Minute,
		WorkDir:                     "/tmp/sandbox",
		AllowedPaths:                []string{"/tmp/sandbox"},
		DeniedPaths:                 []string{"/etc", "/var", "/home", "/root"},
		DarwinExecutorMode:          "auto",
		HypervisorVMImagePath:       "/var/lib/echo/sandbox/vm.img",
		HypervisorMemoryMB:          512,
		HypervisorCPUCount:          1,
	}
}

// ExecutionRequest represents a request to execute code in the sandbox.
type ExecutionRequest struct {
	// ID is the unique execution identifier.
	ID string
	// Tier selects the sandbox isolation tier. Empty defaults to light.
	Tier Tier
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
	config *Config
	light  Executor
	strong Executor
}

type supportReasonProvider interface {
	SupportReason() string
}

type networkEnabledSupportProvider interface {
	SupportsNetworkEnabled() bool
}

// NewManager creates a new sandbox manager.
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	light, err := newPlatformExecutor(config)
	if err != nil {
		return nil, err
	}
	strong, err := newStrongPlatformExecutor(config)
	if err != nil {
		return nil, err
	}
	if config.WorkDir != "" {
		if err := os.MkdirAll(config.WorkDir, 0o755); err != nil {
			return nil, fmt.Errorf("failed to create sandbox workdir %q: %w", config.WorkDir, err)
		}
	}

	return &Manager{
		config: config,
		light:  light,
		strong: strong,
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
	if req.Tier == "" {
		req.Tier = TierLight
	}

	executor := m.executorForTier(req.Tier)
	if executor == nil || !executor.IsSupported() {
		return nil, fmt.Errorf("%w: %s sandbox tier unavailable", ErrSandboxNotSupported, req.Tier)
	}
	return executor.Execute(ctx, req)
}

// GetStatus returns the status of an execution.
func (m *Manager) GetStatus(id string) (*ExecutionResult, error) {
	if m == nil {
		return nil, ErrExecutionNotFound
	}
	for _, executor := range []Executor{m.light, m.strong} {
		if executor == nil {
			continue
		}
		result, err := executor.GetStatus(id)
		if err == nil {
			return result, nil
		}
		if !errors.Is(err, ErrExecutionNotFound) {
			return nil, err
		}
	}
	return nil, ErrExecutionNotFound
}

// Kill kills a running execution.
func (m *Manager) Kill(id string) error {
	if m == nil {
		return ErrExecutionNotFound
	}
	var found bool
	for _, executor := range []Executor{m.light, m.strong} {
		if executor == nil {
			continue
		}
		err := executor.Kill(id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrExecutionNotFound) {
			return err
		}
		found = found || !errors.Is(err, ErrExecutionNotFound)
	}
	if found {
		return nil
	}
	return ErrExecutionNotFound
}

// Cleanup cleans up resources.
func (m *Manager) Cleanup() error {
	if m == nil {
		return nil
	}
	cleaned := map[Executor]struct{}{}
	for _, executor := range []Executor{m.light, m.strong} {
		if executor == nil {
			continue
		}
		if _, ok := cleaned[executor]; ok {
			continue
		}
		if err := executor.Cleanup(); err != nil {
			return err
		}
		cleaned[executor] = struct{}{}
	}
	return nil
}

// IsSupported returns true if sandboxing is supported on this platform.
func (m *Manager) IsSupported() bool {
	return m.supportsExecutor(m.light) || m.supportsExecutor(m.strong)
}

// SupportReason returns a human-readable reason when sandboxing is unavailable.
func (m *Manager) SupportReason() string {
	if m == nil || m.IsSupported() {
		return ""
	}
	for _, executor := range []Executor{m.light, m.strong} {
		if reason := supportReason(executor); reason != "" {
			return reason
		}
	}
	return ErrSandboxNotSupported.Error()
}

// SupportsNetworkEnabled reports whether this sandbox backend can enforce the
// NetworkEnabled runtime toggle.
func (m *Manager) SupportsNetworkEnabled() bool {
	if m == nil {
		return false
	}
	var checked bool
	for _, executor := range []Executor{m.light, m.strong} {
		if executor == nil || !executor.IsSupported() {
			continue
		}
		checked = true
		provider, ok := executor.(networkEnabledSupportProvider)
		if !ok || !provider.SupportsNetworkEnabled() {
			return false
		}
	}
	return checked
}

// SupportsTier reports whether the requested sandbox tier is currently available.
func (m *Manager) SupportsTier(tier Tier) bool {
	return m.supportsExecutor(m.executorForTier(tier))
}

// GetConfig returns the sandbox configuration.
func (m *Manager) GetConfig() *Config {
	return m.config
}

func (m *Manager) executorForTier(tier Tier) Executor {
	if m == nil {
		return nil
	}
	switch tier {
	case TierStrong:
		return m.strong
	default:
		return m.light
	}
}

func (m *Manager) supportsExecutor(executor Executor) bool {
	return executor != nil && executor.IsSupported()
}

func supportReason(executor Executor) string {
	if executor == nil || executor.IsSupported() {
		return ""
	}
	if provider, ok := executor.(supportReasonProvider); ok {
		return provider.SupportReason()
	}
	return ""
}
