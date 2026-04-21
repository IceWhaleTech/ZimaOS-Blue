//go:build windows

package a11y

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestWindowsScreenshot_UsesComputerUseMediaDir(t *testing.T) {
	prevResolve := windowsResolveWindowFunc
	prevGetRect := windowsGetRectFunc
	prevCapture := windowsCaptureWindowFunc
	prevActiveCapture := windowsCaptureActiveWindowFunc
	windowsResolveWindowFunc = func(windowID string) (uintptr, WindowInfo, error) {
		return 42, WindowInfo{ID: windowID, Title: "Feishu"}, nil
	}
	windowsGetRectFunc = func(hwnd uintptr) (windowsRect, error) {
		if hwnd != 42 {
			t.Fatalf("hwnd = %d, want 42", hwnd)
		}
		return windowsRect{Left: 10, Top: 20, Right: 210, Bottom: 120}, nil
	}
	var capturedPath string
	windowsCaptureWindowFunc = func(width int, height int, left int32, top int32, path string) (string, error) {
		capturedPath = path
		if width != 200 || height != 100 {
			t.Fatalf("capture size = %dx%d, want 200x100", width, height)
		}
		if left != 10 || top != 20 {
			t.Fatalf("capture origin = (%d,%d), want (10,20)", left, top)
		}
		if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
			t.Fatalf("write capture file: %v", err)
		}
		return "", nil
	}
	windowsCaptureActiveWindowFunc = func(path string) (string, error) {
		t.Fatalf("active capture fallback should not run, got path %q", path)
		return "", nil
	}
	defer func() {
		windowsResolveWindowFunc = prevResolve
		windowsGetRectFunc = prevGetRect
		windowsCaptureWindowFunc = prevCapture
		windowsCaptureActiveWindowFunc = prevActiveCapture
	}()

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media")).(*windowsBackend)
	result, err := backend.Screenshot(context.Background(), "42")
	if err != nil {
		t.Fatalf("Screenshot() error = %v", err)
	}
	if capturedPath == "" {
		t.Fatal("capturedPath = empty, want capture path")
	}
	if result.ImagePath != capturedPath {
		t.Fatalf("image_path = %q, want %q", result.ImagePath, capturedPath)
	}
	if !strings.HasSuffix(result.ImagePath, "host-window-42.png") {
		t.Fatalf("image_path = %q, want suffix host-window-42.png", result.ImagePath)
	}
	if !strings.Contains(result.ImagePath, string(filepath.Separator)+"computer-use"+string(filepath.Separator)) {
		t.Fatalf("image_path = %q, want computer-use media dir", result.ImagePath)
	}
}
