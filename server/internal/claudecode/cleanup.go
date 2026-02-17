package claudecode

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"go.uber.org/zap"
)

// ProcessCleaner handles cleanup of orphaned CLI processes.
type ProcessCleaner struct {
	logger   *zap.Logger
	mu       sync.Mutex
	tracked  map[int]*trackedProcess
	stopCh   chan struct{}
	interval time.Duration
}

// trackedProcess represents a tracked CLI process.
type trackedProcess struct {
	PID       int
	Command   string
	SessionId string
	StartTime time.Time
	Timeout   time.Duration
}

// NewProcessCleaner creates a new ProcessCleaner.
func NewProcessCleaner(logger *zap.Logger, cleanupInterval time.Duration) *ProcessCleaner {
	if cleanupInterval == 0 {
		cleanupInterval = 5 * time.Minute
	}
	return &ProcessCleaner{
		logger:   logger,
		tracked:  make(map[int]*trackedProcess),
		stopCh:   make(chan struct{}),
		interval: cleanupInterval,
	}
}

// Start begins the background cleanup routine.
func (c *ProcessCleaner) Start() {
	go c.cleanupLoop()
}

// Stop stops the background cleanup routine.
func (c *ProcessCleaner) Stop() {
	close(c.stopCh)
}

// Track adds a process to the tracked list.
func (c *ProcessCleaner) Track(pid int, command, sessionId string, timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tracked[pid] = &trackedProcess{
		PID:       pid,
		Command:   command,
		SessionId: sessionId,
		StartTime: timeutil.NowTime(),
		Timeout:   timeout,
	}
}

// Untrack removes a process from the tracked list.
func (c *ProcessCleaner) Untrack(pid int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.tracked, pid)
}

// cleanupLoop runs the periodic cleanup.
func (c *ProcessCleaner) cleanupLoop() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.cleanup()
		case <-c.stopCh:
			return
		}
	}
}

// cleanup checks for and terminates orphaned processes.
func (c *ProcessCleaner) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := timeutil.NowNano()
	for pid, proc := range c.tracked {
		// Check if process has exceeded its timeout
		if now > proc.StartTime.Add(proc.Timeout).UnixNano() {
			elapsed := time.Duration(now - proc.StartTime.UnixNano())
			c.logger.Warn("Terminating orphaned CLI process",
				zap.Int("pid", pid),
				zap.String("command", proc.Command),
				zap.String("session_id", proc.SessionId),
				zap.Duration("elapsed", elapsed),
			)

			if err := c.killProcess(pid); err != nil {
				c.logger.Error("Failed to kill orphaned process",
					zap.Int("pid", pid),
					zap.Error(err),
				)
			}

			delete(c.tracked, pid)
		}
	}
}

// killProcess terminates a process by PID.
func (c *ProcessCleaner) killProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	// Try graceful termination first
	if err := process.Signal(os.Interrupt); err != nil {
		// If interrupt fails, force kill
		return process.Kill()
	}

	// Wait briefly for graceful shutdown
	done := make(chan error, 1)
	go func() {
		_, err := process.Wait()
		done <- err
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		// Force kill if graceful shutdown takes too long
		return process.Kill()
	}
}

// CleanupSuspendedProcesses finds and terminates suspended Claude CLI processes.
func CleanupSuspendedProcesses(ctx context.Context, logger *zap.Logger, command string) error {
	if command == "" {
		command = "claude"
	}

	// Find processes matching the command
	pids, err := findProcessesByCommand(ctx, command)
	if err != nil {
		return err
	}

	for _, pid := range pids {
		// Check if process is suspended (stopped state)
		if isSuspended(pid) {
			logger.Info("Found suspended CLI process, terminating",
				zap.Int("pid", pid),
				zap.String("command", command),
			)

			process, err := os.FindProcess(pid)
			if err != nil {
				logger.Error("Failed to find process", zap.Int("pid", pid), zap.Error(err))
				continue
			}

			if err := process.Kill(); err != nil {
				logger.Error("Failed to kill suspended process", zap.Int("pid", pid), zap.Error(err))
			}
		}
	}

	return nil
}

// CleanupResumeProcesses cleans up orphaned resume processes for a session.
func CleanupResumeProcesses(ctx context.Context, logger *zap.Logger, sessionId string) error {
	// Find processes with the session ID in their arguments
	cmd := exec.CommandContext(ctx, "pgrep", "-f", sessionId)
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil
		}
		return err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}

		logger.Info("Found orphaned resume process, terminating",
			zap.Int("pid", pid),
			zap.String("session_id", sessionId),
		)

		process, err := os.FindProcess(pid)
		if err != nil {
			logger.Error("Failed to find process", zap.Int("pid", pid), zap.Error(err))
			continue
		}

		if err := process.Kill(); err != nil {
			logger.Error("Failed to kill resume process", zap.Int("pid", pid), zap.Error(err))
		}
	}

	return nil
}

// findProcessesByCommand finds all processes matching a command name.
func findProcessesByCommand(ctx context.Context, command string) ([]int, error) {
	cmd := exec.CommandContext(ctx, "pgrep", "-f", command)
	output, err := cmd.Output()
	if err != nil {
		// pgrep returns exit code 1 if no processes found
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	var pids []int
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		pid, err := strconv.Atoi(line)
		if err != nil {
			continue
		}
		pids = append(pids, pid)
	}

	return pids, nil
}

// isSuspended checks if a process is in suspended (stopped) state.
func isSuspended(pid int) bool {
	// Read process state from /proc on Linux
	statPath := "/proc/" + strconv.Itoa(pid) + "/stat"
	data, err := os.ReadFile(statPath)
	if err != nil {
		return false
	}

	// Parse the stat file - state is the 3rd field
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return false
	}

	// State 'T' means stopped (suspended)
	state := fields[2]
	return state == "T"
}
