//go:build windows

package sandbox

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if !executor.IsSupported() {
		t.Fatal("newPlatformExecutor() should return a supported Windows executor")
	}
}

func TestWindowsExecutor_PreparesHomeSandboxWorkDir(t *testing.T) {
	config := DefaultConfig()
	_, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("UserHomeDir() error = %v, home=%q", err, home)
	}

	wantPrefix := filepath.Join(home, ".zimaos-blue")
	if !strings.HasPrefix(filepath.Clean(config.WorkDir), filepath.Clean(wantPrefix)) {
		t.Fatalf("WorkDir = %q, want path under %q", config.WorkDir, wantPrefix)
	}
}

func TestWindowsExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	if !executor.IsSupported() {
		t.Error("IsSupported() should return true on Windows")
	}
}

func TestWindowsExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("cmd", "/c", "echo", "hello")
	req.Timeout = 5 * time.Second

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil {
		t.Fatal("Execute() returned nil result")
	}
	if result.Status != StatusCompleted {
		t.Fatalf("Execute() status = %v, want %v (stderr=%q, error=%q)", result.Status, StatusCompleted, result.Stderr, result.Error)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("Execute() stdout = %q, want hello", result.Stdout)
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

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute(env) error = %v", err)
	}

	if result.Status != StatusCompleted {
		t.Fatalf("Execute(env) status = %v, want %v (stderr=%q, error=%q)", result.Status, StatusCompleted, result.Stderr, result.Error)
	}
	if strings.TrimSpace(result.Stdout) != "test_value" {
		t.Fatalf("Execute(env) stdout = %q, want test_value", result.Stdout)
	}
}

func TestWindowsExecutor_Execute_Timeout(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	req := NewExecutionRequest("cmd", "/c", "ping", "127.0.0.1", "-n", "6")
	req.Timeout = 100 * time.Millisecond

	result, err := executor.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute(timeout) error = %v", err)
	}

	if result.Status != StatusTimeout {
		t.Fatalf("Execute(timeout) status = %v, want %v (stderr=%q, error=%q)", result.Status, StatusTimeout, result.Stderr, result.Error)
	}
}

func TestWindowsExecutor_GetStatus(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	_, err = executor.GetStatus("missing")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Fatalf("GetStatus() error = %v, want ErrExecutionNotFound", err)
	}
}

func TestWindowsExecutor_Kill(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	err = executor.Kill("missing")
	if !errors.Is(err, ErrExecutionNotFound) {
		t.Fatalf("Kill() error = %v, want ErrExecutionNotFound", err)
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
