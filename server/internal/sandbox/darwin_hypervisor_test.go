//go:build darwin

package sandbox

import (
	"context"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestHypervisorExecutor_NewHypervisorExecutor(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)

	// It's OK if this fails on systems without VM support
	if err != nil {
		t.Logf("NewHypervisorExecutor() error = %v (may be expected on this system)", err)
		return
	}

	if executor == nil {
		t.Fatal("NewHypervisorExecutor() returned nil")
	}

	t.Logf("HypervisorExecutor created with backend: %s, architecture: %s",
		executor.GetBackend(), executor.GetArchitecture())
}

func TestHypervisorExecutor_DetectBestBackend(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	info := executor.GetInfo()
	t.Logf("Hypervisor Info:")
	t.Logf("  Backend: %s", info.Backend)
	t.Logf("  Architecture: %s", info.Architecture)
	t.Logf("  Virtualization.framework: %v", info.VirtualizationFrameworkSupport)
	t.Logf("  QEMU: %v", info.QEMUSupport)
	t.Logf("  xhyve: %v", info.XhyveSupport)
}

func TestHypervisorExecutor_Execute(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	req := NewExecutionRequest("echo", "hello from hypervisor")
	req.Timeout = 10 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if result == nil {
		t.Fatal("Execute() returned nil result")
	}

	t.Logf("Execution result: status=%s, stdout=%q, stderr=%q, error=%q",
		result.Status, result.Stdout, result.Stderr, result.Error)

	if result.Status != StatusCompleted {
		t.Errorf("Execute() Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestHypervisorExecutor_Execute_WithEnv(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	req := NewExecutionRequest("printenv", "TEST_VAR")
	req.Env = map[string]string{"TEST_VAR": "hypervisor_test_value"}
	req.Timeout = 10 * time.Second

	ctx := context.Background()
	result, err := executor.Execute(ctx, req)

	if err != nil {
		t.Fatalf("Execute(env) error = %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("Execute(env) Status = %v, want %v", result.Status, StatusCompleted)
	}
}

func TestHypervisorExecutor_Execute_Timeout(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
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

func TestHypervisorExecutor_IsSupported(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	supported := executor.IsSupported()
	t.Logf("Hypervisor sandbox supported: %v", supported)
}

func TestHypervisorExecutor_Cleanup(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	err = executor.Cleanup()
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestHypervisorExecutor_Architecture(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	arch := executor.GetArchitecture()
	expectedArch := runtime.GOARCH

	if arch != expectedArch {
		t.Errorf("GetArchitecture() = %v, want %v", arch, expectedArch)
	}

	t.Logf("Running on architecture: %s", arch)
}

func TestHypervisorConfig_Default(t *testing.T) {
	config := DefaultHypervisorConfig()

	if config == nil {
		t.Fatal("DefaultHypervisorConfig() returned nil")
	}

	if config.Backend != VMBackendNative {
		t.Errorf("Default backend = %v, want %v", config.Backend, VMBackendNative)
	}

	if config.MemoryMB <= 0 {
		t.Error("Default MemoryMB should be positive")
	}

	if config.CPUCount <= 0 {
		t.Error("Default CPUCount should be positive")
	}
}

func TestDarwinExecutorMode_Selection(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
	}{
		{"auto mode", "auto"},
		{"hypervisor mode", "hypervisor"},
		{"sandbox-exec mode", "sandbox-exec"},
		{"empty mode", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable
			if tt.envValue != "" {
				os.Setenv("BLUE_SANDBOX_MODE", tt.envValue)
				defer os.Unsetenv("BLUE_SANDBOX_MODE")
			} else {
				os.Unsetenv("BLUE_SANDBOX_MODE")
			}

			config := DefaultConfig()
			executor, err := newPlatformExecutor(config)

			// sandbox-exec mode should always work
			if tt.envValue == "sandbox-exec" || tt.envValue == "" {
				if err != nil {
					t.Fatalf("newPlatformExecutor() error = %v", err)
				}
				if executor == nil {
					t.Fatal("newPlatformExecutor() returned nil")
				}
			} else {
				// hypervisor mode may fail if not supported
				if err != nil {
					t.Logf("newPlatformExecutor() error = %v (may be expected)", err)
				}
			}
		})
	}
}

func TestHypervisorExecutor_GenerateSandboxProfile(t *testing.T) {
	config := DefaultConfig()
	executor, err := NewHypervisorExecutor(config, nil)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	req := NewExecutionRequest("echo", "test")
	req.WorkDir = "/tmp/test"

	profile := executor.generateSandboxProfile(req)

	if profile == "" {
		t.Error("generateSandboxProfile() should not return empty string")
	}

	// Check for expected directives
	expectedDirectives := []string{
		"(version 1)",
		"(deny default)",
		"(allow process-fork)",
		"(allow process-exec)",
		"/tmp",
	}

	for _, directive := range expectedDirectives {
		if !containsSubstring(profile, directive) {
			t.Errorf("generateSandboxProfile() should contain '%s'", directive)
		}
	}

	// Check network is denied by default
	if config.NetworkEnabled {
		if !containsSubstring(profile, "allow network") {
			t.Error("generateSandboxProfile() should allow network when enabled")
		}
	} else {
		if !containsSubstring(profile, "deny network") {
			t.Error("generateSandboxProfile() should deny network when disabled")
		}
	}
}

func TestJoinArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected string
	}{
		{"empty", []string{}, ""},
		{"single", []string{"arg1"}, "arg1"},
		{"multiple", []string{"arg1", "arg2", "arg3"}, "arg1 arg2 arg3"},
		{"with spaces", []string{"arg with space", "normal"}, "\"arg with space\" normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinArgs(tt.args)
			if result != tt.expected {
				t.Errorf("joinArgs(%v) = %q, want %q", tt.args, result, tt.expected)
			}
		})
	}
}

func TestContainsSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"nospace", false},
		{"has space", true},
		{"has\ttab", true},
		{"has\nnewline", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := containsSpace(tt.input)
			if result != tt.expected {
				t.Errorf("containsSpace(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestVMBackend_Constants(t *testing.T) {
	// Verify backend constants are defined correctly
	if VMBackendNative != "native" {
		t.Errorf("VMBackendNative = %q, want %q", VMBackendNative, "native")
	}
	if VMBackendXhyve != "xhyve" {
		t.Errorf("VMBackendXhyve = %q, want %q", VMBackendXhyve, "xhyve")
	}
	if VMBackendQEMU != "qemu" {
		t.Errorf("VMBackendQEMU = %q, want %q", VMBackendQEMU, "qemu")
	}
}

func TestHypervisorInfo_MarshalJSON(t *testing.T) {
	info := &HypervisorInfo{
		Backend:                        VMBackendNative,
		Architecture:                   "arm64",
		VirtualizationFrameworkSupport: true,
		QEMUSupport:                    false,
		XhyveSupport:                   false,
	}

	data, err := info.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("MarshalJSON() returned empty data")
	}

	t.Logf("HypervisorInfo JSON: %s", string(data))
}
