package a11y

import (
	"context"
	"testing"
)

func TestWindowsSnapshotWithImageFallback_AttachesImageOnSuccess(t *testing.T) {
	resolveCalls := 0
	snapshotCalls := 0
	captureCalls := 0

	result, err := windowsSnapshotWithImageFallback(
		context.Background(),
		"windows",
		"win-9",
		true,
		func(windowID string) (uintptr, WindowInfo, error) {
			resolveCalls++
			if windowID != "win-9" {
				t.Fatalf("windowID = %q, want win-9", windowID)
			}
			return 9, WindowInfo{ID: "win-9", Title: "Feishu"}, nil
		},
		func(hostOS string, hwnd uintptr, info WindowInfo, interactiveOnly bool) (SnapshotResult, error) {
			snapshotCalls++
			if hostOS != "windows" {
				t.Fatalf("hostOS = %q, want windows", hostOS)
			}
			if hwnd != 9 {
				t.Fatalf("hwnd = %d, want 9", hwnd)
			}
			if info.ID != "win-9" {
				t.Fatalf("info.ID = %q, want win-9", info.ID)
			}
			if !interactiveOnly {
				t.Fatal("interactiveOnly = false, want true")
			}
			return SnapshotResult{
				HostOS:   hostOS,
				WindowID: info.ID,
				Title:    info.Title,
				Tree:     "@1 [button] \"Continue\"",
				RefMap:   map[int]string{1: "win-9|0"},
			}, nil
		},
		func(_ context.Context, windowID string) (ScreenshotResult, error) {
			captureCalls++
			if windowID != "win-9" {
				t.Fatalf("capture windowID = %q, want win-9", windowID)
			}
			return ScreenshotResult{WindowID: windowID, ImagePath: "/tmp/host-window-9.png"}, nil
		},
	)
	if err != nil {
		t.Fatalf("windowsSnapshotWithImageFallback() error = %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("resolveCalls = %d, want 1", resolveCalls)
	}
	if snapshotCalls != 1 {
		t.Fatalf("snapshotCalls = %d, want 1", snapshotCalls)
	}
	if captureCalls != 1 {
		t.Fatalf("captureCalls = %d, want 1", captureCalls)
	}
	if result.WindowID != "win-9" {
		t.Fatalf("window_id = %q, want win-9", result.WindowID)
	}
	if result.ImagePath != "/tmp/host-window-9.png" {
		t.Fatalf("image_path = %q, want /tmp/host-window-9.png", result.ImagePath)
	}
}

func TestWindowsSnapshotWithImageFallback_EnrichesBackendUnavailableWithImage(t *testing.T) {
	err := func() error {
		_, err := windowsSnapshotWithImageFallback(
			context.Background(),
			"windows",
			"win-11",
			false,
			func(windowID string) (uintptr, WindowInfo, error) {
				if windowID != "win-11" {
					t.Fatalf("windowID = %q, want win-11", windowID)
				}
				return 11, WindowInfo{ID: "win-11", Title: "Lark"}, nil
			},
			func(hostOS string, hwnd uintptr, info WindowInfo, interactiveOnly bool) (SnapshotResult, error) {
				if hostOS != "windows" {
					t.Fatalf("hostOS = %q, want windows", hostOS)
				}
				if hwnd != 11 {
					t.Fatalf("hwnd = %d, want 11", hwnd)
				}
				if interactiveOnly {
					t.Fatal("interactiveOnly = true, want false")
				}
				return SnapshotResult{}, NewError("backend_unavailable", "MSAA snapshot is empty", map[string]interface{}{
					"window_id": info.ID,
				})
			},
			func(_ context.Context, windowID string) (ScreenshotResult, error) {
				if windowID != "win-11" {
					t.Fatalf("capture windowID = %q, want win-11", windowID)
				}
				return ScreenshotResult{WindowID: windowID, ImagePath: "/tmp/host-window-11.png"}, nil
			},
		)
		return err
	}()
	if err == nil {
		t.Fatal("windowsSnapshotWithImageFallback() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if got := runtimeErr.Details["window_id"]; got != "win-11" {
		t.Fatalf("window_id = %v, want win-11", got)
	}
	if got := runtimeErr.Details["image_path"]; got != "/tmp/host-window-11.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-11.png", got)
	}
}
