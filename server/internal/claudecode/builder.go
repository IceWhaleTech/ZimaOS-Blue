package claudecode

import (
	"strings"

	"github.com/google/uuid"
)

// CommandBuilder builds CLI command arguments based on configuration and parameters.
type CommandBuilder struct {
	config *CliBackendConfig
}

// NewCommandBuilder creates a new CommandBuilder with the given configuration.
func NewCommandBuilder(config *CliBackendConfig) *CommandBuilder {
	return &CommandBuilder{config: config}
}

// BuildArgs builds the complete CLI arguments for a run.
func (b *CommandBuilder) BuildArgs(params *RunParams) []string {
	var args []string

	// Use resume args if resuming, otherwise use base args
	if params.IsResume && len(b.config.ResumeArgs) > 0 {
		args = b.buildResumeArgs(params.SessionId)
	} else {
		args = append(args, b.config.Args...)
	}

	// Add model argument
	args = append(args, b.buildModelArgs(params.Model)...)

	// Add session ID argument (if not resuming, as resume args already include it)
	if !params.IsResume {
		args = append(args, b.buildSessionArgs(params)...)
	}

	// Add system prompt argument
	args = append(args, b.buildSystemPromptArgs(params)...)

	// Add image arguments
	args = append(args, b.buildImageArgs(params.ImagePaths)...)

	// Add prompt argument (if using arg mode)
	if b.ResolveInputMode(params.Prompt) == InputModeArg {
		args = append(args, params.Prompt)
	}

	return args
}

// buildResumeArgs builds arguments for resuming a session.
func (b *CommandBuilder) buildResumeArgs(sessionId string) []string {
	args := make([]string, 0, len(b.config.ResumeArgs))
	for _, arg := range b.config.ResumeArgs {
		// Replace {sessionId} placeholder
		arg = strings.ReplaceAll(arg, "{sessionId}", sessionId)
		args = append(args, arg)
	}
	return args
}

// buildModelArgs builds the model selection arguments.
func (b *CommandBuilder) buildModelArgs(model string) []string {
	if model == "" || b.config.ModelArg == "" {
		return nil
	}

	// Apply model alias if available
	resolvedModel := b.NormalizeModel(model)

	return []string{b.config.ModelArg, resolvedModel}
}

// buildSessionArgs builds the session ID arguments.
func (b *CommandBuilder) buildSessionArgs(params *RunParams) []string {
	sessionId := b.ResolveSessionId(params)
	if sessionId == "" || b.config.SessionArg == "" {
		return nil
	}

	args := []string{b.config.SessionArg, sessionId}

	// Add extra session args
	for _, arg := range b.config.SessionArgs {
		arg = strings.ReplaceAll(arg, "{sessionId}", sessionId)
		args = append(args, arg)
	}

	return args
}

// buildSystemPromptArgs builds the system prompt arguments.
func (b *CommandBuilder) buildSystemPromptArgs(params *RunParams) []string {
	if !b.ShouldSendSystemPrompt(params) || params.SystemPrompt == "" {
		return nil
	}

	if b.config.SystemPromptArg == "" {
		return nil
	}

	return []string{b.config.SystemPromptArg, params.SystemPrompt}
}

// buildImageArgs builds the image path arguments.
func (b *CommandBuilder) buildImageArgs(imagePaths []string) []string {
	if len(imagePaths) == 0 || b.config.ImageArg == "" {
		return nil
	}

	var args []string

	switch b.config.ImageMode {
	case "list":
		// Pass all images as a single comma-separated list
		args = append(args, b.config.ImageArg, strings.Join(imagePaths, ","))
	case "repeat":
		fallthrough
	default:
		// Repeat the flag for each image
		for _, path := range imagePaths {
			args = append(args, b.config.ImageArg, path)
		}
	}

	return args
}

// ResolveInputMode determines whether to use arg or stdin for the prompt.
func (b *CommandBuilder) ResolveInputMode(prompt string) InputMode {
	// If explicitly configured for stdin, use stdin
	if b.config.Input == "stdin" {
		return InputModeStdin
	}

	// If prompt exceeds max length, use stdin
	if b.config.MaxPromptArgChars > 0 && len(prompt) > b.config.MaxPromptArgChars {
		return InputModeStdin
	}

	// Default to arg mode
	return InputModeArg
}

// ResolveSessionId determines the session ID to use for this run.
func (b *CommandBuilder) ResolveSessionId(params *RunParams) string {
	switch SessionMode(b.config.SessionMode) {
	case SessionModeNone:
		return ""
	case SessionModeExisting:
		// Only return session ID if one already exists
		return params.SessionId
	case SessionModeAlways:
		fallthrough
	default:
		// Generate a new session ID if none exists
		if params.SessionId == "" {
			return uuid.New().String()
		}
		return params.SessionId
	}
}

// ShouldSendSystemPrompt determines whether to send the system prompt for this run.
func (b *CommandBuilder) ShouldSendSystemPrompt(params *RunParams) bool {
	switch SystemPromptWhen(b.config.SystemPromptWhen) {
	case SystemPromptWhenNever:
		return false
	case SystemPromptWhenAlways:
		return true
	case SystemPromptWhenFirst:
		fallthrough
	default:
		return params.IsFirstMessage
	}
}

// NormalizeModel applies model aliases to get the CLI model name.
func (b *CommandBuilder) NormalizeModel(model string) string {
	if alias, ok := b.config.ModelAliases[model]; ok {
		return alias
	}
	return model
}

// GetOutputFormat returns the output format for this run.
func (b *CommandBuilder) GetOutputFormat(isResume bool) OutputFormat {
	if isResume && b.config.ResumeOutput != "" {
		return OutputFormat(b.config.ResumeOutput)
	}
	if b.config.Output != "" {
		return OutputFormat(b.config.Output)
	}
	return OutputFormatJSON
}

// BuildEnv builds the environment variables for the CLI execution.
func (b *CommandBuilder) BuildEnv(baseEnv []string) []string {
	// Create a map of existing env vars
	envMap := make(map[string]string)
	for _, env := range baseEnv {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// Remove cleared env vars
	for _, key := range b.config.ClearEnv {
		delete(envMap, key)
	}

	// Add extra env vars
	for key, value := range b.config.Env {
		envMap[key] = value
	}

	// Convert back to slice
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}

	return result
}
