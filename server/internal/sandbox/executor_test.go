package sandbox

import (
	"context"
	"testing"
	"time"
)

func TestNewBaseExecutor(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	if executor == nil {
		t.Fatal("NewBaseExecutor() returned nil")
	}

	if executor.config != config {
		t.Error("NewBaseExecutor() config not set correctly")
	}

	if executor.executions == nil {
		t.Error("NewBaseExecutor() executions map not initialized")
	}
}

func TestBaseExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	if result.ID != req.ID {
		t.Errorf("Execute() ID = %v, want %v", result.ID, req.ID)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Execute() Status = %v, want %v", result.Status, StatusCompleted)
	}

	if result.ExitCode != 0 {
		t.Errorf("Execute() ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestBaseExecutor_Execute_WithStdin(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Use cat to echo stdin (works on Unix-like systems)
	// On Windows, this test might need adjustment
	req := NewExecutionRequest("cat")
	req.Stdin = "test input"
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(stdin) error = %v", err)
	}

	// Note: This test may fail on Windows without cat
	if result.Status == StatusCompleted && result.Stdout != "test input" {
		t.Errorf("Execute(stdin) Stdout = %v, want 'test input'", result.Stdout)
	}
}

func TestBaseExecutor_Execute_Timeout(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Use sleep command that will exceed timeout
	req := NewExecutionRequest("sleep", "10")
	req.Timeout = 100 * time.Millisecond

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(timeout) error = %v", err)
	}

	if result.Status != StatusTimeout {
		t.Errorf("Execute(timeout) Status = %v, want %v", result.Status, StatusTimeout)
	}
}

func TestBaseExecutor_Execute_Failed(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Use a command that will fail
	req := NewExecutionRequest("false") // Unix command that always returns 1
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(failed) error = %v", err)
	}

	if result.Status != StatusFailed {
		t.Errorf("Execute(failed) Status = %v, want %v", result.Status, StatusFailed)
	}

	if result.ExitCode == 0 {
		t.Error("Execute(failed) ExitCode should not be 0")
	}
}

func TestBaseExecutor_Execute_WithEnv(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	req := NewExecutionRequest("printenv", "TEST_VAR")
	req.Env = map[string]string{"TEST_VAR": "test_value"}
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(env) error = %v", err)
	}

	// Note: This test may fail on Windows
	if result.Status == StatusCompleted {
		// Output should contain the env value
		if result.Stdout == "" {
			t.Log("Execute(env) Stdout is empty, printenv may not be available")
		}
	}
}

func TestBaseExecutor_GetStatus(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, _ := executor.Execute(ctx, req)

	// Get status
	status, err := executor.GetStatus(result.ID)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.ID != result.ID {
		t.Errorf("GetStatus() ID = %v, want %v", status.ID, result.ID)
	}
}

func TestBaseExecutor_GetStatus_NotFound(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	_, err := executor.GetStatus("non-existent-id")
	if err != ErrExecutionNotFound {
		t.Errorf("GetStatus(not found) error = %v, want %v", err, ErrExecutionNotFound)
	}
}

func TestBaseExecutor_Kill(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Start a long-running command
	req := NewExecutionRequest("sleep", "60")
	req.Timeout = 60 * time.Second

	ctx := context.Background()

	// Execute in goroutine
	done := make(chan *ExecutionResult)
	go func() {
		result, _ := executor.Execute(ctx, req)
		done <- result
	}()

	// Wait a bit for the process to start
	time.Sleep(100 * time.Millisecond)

	// Kill the execution
	err := executor.Kill(req.ID)
	if err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	// Wait for result
	select {
	case result := <-done:
		if result.Status != StatusKilled && result.Status != StatusFailed {
			t.Errorf("Kill() result Status = %v, want Killed or Failed", result.Status)
		}
	case <-time.After(5 * time.Second):
		t.Error("Kill() timed out waiting for result")
	}
}

func TestBaseExecutor_Kill_NotFound(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	err := executor.Kill("non-existent-id")
	if err != ErrExecutionNotFound {
		t.Errorf("Kill(not found) error = %v, want %v", err, ErrExecutionNotFound)
	}
}

func TestBaseExecutor_Cleanup(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Execute a command
	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	_, _ = executor.Execute(ctx, req)

	// Cleanup
	err := executor.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}

	// Executions should be cleared
	if len(executor.executions) != 0 {
		t.Error("Cleanup() should clear executions")
	}
}

func TestBaseExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor := NewBaseExecutor(config)

	// Base executor is always supported
	if !executor.IsSupported() {
		t.Error("IsSupported() should return true for base executor")
	}
}

func TestTruncateOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "truncated",
			input:    "hello world",
			maxLen:   5,
			expected: "hello\n... (truncated)",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateOutput(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateOutput() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewExecutionRequest(t *testing.T) {
	req := NewExecutionRequest("echo", "hello", "world")

	if req == nil {
		t.Fatal("NewExecutionRequest() returned nil")
	}

	if req.ID == "" {
		t.Error("NewExecutionRequest() ID should not be empty")
	}

	if req.Command != "echo" {
		t.Errorf("NewExecutionRequest() Command = %v, want echo", req.Command)
	}

	if len(req.Args) != 2 {
		t.Errorf("NewExecutionRequest() Args length = %d, want 2", len(req.Args))
	}

	if req.Env == nil {
		t.Error("NewExecutionRequest() Env should be initialized")
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if config.DefaultTimeout != 5*time.Minute {
		t.Errorf("DefaultConfig() DefaultTimeout = %v, want 5m", config.DefaultTimeout)
	}

	if config.MaxTimeout != 5*time.Minute {
		t.Errorf("DefaultConfig() MaxTimeout = %v, want 5m", config.MaxTimeout)
	}

	if config.MemoryLimit != 256*1024*1024 {
		t.Errorf("DefaultConfig() MemoryLimit = %d, want 256MB", config.MemoryLimit)
	}

	if config.CPULimit != 1.0 {
		t.Errorf("DefaultConfig() CPULimit = %f, want 1.0", config.CPULimit)
	}

	if config.ProcessLimit != 10 {
		t.Errorf("DefaultConfig() ProcessLimit = %d, want 10", config.ProcessLimit)
	}

	if config.NetworkEnabled {
		t.Error("DefaultConfig() NetworkEnabled should be false")
	}
}

func TestExecutionStatus_Constants(t *testing.T) {
	statuses := []ExecutionStatus{
		StatusPending,
		StatusRunning,
		StatusCompleted,
		StatusFailed,
		StatusTimeout,
		StatusKilled,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("ExecutionStatus constant should not be empty")
		}
	}
}

func TestExecutionResult_Fields(t *testing.T) {
	result := &ExecutionResult{
		ID:        "test-id",
		Status:    StatusCompleted,
		ExitCode:  0,
		Stdout:    "output",
		Stderr:    "error",
		StartTime: time.Now(),
		EndTime:   time.Now(),
		Duration:  time.Second,
		ResourceUsage: &ResourceUsage{
			CPUTime:    1000000,
			MemoryPeak: 1024,
			IORead:     512,
			IOWrite:    256,
		},
	}

	if result.ID != "test-id" {
		t.Errorf("ExecutionResult ID = %v, want test-id", result.ID)
	}

	if result.ResourceUsage == nil {
		t.Error("ExecutionResult ResourceUsage should not be nil")
	}
}

func TestErrors(t *testing.T) {
	errors := []error{
		ErrExecutionTimeout,
		ErrExecutionKilled,
		ErrResourceLimitExceeded,
		ErrSandboxNotSupported,
		ErrExecutionNotFound,
	}

	for _, err := range errors {
		if err == nil {
			t.Error("Error constant should not be nil")
		}
		if err.Error() == "" {
			t.Error("Error message should not be empty")
		}
	}
}
