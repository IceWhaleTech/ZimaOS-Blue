//go:build darwin

package a11y

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDarwinCapabilities_DoesNotRequestPromptWhenDenied(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	promptCalls := 0
	darwinAccessibilityPromptProbe = func() bool {
		promptCalls++
		return true
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}
	if result.Message == "" {
		t.Fatal("expected capabilities guidance when accessibility is denied")
	}
	if promptCalls != 0 {
		t.Fatalf("prompt calls = %d, want 0", promptCalls)
	}
}

func TestDarwinCapabilities_IncludesPermissionGuidanceWhenDenied(t *testing.T) {
	prev := darwinAccessibilityGrantedProbe
	darwinAccessibilityGrantedProbe = func() bool { return false }
	defer func() { darwinAccessibilityGrantedProbe = prev }()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.Capabilities(context.Background())
	if err != nil {
		t.Fatalf("Capabilities() error = %v", err)
	}
	if result.Message == "" {
		t.Fatal("expected capabilities message when accessibility is denied")
	}
	if len(result.Permissions) != 1 {
		t.Fatalf("permissions len = %d, want 1", len(result.Permissions))
	}
	if result.Permissions[0].Granted {
		t.Fatal("expected denied accessibility permission")
	}
	if result.Permissions[0].Message == "" {
		t.Fatal("expected permission guidance message")
	}
}

func TestEnsureAccessibilityPermission_OpensSettingsWhenDenied(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	prevDispatch := darwinAccessibilityPromptDispatch
	prevOpenSettings := darwinOpenAccessibilitySettingsFunc
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	promptCalls := 0
	dispatchCalls := 0
	openSettingsCalls := 0
	darwinAccessibilityPromptProbe = func() bool {
		promptCalls++
		return true
	}
	darwinAccessibilityPromptDispatch = func(fn func() bool) bool {
		dispatchCalls++
		if fn != nil {
			fn()
		}
		return true
	}
	darwinOpenAccessibilitySettingsFunc = func() error {
		openSettingsCalls++
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinAccessibilityPromptDispatch = prevDispatch
		darwinOpenAccessibilitySettingsFunc = prevOpenSettings
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	err := backend.ensureAccessibilityPermission()
	if err == nil {
		t.Fatal("expected permission error")
	}
	if promptCalls != 0 {
		t.Fatalf("prompt calls = %d, want 0", promptCalls)
	}
	if dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", dispatchCalls)
	}
	if openSettingsCalls != 1 {
		t.Fatalf("open settings calls = %d, want 1", openSettingsCalls)
	}
}

func TestDarwinActivationWaitBudget_AllowsSlowWindowFocus(t *testing.T) {
	if darwinActivationWaitTimeout < 30*time.Second {
		t.Fatalf("darwinActivationWaitTimeout = %v, want at least 30s", darwinActivationWaitTimeout)
	}
	if darwinActivationWaitTimeout > 60*time.Second {
		t.Fatalf("darwinActivationWaitTimeout = %v, want at most 60s", darwinActivationWaitTimeout)
	}
	if darwinActivationPollInterval < time.Second {
		t.Fatalf("darwinActivationPollInterval = %v, want at least 1s to avoid millisecond polling", darwinActivationPollInterval)
	}
	if darwinActivationPollInterval > 5*time.Second {
		t.Fatalf("darwinActivationPollInterval = %v, want bounded second-level polling", darwinActivationPollInterval)
	}
}

func TestEnsureAccessibilityPermission_ThrottlesRepeatedSettingsOpens(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	prevDispatch := darwinAccessibilityPromptDispatch
	prevOpenSettings := darwinOpenAccessibilitySettingsFunc
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	promptCalls := 0
	dispatchCalls := 0
	openSettingsCalls := 0
	darwinAccessibilityPromptProbe = func() bool {
		promptCalls++
		return true
	}
	darwinAccessibilityPromptDispatch = func(fn func() bool) bool {
		dispatchCalls++
		if fn != nil {
			fn()
		}
		return true
	}
	darwinOpenAccessibilitySettingsFunc = func() error {
		openSettingsCalls++
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinAccessibilityPromptDispatch = prevDispatch
		darwinOpenAccessibilitySettingsFunc = prevOpenSettings
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	if err := backend.ensureAccessibilityPermission(); err == nil {
		t.Fatal("first ensureAccessibilityPermission() error = nil, want permission error")
	}
	if err := backend.ensureAccessibilityPermission(); err == nil {
		t.Fatal("second ensureAccessibilityPermission() error = nil, want permission error")
	}
	if promptCalls != 0 {
		t.Fatalf("prompt calls = %d, want 0", promptCalls)
	}
	if dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", dispatchCalls)
	}
	if openSettingsCalls != 1 {
		t.Fatalf("open settings calls = %d, want 1", openSettingsCalls)
	}
}

func TestEnsureAccessibilityPermission_StillReturnsErrorWhenSettingsOpenUnavailable(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	prevDispatch := darwinAccessibilityPromptDispatch
	prevOpenSettings := darwinOpenAccessibilitySettingsFunc
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	promptCalls := 0
	dispatchCalls := 0
	openSettingsCalls := 0
	darwinAccessibilityPromptProbe = func() bool {
		promptCalls++
		return true
	}
	darwinAccessibilityPromptDispatch = func(fn func() bool) bool {
		dispatchCalls++
		if fn != nil {
			fn()
		}
		return true
	}
	darwinOpenAccessibilitySettingsFunc = func() error {
		openSettingsCalls++
		return errors.New("open failed")
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinAccessibilityPromptDispatch = prevDispatch
		darwinOpenAccessibilitySettingsFunc = prevOpenSettings
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	err := backend.ensureAccessibilityPermission()
	if err == nil {
		t.Fatal("expected permission error")
	}
	if promptCalls != 0 {
		t.Fatalf("prompt calls = %d, want 0", promptCalls)
	}
	if dispatchCalls != 0 {
		t.Fatalf("dispatch calls = %d, want 0", dispatchCalls)
	}
	if openSettingsCalls != 1 {
		t.Fatalf("open settings calls = %d, want 1", openSettingsCalls)
	}
}

func TestFocusWindowRecord_ActivatesAppWithoutAccessibilityPermission(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevActivate := darwinActivateAppFunc
	prevWait := darwinWaitForActivatedWindowRecord
	darwinAccessibilityGrantedProbe = func() bool { return false }
	activatedApp := ""
	darwinActivateAppFunc = func(appName string) error {
		activatedApp = appName
		return nil
	}
	darwinWaitForActivatedWindowRecord = func(_ context.Context, _ *darwinBackend, target darwinWindowRecord) (darwinWindowRecord, bool) {
		return target, false
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinActivateAppFunc = prevActivate
		darwinWaitForActivatedWindowRecord = prevWait
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.focusWindowRecord(context.Background(), darwinWindowRecord{
		ID:      "42",
		Title:   "Feishu",
		AppName: "Lark",
		PID:     100,
	})
	if err != nil {
		t.Fatalf("focusWindowRecord() error = %v", err)
	}
	if activatedApp != "Lark" {
		t.Fatalf("activated app = %q, want Lark", activatedApp)
	}
	if result.WindowID != "42" {
		t.Fatalf("window_id = %q, want 42", result.WindowID)
	}
	if result.ExecutionMode != "automation" {
		t.Fatalf("execution_mode = %q, want automation", result.ExecutionMode)
	}
}

func TestFocusWindowRecord_UsesRefreshedWindowAfterActivation(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevActivate := darwinActivateAppFunc
	prevWait := darwinWaitForActivatedWindowRecord
	darwinAccessibilityGrantedProbe = func() bool { return false }
	darwinActivateAppFunc = func(string) error { return nil }
	waitCalls := 0
	darwinWaitForActivatedWindowRecord = func(_ context.Context, _ *darwinBackend, target darwinWindowRecord) (darwinWindowRecord, bool) {
		waitCalls++
		return darwinWindowRecord{
			ID:      "7001",
			Title:   target.Title,
			AppName: target.AppName,
			PID:     target.PID,
			Focused: true,
		}, true
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinActivateAppFunc = prevActivate
		darwinWaitForActivatedWindowRecord = prevWait
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.focusWindowRecord(context.Background(), darwinWindowRecord{
		ID:      "42",
		Title:   "Feishu",
		AppName: "Lark",
		PID:     100,
	})
	if err != nil {
		t.Fatalf("focusWindowRecord() error = %v", err)
	}
	if waitCalls != 1 {
		t.Fatalf("waitCalls = %d, want 1", waitCalls)
	}
	if result.WindowID != "7001" {
		t.Fatalf("window_id = %q, want 7001", result.WindowID)
	}
	if result.ExecutionMode != "automation" {
		t.Fatalf("execution_mode = %q, want automation", result.ExecutionMode)
	}
}

func TestFocusWindowRecord_ReportsFocusedWindowWithoutAccessibilityWhenSettled(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevActivate := darwinActivateAppFunc
	prevWait := darwinWaitForActivatedWindowRecord
	darwinAccessibilityGrantedProbe = func() bool { return false }
	darwinActivateAppFunc = func(string) error { return nil }
	darwinWaitForActivatedWindowRecord = func(_ context.Context, _ *darwinBackend, target darwinWindowRecord) (darwinWindowRecord, bool) {
		target.Focused = true
		return target, true
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinActivateAppFunc = prevActivate
		darwinWaitForActivatedWindowRecord = prevWait
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.focusWindowRecord(context.Background(), darwinWindowRecord{
		ID:      "42",
		Title:   "Feishu",
		AppName: "Lark",
		PID:     100,
	})
	if err != nil {
		t.Fatalf("focusWindowRecord() error = %v", err)
	}
	if result.ExecutionMode != "automation" {
		t.Fatalf("execution_mode = %q, want automation", result.ExecutionMode)
	}
	if result.Message != "Window focused" {
		t.Fatalf("message = %q, want Window focused", result.Message)
	}
}

func TestDarwinResolveWindowForAction_UsesFocusResolvedWindowID(t *testing.T) {
	prevFocus := darwinFocusWindowForHostAction
	darwinFocusWindowForHostAction = func(_ context.Context, _ *darwinBackend, windowID string) (ActionResult, error) {
		if windowID != "win-1" {
			t.Fatalf("focus windowID = %q, want win-1", windowID)
		}
		return ActionResult{WindowID: "win-9", ExecutionMode: "automation", Message: "Window focused"}, nil
	}
	defer func() { darwinFocusWindowForHostAction = prevFocus }()

	backend := DefaultHostBackend("").(*darwinBackend)
	resolved, err := backend.resolveWindowForAction(context.Background(), "win-1")
	if err != nil {
		t.Fatalf("resolveWindowForAction() error = %v", err)
	}
	if resolved != "win-9" {
		t.Fatalf("resolved = %q, want win-9", resolved)
	}
}

func TestDarwinResolveWindowForAction_LeavesEmptyWindowUntouched(t *testing.T) {
	prevFocus := darwinFocusWindowForHostAction
	focusCalls := 0
	darwinFocusWindowForHostAction = func(context.Context, *darwinBackend, string) (ActionResult, error) {
		focusCalls++
		return ActionResult{}, nil
	}
	defer func() { darwinFocusWindowForHostAction = prevFocus }()

	backend := DefaultHostBackend("").(*darwinBackend)
	resolved, err := backend.resolveWindowForAction(context.Background(), "")
	if err != nil {
		t.Fatalf("resolveWindowForAction() error = %v", err)
	}
	if resolved != "" {
		t.Fatalf("resolved = %q, want empty", resolved)
	}
	if focusCalls != 0 {
		t.Fatalf("focusCalls = %d, want 0", focusCalls)
	}
}

func TestDarwinScreenshot_RetriesWithRefreshedWindowIDAfterFailure(t *testing.T) {
	prevResolve := darwinResolveWindowRecordForCapture
	prevRefresh := darwinRefreshWindowRecordForCapture
	prevCapture := darwinCaptureWindowImage
	prevSleep := darwinScreenshotRetrySleep
	attempts := 0
	var capturedIDs []string
	darwinResolveWindowRecordForCapture = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinRefreshWindowRecordForCapture = func(_ *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		if current.ID == "6263" {
			return darwinWindowRecord{ID: "7001", AppName: "Feishu", Title: "Feishu", PID: 100}, nil
		}
		return current, nil
	}
	darwinCaptureWindowImage = func(_ context.Context, windowID string, path string) (string, error) {
		attempts++
		capturedIDs = append(capturedIDs, windowID)
		if attempts == 1 {
			return "could not create image from window", errors.New("exit status 1")
		}
		if err := os.WriteFile(path, []byte("png"), 0o600); err != nil {
			t.Fatalf("write capture file: %v", err)
		}
		return "", nil
	}
	darwinScreenshotRetrySleep = func(context.Context, time.Duration) error { return nil }
	defer func() {
		darwinResolveWindowRecordForCapture = prevResolve
		darwinRefreshWindowRecordForCapture = prevRefresh
		darwinCaptureWindowImage = prevCapture
		darwinScreenshotRetrySleep = prevSleep
	}()

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media")).(*darwinBackend)
	result, err := backend.screenshot(context.Background(), "6263")
	if err != nil {
		t.Fatalf("screenshot() error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("attempts = %d, want 2", attempts)
	}
	if strings.Join(capturedIDs, ",") != "6263,7001" {
		t.Fatalf("captured IDs = %v, want [6263 7001]", capturedIDs)
	}
	if result.WindowID != "7001" {
		t.Fatalf("window_id = %q, want 7001", result.WindowID)
	}
	if !strings.HasSuffix(result.ImagePath, "host-window-7001.png") {
		t.Fatalf("image_path = %q, want suffix host-window-7001.png", result.ImagePath)
	}
}

func TestDarwinScreenshot_ReturnsLastCaptureErrorAfterRetries(t *testing.T) {
	prevResolve := darwinResolveWindowRecordForCapture
	prevRefresh := darwinRefreshWindowRecordForCapture
	prevCapture := darwinCaptureWindowImage
	prevSleep := darwinScreenshotRetrySleep
	attempts := 0
	darwinResolveWindowRecordForCapture = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinRefreshWindowRecordForCapture = func(_ *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		return current, nil
	}
	darwinCaptureWindowImage = func(_ context.Context, _ string, _ string) (string, error) {
		attempts++
		return "could not create image from window", errors.New("exit status 1")
	}
	darwinScreenshotRetrySleep = func(context.Context, time.Duration) error { return nil }
	defer func() {
		darwinResolveWindowRecordForCapture = prevResolve
		darwinRefreshWindowRecordForCapture = prevRefresh
		darwinCaptureWindowImage = prevCapture
		darwinScreenshotRetrySleep = prevSleep
	}()

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media")).(*darwinBackend)
	_, err := backend.screenshot(context.Background(), "6263")
	if err == nil {
		t.Fatal("screenshot() error = nil, want failure")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if !strings.Contains(err.Error(), "could not create image from window") {
		t.Fatalf("error = %v, want capture output", err)
	}
}

func TestDarwinScreenshotForGrounding_ReturnsInMemoryPNGBytes(t *testing.T) {
	prevResolve := darwinResolveWindowRecordForCapture
	prevCaptureBytes := darwinCaptureWindowPNGBytes
	captureCalls := 0
	darwinResolveWindowRecordForCapture = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinCaptureWindowPNGBytes = func(_ context.Context, record darwinWindowRecord) ([]byte, error) {
		captureCalls++
		if record.ID != "6263" {
			t.Fatalf("record.ID = %q, want 6263", record.ID)
		}
		return []byte("png-bytes"), nil
	}
	defer func() {
		darwinResolveWindowRecordForCapture = prevResolve
		darwinCaptureWindowPNGBytes = prevCaptureBytes
	}()

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media")).(*darwinBackend)
	result, err := backend.ScreenshotForGrounding(context.Background(), "6263")
	if err != nil {
		t.Fatalf("ScreenshotForGrounding() error = %v", err)
	}
	if captureCalls != 1 {
		t.Fatalf("captureCalls = %d, want 1", captureCalls)
	}
	if result.WindowID != "6263" {
		t.Fatalf("window_id = %q, want 6263", result.WindowID)
	}
	if got := string(result.ImageBytes); got != "png-bytes" {
		t.Fatalf("image bytes = %q, want png-bytes", got)
	}
	if result.ImagePath != "" {
		t.Fatalf("image_path = %q, want empty for grounding capture", result.ImagePath)
	}
}

func TestDarwinSnapshot_PermissionDeniedIncludesNativeScreenshotHint(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	prevResolve := darwinResolveWindowRecordForSnapshot
	prevSnapshotCapture := darwinCaptureSnapshotScreenshot
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	darwinAccessibilityPromptProbe = func() bool { return false }
	darwinResolveWindowRecordForSnapshot = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	captureCalls := 0
	darwinCaptureSnapshotScreenshot = func(_ context.Context, _ *darwinBackend, record darwinWindowRecord) (ScreenshotResult, error) {
		captureCalls++
		if record.ID != "6263" {
			t.Fatalf("record.ID = %q, want 6263", record.ID)
		}
		return ScreenshotResult{
			HostOS:    "darwin",
			WindowID:  record.ID,
			ImagePath: "/tmp/host-window-6263.png",
		}, nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinResolveWindowRecordForSnapshot = prevResolve
		darwinCaptureSnapshotScreenshot = prevSnapshotCapture
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend(filepath.Join(t.TempDir(), "media")).(*darwinBackend)
	_, err := backend.snapshot(context.Background(), "6263", false)
	if err == nil {
		t.Fatal("snapshot() error = nil, want permission_required")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "permission_required" {
		t.Fatalf("code = %q, want permission_required", runtimeErr.Code)
	}
	if captureCalls != 1 {
		t.Fatalf("captureCalls = %d, want 1", captureCalls)
	}
	if got := runtimeErr.Details["window_id"]; got != "6263" {
		t.Fatalf("window_id = %v, want 6263", got)
	}
	if got := runtimeErr.Details["image_path"]; got != "/tmp/host-window-6263.png" {
		t.Fatalf("image_path = %v, want /tmp/host-window-6263.png", got)
	}
}

func TestEnsureAccessibilityPermission_ReturnsGuidedRuntimeError(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevPrompt := darwinAccessibilityPromptProbe
	prevDispatch := darwinAccessibilityPromptDispatch
	prevOpenSettings := darwinOpenAccessibilitySettingsFunc
	darwinResetAccessibilityPromptState()
	darwinAccessibilityGrantedProbe = func() bool { return false }
	darwinAccessibilityPromptProbe = func() bool { return true }
	darwinAccessibilityPromptDispatch = func(fn func() bool) bool {
		fn()
		return true
	}
	darwinOpenAccessibilitySettingsFunc = func() error { return nil }
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinAccessibilityPromptProbe = prevPrompt
		darwinAccessibilityPromptDispatch = prevDispatch
		darwinOpenAccessibilitySettingsFunc = prevOpenSettings
		darwinResetAccessibilityPromptState()
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	err := backend.ensureAccessibilityPermission()
	if err == nil {
		t.Fatal("expected permission error")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("err type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "permission_required" {
		t.Fatalf("code = %q, want permission_required", runtimeErr.Code)
	}
	if runtimeErr.Message == "" {
		t.Fatal("expected guidance message")
	}
	if _, ok := runtimeErr.Details["permissions"]; !ok {
		t.Fatal("expected permissions detail")
	}
}

func TestDarwinListWindowRecords_ReturnsBackendUnavailableWhenArrayBindingsMissing(t *testing.T) {
	initDarwinRuntime()
	prevCopyInfo := darwinCGWindowListCopyInfo
	prevGetCount := darwinCFArrayGetCount
	prevGetValue := darwinCFArrayGetValueAtIndex
	prevRelease := darwinCFRelease
	darwinCGWindowListCopyInfo = func(uint32, uint32) uintptr { return 1 }
	darwinCFArrayGetCount = nil
	darwinCFArrayGetValueAtIndex = nil
	darwinCFRelease = func(uintptr) {}
	defer func() {
		darwinCGWindowListCopyInfo = prevCopyInfo
		darwinCFArrayGetCount = prevGetCount
		darwinCFArrayGetValueAtIndex = prevGetValue
		darwinCFRelease = prevRelease
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	var (
		records []darwinWindowRecord
		err     error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("listWindowRecords() panicked: %v", r)
			}
		}()
		records, err = backend.listWindowRecords()
	}()
	if err == nil {
		t.Fatal("listWindowRecords() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
	if len(records) != 0 {
		t.Fatalf("records len = %d, want 0", len(records))
	}
}

func TestDarwinFindWindowElement_ReturnsZeroWhenArrayBindingsMissing(t *testing.T) {
	initDarwinRuntime()
	prevCopyAttr := darwinAXUIElementCopyAttributeValue
	prevStringCreate := darwinCFStringCreate
	prevGetCount := darwinCFArrayGetCount
	prevGetValue := darwinCFArrayGetValueAtIndex
	prevRelease := darwinCFRelease
	call := 0
	darwinAXUIElementCopyAttributeValue = func(_ uintptr, _ uintptr, _ *uintptr) int32 {
		call++
		return 1
	}
	darwinCFStringCreate = func(uintptr, *byte, uint32) uintptr { return 9 }
	darwinCFArrayGetCount = nil
	darwinCFArrayGetValueAtIndex = nil
	darwinCFRelease = func(uintptr) {}
	defer func() {
		darwinAXUIElementCopyAttributeValue = prevCopyAttr
		darwinCFStringCreate = prevStringCreate
		darwinCFArrayGetCount = prevGetCount
		darwinCFArrayGetValueAtIndex = prevGetValue
		darwinCFRelease = prevRelease
	}()

	var got uintptr
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("darwinFindWindowElement() panicked: %v", r)
			}
		}()
		got = darwinFindWindowElement(1, darwinWindowRecord{ID: "win-1"})
	}()
	if got != 0 {
		t.Fatalf("darwinFindWindowElement() = %d, want 0", got)
	}
}

func TestDarwinCopyActionNamesForElement_ReturnsNilWhenArrayBindingsMissing(t *testing.T) {
	initDarwinRuntime()
	prevCopyActions := darwinAXUIElementCopyActionNames
	prevGetCount := darwinCFArrayGetCount
	prevGetValue := darwinCFArrayGetValueAtIndex
	prevRelease := darwinCFRelease
	darwinAXUIElementCopyActionNames = func(_ uintptr, out *uintptr) int32 {
		if out != nil {
			*out = 2
		}
		return darwinAXErrorSuccess
	}
	darwinCFArrayGetCount = nil
	darwinCFArrayGetValueAtIndex = nil
	darwinCFRelease = func(uintptr) {}
	defer func() {
		darwinAXUIElementCopyActionNames = prevCopyActions
		darwinCFArrayGetCount = prevGetCount
		darwinCFArrayGetValueAtIndex = prevGetValue
		darwinCFRelease = prevRelease
	}()

	var names []string
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("darwinCopyActionNamesForElement() panicked: %v", r)
			}
		}()
		names = darwinCopyActionNamesForElement(1)
	}()
	if len(names) != 0 {
		t.Fatalf("names = %v, want nil/empty", names)
	}
}

func TestDarwinBuildSnapshotNode_ToleratesMissingArrayValueGetter(t *testing.T) {
	initDarwinRuntime()
	prevCopyAttr := darwinAXUIElementCopyAttributeValue
	prevCopyActions := darwinAXUIElementCopyActionNames
	prevAttrSettable := darwinAXUIElementIsAttributeSettable
	prevStringCreate := darwinCFStringCreate
	prevGetCount := darwinCFArrayGetCount
	prevGetValue := darwinCFArrayGetValueAtIndex
	prevRelease := darwinCFRelease
	call := 0
	darwinAXUIElementCopyAttributeValue = func(_ uintptr, _ uintptr, out *uintptr) int32 {
		call++
		if call == 8 && out != nil {
			*out = 2
			return darwinAXErrorSuccess
		}
		if out != nil {
			*out = 0
		}
		return 1
	}
	darwinCFStringCreate = func(uintptr, *byte, uint32) uintptr { return 9 }
	darwinAXUIElementCopyActionNames = nil
	darwinAXUIElementIsAttributeSettable = nil
	darwinCFArrayGetCount = func(uintptr) int64 { return 1 }
	darwinCFArrayGetValueAtIndex = nil
	darwinCFRelease = func(uintptr) {}
	defer func() {
		darwinAXUIElementCopyAttributeValue = prevCopyAttr
		darwinAXUIElementCopyActionNames = prevCopyActions
		darwinAXUIElementIsAttributeSettable = prevAttrSettable
		darwinCFStringCreate = prevStringCreate
		darwinCFArrayGetCount = prevGetCount
		darwinCFArrayGetValueAtIndex = prevGetValue
		darwinCFRelease = prevRelease
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	visited := 0
	var node *Node
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("buildSnapshotNode() panicked: %v", r)
			}
		}()
		node = backend.buildSnapshotNode(1, 0, &visited)
	}()
	if node == nil {
		t.Fatal("buildSnapshotNode() = nil, want node")
	}
}

func TestDarwinDictionaryValue_ToleratesMissingCFRelease(t *testing.T) {
	initDarwinRuntime()
	prevDictGet := darwinCFDictionaryGetValue
	prevStringCreate := darwinCFStringCreate
	prevRelease := darwinCFRelease
	darwinCFDictionaryGetValue = func(uintptr, uintptr) uintptr { return 5 }
	darwinCFStringCreate = func(uintptr, *byte, uint32) uintptr { return 7 }
	darwinCFRelease = nil
	defer func() {
		darwinCFDictionaryGetValue = prevDictGet
		darwinCFStringCreate = prevStringCreate
		darwinCFRelease = prevRelease
	}()

	var got uintptr
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("darwinDictionaryValue() panicked: %v", r)
			}
		}()
		got = darwinDictionaryValue(1, "kCGWindowNumber")
	}()
	if got != 5 {
		t.Fatalf("darwinDictionaryValue() = %d, want 5", got)
	}
}

func TestFocusWindowRecord_ToleratesMissingCreateApplicationBinding(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevActivate := darwinActivateAppFunc
	prevWait := darwinWaitForActivatedWindowRecord
	prevCreateApp := darwinAXUIElementCreateApplication
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinActivateAppFunc = func(string) error { return nil }
	darwinWaitForActivatedWindowRecord = func(_ context.Context, _ *darwinBackend, target darwinWindowRecord) (darwinWindowRecord, bool) {
		return target, false
	}
	darwinAXUIElementCreateApplication = nil
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinActivateAppFunc = prevActivate
		darwinWaitForActivatedWindowRecord = prevWait
		darwinAXUIElementCreateApplication = prevCreateApp
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	var (
		result ActionResult
		err    error
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("focusWindowRecord() panicked: %v", r)
			}
		}()
		result, err = backend.focusWindowRecord(context.Background(), darwinWindowRecord{
			ID:      "42",
			Title:   "Feishu",
			AppName: "Lark",
			PID:     100,
		})
	}()
	if err != nil {
		t.Fatalf("focusWindowRecord() error = %v", err)
	}
	if result.ExecutionMode != "automation" {
		t.Fatalf("execution_mode = %q, want automation", result.ExecutionMode)
	}
	if result.Message != "Application activated" {
		t.Fatalf("message = %q, want Application activated", result.Message)
	}
}

func TestDarwinSnapshot_ReturnsBackendUnavailableWhenCreateApplicationBindingMissing(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevResolve := darwinResolveWindowRecordForSnapshot
	prevCreateApp := darwinAXUIElementCreateApplication
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinResolveWindowRecordForSnapshot = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinAXUIElementCreateApplication = nil
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinResolveWindowRecordForSnapshot = prevResolve
		darwinAXUIElementCreateApplication = prevCreateApp
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("snapshot() panicked: %v", r)
			}
		}()
		_, err = backend.snapshot(context.Background(), "6263", false)
	}()
	if err == nil {
		t.Fatal("snapshot() error = nil, want backend_unavailable")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error type = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "backend_unavailable" {
		t.Fatalf("code = %q, want backend_unavailable", runtimeErr.Code)
	}
}

func TestDarwinResolveWindowRecordFromLists_FallsBackToAllWindowsByID(t *testing.T) {
	record, err := darwinResolveWindowRecordFromLists("win-feishu", []darwinWindowRecord{
		{ID: "win-code", Title: "Code", AppName: "Code", Focused: true},
	}, []darwinWindowRecord{
		{ID: "win-feishu", Title: "飞书", AppName: "飞书"},
	})
	if err != nil {
		t.Fatalf("darwinResolveWindowRecordFromLists() error = %v", err)
	}
	if record.ID != "win-feishu" {
		t.Fatalf("record.ID = %q, want win-feishu", record.ID)
	}
}

func TestDarwinRefreshWindowRecordFromLists_PrefersExactAllWindowMatchBeforeSimilarityFallback(t *testing.T) {
	current := darwinWindowRecord{ID: "win-feishu", Title: "飞书", AppName: "飞书", PID: 100}
	refreshed := darwinRefreshWindowRecordFromLists(current, []darwinWindowRecord{
		{ID: "win-code", Title: "Code", AppName: "Code", PID: 200, Focused: true},
	}, []darwinWindowRecord{
		{ID: "win-feishu", Title: "飞书", AppName: "飞书", PID: 100},
	})
	if refreshed.ID != "win-feishu" {
		t.Fatalf("refreshed.ID = %q, want win-feishu", refreshed.ID)
	}
	if refreshed.PID != 100 {
		t.Fatalf("refreshed.PID = %d, want 100", refreshed.PID)
	}
}
