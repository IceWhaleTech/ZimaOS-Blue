package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const defaultCodexSandboxProbeTimeout = 2 * time.Second

// CodexSandboxExecutorOptions controls how a Codex-backed sandbox executor is created.
type CodexSandboxExecutorOptions struct {
	Binary         string
	BinaryResolver codexBinaryResolver
	Subcommand     string
	ProbeTimeout   time.Duration
	LookPath       func(file string) (string, error)
	CommandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
}

// CodexSandboxExecutor wraps the local Codex CLI sandbox subcommands.
type CodexSandboxExecutor struct {
	*BaseExecutor
	binary         string
	subcommand     string
	probeTimeout   time.Duration
	lookPath       func(file string) (string, error)
	commandContext func(ctx context.Context, name string, args ...string) *exec.Cmd
	binaryResolver codexBinaryResolver
	binaryHint     string
	binaryMu       sync.Mutex
	supportReason  string
}

// NewCodexSandboxExecutor creates a Codex-backed sandbox executor.
func NewCodexSandboxExecutor(config *Config, options CodexSandboxExecutorOptions) (*CodexSandboxExecutor, error) {
	if config == nil {
		config = DefaultConfig()
	}

	executor := &CodexSandboxExecutor{
		BaseExecutor:   NewBaseExecutor(config),
		subcommand:     strings.TrimSpace(options.Subcommand),
		probeTimeout:   options.ProbeTimeout,
		lookPath:       options.LookPath,
		commandContext: options.CommandContext,
	}
	if executor.subcommand == "" {
		executor.subcommand = "windows"
	}
	if executor.probeTimeout <= 0 {
		executor.probeTimeout = defaultCodexSandboxProbeTimeout
	}
	if executor.lookPath == nil {
		executor.lookPath = exec.LookPath
	}
	if executor.commandContext == nil {
		executor.commandContext = exec.CommandContext
	}

	executor.binaryHint = strings.TrimSpace(options.Binary)
	if executor.binaryHint == "" && config != nil {
		executor.binaryHint = strings.TrimSpace(config.WindowsCodexExecutable)
	}
	if executor.binaryHint == "" {
		executor.binaryHint = "codex"
	}
	executor.binaryResolver = options.BinaryResolver
	if executor.binaryResolver == nil {
		executor.binaryResolver = NewCodexBinaryManager(configWithCodexBinaryOverride(config, options.Binary), CodexBinaryManagerOptions{
			LookPath: executor.lookPath,
		})
	}

	if config.NetworkEnabled {
		executor.supportReason = "codex sandbox currently requires network_enabled=false"
		return executor, nil
	}

	readyBinary, ok, err := executor.binaryResolver.ReadyPath()
	switch {
	case err != nil && !executor.binaryResolver.AutoDownloadEnabled():
		executor.supportReason = err.Error()
	case ok:
		if probeErr := executor.probeBinary(readyBinary); probeErr == nil {
			executor.binary = readyBinary
		} else if !executor.binaryResolver.AutoDownloadEnabled() {
			executor.supportReason = probeErr.Error()
		}
	case !executor.binaryResolver.AutoDownloadEnabled():
		executor.supportReason = executor.missingBinaryReason()
	}

	return executor, nil
}

// Execute executes a command under the requested Codex sandbox subcommand.
func (e *CodexSandboxExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	if !e.IsSupported() {
		return nil, fmt.Errorf("%w: %s", ErrSandboxNotSupported, e.SupportReason())
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Timeout <= 0 {
		req.Timeout = e.config.DefaultTimeout
	}

	execCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()

	binary, err := e.ensureBinaryReady(execCtx)
	if err != nil {
		return nil, err
	}

	cmd := e.commandContext(execCtx, binary, e.buildCommandArgs(req)...)
	if req.WorkDir != "" {
		cmd.Dir = req.WorkDir
	}

	env := append([]string{}, os.Environ()...)
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	cmd.Env = env

	if req.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(req.Stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

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

	err = cmd.Run()
	result.EndTime = timeutil.NowTime()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = truncateOutput(stdout.String(), 1024*1024)
	result.Stderr = truncateOutput(stderr.String(), 1024*1024)

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

// IsSupported reports whether the Codex sandbox probe succeeded.
func (e *CodexSandboxExecutor) IsSupported() bool {
	return e != nil && strings.TrimSpace(e.supportReason) == ""
}

// SupportReason returns the cached unavailability reason.
func (e *CodexSandboxExecutor) SupportReason() string {
	if e == nil {
		return ""
	}
	return e.supportReason
}

// SupportsNetworkEnabled reports whether the executor can honor Blue's unified
// network toggle. The current Codex integration is intentionally conservative.
func (e *CodexSandboxExecutor) SupportsNetworkEnabled() bool {
	return false
}

func (e *CodexSandboxExecutor) buildCommandArgs(req *ExecutionRequest) []string {
	args := []string{"sandbox", e.subcommand, "--full-auto", req.Command}
	args = append(args, req.Args...)
	return args
}

func (e *CodexSandboxExecutor) probe() error {
	return e.probeBinary(e.binary)
}

func (e *CodexSandboxExecutor) probeBinary(binary string) error {
	ctx, cancel := context.WithTimeout(context.Background(), e.probeTimeout)
	defer cancel()

	cmd := e.commandContext(ctx, binary, "sandbox", e.subcommand, "--help")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("codex sandbox %s probe timed out: %w", e.subcommand, err)
		}
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("codex sandbox %s probe failed: %s", e.subcommand, detail)
		}
		return fmt.Errorf("codex sandbox %s probe failed: %w", e.subcommand, err)
	}
	return nil
}

func (e *CodexSandboxExecutor) ensureBinaryReady(ctx context.Context) (string, error) {
	if e == nil || e.binaryResolver == nil {
		return "", fmt.Errorf("%w: codex sandbox binary resolver is not configured", ErrSandboxNotSupported)
	}

	e.binaryMu.Lock()
	defer e.binaryMu.Unlock()

	if strings.TrimSpace(e.binary) != "" {
		return e.binary, nil
	}

	readyBinary, ok, err := e.binaryResolver.ReadyPath()
	switch {
	case err != nil:
		if !e.binaryResolver.AutoDownloadEnabled() {
			return "", wrapCodexSandboxUnsupported(err)
		}
		return e.downloadManagedBinary(ctx)
	case ok:
		probeErr := e.probeBinary(readyBinary)
		if probeErr == nil {
			e.binary = readyBinary
			return e.binary, nil
		}
		if !e.binaryResolver.AutoDownloadEnabled() {
			return "", wrapCodexSandboxUnsupported(probeErr)
		}
		return e.downloadManagedBinary(ctx)
	default:
		if !e.binaryResolver.AutoDownloadEnabled() {
			return "", fmt.Errorf("%w: %s", ErrSandboxNotSupported, e.missingBinaryReason())
		}
		return e.ensureProvisionedBinary(ctx)
	}
}

func (e *CodexSandboxExecutor) ensureProvisionedBinary(ctx context.Context) (string, error) {
	binary, err := e.binaryResolver.Ensure(ctx)
	if err != nil {
		return "", wrapCodexSandboxUnsupported(err)
	}
	if err := e.probeBinary(binary); err != nil {
		return "", wrapCodexSandboxUnsupported(err)
	}
	e.binary = binary
	return e.binary, nil
}

func (e *CodexSandboxExecutor) downloadManagedBinary(ctx context.Context) (string, error) {
	binary, err := e.binaryResolver.Download(ctx)
	if err != nil {
		return "", wrapCodexSandboxUnsupported(err)
	}
	if err := e.probeBinary(binary); err != nil {
		return "", wrapCodexSandboxUnsupported(err)
	}
	e.binary = binary
	return e.binary, nil
}

func (e *CodexSandboxExecutor) missingBinaryReason() string {
	binary := strings.TrimSpace(e.binaryHint)
	if binary == "" {
		binary = "codex"
	}
	return fmt.Sprintf("resolve codex sandbox binary %q: not found", binary)
}

func wrapCodexSandboxUnsupported(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSandboxNotSupported) {
		return err
	}
	return fmt.Errorf("%w: %v", ErrSandboxNotSupported, err)
}

func configWithCodexBinaryOverride(config *Config, binaryOverride string) *Config {
	if config == nil {
		config = DefaultConfig()
	}
	binaryOverride = strings.TrimSpace(binaryOverride)
	if binaryOverride == "" {
		return config
	}
	cloned := *config
	cloned.WindowsCodexExecutable = binaryOverride
	return &cloned
}

var _ Executor = (*CodexSandboxExecutor)(nil)
