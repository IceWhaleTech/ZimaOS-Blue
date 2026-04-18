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
	"unsafe"
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

func TestDarwinTypeFocusedText_ResolvesWindowAndUsesClipboardPaste(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevFocus := darwinFocusWindowForHostAction
	prevPaste := darwinPasteTextFunc
	prevUnicode := darwinUnicodeTextInputFunc
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinFocusWindowForHostAction = func(_ context.Context, _ *darwinBackend, windowID string) (ActionResult, error) {
		if windowID != "win-1" {
			t.Fatalf("focus windowID = %q, want win-1", windowID)
		}
		return ActionResult{WindowID: "win-9", ExecutionMode: "automation", Message: "Window focused"}, nil
	}
	pasteCalls := 0
	unicodeCalls := 0
	darwinPasteTextFunc = func(value string) error {
		pasteCalls++
		if value != "hello" {
			t.Fatalf("paste value = %q, want hello", value)
		}
		return nil
	}
	darwinUnicodeTextInputFunc = func(string) error {
		unicodeCalls++
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinFocusWindowForHostAction = prevFocus
		darwinPasteTextFunc = prevPaste
		darwinUnicodeTextInputFunc = prevUnicode
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.TypeFocusedText(context.Background(), "win-1", "hello", 600)
	if err != nil {
		t.Fatalf("TypeFocusedText() error = %v", err)
	}
	if result.WindowID != "win-9" {
		t.Fatalf("window_id = %q, want win-9", result.WindowID)
	}
	if result.InputMethod != "clipboard" {
		t.Fatalf("input_method = %q, want clipboard", result.InputMethod)
	}
	if result.VerificationMethod != "focused_text" {
		t.Fatalf("verification_method = %q, want focused_text", result.VerificationMethod)
	}
	if pasteCalls != 1 {
		t.Fatalf("pasteCalls = %d, want 1", pasteCalls)
	}
	if unicodeCalls != 0 {
		t.Fatalf("unicodeCalls = %d, want 0", unicodeCalls)
	}
}

func TestDarwinTypeFocusedText_FallsBackToUnicodeInput(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevFocus := darwinFocusWindowForHostAction
	prevPaste := darwinPasteTextFunc
	prevUnicode := darwinUnicodeTextInputFunc
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinFocusWindowForHostAction = func(_ context.Context, _ *darwinBackend, windowID string) (ActionResult, error) {
		return ActionResult{WindowID: windowID, ExecutionMode: "automation", Message: "Window focused"}, nil
	}
	pasteCalls := 0
	unicodeCalls := 0
	darwinPasteTextFunc = func(string) error {
		pasteCalls++
		return errors.New("clipboard busy")
	}
	darwinUnicodeTextInputFunc = func(value string) error {
		unicodeCalls++
		if value != "hello" {
			t.Fatalf("unicode value = %q, want hello", value)
		}
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinFocusWindowForHostAction = prevFocus
		darwinPasteTextFunc = prevPaste
		darwinUnicodeTextInputFunc = prevUnicode
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.TypeFocusedText(context.Background(), "win-1", "hello", 600)
	if err != nil {
		t.Fatalf("TypeFocusedText() error = %v", err)
	}
	if result.InputMethod != "unicode" {
		t.Fatalf("input_method = %q, want unicode", result.InputMethod)
	}
	if pasteCalls != 1 {
		t.Fatalf("pasteCalls = %d, want 1", pasteCalls)
	}
	if unicodeCalls != 1 {
		t.Fatalf("unicodeCalls = %d, want 1", unicodeCalls)
	}
}

func TestDarwinClickWindowPixel_RefreshesWindowRecordAndClicksRelativePoint(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevResolve := darwinResolveWindowRecordForPointClick
	prevRefresh := darwinRefreshWindowRecordForPointClick
	prevClick := darwinClickPointForHostAction
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinResolveWindowRecordForPointClick = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		if windowID != "win-1" {
			t.Fatalf("resolve windowID = %q, want win-1", windowID)
		}
		return darwinWindowRecord{
			ID:    "win-1",
			Title: "Feishu",
			PID:   123,
			Bounds: darwinRect{
				Origin: darwinPoint{X: 10, Y: 20},
				Size:   darwinSize{Width: 320, Height: 240},
			},
		}, nil
	}
	darwinRefreshWindowRecordForPointClick = func(_ *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		if current.ID != "win-1" {
			t.Fatalf("refresh current.ID = %q, want win-1", current.ID)
		}
		current.ID = "win-9"
		current.Bounds = darwinRect{
			Origin: darwinPoint{X: 40, Y: 60},
			Size:   darwinSize{Width: 400, Height: 300},
		}
		return current, nil
	}
	var clicked darwinPoint
	clickHoldMS := 0
	darwinClickPointForHostAction = func(center darwinPoint, button uint32, doubleClick bool, hold bool, holdMS int) error {
		clicked = center
		clickHoldMS = holdMS
		if button != darwinCGMouseButtonLeft {
			t.Fatalf("button = %d, want left", button)
		}
		if doubleClick {
			t.Fatal("doubleClick = true, want false")
		}
		if hold {
			t.Fatal("hold = true, want false")
		}
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinResolveWindowRecordForPointClick = prevResolve
		darwinRefreshWindowRecordForPointClick = prevRefresh
		darwinClickPointForHostAction = prevClick
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	result, err := backend.ClickWindowPixel(context.Background(), "win-1", 100, 120, 750)
	if err != nil {
		t.Fatalf("ClickWindowPixel() error = %v", err)
	}
	if result.WindowID != "win-9" {
		t.Fatalf("window_id = %q, want win-9", result.WindowID)
	}
	if result.ExecutionMode != "input" {
		t.Fatalf("execution_mode = %q, want input", result.ExecutionMode)
	}
	if clicked.X != 140 || clicked.Y != 180 {
		t.Fatalf("clicked = %#v, want {X:140 Y:180}", clicked)
	}
	if clickHoldMS != 750 {
		t.Fatalf("clickHoldMS = %d, want 750", clickHoldMS)
	}
}

func TestDarwinClickWindowPixel_RejectsOutOfBoundsPoint(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevResolve := darwinResolveWindowRecordForPointClick
	prevRefresh := darwinRefreshWindowRecordForPointClick
	prevClick := darwinClickPointForHostAction
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinResolveWindowRecordForPointClick = func(_ *darwinBackend, _ string) (darwinWindowRecord, error) {
		return darwinWindowRecord{
			ID: "win-1",
			Bounds: darwinRect{
				Origin: darwinPoint{X: 10, Y: 20},
				Size:   darwinSize{Width: 80, Height: 60},
			},
		}, nil
	}
	darwinRefreshWindowRecordForPointClick = func(_ *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		return current, nil
	}
	clickCalls := 0
	darwinClickPointForHostAction = func(darwinPoint, uint32, bool, bool, int) error {
		clickCalls++
		return nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinResolveWindowRecordForPointClick = prevResolve
		darwinRefreshWindowRecordForPointClick = prevRefresh
		darwinClickPointForHostAction = prevClick
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	_, err := backend.ClickWindowPixel(context.Background(), "win-1", 100, 10, 600)
	if err == nil {
		t.Fatal("ClickWindowPixel() error = nil, want out-of-bounds error")
	}
	runtimeErr, ok := err.(*RuntimeError)
	if !ok {
		t.Fatalf("error = %T, want *RuntimeError", err)
	}
	if runtimeErr.Code != "unsupported_action" {
		t.Fatalf("code = %q, want unsupported_action", runtimeErr.Code)
	}
	if clickCalls != 0 {
		t.Fatalf("clickCalls = %d, want 0", clickCalls)
	}
}

func TestDarwinFindSnapshotWindowElement_RetriesWithRefreshedRecord(t *testing.T) {
	prevRefresh := darwinRefreshWindowRecordForSnapshot
	prevFind := darwinFindWindowElementForSnapshot
	findCalls := make([]string, 0, 2)
	darwinRefreshWindowRecordForSnapshot = func(_ *darwinBackend, current darwinWindowRecord) (darwinWindowRecord, error) {
		if current.ID != "win-1" {
			t.Fatalf("refresh current.ID = %q, want win-1", current.ID)
		}
		current.ID = "win-9"
		return current, nil
	}
	darwinFindWindowElementForSnapshot = func(_ uintptr, record darwinWindowRecord) uintptr {
		findCalls = append(findCalls, record.ID)
		if record.ID == "win-9" {
			return 2
		}
		return 0
	}
	defer func() {
		darwinRefreshWindowRecordForSnapshot = prevRefresh
		darwinFindWindowElementForSnapshot = prevFind
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	record, window := backend.findSnapshotWindowElement(1, darwinWindowRecord{ID: "win-1", PID: 123, Title: "Feishu"})
	if record.ID != "win-9" {
		t.Fatalf("record.ID = %q, want win-9", record.ID)
	}
	if window != 2 {
		t.Fatalf("window = %d, want 2", window)
	}
	if len(findCalls) == 0 {
		t.Fatal("expected AX window lookup attempts")
	}
	if len(findCalls) != 2 || findCalls[0] != "win-1" || findCalls[1] != "win-9" {
		t.Fatalf("findCalls = %#v, want [win-1 win-9]", findCalls)
	}
}

func TestDarwinFindWindowElement_UsesFocusedWindowFallbackWhenFocusedRecordAndWindowListUnavailable(t *testing.T) {
	initDarwinRuntime()
	prevCopyAttr := darwinAXUIElementCopyAttributeValue
	prevStringCreate := darwinCFStringCreate
	prevRelease := darwinCFRelease
	attrNames := map[uintptr]string{}
	nextAttrRef := uintptr(100)
	darwinCFStringCreate = func(_ uintptr, cstr *byte, _ uint32) uintptr {
		if cstr == nil {
			return 0
		}
		buf := make([]byte, 0, 32)
		for ptr := uintptr(unsafe.Pointer(cstr)); ; ptr++ {
			ch := *(*byte)(unsafe.Pointer(ptr))
			if ch == 0 {
				break
			}
			buf = append(buf, ch)
		}
		ref := nextAttrRef
		nextAttrRef++
		attrNames[ref] = string(buf)
		return ref
	}
	darwinAXUIElementCopyAttributeValue = func(element uintptr, attrRef uintptr, out *uintptr) int32 {
		switch {
		case element == 1 && attrNames[attrRef] == "AXFocusedWindow":
			if out != nil {
				*out = 2
			}
			return darwinAXErrorSuccess
		case element == 1 && attrNames[attrRef] == "AXWindows":
			if out != nil {
				*out = 0
			}
			return 1
		default:
			if out != nil {
				*out = 0
			}
			return 1
		}
	}
	darwinCFRelease = func(uintptr) {}
	defer func() {
		darwinAXUIElementCopyAttributeValue = prevCopyAttr
		darwinCFStringCreate = prevStringCreate
		darwinCFRelease = prevRelease
	}()

	got := darwinFindWindowElement(1, darwinWindowRecord{
		ID:      "win-1",
		AppName: "Feishu",
		Title:   "Feishu",
		Focused: true,
	})
	if got != 2 {
		t.Fatalf("darwinFindWindowElement() = %d, want focused window fallback 2", got)
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

func TestDarwinSnapshot_DoesNotCaptureNativeScreenshotOnSuccess(t *testing.T) {
	prevGranted := darwinAccessibilityGrantedProbe
	prevResolve := darwinResolveWindowRecordForSnapshot
	prevResolveCapture := darwinResolveWindowRecordForCapture
	prevCapture := darwinCaptureWindowImage
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinResolveWindowRecordForSnapshot = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	captureCalls := 0
	darwinResolveWindowRecordForCapture = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		captureCalls++
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinCaptureWindowImage = func(_ context.Context, _ string, _ string) (string, error) {
		captureCalls++
		return "", nil
	}
	defer func() {
		darwinAccessibilityGrantedProbe = prevGranted
		darwinResolveWindowRecordForSnapshot = prevResolve
		darwinResolveWindowRecordForCapture = prevResolveCapture
		darwinCaptureWindowImage = prevCapture
	}()

	backend := DefaultHostBackend("").(*darwinBackend)
	snapshot := BuildStructuredSnapshot(BuildStructuredSnapshotOptions{
		WindowID: "6263",
		Title:    "Feishu",
		Mode:     "ax",
	}, &Node{
		Role: "window",
		Name: "Feishu",
		Children: []*Node{{
			Token:       "token-search",
			Role:        "search_field",
			Name:        "Search",
			Interactive: true,
		}},
	})
	if snapshot == nil {
		t.Fatal("BuildStructuredSnapshot() = nil")
	}
	backend.snapshots.Swap(snapshot)

	result, err := backend.snapshot(context.Background(), "6263", false)
	if err != nil {
		t.Fatalf("snapshot() error = %v", err)
	}
	if captureCalls != 0 {
		t.Fatalf("captureCalls = %d, want 0", captureCalls)
	}
	if result.ImagePath != "" {
		t.Fatalf("image_path = %q, want empty on successful AX snapshot", result.ImagePath)
	}
	if result.Tree == "" {
		t.Fatal("tree = empty, want cached snapshot tree")
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

func TestDarwinSnapshotDescriptionWithState_AppendsSelectedAndKeepsFocusedUndeduped(t *testing.T) {
	got := darwinSnapshotDescriptionWithState("editable focused", true, true, false)
	if got != "editable focused selected" {
		t.Fatalf("description = %q, want %q", got, "editable focused selected")
	}
}

func TestDarwinSnapshotDescriptionWithState_UsesStateWhenBaseIsEmpty(t *testing.T) {
	got := darwinSnapshotDescriptionWithState("", false, true, true)
	if got != "selected expanded" {
		t.Fatalf("description = %q, want %q", got, "selected expanded")
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
	prevSnapshotCapture := darwinCaptureSnapshotScreenshot
	darwinAccessibilityGrantedProbe = func() bool { return true }
	darwinResolveWindowRecordForSnapshot = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Title: "Feishu", PID: 100}, nil
	}
	darwinAXUIElementCreateApplication = nil
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
		darwinResolveWindowRecordForSnapshot = prevResolve
		darwinAXUIElementCreateApplication = prevCreateApp
		darwinCaptureSnapshotScreenshot = prevSnapshotCapture
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
