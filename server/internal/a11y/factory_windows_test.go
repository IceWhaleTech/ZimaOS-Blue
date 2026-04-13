//go:build windows

package a11y

import (
	"context"
	"testing"
)

func TestDefaultHostBackend_WindowsHostOS(t *testing.T) {
	backend := DefaultHostBackend("")
	if backend == nil {
		t.Fatal("expected windows backend")
	}
	if backend.HostOS() != "windows" {
		t.Fatalf("HostOS() = %q, want windows", backend.HostOS())
	}
}

func TestWindowsCapabilities_ReportsMSAAPermission(t *testing.T) {
	backend := DefaultHostBackend("")
	result, err := backend.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}
	if len(result.Permissions) != 1 {
		t.Fatalf("permissions len = %d, want 1", len(result.Permissions))
	}
	if result.Permissions[0].Name != "msaa" {
		t.Fatalf("permission name = %q, want msaa", result.Permissions[0].Name)
	}
}
