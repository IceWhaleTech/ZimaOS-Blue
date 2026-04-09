//go:build windows

package sandbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"golang.org/x/sys/windows"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const windowsSandboxProbeTimeout = 2 * time.Second

// WindowsExecutor provides sandboxed execution on Windows using Job Objects for
// process-tree control and resource limits.
type WindowsExecutor struct {
	*BaseExecutor
	jobHandles map[string]windows.Handle
}

// newPlatformExecutor creates a new Windows executor.
func newPlatformExecutor(config *Config) (Executor, error) {
	if config == nil {
		config = DefaultConfig()
	}

	if err := prepareWindowsSandboxConfig(config); err != nil {
		return newUnsupportedExecutor(err.Error()), nil
	}

	executor := &WindowsExecutor{
		BaseExecutor: NewBaseExecutor(config),
		jobHandles:   make(map[string]windows.Handle),
	}
	if err := executor.probeIsolation(); err != nil {
		_ = executor.Cleanup()
		return newUnsupportedExecutor(err.Error()), nil
	}

	return executor, nil
}

func newStrongPlatformExecutor(config *Config) (Executor, error) {
	return NewCodexSandboxExecutor(config, CodexSandboxExecutorOptions{
		Subcommand: "windows",
	})
}

// Execute executes a command with Windows-specific isolation.
func (e *WindowsExecutor) Execute(ctx context.Context, req *ExecutionRequest) (*ExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if req.Timeout <= 0 {
		req.Timeout = e.config.DefaultTimeout
	}

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
		env := append([]string{}, os.Environ()...)
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
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB,
		// Hide the console window
		HideWindow: true,
	}

	job, err := e.createJobObject(req)
	if err != nil {
		return nil, fmt.Errorf("create Windows sandbox job object: %w", err)
	}
	defer windows.CloseHandle(job)

	cmd.Cancel = func() error {
		if termErr := windows.TerminateJobObject(job, 1); termErr != nil && !errors.Is(termErr, windows.ERROR_ACCESS_DENIED) {
			if cmd.Process != nil {
				if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
					return killErr
				}
			}
			return termErr
		}
		return os.ErrProcessDone
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

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	processHandle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		killStartedCommand(cmd)
		return nil, fmt.Errorf("open sandbox process handle: %w", err)
	}
	assignErr := windows.AssignProcessToJobObject(job, processHandle)
	windows.CloseHandle(processHandle)
	if assignErr != nil {
		killStartedCommand(cmd)
		return nil, fmt.Errorf("assign sandbox process to job object: %w", assignErr)
	}

	e.mu.Lock()
	e.executions[req.ID] = state
	e.jobHandles[req.ID] = job
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		delete(e.jobHandles, req.ID)
		e.mu.Unlock()
	}()

	// Run command
	err = cmd.Wait()
	result.EndTime = timeutil.NowTime()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.Stdout = truncateOutput(stdout.String(), 1024*1024)
	result.Stderr = truncateOutput(stderr.String(), 1024*1024)
	result.ResourceUsage = windowsResourceUsage(cmd.ProcessState, job)

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

func (e *WindowsExecutor) SupportsNetworkEnabled() bool {
	return false
}

// Kill kills a running execution and its child-process tree.
func (e *WindowsExecutor) Kill(id string) error {
	e.mu.RLock()
	state, ok := e.executions[id]
	job := e.jobHandles[id]
	e.mu.RUnlock()

	if !ok {
		return ErrExecutionNotFound
	}

	if state.cancel != nil {
		state.cancel()
	}
	if job != 0 {
		if err := windows.TerminateJobObject(job, 1); err != nil && !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			return err
		}
		return nil
	}
	if state.cmd != nil && state.cmd.Process != nil {
		if err := state.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
	}

	return nil
}

// Cleanup cleans up resources including active Job Objects.
func (e *WindowsExecutor) Cleanup() error {
	e.mu.RLock()
	jobs := make([]windows.Handle, 0, len(e.jobHandles))
	for _, job := range e.jobHandles {
		if job != 0 {
			jobs = append(jobs, job)
		}
	}
	e.mu.RUnlock()

	for _, job := range jobs {
		_ = windows.TerminateJobObject(job, 1)
	}

	return e.BaseExecutor.Cleanup()
}

func (e *WindowsExecutor) probeIsolation() error {
	req := NewExecutionRequest("cmd", "/c", "exit", "0")
	req.Timeout = windowsSandboxProbeTimeout
	req.WorkDir = e.config.WorkDir

	ctx, cancel := context.WithTimeout(context.Background(), windowsSandboxProbeTimeout)
	defer cancel()

	result, err := e.Execute(ctx, req)
	if err != nil {
		return fmt.Errorf("windows sandbox probe failed: %w", err)
	}
	if result == nil {
		return fmt.Errorf("windows sandbox probe returned no result")
	}
	if result.Status != StatusCompleted {
		return fmt.Errorf("windows sandbox probe exited with status %s", result.Status)
	}

	e.mu.Lock()
	delete(e.executions, req.ID)
	delete(e.jobHandles, req.ID)
	e.mu.Unlock()
	return nil
}

func (e *WindowsExecutor) createJobObject(req *ExecutionRequest) (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}

	var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE |
		windows.JOB_OBJECT_LIMIT_DIE_ON_UNHANDLED_EXCEPTION

	if req.MemoryLimit > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_PROCESS_MEMORY
		info.ProcessMemoryLimit = uintptr(req.MemoryLimit)
	}
	if e.config != nil && e.config.ProcessLimit > 0 {
		info.BasicLimitInformation.LimitFlags |= windows.JOB_OBJECT_LIMIT_ACTIVE_PROCESS
		info.BasicLimitInformation.ActiveProcessLimit = uint32(e.config.ProcessLimit)
	}

	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return 0, err
	}

	return job, nil
}

func prepareWindowsSandboxConfig(config *Config) error {
	if config == nil {
		return nil
	}

	if looksLikeUnixSandboxPath(config.WorkDir) || strings.TrimSpace(config.WorkDir) == "" {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return fmt.Errorf("resolve windows sandbox home directory: %w", err)
		}
		workDir := filepath.Join(home, ".zimaos-blue", "data", "sandbox")
		config.WorkDir = workDir
		if len(config.AllowedPaths) == 0 || (len(config.AllowedPaths) == 1 && looksLikeUnixSandboxPath(config.AllowedPaths[0])) {
			config.AllowedPaths = []string{workDir}
		}
	}

	if config.WorkDir == "" {
		return fmt.Errorf("windows sandbox workdir is empty")
	}
	if err := os.MkdirAll(config.WorkDir, 0755); err != nil {
		return fmt.Errorf("failed to create windows sandbox workdir %q: %w", config.WorkDir, err)
	}

	return nil
}

func looksLikeUnixSandboxPath(path string) bool {
	return filepath.ToSlash(strings.TrimSpace(path)) == "/tmp/sandbox"
}

func windowsResourceUsage(state *os.ProcessState, job windows.Handle) *ResourceUsage {
	usage := &ResourceUsage{}
	hasUsage := false

	if state != nil {
		usage.CPUTime = state.UserTime().Nanoseconds() + state.SystemTime().Nanoseconds()
		hasUsage = true
	}

	if job != 0 {
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		if err := windows.QueryInformationJobObject(
			job,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
			nil,
		); err == nil {
			usage.MemoryPeak = int64(info.PeakProcessMemoryUsed)
			usage.IORead = int64(info.IoInfo.ReadTransferCount)
			usage.IOWrite = int64(info.IoInfo.WriteTransferCount)
			hasUsage = true
		}
	}

	if !hasUsage {
		return nil
	}
	return usage
}

func killStartedCommand(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}

// Ensure WindowsExecutor implements Executor
var _ Executor = (*WindowsExecutor)(nil)
