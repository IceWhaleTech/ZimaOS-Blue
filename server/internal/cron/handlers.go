// Package cron provides scheduled task management.
package cron

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

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

// RegisterBuiltinHandlers registers all built-in job handlers.
func (s *Service) RegisterBuiltinHandlers() {
	s.RegisterHandler("command", s.commandHandler)
	s.RegisterHandler("http", s.httpHandler)
	s.logger.Info("built-in handlers registered", zap.Int("count", 2))
}

// commandHandler executes a shell command.
func (s *Service) commandHandler(ctx context.Context, job *Job) (interface{}, error) {
	// Get command from payload
	cmdStr, ok := job.Payload["command"].(string)
	if !ok || cmdStr == "" {
		return nil, fmt.Errorf("command not specified in payload")
	}

	// Get optional working directory
	workDir, _ := job.Payload["workdir"].(string)

	// Get optional timeout (in seconds)
	timeout := 60 // default 60 seconds
	if t, ok := job.Payload["timeout"].(float64); ok && t > 0 {
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

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &CommandResult{
		Command:  cmdStr,
		ExitCode: 0,
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
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

	method := "GET"
	if m, ok := job.Payload["method"].(string); ok && m != "" {
		method = strings.ToUpper(m)
	}

	// Use curl command for simplicity
	timeout := 30
	if t, ok := job.Payload["timeout"].(float64); ok && t > 0 {
		timeout = int(t)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	args := []string{"-s", "-o", "/dev/null", "-w", "%{http_code}", "-X", method}

	// Add headers if specified
	if headers, ok := job.Payload["headers"].(map[string]interface{}); ok {
		for k, v := range headers {
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

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

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
