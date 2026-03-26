//go:build windows

package sandbox

import (
	"errors"
	"testing"
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
	if executor.IsSupported() {
		t.Fatal("newPlatformExecutor() should fail closed until Windows Job Object isolation is implemented")
	}
}

func TestWindowsExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	if executor.IsSupported() {
		t.Error("IsSupported() should return false until Windows sandbox isolation is implemented")
	}
}

func TestWindowsExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	result, err := executor.Execute(nil, NewExecutionRequest("cmd", "/c", "echo", "hello"))
	if !errors.Is(err, ErrSandboxNotSupported) {
		t.Fatalf("Execute() error = %v, want ErrSandboxNotSupported", err)
	}
	if result != nil {
		t.Fatalf("Execute() result = %#v, want nil when sandbox is unsupported", result)
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
