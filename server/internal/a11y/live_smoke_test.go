//go:build darwin

package a11y

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveLarkSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("BLUE_A11Y_LIVE")) != "1" {
		t.Skip("set BLUE_A11Y_LIVE=1 to run live a11y smoke tests")
	}

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media"))
	if backend == nil {
		t.Fatal("expected darwin a11y backend")
	}

	ctx := context.Background()
	caps, err := backend.Capabilities(ctx)
	if err != nil {
		t.Fatalf("Capabilities() error: %v", err)
	}
	t.Logf("capabilities: host=%s permissions=%+v", caps.HostOS, caps.Permissions)

	windows, err := backend.ListWindows(ctx)
	if err != nil {
		t.Fatalf("ListWindows() error: %v", err)
	}
	target, ok := liveFindWindow(windows, "lark", "feishu", "飞书")
	if !ok {
		if err := liveLaunchLark(ctx); err != nil {
			t.Logf("launch Lark/Feishu hint failed: %v", err)
		}
		windows, err = liveWaitForWindow(ctx, backend, 6*time.Second, "lark", "feishu", "飞书")
		if err != nil {
			t.Fatalf("Lark/Feishu window not found after launch retry: %v (windows=%+v)", err, windows)
		}
		target, ok = liveFindWindow(windows, "lark", "feishu", "飞书")
		if !ok {
			t.Fatalf("Lark/Feishu window not found in %+v", windows)
		}
	}
	t.Logf("target window: id=%s app=%s title=%s focused=%v", target.ID, target.AppName, target.Title, target.Focused)

	focusResult, err := backend.FocusWindow(ctx, target.ID)
	if err != nil {
		t.Fatalf("FocusWindow() error: %v", err)
	}
	t.Logf("focus result: %+v", focusResult)

	screenshot, err := backend.Screenshot(ctx, target.ID)
	if err != nil {
		t.Fatalf("Screenshot() error: %v", err)
	}
	if strings.TrimSpace(screenshot.ImagePath) == "" {
		t.Fatalf("Screenshot() returned empty path: %+v", screenshot)
	}
	t.Logf("screenshot path: %s", screenshot.ImagePath)

	if !livePermissionGranted(caps.Permissions, "accessibility") {
		prevPrompt := darwinAccessibilityPromptProbe
		darwinResetAccessibilityPromptState()
		darwinAccessibilityPromptProbe = func() bool { return false }
		defer func() {
			darwinAccessibilityPromptProbe = prevPrompt
			darwinResetAccessibilityPromptState()
		}()

		_, err := backend.Snapshot(ctx, target.ID)
		if err == nil {
			t.Fatal("Snapshot() error = nil, want permission_required without Accessibility")
		}
		runtimeErr, ok := err.(*RuntimeError)
		if !ok {
			t.Fatalf("Snapshot() error type = %T, want *RuntimeError", err)
		}
		if runtimeErr.Code != "permission_required" {
			t.Fatalf("Snapshot() error_code = %q, want permission_required", runtimeErr.Code)
		}
		if imagePath, _ := runtimeErr.Details["image_path"].(string); strings.TrimSpace(imagePath) == "" {
			t.Fatalf("Snapshot() permission error missing image_path: %+v", runtimeErr.Details)
		}
		t.Logf("snapshot fallback image path: %v", runtimeErr.Details["image_path"])
		t.Log("Accessibility permission is not granted; verified windows, focus, screenshot, and snapshot fallback image only")
		return
	}

	snapshot, err := backend.Snapshot(ctx, target.ID)
	if err != nil {
		t.Fatalf("Snapshot() error: %v", err)
	}
	if strings.TrimSpace(snapshot.Tree) == "" {
		t.Fatalf("Snapshot() returned empty tree: %+v", snapshot)
	}
	t.Logf("snapshot title=%q refs=%d", snapshot.Title, len(snapshot.RefMap))
	if strings.TrimSpace(snapshot.ImagePath) != "" {
		t.Logf("snapshot image path (optional on success): %s", snapshot.ImagePath)
	}
	t.Logf("snapshot preview:\n%s", liveTreePreview(snapshot.Tree, 24))
}

func livePermissionGranted(permissions []PermissionStatus, name string) bool {
	for _, permission := range permissions {
		if strings.EqualFold(strings.TrimSpace(permission.Name), strings.TrimSpace(name)) {
			return permission.Granted
		}
	}
	return false
}

func liveFindWindow(windows []WindowInfo, needles ...string) (WindowInfo, bool) {
	for _, window := range windows {
		if liveMatchesWindow(window, needles...) {
			return window, true
		}
	}
	return WindowInfo{}, false
}

func liveWaitForWindow(ctx context.Context, backend Backend, timeout time.Duration, needles ...string) ([]WindowInfo, error) {
	deadline := time.Now().Add(timeout)
	var last []WindowInfo
	for {
		windows, err := backend.ListWindows(ctx)
		if err == nil {
			last = windows
			if _, ok := liveFindWindow(windows, needles...); ok {
				return windows, nil
			}
		}
		if time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return last, ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
	return last, context.DeadlineExceeded
}

func liveLaunchLark(ctx context.Context) error {
	candidates := []string{"/Applications/Lark.app", "Lark", "Feishu", "飞书"}
	var lastErr error
	for _, candidate := range candidates {
		args := []string{"-a", candidate}
		if strings.HasSuffix(candidate, ".app") {
			args = []string{candidate}
		}
		cmd := exec.CommandContext(ctx, "open", args...)
		if output, err := cmd.CombinedOutput(); err == nil {
			return nil
		} else {
			lastErr = err
			if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
				lastErr = execErrorWithOutput(err, trimmed)
			}
		}
	}
	return lastErr
}

func execErrorWithOutput(err error, output string) error {
	if err == nil {
		return nil
	}
	return &liveCommandError{cause: err, output: output}
}

type liveCommandError struct {
	cause  error
	output string
}

func (e *liveCommandError) Error() string {
	if e == nil {
		return ""
	}
	if e.output == "" {
		return e.cause.Error()
	}
	return e.output + ": " + e.cause.Error()
}

func liveMatchesWindow(window WindowInfo, needles ...string) bool {
	fields := []string{strings.ToLower(window.AppName), strings.ToLower(window.Title)}
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}
		for _, field := range fields {
			if strings.Contains(field, needle) {
				return true
			}
		}
	}
	return false
}

func liveTreePreview(tree string, maxLines int) string {
	lines := strings.Split(strings.TrimSpace(tree), "\n")
	if len(lines) <= maxLines {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[:maxLines], "\n")
}
