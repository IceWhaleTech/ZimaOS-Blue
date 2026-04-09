package bootstrap

import (
	"testing"
	"time"
)

func TestNewRuntimeExecConfig_AppliesSharedDefaultsAndCopiesInputs(t *testing.T) {
	allowedDirs := []string{"/tmp/workspace", "/tmp/cache"}

	cfg := NewRuntimeExecConfig(" /tmp/data ", allowedDirs)

	if cfg.DataDir != "/tmp/data" {
		t.Fatalf("DataDir = %q, want %q", cfg.DataDir, "/tmp/data")
	}
	if cfg.DefaultTimeout != 5*time.Minute {
		t.Fatalf("DefaultTimeout = %v, want %v", cfg.DefaultTimeout, 5*time.Minute)
	}
	if cfg.MaxTimeout != 30*time.Minute {
		t.Fatalf("MaxTimeout = %v, want %v", cfg.MaxTimeout, 30*time.Minute)
	}
	if len(cfg.AllowedDirs) != 2 || cfg.AllowedDirs[0] != "/tmp/workspace" || cfg.AllowedDirs[1] != "/tmp/cache" {
		t.Fatalf("AllowedDirs = %#v, want copied input", cfg.AllowedDirs)
	}

	allowedDirs[0] = "/mutated"
	if cfg.AllowedDirs[0] != "/tmp/workspace" {
		t.Fatalf("AllowedDirs should be copied, got %#v", cfg.AllowedDirs)
	}
}
