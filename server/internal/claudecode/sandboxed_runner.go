package claudecode

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// SandboxedRunner wraps a Runner with sandbox execution support.
type SandboxedRunner struct {
	runner         *Runner
	sandboxManager *sandbox.Manager
	config         *ClaudeCodeConfig
	enabled        bool
}

// NewSandboxedRunner creates a new SandboxedRunner.
func NewSandboxedRunner(config *ClaudeCodeConfig) (*SandboxedRunner, error) {
	runner := NewRunner(config)

	sr := &SandboxedRunner{
		runner:  runner,
		config:  config,
		enabled: config.Sandbox.Enabled,
	}

	// Initialize sandbox manager if enabled
	if config.Sandbox.Enabled {
		sandboxConfig := sr.buildSandboxConfig()
		manager, err := sandbox.NewManager(sandboxConfig)
		if err != nil {
			// Fall back to non-sandboxed execution if sandbox is not supported
			sr.enabled = false
		} else {
			sr.sandboxManager = manager
		}
	}

	return sr, nil
}

// buildSandboxConfig builds sandbox configuration from ClaudeCodeConfig.
func (sr *SandboxedRunner) buildSandboxConfig() *sandbox.Config {
	cfg := sr.config.Sandbox

	// Build allowed paths
	allowedPaths := make([]string, 0, len(cfg.AllowedPaths)+1)
	if sr.config.WorkspaceDir != "" {
		absPath, err := filepath.Abs(sr.config.WorkspaceDir)
		if err == nil {
			allowedPaths = append(allowedPaths, absPath)
		} else {
			allowedPaths = append(allowedPaths, sr.config.WorkspaceDir)
		}
	}
	allowedPaths = append(allowedPaths, cfg.AllowedPaths...)

	return &sandbox.Config{
		DefaultTimeout: sr.config.Timeout,
		MaxTimeout:     sr.config.Timeout * 2,
		MemoryLimit:    cfg.MemoryLimit,
		CPULimit:       cfg.CPULimit,
		ProcessLimit:   cfg.ProcessLimit,
		NetworkEnabled: cfg.NetworkEnabled,
		WorkDir:        sr.config.WorkspaceDir,
		AllowedPaths:   allowedPaths,
		DeniedPaths:    cfg.DeniedPaths,
	}
}

// Run executes a CLI command, using sandbox if enabled.
func (sr *SandboxedRunner) Run(ctx context.Context, params *RunParams) (*RunResult, error) {
	if !sr.enabled || sr.sandboxManager == nil {
		return sr.runner.Run(ctx, params)
	}

	return sr.runSandboxed(ctx, params)
}

// RunStream executes a CLI command with streaming, using sandbox if enabled.
func (sr *SandboxedRunner) RunStream(ctx context.Context, params *RunParams) (<-chan CliStreamChunk, error) {
	if !sr.enabled || sr.sandboxManager == nil {
		return sr.runner.RunStream(ctx, params)
	}

	// For streaming, we need to run in sandbox and stream the output
	return sr.runStreamSandboxed(ctx, params)
}

// runSandboxed executes a command in the sandbox.
func (sr *SandboxedRunner) runSandboxed(ctx context.Context, params *RunParams) (*RunResult, error) {
	startTime := timeutil.NowTime()

	// Apply defaults
	if params.Backend == nil {
		backend := sr.config.Backend.WithDefaults()
		params.Backend = &backend
	}
	if params.Timeout == 0 {
		params.Timeout = sr.config.Timeout
	}
	if params.WorkspaceDir == "" {
		params.WorkspaceDir = sr.config.WorkspaceDir
	}

	// Build command arguments
	builder := NewCommandBuilder(params.Backend)
	args := builder.BuildArgs(params)

	// Build environment
	env := make(map[string]string)
	for k, v := range params.Backend.Env {
		env[k] = v
	}

	// Create sandbox execution request
	req := sandbox.NewExecutionRequest(params.Backend.Command, args...)
	req.Env = env
	req.WorkDir = params.WorkspaceDir
	req.Timeout = params.Timeout

	// Handle stdin input
	if builder.ResolveInputMode(params.Prompt) == InputModeStdin {
		req.Stdin = params.Prompt
	}

	// Execute in sandbox
	result, err := sr.sandboxManager.Execute(ctx, req)
	if err != nil {
		return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
	}

	duration := timeutil.SinceTime(startTime)

	// Check execution status
	switch result.Status {
	case sandbox.StatusTimeout:
		return nil, ErrTimeout{Duration: params.Timeout}
	case sandbox.StatusKilled:
		return nil, ctx.Err()
	case sandbox.StatusFailed:
		return nil, ErrCliExecution{
			Command:  params.Backend.Command,
			ExitCode: result.ExitCode,
			Stderr:   result.Stderr,
		}
	}

	// Parse output
	parser := NewOutputParser(params.Backend)
	outputFormat := builder.GetOutputFormat(params.IsResume)
	output, parseErr := parser.Parse(result.Stdout, outputFormat)
	if parseErr != nil {
		if result.Stderr != "" {
			return nil, ErrCliExecution{
				Command:  params.Backend.Command,
				ExitCode: result.ExitCode,
				Stderr:   result.Stderr,
				Cause:    parseErr,
			}
		}
		return nil, parseErr
	}

	return &RunResult{
		Output:    output,
		SessionId: output.SessionId,
		Duration:  duration,
		ExitCode:  result.ExitCode,
		Stderr:    result.Stderr,
	}, nil
}

// runStreamSandboxed executes a streaming command in the sandbox.
// Note: True streaming is limited in sandbox mode; we execute and then stream the result.
func (sr *SandboxedRunner) runStreamSandboxed(ctx context.Context, params *RunParams) (<-chan CliStreamChunk, error) {
	resultCh := make(chan CliStreamChunk, 10)

	go func() {
		defer close(resultCh)

		// Execute in sandbox (non-streaming)
		result, err := sr.runSandboxed(ctx, params)
		if err != nil {
			resultCh <- CliStreamChunk{Error: err}
			return
		}

		// Stream the result in chunks to simulate streaming
		text := result.Output.Text
		chunkSize := 100 // Characters per chunk

		for i := 0; i < len(text); i += chunkSize {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}

			chunk := CliStreamChunk{
				Text:      text[i:end],
				SessionId: result.SessionId,
			}

			// Add usage info on last chunk
			if end >= len(text) {
				chunk.Done = true
				chunk.Usage = &result.Output.Usage
			}

			select {
			case resultCh <- chunk:
			case <-ctx.Done():
				return
			}

			// Small delay to simulate streaming
			select {
			case <-time.After(10 * time.Millisecond):
			case <-ctx.Done():
				return
			}
		}

		// Ensure we send a done chunk if text was empty
		if len(text) == 0 {
			resultCh <- CliStreamChunk{
				Done:      true,
				SessionId: result.SessionId,
				Usage:     &result.Output.Usage,
			}
		}
	}()

	return resultCh, nil
}

// Close shuts down the runner and cleans up resources.
func (sr *SandboxedRunner) Close() error {
	if sr.sandboxManager != nil {
		sr.sandboxManager.Cleanup()
	}
	return sr.runner.Close()
}

// IsSandboxEnabled returns whether sandbox mode is enabled and supported.
func (sr *SandboxedRunner) IsSandboxEnabled() bool {
	return sr.enabled && sr.sandboxManager != nil
}

// GetSandboxStatus returns the current sandbox status.
func (sr *SandboxedRunner) GetSandboxStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":   sr.config.Sandbox.Enabled,
		"active":    sr.enabled,
		"supported": sr.sandboxManager != nil,
	}

	if sr.sandboxManager != nil {
		cfg := sr.sandboxManager.GetConfig()
		status["config"] = map[string]interface{}{
			"memory_limit":    cfg.MemoryLimit,
			"cpu_limit":       cfg.CPULimit,
			"process_limit":   cfg.ProcessLimit,
			"network_enabled": cfg.NetworkEnabled,
			"work_dir":        cfg.WorkDir,
			"allowed_paths":   cfg.AllowedPaths,
			"denied_paths":    cfg.DeniedPaths,
		}
	}

	return status
}

// ValidatePath checks if a path is allowed by the sandbox configuration.
func (sr *SandboxedRunner) ValidatePath(path string) error {
	if !sr.enabled || sr.sandboxManager == nil {
		return nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	cfg := sr.sandboxManager.GetConfig()

	// Check denied paths
	for _, denied := range cfg.DeniedPaths {
		if strings.HasPrefix(absPath, denied) {
			return fmt.Errorf("path %s is denied by sandbox policy", path)
		}
	}

	// Check allowed paths
	allowed := false
	for _, allowedPath := range cfg.AllowedPaths {
		if strings.HasPrefix(absPath, allowedPath) {
			allowed = true
			break
		}
	}

	if !allowed && len(cfg.AllowedPaths) > 0 {
		return fmt.Errorf("path %s is not in allowed paths", path)
	}

	return nil
}
