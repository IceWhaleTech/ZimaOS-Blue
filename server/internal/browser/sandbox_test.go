package browser

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "sandbox-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	config := &SandboxConfig{
		Enabled:       true,
		TempDir:       tempDir,
		MaxMemoryMB:   256,
		CleanupOnExit: true,
	}

	sandbox, err := NewSandbox(config)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	defer sandbox.Close()

	if sandbox == nil {
		t.Fatal("expected non-nil sandbox")
	}
}

func TestSandbox_CreateSession(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "sandbox-test-"+time.Now().Format("20060102150405"))
	defer os.RemoveAll(tempDir)

	config := &SandboxConfig{
		Enabled:        true,
		TempDir:        tempDir,
		SessionTimeout: 1 * time.Hour,
		CleanupOnExit:  true,
	}

	sandbox, err := NewSandbox(config)
	if err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
	defer sandbox.Close()

	ctx := context.Background()

	t.Run("create session", func(t *testing.T) {
		session, err := sandbox.CreateSession(ctx, "test-session-1")
		if err != nil {
			t.Fatalf("failed to create session: %v", err)
		}

		if session.ID != "test-session-1" {
			t.Errorf("expected ID 'test-session-1', got '%s'", session.ID)
		}
		if !session.Active {
			t.Error("expected session to be active")
		}
		if session.ProfilePath == "" {
			t.Error("expected non-empty profile path")
		}
	})

	t.Run("duplicate session", func(t *testing.T) {
		_, err := sandbox.CreateSession(ctx, "test-session-1")
		if err == nil {
			t.Error("expected error for duplicate session")
		}
	})

	t.Run("get session", func(t *testing.T) {
		session, err := sandbox.GetSession("test-session-1")
		if err != nil {
			t.Fatalf("failed to get session: %v", err)
		}
		if session.ID != "test-session-1" {
			t.Errorf("expected ID 'test-session-1', got '%s'", session.ID)
		}
	})

	t.Run("list sessions", func(t *testing.T) {
		sessions := sandbox.ListSessions()
		if len(sessions) != 1 {
			t.Errorf("expected 1 session, got %d", len(sessions))
		}
	})

	t.Run("destroy session", func(t *testing.T) {
		err := sandbox.DestroySession(ctx, "test-session-1")
		if err != nil {
			t.Fatalf("failed to destroy session: %v", err)
		}

		sessions := sandbox.ListSessions()
		if len(sessions) != 0 {
			t.Errorf("expected 0 sessions, got %d", len(sessions))
		}
	})
}

func TestSandbox_GetBrowserArgs(t *testing.T) {
	config := &SandboxConfig{
		Enabled:           true,
		TempDir:           os.TempDir(),
		DisableGPU:        true,
		DisableJavaScript: false,
		DisableImages:     true,
		DisablePlugins:    true,
		DisablePopups:     true,
		MaxMemoryMB:       512,
	}

	sandbox, _ := NewSandbox(config)
	defer sandbox.Close()

	session := &SandboxSession{
		ID:          "test",
		ProfilePath: "/tmp/test-profile",
	}

	args := sandbox.GetBrowserArgs(session)

	// Check that essential args are present
	hasUserDataDir := false
	hasDisableGPU := false
	hasDisableImages := false

	for _, arg := range args {
		if arg == "--user-data-dir=/tmp/test-profile" {
			hasUserDataDir = true
		}
		if arg == "--disable-gpu" {
			hasDisableGPU = true
		}
		if arg == "--blink-settings=imagesEnabled=false" {
			hasDisableImages = true
		}
	}

	if !hasUserDataDir {
		t.Error("expected --user-data-dir argument")
	}
	if !hasDisableGPU {
		t.Error("expected --disable-gpu argument")
	}
	if !hasDisableImages {
		t.Error("expected --blink-settings=imagesEnabled=false argument")
	}
}

func TestSandbox_ResourceLimits(t *testing.T) {
	config := &SandboxConfig{
		Enabled:       true,
		TempDir:       os.TempDir(),
		MaxMemoryMB:   256,
		MaxCPUPercent: 50,
	}

	sandbox, _ := NewSandbox(config)
	defer sandbox.Close()

	t.Run("within limits", func(t *testing.T) {
		session := &SandboxSession{
			ResourceUsage: &ResourceUsage{
				MemoryMB:   128,
				CPUPercent: 25,
			},
		}

		err := sandbox.CheckResourceLimits(session)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("memory exceeded", func(t *testing.T) {
		session := &SandboxSession{
			ResourceUsage: &ResourceUsage{
				MemoryMB:   512,
				CPUPercent: 25,
			},
		}

		err := sandbox.CheckResourceLimits(session)
		if err != ErrResourceLimitExceeded {
			t.Errorf("expected ErrResourceLimitExceeded, got %v", err)
		}
	})

	t.Run("cpu exceeded", func(t *testing.T) {
		session := &SandboxSession{
			ResourceUsage: &ResourceUsage{
				MemoryMB:   128,
				CPUPercent: 75,
			},
		}

		err := sandbox.CheckResourceLimits(session)
		if err != ErrResourceLimitExceeded {
			t.Errorf("expected ErrResourceLimitExceeded, got %v", err)
		}
	})
}

func TestSandbox_NetworkIsolation(t *testing.T) {
	config := &SandboxConfig{
		Enabled:          true,
		TempDir:          os.TempDir(),
		NetworkIsolation: true,
		AllowedHosts:     []string{"example.com", "api.example.com"},
	}

	sandbox, _ := NewSandbox(config)
	defer sandbox.Close()

	t.Run("allowed host", func(t *testing.T) {
		if !sandbox.IsNetworkAllowed("example.com") {
			t.Error("expected example.com to be allowed")
		}
		if !sandbox.IsNetworkAllowed("api.example.com") {
			t.Error("expected api.example.com to be allowed")
		}
	})

	t.Run("blocked host", func(t *testing.T) {
		if sandbox.IsNetworkAllowed("other.com") {
			t.Error("expected other.com to be blocked")
		}
	})

	t.Run("no isolation", func(t *testing.T) {
		config.NetworkIsolation = false
		if !sandbox.IsNetworkAllowed("any.com") {
			t.Error("expected any.com to be allowed when isolation is disabled")
		}
	})
}
