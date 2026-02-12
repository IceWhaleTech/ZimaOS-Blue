package claudecode

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Runner executes Claude Code CLI commands.
type Runner struct {
	config        *ClaudeCodeConfig
	builder       *CommandBuilder
	parser        *OutputParser
	queue         *runQueue
	binaryManager *BinaryManager
}

// runQueue manages serialized CLI runs.
type runQueue struct {
	mu      sync.Mutex
	running bool
	queue   chan *queuedRun
}

// queuedRun represents a queued CLI run.
type queuedRun struct {
	params   *RunParams
	resultCh chan *runQueueResult
}

// runQueueResult contains the result of a queued run.
type runQueueResult struct {
	result *RunResult
	err    error
}

// NewRunner creates a new Runner with the given configuration.
func NewRunner(config *ClaudeCodeConfig) *Runner {
	// Use the backend config directly - it should already have defaults applied
	// by ClaudeCodeConfig.WithDefaults() which also sets up API key in Env
	backendConfig := config.Backend
	return &Runner{
		config:        config,
		builder:       NewCommandBuilder(&backendConfig),
		parser:        NewOutputParser(&backendConfig),
		binaryManager: NewBinaryManager(""),
		queue: &runQueue{
			queue: make(chan *queuedRun, 100),
		},
	}
}

// Run executes a CLI command and returns the result.
func (r *Runner) Run(ctx context.Context, params *RunParams) (*RunResult, error) {
	// Apply defaults from config
	if params.Backend == nil {
		backend := r.config.Backend.WithDefaults()
		params.Backend = &backend
	}
	if params.Timeout == 0 {
		params.Timeout = r.config.Timeout
	}
	if params.WorkspaceDir == "" {
		params.WorkspaceDir = r.config.WorkspaceDir
	}

	// Check if serialization is required
	if params.Backend.Serialize {
		return r.runSerialized(ctx, params)
	}

	return r.runDirect(ctx, params)
}

// runSerialized runs the command through the serialization queue.
func (r *Runner) runSerialized(ctx context.Context, params *RunParams) (*RunResult, error) {
	resultCh := make(chan *runQueueResult, 1)

	// Start queue processor if not running
	r.queue.mu.Lock()
	if !r.queue.running {
		r.queue.running = true
		go r.processQueue()
	}
	r.queue.mu.Unlock()

	// Queue the run
	select {
	case r.queue.queue <- &queuedRun{params: params, resultCh: resultCh}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Wait for result
	select {
	case result := <-resultCh:
		return result.result, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// processQueue processes queued runs sequentially.
func (r *Runner) processQueue() {
	for run := range r.queue.queue {
		ctx, cancel := context.WithTimeout(context.Background(), run.params.Timeout)
		result, err := r.runDirect(ctx, run.params)
		cancel()

		run.resultCh <- &runQueueResult{result: result, err: err}
	}
}

// runDirect executes the CLI command directly.
func (r *Runner) runDirect(ctx context.Context, params *RunParams) (*RunResult, error) {
	startNano := timeutil.NowNano()

	// Build command arguments
	args := r.builder.BuildArgs(params)

	// Resolve command path - prefer embedded binary, fall back to config
	cmdPath := params.Backend.Command
	if embeddedPath, err := r.binaryManager.GetBinaryPath(); err == nil {
		cmdPath = embeddedPath
	}

	// Create command
	cmd := exec.CommandContext(ctx, cmdPath, args...)

	// Set working directory
	if params.WorkspaceDir != "" {
		cmd.Dir = params.WorkspaceDir
	}

	// Set environment
	cmd.Env = r.builder.BuildEnv(os.Environ())

	// Set up stdin if needed
	var stdin io.WriteCloser
	if r.builder.ResolveInputMode(params.Prompt) == InputModeStdin {
		var err error
		stdin, err = cmd.StdinPipe()
		if err != nil {
			return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
		}
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start command
	if err := cmd.Start(); err != nil {
		return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
	}

	// Write prompt to stdin if needed
	if stdin != nil {
		if _, err := stdin.Write([]byte(params.Prompt)); err != nil {
			return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
		}
		stdin.Close()
	}

	// Wait for command to complete
	err := cmd.Wait()
	duration := timeutil.Since(startNano)

	// Check for context cancellation (timeout)
	if ctx.Err() == context.DeadlineExceeded {
		return nil, ErrTimeout{Duration: params.Timeout}
	}
	if ctx.Err() == context.Canceled {
		return nil, ctx.Err()
	}

	// Get exit code
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
		}
	}

	// Parse output
	outputFormat := r.builder.GetOutputFormat(params.IsResume)
	output, parseErr := r.parser.Parse(stdout.String(), outputFormat)
	if parseErr != nil {
		// If parsing fails but we have stderr, include it in the error
		if stderr.Len() > 0 {
			return nil, ErrCliExecution{
				Command:  params.Backend.Command,
				ExitCode: exitCode,
				Stderr:   stderr.String(),
				Cause:    parseErr,
			}
		}
		return nil, parseErr
	}

	// Check for non-zero exit code
	if exitCode != 0 {
		return nil, ErrCliExecution{
			Command:  params.Backend.Command,
			ExitCode: exitCode,
			Stderr:   stderr.String(),
		}
	}

	return &RunResult{
		Output:    output,
		SessionId: output.SessionId,
		Duration:  duration,
		ExitCode:  exitCode,
		Stderr:    stderr.String(),
	}, nil
}

// RunStream executes a CLI command and returns a channel of stream chunks.
func (r *Runner) RunStream(ctx context.Context, params *RunParams) (<-chan CliStreamChunk, error) {
	// Apply defaults from config
	if params.Backend == nil {
		backend := r.config.Backend.WithDefaults()
		params.Backend = &backend
	}
	if params.Timeout == 0 {
		params.Timeout = r.config.Timeout
	}
	if params.WorkspaceDir == "" {
		params.WorkspaceDir = r.config.WorkspaceDir
	}

	// Build command arguments
	args := r.builder.BuildArgs(params)

	// Resolve command path - prefer embedded binary, fall back to config
	cmdPath := params.Backend.Command
	if embeddedPath, err := r.binaryManager.GetBinaryPath(); err == nil {
		cmdPath = embeddedPath
	}

	// Create command with timeout context
	ctx, cancel := context.WithTimeout(ctx, params.Timeout)

	cmd := exec.CommandContext(ctx, cmdPath, args...)

	// Set working directory
	if params.WorkspaceDir != "" {
		cmd.Dir = params.WorkspaceDir
	}

	// Set environment
	cmd.Env = r.builder.BuildEnv(os.Environ())

	// Set up stdin if needed
	var stdin io.WriteCloser
	if r.builder.ResolveInputMode(params.Prompt) == InputModeStdin {
		var err error
		stdin, err = cmd.StdinPipe()
		if err != nil {
			cancel()
			return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
		}
	}

	// Get stdout pipe for streaming
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
	}

	// Capture stderr
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Start command
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
	}

	// Write prompt to stdin if needed
	if stdin != nil {
		if _, err := stdin.Write([]byte(params.Prompt)); err != nil {
			cancel()
			return nil, ErrCliExecution{Command: params.Backend.Command, Cause: err}
		}
		stdin.Close()
	}

	// Create output channel
	outputFormat := r.builder.GetOutputFormat(params.IsResume)
	streamCh := r.parser.ParseStream(stdout, outputFormat)

	// Create result channel that handles cleanup
	resultCh := make(chan CliStreamChunk, 10)

	go func() {
		defer close(resultCh)
		defer cancel()

		for chunk := range streamCh {
			select {
			case resultCh <- chunk:
			case <-ctx.Done():
				return
			}
		}

		// Wait for command to complete
		if err := cmd.Wait(); err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				resultCh <- CliStreamChunk{Error: ErrTimeout{Duration: params.Timeout}}
			} else if ctx.Err() != context.Canceled {
				// Get stderr content for error message
				stderrContent := stderr.String()
				if exitErr, ok := err.(*exec.ExitError); ok {
					resultCh <- CliStreamChunk{
						Error: ErrCliExecution{
							Command:  params.Backend.Command,
							ExitCode: exitErr.ExitCode(),
							Stderr:   stderrContent,
						},
					}
				} else {
					// Handle other error types (e.g., command not found)
					resultCh <- CliStreamChunk{
						Error: ErrCliExecution{
							Command:  params.Backend.Command,
							ExitCode: -1,
							Stderr:   stderrContent,
							Cause:    err,
						},
					}
				}
			}
		} else {
			// Command succeeded but check if stderr has warnings/errors
			stderrContent := stderr.String()
			if stderrContent != "" && strings.Contains(strings.ToLower(stderrContent), "error") {
				// CLI wrote error to stderr but exited with 0
				resultCh <- CliStreamChunk{
					Error: ErrCliExecution{
						Command:  params.Backend.Command,
						ExitCode: 0,
						Stderr:   stderrContent,
					},
				}
			}
		}
	}()

	return resultCh, nil
}

// Close shuts down the runner and cleans up resources.
func (r *Runner) Close() error {
	r.queue.mu.Lock()
	defer r.queue.mu.Unlock()

	if r.queue.running {
		close(r.queue.queue)
		r.queue.running = false
	}

	return nil
}
