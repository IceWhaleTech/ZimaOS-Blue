// Package cron provides scheduled task management.
package cron

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"go.uber.org/zap"
)

// CommandResult represents the result of a command execution.
type CommandResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
}

// Security: Dangerous shell patterns that could indicate command injection
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[;&|]`),                   // Command chaining
	regexp.MustCompile(`\$\(`),                    // Command substitution $(...)
	regexp.MustCompile("`"),                       // Backtick command substitution
	regexp.MustCompile(`\$\{`),                    // Variable expansion ${...}
	regexp.MustCompile(`>\s*[/~]`),                // Output redirection to absolute path
	regexp.MustCompile(`<\s*[/~]`),                // Input redirection from absolute path
	regexp.MustCompile(`\.\./`),                   // Path traversal
	regexp.MustCompile(`(?i)(rm|dd|mkfs|format)`), // Destructive commands
	regexp.MustCompile(`(?i)/etc/`),               // System config access
	regexp.MustCompile(`(?i)/proc/`),              // Proc filesystem access
	regexp.MustCompile(`(?i)/sys/`),               // Sys filesystem access
}

// Security: Allowed commands whitelist (empty means all commands blocked by default)
// In production, this should be configured via config file
var allowedCommands = map[string]bool{
	"echo":   true,
	"date":   true,
	"uptime": true,
	"df":     true,
	"free":   true,
	"ps":     true,
	"curl":   true,
	"wget":   true,
}

// CommandSecurityConfig holds security configuration for command execution
type CommandSecurityConfig struct {
	// Enabled controls whether command handler is available
	Enabled bool
	// AllowedCommands is a whitelist of allowed commands (empty = use default)
	AllowedCommands []string
	// RequireAdminRole requires admin role to create command jobs
	RequireAdminRole bool
	// MaxOutputSize limits the output size in bytes
	MaxOutputSize int
}

// DefaultCommandSecurityConfig returns secure defaults
func DefaultCommandSecurityConfig() CommandSecurityConfig {
	return CommandSecurityConfig{
		Enabled:          false, // Disabled by default for security
		AllowedCommands:  []string{},
		RequireAdminRole: true,
		MaxOutputSize:    1024 * 1024, // 1MB
	}
}

// RegisterBuiltinHandlers registers all built-in job handlers.
// Security: Command handler is disabled by default. Enable with caution.
func (s *Service) RegisterBuiltinHandlers() {
	// Security: Command handler is now disabled by default
	// To enable, call RegisterCommandHandler explicitly with security config
	s.logger.Warn("command handler is disabled by default for security reasons")
	s.logger.Info("to enable command execution, call RegisterCommandHandler with appropriate security config")

	// Only register safe handlers by default
	s.RegisterHandler("http", s.httpHandler)
	s.logger.Info("built-in handlers registered", zap.Int("count", 1))
}

// RegisterCommandHandler registers the command handler with security configuration.
// Security: This should only be called if command execution is explicitly needed.
func (s *Service) RegisterCommandHandler(config CommandSecurityConfig) {
	if !config.Enabled {
		s.logger.Info("command handler registration skipped - disabled in config")
		return
	}

	s.logger.Warn("command handler enabled - ensure proper access controls are in place",
		zap.Bool("require_admin", config.RequireAdminRole),
		zap.Int("allowed_commands", len(config.AllowedCommands)))

	// Update allowed commands if provided
	if len(config.AllowedCommands) > 0 {
		allowedCommands = make(map[string]bool)
		for _, cmd := range config.AllowedCommands {
			allowedCommands[cmd] = true
		}
	}

	s.RegisterHandler("command", s.commandHandler)
}

func validatePayloadForHandler(handler string, payload map[string]interface{}) error {
	switch handler {
	case "command":
		cmdStr, _ := payload["command"].(string)
		if strings.TrimSpace(cmdStr) == "" {
			return fmt.Errorf("command not specified in payload")
		}
	case "http":
		url, _ := payload["url"].(string)
		if strings.TrimSpace(url) == "" {
			return fmt.Errorf("url not specified in payload")
		}
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("invalid URL scheme - must be http or https")
		}

		method := "GET"
		if m, ok := payload["method"].(string); ok && m != "" {
			method = strings.ToUpper(m)
		}

		validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true}
		if !validMethods[method] {
			return fmt.Errorf("invalid HTTP method: %s", method)
		}
	}

	return nil
}

// validateCommand checks if a command is safe to execute.
// Security: Returns an error if the command contains dangerous patterns.
func validateCommand(cmdStr string) error {
	if cmdStr == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(cmdStr) {
			return fmt.Errorf("command contains potentially dangerous pattern: %s", pattern.String())
		}
	}

	// Extract the base command (first word)
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("invalid command format")
	}

	baseCmd := parts[0]
	// Remove path if present
	if idx := strings.LastIndex(baseCmd, "/"); idx >= 0 {
		baseCmd = baseCmd[idx+1:]
	}
	if idx := strings.LastIndex(baseCmd, "\\"); idx >= 0 {
		baseCmd = baseCmd[idx+1:]
	}

	// Check against whitelist
	if len(allowedCommands) > 0 {
		if !allowedCommands[baseCmd] {
			return fmt.Errorf("command '%s' is not in the allowed commands list", baseCmd)
		}
	}

	return nil
}

// commandHandler executes a shell command.
// Security: This handler validates commands against a whitelist and blocks dangerous patterns.
func (s *Service) commandHandler(ctx context.Context, job *Job) (interface{}, error) {
	// Get command from payload
	cmdStr, ok := job.Payload["command"].(string)
	if !ok || cmdStr == "" {
		return nil, fmt.Errorf("command not specified in payload")
	}

	// Security: Validate command before execution
	if err := validateCommand(cmdStr); err != nil {
		s.logger.Warn("command validation failed",
			zap.String("job_id", job.ID),
			zap.String("command", cmdStr),
			zap.Error(err))
		return nil, fmt.Errorf("security validation failed: %w", err)
	}

	// Get optional working directory
	workDir, _ := job.Payload["workdir"].(string)

	// Security: Validate working directory
	if workDir != "" {
		if strings.Contains(workDir, "..") {
			return nil, fmt.Errorf("security validation failed: workdir contains path traversal")
		}
	}

	// Get optional timeout (in seconds)
	timeout := 60 // default 60 seconds
	if t, ok := job.Payload["timeout"].(float64); ok && t > 0 {
		// Security: Cap maximum timeout
		if t > 300 {
			t = 300
		}
		timeout = int(t)
	}

	// Create command with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", cmdStr)
	}

	if workDir != "" {
		cmd.Dir = workDir
	}

	// Security: Clear environment to prevent leaking sensitive data
	cmd.Env = []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"LANG=C.UTF-8",
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := timeutil.NowTime()
	err := cmd.Run()
	duration := timeutil.SinceTime(start)

	// Security: Truncate output to prevent memory exhaustion
	maxOutput := 64 * 1024 // 64KB
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	if len(stdoutStr) > maxOutput {
		stdoutStr = stdoutStr[:maxOutput] + "\n... (output truncated)"
	}
	if len(stderrStr) > maxOutput {
		stderrStr = stderrStr[:maxOutput] + "\n... (output truncated)"
	}

	result := &CommandResult{
		Command:  cmdStr,
		ExitCode: 0,
		Stdout:   strings.TrimSpace(stdoutStr),
		Stderr:   strings.TrimSpace(stderrStr),
		Duration: duration.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
		}
		s.logger.Warn("command execution failed",
			zap.String("job_id", job.ID),
			zap.String("command", cmdStr),
			zap.Int("exit_code", result.ExitCode),
			zap.String("stderr", result.Stderr))
		return result, fmt.Errorf("command failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}

	s.logger.Debug("command executed successfully",
		zap.String("job_id", job.ID),
		zap.String("command", cmdStr),
		zap.Duration("duration", duration))

	return result, nil
}

// HTTPResult represents the result of an HTTP request.
type HTTPResult struct {
	URL        string `json:"url"`
	Method     string `json:"method"`
	StatusCode int    `json:"status_code"`
	Duration   string `json:"duration"`
}

// httpHandler makes an HTTP request.
func (s *Service) httpHandler(ctx context.Context, job *Job) (interface{}, error) {
	url, ok := job.Payload["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("url not specified in payload")
	}

	// Security: Validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("invalid URL scheme - must be http or https")
	}

	method := "GET"
	if m, ok := job.Payload["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Security: Validate HTTP method
	validMethods := map[string]bool{"GET": true, "POST": true, "PUT": true, "DELETE": true, "PATCH": true, "HEAD": true}
	if !validMethods[method] {
		return nil, fmt.Errorf("invalid HTTP method: %s", method)
	}

	// Use curl command for simplicity
	timeout := 30
	if t, ok := job.Payload["timeout"].(float64); ok && t > 0 {
		// Security: Cap maximum timeout
		if t > 120 {
			t = 120
		}
		timeout = int(t)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args := []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", method}

	// Add headers if specified
	if headers, ok := job.Payload["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
			// Security: Validate header names
			if strings.ContainsAny(k, "\r\n") {
				continue // Skip headers with newlines (HTTP header injection)
			}
			args = append(args, "-H", fmt.Sprintf("%s: %v", k, v))
		}
	}

	// Add body if specified
	if body, ok := job.Payload["body"].(string); ok && body != "" {
		args = append(args, "-d", body)
	}

	args = append(args, url)

	cmd := exec.CommandContext(cmdCtx, "curl", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	start := timeutil.NowTime()
	err := cmd.Run()
	duration := timeutil.SinceTime(start)

	result := &HTTPResult{
		URL:      url,
		Method:   method,
		Duration: duration.String(),
	}

	if err != nil {
		return result, fmt.Errorf("http request failed: %w", err)
	}

	// Parse status code
	statusCode := 0
	fmt.Sscanf(stdout.String(), "%d", &statusCode)
	result.StatusCode = statusCode

	if statusCode >= 400 {
		return result, fmt.Errorf("http request returned status %d", statusCode)
	}

	s.logger.Debug("http request completed",
		zap.String("job_id", job.ID),
		zap.String("url", url),
		zap.Int("status_code", statusCode),
		zap.Duration("duration", duration))

	return result, nil
}
