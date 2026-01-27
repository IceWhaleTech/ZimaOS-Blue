//go:build darwin

package sandbox

import (
	"context"
	"testing"
	"time"
)

func TestDarwinExecutor_NewPlatformExecutor(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)

	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	if executor == nil {
		t.Fatal("newPlatformExecutor() returned nil")
	}
}

func TestDarwinExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

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

	if result.Status != StatusCompleted {
		t.Errorf("Execute() Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestDarwinExecutor_Execute_WithEnv(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("printenv", "TEST_VAR")
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

func TestDarwinExecutor_Execute_Timeout(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

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

func TestDarwinExecutor_Execute_Failed(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("false")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(failed) error = %v", err)
	}

	if result.Status != StatusFailed {
		t.Errorf("Execute(failed) Status = %v, want %v", result.Status, StatusFailed)
	}
}

func TestDarwinExecutor_Execute_ResourceUsage(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.ResourceUsage == nil {
		t.Log("ResourceUsage is nil, may not be available")
	} else {
		if result.ResourceUsage.CPUTime < 0 {
			t.Error("ResourceUsage CPUTime should not be negative")
		}
		// On macOS, Maxrss is already in bytes
		if result.ResourceUsage.MemoryPeak < 0 {
			t.Error("ResourceUsage MemoryPeak should not be negative")
		}
	}
}

func TestDarwinExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// IsSupported checks for sandbox-exec availability
	supported := executor.IsSupported()
	t.Logf("Darwin sandbox supported: %v", supported)
}

func TestDarwinExecutor_GetStatus(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("echo", "hello")
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

func TestDarwinExecutor_Kill(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

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

func TestDarwinExecutor_Cleanup(t *testing.T) {
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

func TestSandboxProfile(t *testing.T) {
	profile := sandboxProfile()

	if profile == "" {
		t.Error("sandboxProfile() should not return empty string")
	}

	// Check that profile contains expected directives
	expectedDirectives := []string{
		"(version 1)",
		"(deny default)",
		"(allow process-fork)",
		"(allow process-exec)",
		"(allow file-read*)",
	}

	for _, directive := range expectedDirectives {
		if !containsString(profile, directive) {
			t.Errorf("sandboxProfile() should contain '%s'", directive)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
