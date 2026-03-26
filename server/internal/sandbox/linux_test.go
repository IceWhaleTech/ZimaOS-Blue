//go:build linux

package sandbox

import (
	"context"
	"testing"
	"time"
)

func skipIfSandboxUnsupported(t *testing.T, executor Executor) {
	t.Helper()
	if executor == nil || !executor.IsSupported() {
		t.Skip("sandbox is not supported in this Linux test environment")
	}
}

func TestLinuxExecutor_NewPlatformExecutor(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)

	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	if executor == nil {
		t.Fatal("newPlatformExecutor() returned nil")
	}
}

func TestLinuxExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}
	skipIfSandboxUnsupported(t, executor)

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

func TestLinuxExecutor_Execute_WithResourceLimits(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}
	skipIfSandboxUnsupported(t, executor)

	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second
	req.MemoryLimit = 128 * 1024 * 1024 // 128MB
	req.CPULimit = 0.5

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(limits) error = %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Execute(limits) Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestLinuxExecutor_Execute_ResourceUsage(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}
	skipIfSandboxUnsupported(t, executor)

	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result.ResourceUsage == nil {
		t.Log("ResourceUsage is nil, may not be available on this system")
	} else {
		if result.ResourceUsage.CPUTime < 0 {
			t.Error("ResourceUsage CPUTime should not be negative")
		}
	}
}

func TestLinuxExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := newPlatformExecutor(config)
	if err != nil {
		t.Fatalf("newPlatformExecutor() error = %v", err)
	}

	// IsSupported should return a boolean without error
	supported := executor.IsSupported()
	t.Logf("Linux sandbox supported: %v", supported)
}

func TestLinuxExecutor_Cleanup(t *testing.T) {
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

func TestDefaultSeccompProfile(t *testing.T) {
	profile := DefaultSeccompProfile()

	if profile == nil {
		t.Fatal("DefaultSeccompProfile() returned nil")
	}

	if profile.DefaultAction == "" {
		t.Error("DefaultSeccompProfile() DefaultAction should not be empty")
	}

	if len(profile.Syscalls) == 0 {
		t.Error("DefaultSeccompProfile() Syscalls should not be empty")
	}

	// Check that common syscalls are allowed
	foundRead := false
	foundWrite := false
	for _, rule := range profile.Syscalls {
		for _, name := range rule.Names {
			if name == "read" {
				foundRead = true
			}
			if name == "write" {
				foundWrite = true
			}
		}
	}

	if !foundRead {
		t.Error("DefaultSeccompProfile() should allow 'read' syscall")
	}

	if !foundWrite {
		t.Error("DefaultSeccompProfile() should allow 'write' syscall")
	}
}

func TestSeccompProfile_Fields(t *testing.T) {
	profile := &SeccompProfile{
		DefaultAction: "SCMP_ACT_ERRNO",
		Syscalls: []SyscallRule{
			{
				Names:  []string{"read", "write"},
				Action: "SCMP_ACT_ALLOW",
			},
		},
	}

	if profile.DefaultAction != "SCMP_ACT_ERRNO" {
		t.Errorf("SeccompProfile DefaultAction = %v, want SCMP_ACT_ERRNO", profile.DefaultAction)
	}

	if len(profile.Syscalls) != 1 {
		t.Errorf("SeccompProfile Syscalls length = %d, want 1", len(profile.Syscalls))
	}
}

func TestSyscallRule_Fields(t *testing.T) {
	rule := SyscallRule{
		Names:  []string{"read", "write", "open"},
		Action: "SCMP_ACT_ALLOW",
	}

	if len(rule.Names) != 3 {
		t.Errorf("SyscallRule Names length = %d, want 3", len(rule.Names))
	}

	if rule.Action != "SCMP_ACT_ALLOW" {
		t.Errorf("SyscallRule Action = %v, want SCMP_ACT_ALLOW", rule.Action)
	}
}
