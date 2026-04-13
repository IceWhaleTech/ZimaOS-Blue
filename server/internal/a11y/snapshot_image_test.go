package a11y

import (
	"context"
	"errors"
	"testing"
)

func TestAttachSnapshotImage_AttachesNativeScreenshotOnSuccess(t *testing.T) {
	base := SnapshotResult{
		HostOS:   "darwin",
		WindowID: "win-1",
		Title:    "Feishu",
		Tree:     "@1 [button] \"Open\"",
		RefMap:   map[int]string{1: "token-open"},
		Message:  "Host accessibility snapshot ready",
	}
	calls := 0

	got := attachSnapshotImage(context.Background(), base, func(_ context.Context, windowID string) (ScreenshotResult, error) {
		calls++
		if windowID != "win-1" {
			t.Fatalf("windowID = %q, want win-1", windowID)
		}
		return ScreenshotResult{
			HostOS:    "darwin",
			WindowID:  "win-9",
			ImagePath: "/tmp/host-window-9.png",
			Message:   "Host screenshot captured",
		}, nil
	})

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if got.ImagePath != "/tmp/host-window-9.png" {
		t.Fatalf("image_path = %q, want /tmp/host-window-9.png", got.ImagePath)
	}
	if got.WindowID != "win-1" {
		t.Fatalf("window_id = %q, want original win-1", got.WindowID)
	}
	if got.Tree != base.Tree {
		t.Fatalf("tree = %q, want %q", got.Tree, base.Tree)
	}
}

func TestAttachSnapshotImage_PreservesSnapshotWhenScreenshotFails(t *testing.T) {
	base := SnapshotResult{
		HostOS:   "darwin",
		WindowID: "win-1",
		Title:    "Feishu",
		Tree:     "@1 [button] \"Open\"",
		RefMap:   map[int]string{1: "token-open"},
		Message:  "Host accessibility snapshot ready",
	}
	calls := 0

	got := attachSnapshotImage(context.Background(), base, func(context.Context, string) (ScreenshotResult, error) {
		calls++
		return ScreenshotResult{}, errors.New("capture failed")
	})

	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	if got.ImagePath != "" {
		t.Fatalf("image_path = %q, want empty on capture failure", got.ImagePath)
	}
	if got.WindowID != base.WindowID {
		t.Fatalf("window_id = %q, want %q", got.WindowID, base.WindowID)
	}
	if got.Tree != base.Tree {
		t.Fatalf("tree = %q, want %q", got.Tree, base.Tree)
	}
}

func TestAttachSnapshotImage_SkipsCaptureWhenWindowIDIsEmpty(t *testing.T) {
	base := SnapshotResult{HostOS: "darwin", Tree: "@1 [button] \"Open\""}
	calls := 0

	got := attachSnapshotImage(context.Background(), base, func(context.Context, string) (ScreenshotResult, error) {
		calls++
		return ScreenshotResult{ImagePath: "/tmp/host-window.png"}, nil
	})

	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
	if got.ImagePath != "" {
		t.Fatalf("image_path = %q, want empty when no window_id is available", got.ImagePath)
	}
}
