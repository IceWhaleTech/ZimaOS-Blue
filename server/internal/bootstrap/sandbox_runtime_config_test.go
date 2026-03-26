package bootstrap

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func TestNewSandboxManagerFromConfig_Disabled(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			Sandbox: config.SandboxConfig{
				Enabled: false,
			},
		},
	}

	manager, err := NewSandboxManagerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewSandboxManagerFromConfig() error = %v", err)
	}
	if manager != nil {
		t.Fatal("NewSandboxManagerFromConfig() manager should be nil when sandbox is disabled")
	}
}

func TestNewSandboxManagerFromConfig_AppliesRuntimeConfig(t *testing.T) {
	cfg := &config.Config{
		Security: config.SecurityConfig{
			Sandbox: config.SandboxConfig{
				Enabled:        true,
				DefaultTimeout: 45 * time.Second,
				MaxTimeout:     2 * time.Minute,
				MemoryLimit:    "512MB",
				CPULimit:       1.5,
				ProcessLimit:   16,
				NetworkEnabled: true,
			},
		},
	}

	manager, err := NewSandboxManagerFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewSandboxManagerFromConfig() error = %v", err)
	}
	if manager == nil {
		t.Fatal("NewSandboxManagerFromConfig() manager should not be nil")
	}

	runtimeCfg := manager.GetConfig()
	if runtimeCfg.DefaultTimeout != 45*time.Second {
		t.Fatalf("DefaultTimeout = %v, want %v", runtimeCfg.DefaultTimeout, 45*time.Second)
	}
	if runtimeCfg.MaxTimeout != 2*time.Minute {
		t.Fatalf("MaxTimeout = %v, want %v", runtimeCfg.MaxTimeout, 2*time.Minute)
	}
	if runtimeCfg.MemoryLimit != 512*1024*1024 {
		t.Fatalf("MemoryLimit = %d, want %d", runtimeCfg.MemoryLimit, 512*1024*1024)
	}
	if runtimeCfg.CPULimit != 1.5 {
		t.Fatalf("CPULimit = %v, want %v", runtimeCfg.CPULimit, 1.5)
	}
	if runtimeCfg.ProcessLimit != 16 {
		t.Fatalf("ProcessLimit = %d, want %d", runtimeCfg.ProcessLimit, 16)
	}
	if !runtimeCfg.NetworkEnabled {
		t.Fatal("NetworkEnabled = false, want true")
	}
}

func TestParseSandboxMemoryLimitBytes(t *testing.T) {
	got, err := parseSandboxMemoryLimitBytes("1.5GiB")
	if err != nil {
		t.Fatalf("parseSandboxMemoryLimitBytes() error = %v", err)
	}
	want := int64(1536 * 1024 * 1024)
	if got != want {
		t.Fatalf("parseSandboxMemoryLimitBytes() = %d, want %d", got, want)
	}
}
