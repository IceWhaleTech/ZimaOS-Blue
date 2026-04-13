package a11y

import (
	"context"
	"errors"
	"testing"
)

func TestEnrichSnapshotErrorWithImage_PreservesRuntimeErrorCodeAndAddsImage(t *testing.T) {
	err := NewError("permission_required", "grant permission", map[string]interface{}{
		"permissions": []PermissionStatus{{Name: "accessibility", Granted: false, Required: true}},
	})

	enriched := enrichSnapshotErrorWithImage(context.Background(), err, "win-1", func(_ context.Context, windowID string) (ScreenshotResult, error) {
		if windowID != "win-1" {
			t.Fatalf("windowID = %q, want win-1", windowID)
		}
		return ScreenshotResult{ImagePath: "/tmp/host-window-1.png"}, nil
	})

	runtimeErr, ok := enriched.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", enriched)
	}
	if runtimeErr.Code != "permission_required" {
		t.Fatalf("code = %q, want permission_required", runtimeErr.Code)
	}
	if got := runtimeErr.Details["window_id"]; got != "win-1" {
		t.Fatalf("window_id = %v, want win-1", got)
	}
	if got := runtimeErr.Details["image_path"]; got != "/tmp/host-window-1.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-1.png", got)
	}
}

func TestEnrichSnapshotErrorWithImage_WrapsPlainErrorsAsBackendUnavailable(t *testing.T) {
	enriched := enrichSnapshotErrorWithImage(context.Background(), errors.New("msaa failed"), "win-9", func(context.Context, string) (ScreenshotResult, error) {
		return ScreenshotResult{ImagePath: "/tmp/host-window-9.png"}, nil
	})

	runtimeErr, ok := enriched.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", enriched)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if runtimeErr.Message != "msaa failed" {
		t.Fatalf("message = %q, want msaa failed", runtimeErr.Message)
	}
	if got := runtimeErr.Details["image_path"]; got != "/tmp/host-window-9.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-9.png", got)
	}
}

func TestEnrichSnapshotErrorWithImage_PreservesErrorWhenCaptureFails(t *testing.T) {
	err := NewError("backend_unavailable", "snapshot empty", map[string]interface{}{"window_id": "win-3"})

	enriched := enrichSnapshotErrorWithImage(context.Background(), err, "win-3", func(context.Context, string) (ScreenshotResult, error) {
		return ScreenshotResult{}, errors.New("capture failed")
	})

	runtimeErr, ok := enriched.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", enriched)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if _, ok := runtimeErr.Details["image_path"]; ok {
		t.Fatalf("image_path should be omitted on capture failure: %+v", runtimeErr.Details)
	}
}
