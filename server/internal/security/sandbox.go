// Package security provides security utilities for ZimaOS Echo.
package security

import (
	"fmt"
	"regexp"
	"strings"
)

// EnvSanitizer provides utilities for sanitizing environment variables
// to prevent injection attacks.
type EnvSanitizer struct {
	// AllowedEnvVars is a list of environment variable names that are allowed.
	// If empty, all variables are allowed (subject to other checks).
	AllowedEnvVars []string
	// DeniedEnvVars is a list of environment variable names that are denied.
	DeniedEnvVars []string
	// MaxValueLength is the maximum length of an environment variable value.
	MaxValueLength int
}

// DefaultEnvSanitizer returns a sanitizer with sensible defaults.
func DefaultEnvSanitizer() *EnvSanitizer {
	return &EnvSanitizer{
		DeniedEnvVars: []string{
			"LD_PRELOAD",
			"LD_LIBRARY_PATH",
			"DYLD_INSERT_LIBRARIES",
			"DYLD_LIBRARY_PATH",
		},
		MaxValueLength: 32768, // 32KB
	}
}

// SanitizeEnv sanitizes environment variables for safe execution.
func (s *EnvSanitizer) SanitizeEnv(env map[string]string) (map[string]string, error) {
	result := make(map[string]string, len(env))

	for key, value := range env {
		// Check if key is denied
		if s.isDenied(key) {
			continue // Skip denied variables
		}

		// Check if key is allowed (if allowlist is set)
		if len(s.AllowedEnvVars) > 0 && !s.isAllowed(key) {
			continue // Skip non-allowed variables
		}

		// Validate key format
		if !isValidEnvKey(key) {
			return nil, fmt.Errorf("invalid environment variable name: %s", key)
		}

		// Check value length
		if s.MaxValueLength > 0 && len(value) > s.MaxValueLength {
			return nil, fmt.Errorf("environment variable %s value too long", key)
		}

		result[key] = value
	}

	return result, nil
}

func (s *EnvSanitizer) isDenied(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, denied := range s.DeniedEnvVars {
		if strings.ToUpper(denied) == upperKey {
			return true
		}
	}
	return false
}

func (s *EnvSanitizer) isAllowed(key string) bool {
	upperKey := strings.ToUpper(key)
	for _, allowed := range s.AllowedEnvVars {
		if strings.ToUpper(allowed) == upperKey {
			return true
		}
	}
	return false
}

// isValidEnvKey checks if an environment variable name is valid.
func isValidEnvKey(key string) bool {
	if key == "" {
		return false
	}
	// Environment variable names should only contain alphanumeric and underscore
	// and should not start with a digit
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_]*$`, key)
	return matched
}

// BuildDockerExecArgs builds Docker exec arguments with PATH injection prevention.
// Instead of interpolating PATH into the shell command, it passes PATH via
// an internal environment variable to prevent shell injection attacks.
func BuildDockerExecArgs(params DockerExecParams) []string {
	args := []string{"exec"}

	if params.TTY {
		args = append(args, "-t")
	}

	if params.Interactive {
		args = append(args, "-i")
	}

	// Add environment variables
	for key, value := range params.Env {
		// Skip PATH - we'll handle it specially
		if strings.ToUpper(key) == "PATH" {
			continue
		}
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Handle PATH specially to prevent injection
	hasCustomPath := false
	customPath := ""
	if path, ok := params.Env["PATH"]; ok && path != "" {
		hasCustomPath = true
		customPath = path
	}

	if hasCustomPath {
		// Pass PATH via an internal env var to avoid shell interpolation
		args = append(args, "-e", fmt.Sprintf("ZIMAOS_PREPEND_PATH=%s", customPath))
	}

	// Build the command with safe PATH handling
	// Login shell (-l) sources /etc/profile which resets PATH to a minimal set.
	// We prepend custom PATH after profile sourcing using an env var reference
	// (not direct interpolation) to prevent injection.
	var pathExport string
	if hasCustomPath {
		// Use env var reference instead of direct value interpolation
		pathExport = `export PATH="${ZIMAOS_PREPEND_PATH}:$PATH"; unset ZIMAOS_PREPEND_PATH; `
	}

	args = append(args, params.ContainerName, "sh", "-lc", pathExport+params.Command)

	return args
}

// DockerExecParams contains parameters for building Docker exec arguments.
type DockerExecParams struct {
	// ContainerName is the name of the container to exec into.
	ContainerName string
	// Command is the command to execute.
	Command string
	// Env is a map of environment variables.
	Env map[string]string
	// TTY allocates a pseudo-TTY.
	TTY bool
	// Interactive keeps STDIN open.
	Interactive bool
}

// SanitizeCommand sanitizes a command string to prevent injection.
// This is a basic sanitization - for untrusted input, use a proper sandbox.
func SanitizeCommand(cmd string) (string, error) {
	if cmd == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// Check for obvious injection attempts
	dangerousPatterns := []string{
		"$(", // Command substitution
		"`",  // Backtick command substitution
		"&&", // Command chaining (might be legitimate)
		"||", // Command chaining
		";",  // Command separator
		"|",  // Pipe (might be legitimate)
		">",  // Redirect
		"<",  // Redirect
		"\n", // Newline
		"\r", // Carriage return
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmd, pattern) {
			return "", fmt.Errorf("command contains potentially dangerous pattern: %s", pattern)
		}
	}

	return cmd, nil
}

// SanitizeCommandArgs sanitizes command arguments.
func SanitizeCommandArgs(args []string) ([]string, error) {
	result := make([]string, 0, len(args))

	for _, arg := range args {
		// Check for null bytes
		if strings.Contains(arg, "\x00") {
			return nil, fmt.Errorf("argument contains null byte")
		}

		// Check for newlines (could be used for injection)
		if strings.Contains(arg, "\n") || strings.Contains(arg, "\r") {
			return nil, fmt.Errorf("argument contains newline")
		}

		result = append(result, arg)
	}

	return result, nil
}
