package claudecode

import (
	"fmt"
	"time"
)

// CliOutput represents the parsed output from a CLI execution.
type CliOutput struct {
	// Text is the main text content from the CLI response.
	Text string `json:"text"`

	// SessionId is the session identifier returned by the CLI.
	SessionId string `json:"session_id,omitempty"`

	// Usage contains token usage statistics.
	Usage CliUsage `json:"usage"`

	// Raw contains the raw output for debugging.
	Raw string `json:"-"`
}

// CliUsage represents token usage statistics from a CLI execution.
type CliUsage struct {
	// InputTokens is the number of input tokens used.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of output tokens generated.
	OutputTokens int `json:"output_tokens"`

	// CacheReadTokens is the number of tokens read from cache.
	CacheReadTokens int `json:"cache_read_tokens,omitempty"`

	// CacheWriteTokens is the number of tokens written to cache.
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`

	// TotalTokens is the total number of tokens used.
	TotalTokens int `json:"total_tokens"`
}

// RunParams contains parameters for a CLI execution.
type RunParams struct {
	// Prompt is the user prompt to send to the CLI.
	Prompt string

	// Model is the model to use (e.g., "opus", "sonnet", "haiku").
	Model string

	// SessionId is the session identifier for conversation continuity.
	SessionId string

	// IsResume indicates whether this is resuming an existing session.
	IsResume bool

	// SystemPrompt is the system prompt to append.
	SystemPrompt string

	// IsFirstMessage indicates whether this is the first message in the session.
	IsFirstMessage bool

	// WorkspaceDir is the working directory for the CLI execution.
	WorkspaceDir string

	// ImagePaths contains paths to images to include in the request.
	ImagePaths []string

	// Timeout is the maximum duration for this execution.
	Timeout time.Duration

	// Backend is the CLI backend configuration to use.
	Backend *CliBackendConfig
}

// CliStreamChunk represents a chunk of streaming output from the CLI.
type CliStreamChunk struct {
	// Text is the text content of this chunk.
	Text string `json:"text"`

	// SessionId is the session identifier (may be set on first or last chunk).
	SessionId string `json:"session_id,omitempty"`

	// Usage contains token usage (typically set on the last chunk).
	Usage *CliUsage `json:"usage,omitempty"`

	// Done indicates whether this is the final chunk.
	Done bool `json:"done"`

	// Error contains any error that occurred.
	Error error `json:"-"`
}

// RunResult contains the result of a CLI execution.
type RunResult struct {
	// Output is the parsed CLI output.
	Output *CliOutput

	// SessionId is the session identifier for the next request.
	SessionId string

	// Duration is how long the execution took.
	Duration time.Duration

	// ExitCode is the CLI process exit code.
	ExitCode int

	// Stderr contains any stderr output.
	Stderr string
}

// InputMode represents how the prompt is passed to the CLI.
type InputMode string

const (
	// InputModeArg passes the prompt as a command-line argument.
	InputModeArg InputMode = "arg"

	// InputModeStdin passes the prompt via stdin.
	InputModeStdin InputMode = "stdin"
)

// OutputFormat represents the CLI output format.
type OutputFormat string

const (
	// OutputFormatJSON parses output as a single JSON object.
	OutputFormatJSON OutputFormat = "json"

	// OutputFormatJSONL parses output as newline-delimited JSON.
	OutputFormatJSONL OutputFormat = "jsonl"

	// OutputFormatText treats output as plain text.
	OutputFormatText OutputFormat = "text"
)

// SessionMode represents when to pass session IDs.
type SessionMode string

const (
	// SessionModeAlways always passes a session ID.
	SessionModeAlways SessionMode = "always"

	// SessionModeExisting only passes session ID for existing sessions.
	SessionModeExisting SessionMode = "existing"

	// SessionModeNone never passes a session ID.
	SessionModeNone SessionMode = "none"
)

// SystemPromptWhen represents when to send the system prompt.
type SystemPromptWhen string

const (
	// SystemPromptWhenFirst sends system prompt only on the first message.
	SystemPromptWhenFirst SystemPromptWhen = "first"

	// SystemPromptWhenAlways sends system prompt on every message.
	SystemPromptWhenAlways SystemPromptWhen = "always"

	// SystemPromptWhenNever never sends the system prompt.
	SystemPromptWhenNever SystemPromptWhen = "never"
)

// ErrInvalidConfig represents a configuration validation error.
type ErrInvalidConfig struct {
	Field   string
	Message string
}

func (e ErrInvalidConfig) Error() string {
	return fmt.Sprintf("invalid config: %s: %s", e.Field, e.Message)
}

// ErrCliExecution represents a CLI execution error.
type ErrCliExecution struct {
	Command  string
	ExitCode int
	Stderr   string
	Cause    error
}

func (e ErrCliExecution) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("CLI execution failed: %s (exit code %d): %v", e.Command, e.ExitCode, e.Cause)
	}
	if e.Stderr != "" {
		return fmt.Sprintf("CLI execution failed: %s (exit code %d): %s", e.Command, e.ExitCode, e.Stderr)
	}
	return fmt.Sprintf("CLI execution failed: %s (exit code %d)", e.Command, e.ExitCode)
}

func (e ErrCliExecution) Unwrap() error {
	return e.Cause
}

// ErrTimeout represents a timeout error.
type ErrTimeout struct {
	Duration time.Duration
}

func (e ErrTimeout) Error() string {
	return fmt.Sprintf("CLI execution timed out after %v", e.Duration)
}

// ErrSessionNotFound represents a session not found error.
type ErrSessionNotFound struct {
	SessionId string
}

func (e ErrSessionNotFound) Error() string {
	return fmt.Sprintf("session not found: %s", e.SessionId)
}

// ErrParseOutput represents an output parsing error.
type ErrParseOutput struct {
	Format  string
	Content string
	Cause   error
}

func (e ErrParseOutput) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("failed to parse %s output: %v", e.Format, e.Cause)
	}
	return fmt.Sprintf("failed to parse %s output", e.Format)
}

func (e ErrParseOutput) Unwrap() error {
	return e.Cause
}
