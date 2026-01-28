// Package claudecode provides Claude Code CLI integration for LLM operations.
// It wraps the Claude Code CLI to leverage its context engineering capabilities
// including automatic file reading, code search, tool calling, and session management.
package claudecode

import (
	"time"
)

// CliBackendConfig defines the configuration for a CLI backend.
// This is modeled after Clawdbot's CliBackendConfig for compatibility.
type CliBackendConfig struct {
	// Command is the CLI executable path (absolute or on PATH).
	Command string `mapstructure:"command" json:"command"`

	// Args are base arguments applied to every invocation.
	Args []string `mapstructure:"args" json:"args,omitempty"`

	// Output is the output parsing mode: "json", "text", or "jsonl".
	Output string `mapstructure:"output" json:"output,omitempty"`

	// ResumeOutput is the output parsing mode when resuming a CLI session.
	ResumeOutput string `mapstructure:"resume_output" json:"resume_output,omitempty"`

	// Input is the prompt input mode: "arg" or "stdin".
	Input string `mapstructure:"input" json:"input,omitempty"`

	// MaxPromptArgChars is the max prompt length for arg mode (if exceeded, stdin is used).
	MaxPromptArgChars int `mapstructure:"max_prompt_arg_chars" json:"max_prompt_arg_chars,omitempty"`

	// Env contains extra environment variables injected for this CLI.
	Env map[string]string `mapstructure:"env" json:"env,omitempty"`

	// ClearEnv lists environment variables to remove before launching this CLI.
	ClearEnv []string `mapstructure:"clear_env" json:"clear_env,omitempty"`

	// ModelArg is the flag used to pass model id (e.g., "--model").
	ModelArg string `mapstructure:"model_arg" json:"model_arg,omitempty"`

	// ModelAliases maps config model id to CLI model id.
	ModelAliases map[string]string `mapstructure:"model_aliases" json:"model_aliases,omitempty"`

	// SessionArg is the flag used to pass session id (e.g., "--session-id").
	SessionArg string `mapstructure:"session_arg" json:"session_arg,omitempty"`

	// SessionArgs are extra args used when resuming a session (use {sessionId} placeholder).
	SessionArgs []string `mapstructure:"session_args" json:"session_args,omitempty"`

	// ResumeArgs are alternate args to use when resuming a session (use {sessionId} placeholder).
	ResumeArgs []string `mapstructure:"resume_args" json:"resume_args,omitempty"`

	// SessionMode determines when to pass session ids: "always", "existing", or "none".
	SessionMode string `mapstructure:"session_mode" json:"session_mode,omitempty"`

	// SessionIdFields are JSON fields to read session id from (in order).
	SessionIdFields []string `mapstructure:"session_id_fields" json:"session_id_fields,omitempty"`

	// SystemPromptArg is the flag used to pass system prompt (e.g., "--append-system-prompt").
	SystemPromptArg string `mapstructure:"system_prompt_arg" json:"system_prompt_arg,omitempty"`

	// SystemPromptMode is the system prompt behavior: "append" or "replace".
	SystemPromptMode string `mapstructure:"system_prompt_mode" json:"system_prompt_mode,omitempty"`

	// SystemPromptWhen determines when to send system prompt: "first", "always", or "never".
	SystemPromptWhen string `mapstructure:"system_prompt_when" json:"system_prompt_when,omitempty"`

	// ImageArg is the flag used to pass image paths.
	ImageArg string `mapstructure:"image_arg" json:"image_arg,omitempty"`

	// ImageMode determines how to pass multiple images: "repeat" or "list".
	ImageMode string `mapstructure:"image_mode" json:"image_mode,omitempty"`

	// Serialize indicates whether to serialize runs for this CLI.
	Serialize bool `mapstructure:"serialize" json:"serialize,omitempty"`
}

// ClaudeCodeConfig holds the main Claude Code CLI configuration.
type ClaudeCodeConfig struct {
	// Enabled indicates whether Claude Code CLI integration is enabled.
	Enabled bool `mapstructure:"enabled" json:"enabled"`

	// Command is the path to the claude CLI executable.
	Command string `mapstructure:"command" json:"command"`

	// WorkspaceDir is the default workspace directory for CLI operations.
	WorkspaceDir string `mapstructure:"workspace_dir" json:"workspace_dir"`

	// DefaultModel is the default model to use (e.g., "opus", "sonnet", "haiku").
	DefaultModel string `mapstructure:"default_model" json:"default_model"`

	// Timeout is the maximum duration for a CLI execution.
	Timeout time.Duration `mapstructure:"timeout" json:"timeout"`

	// SessionTTL is the time-to-live for CLI sessions.
	SessionTTL time.Duration `mapstructure:"session_ttl" json:"session_ttl"`

	// Backend contains the CLI backend configuration.
	Backend CliBackendConfig `mapstructure:"backend" json:"backend"`
}

// DefaultClaudeCodeBackend returns the default Claude Code CLI backend configuration.
// This matches the configuration used by Clawdbot for Claude Code CLI.
func DefaultClaudeCodeBackend() CliBackendConfig {
	return CliBackendConfig{
		Command: "claude",
		Args: []string{
			"-p",
			"--output-format", "json",
			"--dangerously-skip-permissions",
		},
		Output: "json",
		Input:  "arg",
		ResumeArgs: []string{
			"-p",
			"--output-format", "json",
			"--dangerously-skip-permissions",
			"--resume", "{sessionId}",
		},
		ResumeOutput:     "json",
		MaxPromptArgChars: 100000,
		ModelArg:         "--model",
		ModelAliases: map[string]string{
			"opus":   "opus",
			"sonnet": "sonnet",
			"haiku":  "haiku",
			// Full model names
			"claude-opus-4-5-20251101":   "opus",
			"claude-sonnet-4-20250514":   "sonnet",
			"claude-3-5-haiku-20241022":  "haiku",
		},
		SessionArg:  "--session-id",
		SessionMode: "always",
		SessionIdFields: []string{
			"session_id",
			"sessionId",
		},
		SystemPromptArg:  "--append-system-prompt",
		SystemPromptMode: "append",
		SystemPromptWhen: "first",
		ClearEnv: []string{
			"ANTHROPIC_API_KEY",
			"ANTHROPIC_API_KEY_OLD",
		},
		Serialize: true,
	}
}

// DefaultOpenCodeBackend returns the default OpenCode CLI backend configuration.
// OpenCode is an open-source alternative to Claude Code CLI.
func DefaultOpenCodeBackend() CliBackendConfig {
	return CliBackendConfig{
		Command: "opencode",
		Args: []string{
			"--output-format", "json",
		},
		Output:            "json",
		Input:             "arg",
		MaxPromptArgChars: 100000,
		ModelArg:          "--model",
		ModelAliases: map[string]string{
			"opus":   "claude-opus-4-5-20251101",
			"sonnet": "claude-sonnet-4-20250514",
			"haiku":  "claude-3-5-haiku-20241022",
		},
		SessionArg:  "--session",
		SessionMode: "always",
		SessionIdFields: []string{
			"session_id",
			"sessionId",
		},
		SystemPromptArg:  "--system-prompt",
		SystemPromptMode: "append",
		SystemPromptWhen: "first",
		Serialize:        true,
	}
}

// DefaultClaudeCodeConfig returns the default Claude Code configuration.
func DefaultClaudeCodeConfig() ClaudeCodeConfig {
	return ClaudeCodeConfig{
		Enabled:      false,
		Command:      "claude",
		WorkspaceDir: ".",
		DefaultModel: "sonnet",
		Timeout:      5 * time.Minute,
		SessionTTL:   24 * time.Hour,
		Backend:      DefaultClaudeCodeBackend(),
	}
}

// Validate validates the CLI backend configuration.
func (c *CliBackendConfig) Validate() error {
	if c.Command == "" {
		return ErrInvalidConfig{Field: "command", Message: "command is required"}
	}
	if c.Output != "" && c.Output != "json" && c.Output != "text" && c.Output != "jsonl" {
		return ErrInvalidConfig{Field: "output", Message: "output must be json, text, or jsonl"}
	}
	if c.Input != "" && c.Input != "arg" && c.Input != "stdin" {
		return ErrInvalidConfig{Field: "input", Message: "input must be arg or stdin"}
	}
	if c.SessionMode != "" && c.SessionMode != "always" && c.SessionMode != "existing" && c.SessionMode != "none" {
		return ErrInvalidConfig{Field: "session_mode", Message: "session_mode must be always, existing, or none"}
	}
	if c.SystemPromptMode != "" && c.SystemPromptMode != "append" && c.SystemPromptMode != "replace" {
		return ErrInvalidConfig{Field: "system_prompt_mode", Message: "system_prompt_mode must be append or replace"}
	}
	if c.SystemPromptWhen != "" && c.SystemPromptWhen != "first" && c.SystemPromptWhen != "always" && c.SystemPromptWhen != "never" {
		return ErrInvalidConfig{Field: "system_prompt_when", Message: "system_prompt_when must be first, always, or never"}
	}
	if c.ImageMode != "" && c.ImageMode != "repeat" && c.ImageMode != "list" {
		return ErrInvalidConfig{Field: "image_mode", Message: "image_mode must be repeat or list"}
	}
	return nil
}

// Validate validates the Claude Code configuration.
func (c *ClaudeCodeConfig) Validate() error {
	if c.Enabled {
		if c.Command == "" {
			return ErrInvalidConfig{Field: "command", Message: "command is required when enabled"}
		}
		if c.Timeout <= 0 {
			return ErrInvalidConfig{Field: "timeout", Message: "timeout must be positive"}
		}
		if c.SessionTTL <= 0 {
			return ErrInvalidConfig{Field: "session_ttl", Message: "session_ttl must be positive"}
		}
		if err := c.Backend.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// WithDefaults returns a copy of the config with default values applied.
func (c *CliBackendConfig) WithDefaults() CliBackendConfig {
	result := *c
	defaults := DefaultClaudeCodeBackend()

	if result.Command == "" {
		result.Command = defaults.Command
	}
	if len(result.Args) == 0 {
		result.Args = defaults.Args
	}
	if result.Output == "" {
		result.Output = defaults.Output
	}
	if result.Input == "" {
		result.Input = defaults.Input
	}
	if result.MaxPromptArgChars == 0 {
		result.MaxPromptArgChars = defaults.MaxPromptArgChars
	}
	if result.ModelArg == "" {
		result.ModelArg = defaults.ModelArg
	}
	if len(result.ModelAliases) == 0 {
		result.ModelAliases = defaults.ModelAliases
	}
	if result.SessionArg == "" {
		result.SessionArg = defaults.SessionArg
	}
	if result.SessionMode == "" {
		result.SessionMode = defaults.SessionMode
	}
	if len(result.SessionIdFields) == 0 {
		result.SessionIdFields = defaults.SessionIdFields
	}
	if result.SystemPromptArg == "" {
		result.SystemPromptArg = defaults.SystemPromptArg
	}
	if result.SystemPromptMode == "" {
		result.SystemPromptMode = defaults.SystemPromptMode
	}
	if result.SystemPromptWhen == "" {
		result.SystemPromptWhen = defaults.SystemPromptWhen
	}
	if len(result.ResumeArgs) == 0 {
		result.ResumeArgs = defaults.ResumeArgs
	}
	if result.ResumeOutput == "" {
		result.ResumeOutput = defaults.ResumeOutput
	}
	if len(result.ClearEnv) == 0 {
		result.ClearEnv = defaults.ClearEnv
	}

	return result
}

// WithDefaults returns a copy of the config with default values applied.
func (c *ClaudeCodeConfig) WithDefaults() ClaudeCodeConfig {
	result := *c
	defaults := DefaultClaudeCodeConfig()

	if result.Command == "" {
		result.Command = defaults.Command
	}
	if result.WorkspaceDir == "" {
		result.WorkspaceDir = defaults.WorkspaceDir
	}
	if result.DefaultModel == "" {
		result.DefaultModel = defaults.DefaultModel
	}
	if result.Timeout == 0 {
		result.Timeout = defaults.Timeout
	}
	if result.SessionTTL == 0 {
		result.SessionTTL = defaults.SessionTTL
	}

	result.Backend = result.Backend.WithDefaults()

	return result
}
