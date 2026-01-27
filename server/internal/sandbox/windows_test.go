//go:build windows

package sandbox

import (
	"context"
	"testing"
	"time"
)

func TestWindowsExecutor_NewPlatformExecutor(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)

	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	if executor == nil {
		t.Fatal("newPlatformExecutor() returned nil")
	}
}

func TestWindowsExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// Use cmd.exe on Windows
	req := NewExecutionRequest("cmd", "/c", "echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	if result.Status != StatusCompleted {
		t.Errorf("Execute() Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestWindowsExecutor_Execute_WithEnv(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("cmd", "/c", "echo", "%TEST_VAR%")
	req.Env = map[string]string{"TEST_VAR": "test_value"}
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(env) error = %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Execute(env) Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestWindowsExecutor_Execute_Timeout(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// Use ping with long timeout to simulate long-running process
	req := NewExecutionRequest("ping", "-n", "100", "127.0.0.1")
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

func TestWindowsExecutor_Execute_Failed(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// Use cmd /c exit 1 to simulate failure
	req := NewExecutionRequest("cmd", "/c", "exit", "1")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(failed) error = %v", err)
	}

	if result.Status != StatusFailed {
		t.Errorf("Execute(failed) Status = %v, want %v", result.Status, StatusFailed)
	}

	if result.ExitCode != 1 {
		t.Errorf("Execute(failed) ExitCode = %d, want 1", result.ExitCode)
	}
}

func TestWindowsExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// Windows executor should always be supported on Windows
	if !executor.IsSupported() {
		t.Error("IsSupported() should return true on Windows")
	}
}

func TestWindowsExecutor_GetStatus(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("cmd", "/c", "echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, _ := executor.Execute(ctx, req)

	status, err := executor.GetStatus(result.ID)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if status.ID != result.ID {
		t.Errorf("GetStatus() ID = %v, want %v", status.ID, result.ID)
	}
}

func TestWindowsExecutor_Kill(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// Start a long-running command
	req := NewExecutionRequest("ping", "-n", "100", "127.0.0.1")
	req.Timeout = 60 * time.Second

	ctx := context.Background()

	// Execute in goroutine
	done := make(chan *ExecutionResult)
	go func() {
		result, _ := executor.Execute(ctx, req)
		done <- result
	}()

	// Wait for process to start
	time.Sleep(100 * time.Millisecond)

	// Kill the execution
	err = executor.Kill(req.ID)
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

func TestWindowsExecutor_Cleanup(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	err = executor.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}
