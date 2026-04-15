//go:build darwin

package a11y

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"
)

const (
	darwinAXErrorSuccess = 0

	darwinAXValueCGPointType = 1
	darwinAXValueCGSizeType  = 2

	darwinCGEventLeftMouseDown  = 1
	darwinCGEventLeftMouseUp    = 2
	darwinCGEventRightMouseDown = 3
	darwinCGEventRightMouseUp   = 4

	darwinCGMouseButtonLeft  = 0
	darwinCGMouseButtonRight = 1

	darwinCGHIDEventTap           = 0
	darwinCGScrollEventUnitLine   = 1
	darwinCGMouseEventClickState  = 1
	darwinSnapshotMaxDepth        = 6
	darwinSnapshotMaxNodes        = 256
	darwinSnapshotMaxChildren     = 96
	darwinSyntheticClickDelay     = 60 * time.Millisecond
	darwinSyntheticTextFocusDelay = 80 * time.Millisecond
	darwinActivationWaitTimeout   = 30 * time.Second
	darwinActivationPollInterval  = 1 * time.Second
)

var darwinActivateAppFunc = darwinActivateApp
var darwinWaitForActivatedWindowRecord = func(ctx context.Context, b *darwinBackend, target darwinWindowRecord) (darwinWindowRecord, bool) {
	return b.waitForActivatedWindowRecord(ctx, target, darwinActivationWaitTimeout)
}
var darwinResolveWindowRecordForSnapshot = func(b *darwinBackend, windowID string) (darwinWindowRecord, error) {
	return b.resolveWindowRecord(windowID)
}
var darwinCaptureSnapshotScreenshot = func(ctx context.Context, b *darwinBackend, record darwinWindowRecord) (ScreenshotResult, error) {
	return b.screenshotWindowRecord(ctx, record)
}
var darwinFocusWindowForHostAction = func(ctx context.Context, b *darwinBackend, windowID string) (ActionResult, error) {
	return b.focusWindow(ctx, windowID)
}
var darwinPasteTextFunc = darwinPasteTextInput
var darwinUnicodeTextInputFunc = darwinSendText
var darwinHighlightInputBoundsFunc = func(bounds darwinRect, duration time.Duration) error {
	return darwinCLIFallback.showHighlightOverlay(nil, bounds, duration)
}
var darwinCaptureRegionPNGFunc = darwinCaptureRegionPNG
var darwinExtractTextFromPNGFunc = darwinExtractTextFromPNG

func (b *darwinBackend) focusWindow(ctx context.Context, windowID string) (ActionResult, error) {
	record, err := b.resolveWindowRecord(windowID)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return b.focusWindowRecord(ctx, record)
}

func (b *darwinBackend) focusWindowRecord(ctx context.Context, record darwinWindowRecord) (ActionResult, error) {
	if err := darwinActivateAppFunc(record.AppName); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if refreshed, ok := darwinWaitForActivatedWindowRecord(ctx, b, record); ok || strings.TrimSpace(refreshed.ID) != "" {
		record = refreshed
	}
	if !darwinAccessibilityGrantedProbe() {
		message := "Application activated; Accessibility permission is required to raise a specific window"
		if record.Focused {
			message = "Window focused"
		}
		return ActionResult{
			HostOS:        b.HostOS(),
			WindowID:      record.ID,
			ExecutionMode: "automation",
			Message:       message,
		}, nil
	}
	if darwinAXUIElementCreateApplication == nil {
		return ActionResult{HostOS: b.HostOS(), WindowID: record.ID, ExecutionMode: "automation", Message: "Application activated"}, nil
	}
	app := darwinAXUIElementCreateApplication(int32(record.PID))
	if app == 0 {
		return ActionResult{HostOS: b.HostOS(), WindowID: record.ID, ExecutionMode: "automation", Message: "Application activated"}, nil
	}
	defer darwinRelease(app)

	window := darwinFindWindowElement(app, record)
	if window == 0 {
		return ActionResult{HostOS: b.HostOS(), WindowID: record.ID, ExecutionMode: "automation", Message: "Application activated"}, nil
	}
	defer darwinRelease(window)

	if err := darwinPerformAXAction(window, "AXRaise"); err == nil {
		return ActionResult{HostOS: b.HostOS(), WindowID: record.ID, ExecutionMode: "semantic", Message: "Window focused"}, nil
	}
	return ActionResult{HostOS: b.HostOS(), WindowID: record.ID, ExecutionMode: "automation", Message: "Application activated"}, nil
}

func (b *darwinBackend) waitForActivatedWindowRecord(ctx context.Context, target darwinWindowRecord, timeout time.Duration) (darwinWindowRecord, bool) {
	best := target
	deadline := time.Now().Add(timeout)
	for {
		refreshed, err := b.refreshWindowRecord(target)
		if err == nil && strings.TrimSpace(refreshed.ID) != "" {
			best = refreshed
			if darwinActivatedWindowReady(target, refreshed) {
				return refreshed, true
			}
		}
		if timeout <= 0 || time.Now().After(deadline) {
			break
		}
		if err := darwinSleepWithContext(ctx, darwinActivationPollInterval); err != nil {
			return best, false
		}
	}
	return best, false
}

func darwinActivatedWindowReady(target darwinWindowRecord, candidate darwinWindowRecord) bool {
	if !candidate.Focused {
		return false
	}
	return darwinWindowRecordSimilarityScore(target, candidate) >= 2
}

func (b *darwinBackend) snapshot(ctx context.Context, windowID string, interactiveOnly bool) (SnapshotResult, error) {
	record, err := darwinResolveWindowRecordForSnapshot(b, windowID)
	if err != nil {
		return SnapshotResult{HostOS: b.HostOS()}, err
	}
	if err := b.ensureAccessibilityPermission(); err != nil {
		return SnapshotResult{HostOS: b.HostOS()}, b.enrichSnapshotPermissionError(ctx, record, err)
	}
	start := time.Now()
	snapshot, cacheHit, telemetry, err := b.ensureStructuredSnapshot(ctx, record)
	if err != nil {
		return SnapshotResult{HostOS: b.HostOS()}, err
	}
	mode := SnapshotProjectionFull
	if interactiveOnly {
		mode = SnapshotProjectionInteractive
	}
	projection := snapshot.Projection(mode)
	telemetry.SnapshotRevision = snapshot.Revision
	telemetry.CacheHit = cacheHit
	telemetry.NodeCount = len(snapshot.Nodes)
	telemetry.EndToEndMS = time.Since(start).Milliseconds()
	result := SnapshotResult{
		HostOS:          b.HostOS(),
		WindowID:        record.ID,
		Title:           record.Title,
		Tree:            projection.Tree,
		RefMap:          projection.RefMap,
		Message:         "Host accessibility snapshot ready",
		ActionTelemetry: telemetry,
	}
	return attachSnapshotImage(ctx, result, b.screenshot), nil
}

func (b *darwinBackend) enrichSnapshotPermissionError(ctx context.Context, record darwinWindowRecord, err error) error {
	return enrichSnapshotErrorWithImage(ctx, err, record.ID, func(ctx context.Context, _ string) (ScreenshotResult, error) {
		return darwinCaptureSnapshotScreenshot(ctx, b, record)
	})
}

func (b *darwinBackend) act(ctx context.Context, windowID string, ref int, refMap map[int]string, actType string, value string, holdMS int) (ActionResult, error) {
	start := time.Now()
	buildTelemetry := func(verificationMS int64, fallbacks []string) ActionTelemetry {
		total := time.Since(start).Milliseconds()
		actionMS := total - verificationMS
		if actionMS < 0 {
			actionMS = 0
		}
		return ActionTelemetry{
			ActionMS:       actionMS,
			VerificationMS: verificationMS,
			EndToEndMS:     total,
			Fallbacks:      append([]string(nil), fallbacks...),
		}
	}
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	token := strings.TrimSpace(refMap[ref])
	if token == "" {
		return ActionResult{HostOS: b.HostOS()}, NewError("stale_ref", fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref), nil)
	}
	element, ok := b.lookupSnapshotElement(token)
	if !ok {
		return ActionResult{HostOS: b.HostOS()}, NewError("stale_ref", fmt.Sprintf("ref @%d is no longer valid; take a new snapshot first", ref), nil)
	}
	meta := darwinInspectActionMetadata(element)
	plan := planDarwinAction(actType, meta)
	if plan.Unsupported {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", plan.UnsupportedReason, map[string]interface{}{"act_type": actType})
	}
	holdMS = NormalizeHoldMS(holdMS)
	var typeBounds darwinRect
	hasTypeBounds := false
	overlayMode := ""
	if strings.EqualFold(strings.TrimSpace(actType), "type") {
		typeBounds, hasTypeBounds = darwinElementBounds(element)
		darwinHighlightInputBounds(typeBounds, hasTypeBounds, darwinHighlightInputBoundsFunc)
		if hasTypeBounds {
			overlayMode = "mask"
		}
	}

	fallbacks := make([]string, 0, 3)
	if plan.SetValue {
		if err := darwinSetStringAttribute(element, "AXValue", value); err == nil {
			verifyStart := time.Now()
			if ok, method := darwinVerifySemanticTextEntry(
				ctx,
				value,
				func() string { return darwinCopyStringAttribute(element, "AXValue") },
				typeBounds,
				hasTypeBounds,
				darwinCaptureRegionPNGFunc,
				darwinExtractTextFromPNGFunc,
			); ok {
				verificationMS := time.Since(verifyStart).Milliseconds()
				return ActionResult{
					HostOS:             b.HostOS(),
					WindowID:           strings.TrimSpace(windowID),
					ExecutionMode:      "semantic",
					TargetHit:          true,
					InputMethod:        "set_value",
					VerificationPassed: true,
					VerificationMethod: method,
					OverlayMode:        overlayMode,
					Message:            "Host action completed",
					ActionTelemetry:    buildTelemetry(verificationMS, nil),
				}, nil
			}
		}
		fallbacks = append(fallbacks, "set_value", "verify_failed")
	}
	if plan.SemanticAction != "" {
		if err := darwinPerformAXAction(element, plan.SemanticAction); err == nil {
			return ActionResult{
				HostOS:             b.HostOS(),
				WindowID:           strings.TrimSpace(windowID),
				ExecutionMode:      "semantic",
				TargetHit:          true,
				InputMethod:        "semantic_action",
				VerificationPassed: true,
				VerificationMethod: "semantic_action",
				OverlayMode:        overlayMode,
				Message:            "Host action completed",
				ActionTelemetry:    buildTelemetry(0, nil),
			}, nil
		}
	}
	if plan.InputFallback == "" {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", "element could not be activated on this host", map[string]interface{}{"act_type": actType})
	}
	mode, inputMethod, err := darwinExecuteInputFallback(element, plan.InputFallback, value, holdMS)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if plan.InputFallback != "" {
		fallbacks = append(fallbacks, plan.InputFallback)
	}
	if strings.TrimSpace(inputMethod) != "" {
		fallbacks = append(fallbacks, inputMethod)
	}
	verificationPassed := true
	verificationMethod := "input_action"
	verificationMS := int64(0)
	if strings.EqualFold(strings.TrimSpace(actType), "type") {
		verifyStart := time.Now()
		ok, method := darwinVerifySemanticTextEntry(
			ctx,
			value,
			func() string { return darwinCopyStringAttribute(element, "AXValue") },
			typeBounds,
			hasTypeBounds,
			darwinCaptureRegionPNGFunc,
			darwinExtractTextFromPNGFunc,
		)
		verificationMS = time.Since(verifyStart).Milliseconds()
		verificationPassed = ok
		verificationMethod = method
		if !ok {
			fallbacks = append(fallbacks, "verify_failed")
		}
	}
	if strings.TrimSpace(inputMethod) == "" {
		inputMethod = plan.InputFallback
	}
	return ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           strings.TrimSpace(windowID),
		ExecutionMode:      mode,
		TargetHit:          true,
		InputMethod:        inputMethod,
		VerificationPassed: verificationPassed,
		VerificationMethod: verificationMethod,
		Fallbacks:          fallbacks,
		OverlayMode:        overlayMode,
		Message:            "Host action completed",
		ActionTelemetry:    buildTelemetry(verificationMS, fallbacks),
	}, nil
}

func (b *darwinBackend) scroll(ctx context.Context, windowID string, direction string, lines int) (ActionResult, error) {
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if darwinCGEventCreateScrollWheelEvent == nil || darwinCGEventPost == nil {
		return ActionResult{HostOS: b.HostOS()}, NewError("backend_unavailable", "CGEvent scroll injection is unavailable", nil)
	}
	lines = normalizeScrollLines(lines)
	resolvedWindowID, err := b.resolveWindowForAction(ctx, windowID)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	var vertical int32
	var horizontal int32
	switch strings.TrimSpace(strings.ToLower(direction)) {
	case "up":
		vertical = int32(lines)
	case "left":
		horizontal = int32(-lines)
	case "right":
		horizontal = int32(lines)
	default:
		vertical = int32(-lines)
	}
	event := darwinCGEventCreateScrollWheelEvent(0, darwinCGScrollEventUnitLine, 2, vertical, horizontal)
	if event == 0 {
		return ActionResult{HostOS: b.HostOS()}, NewError("backend_unavailable", "CGEvent scroll creation failed", nil)
	}
	defer darwinRelease(event)
	darwinCGEventPost(darwinCGHIDEventTap, event)
	return ActionResult{HostOS: b.HostOS(), WindowID: resolvedWindowID, ExecutionMode: "input", Message: "Scroll completed"}, nil
}

func (b *darwinBackend) pointerMove(_ context.Context, x int, y int) (ActionResult, error) {
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if darwinCGWarpMouseCursorPosition == nil {
		return ActionResult{HostOS: b.HostOS()}, NewError("backend_unavailable", "pointer movement is unavailable", nil)
	}
	darwinCGWarpMouseCursorPosition(darwinPoint{X: float64(x), Y: float64(y)})
	return ActionResult{HostOS: b.HostOS(), ExecutionMode: "input", Message: "Pointer moved"}, nil
}

func (b *darwinBackend) clickWindowPoint(ctx context.Context, windowID string, point NormalizedPoint, holdMS int) (ActionResult, error) {
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if point.X < 0 || point.X > 1 || point.Y < 0 || point.Y > 1 {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", "normalized click point must be between 0 and 1", map[string]interface{}{
			"x": point.X,
			"y": point.Y,
		})
	}
	record, err := b.resolveWindowRecord(strings.TrimSpace(windowID))
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if refreshed, refreshErr := b.refreshWindowRecord(record); refreshErr == nil {
		record = refreshed
	}
	if !darwinRectDefined(record.Bounds) || record.Bounds.Size.Width <= 0 || record.Bounds.Size.Height <= 0 {
		return ActionResult{HostOS: b.HostOS()}, NewError("backend_unavailable", "target window has no visible bounds", map[string]interface{}{
			"window_id": record.ID,
		})
	}
	clickPoint := darwinPoint{
		X: record.Bounds.Origin.X + point.X*record.Bounds.Size.Width,
		Y: record.Bounds.Origin.Y + point.Y*record.Bounds.Size.Height,
	}
	if err := darwinClickPoint(clickPoint, darwinCGMouseButtonLeft, false, false, NormalizeHoldMS(holdMS)); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           record.ID,
		ExecutionMode:      "input",
		TargetHit:          true,
		InputMethod:        "input_click",
		VerificationPassed: true,
		VerificationMethod: "point_click",
		Message:            "Host action completed",
	}, nil
}

func (b *darwinBackend) key(ctx context.Context, windowID string, keys []string, holdMS int) (ActionResult, error) {
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	resolvedWindowID, err := b.resolveWindowForAction(ctx, windowID)
	if err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	if err := darwinSendKeySequence(keys, NormalizeHoldMS(holdMS)); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	return ActionResult{HostOS: b.HostOS(), WindowID: resolvedWindowID, ExecutionMode: "input", Message: "Keys sent"}, nil
}

func (b *darwinBackend) resolveWindowForAction(ctx context.Context, windowID string) (string, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return "", nil
	}
	result, err := darwinFocusWindowForHostAction(ctx, b, windowID)
	if err != nil {
		return "", err
	}
	resolved := strings.TrimSpace(result.WindowID)
	if resolved != "" {
		return resolved, nil
	}
	return windowID, nil
}

func (b *darwinBackend) ensureAccessibilityPermission() error {
	if darwinAccessibilityGrantedProbe() {
		darwinResetAccessibilityPromptState()
		return nil
	}
	darwinRequestAccessibilityPromptIfNeeded()
	message := darwinAccessibilityPermissionMessage()
	return NewError("permission_required", message, map[string]interface{}{
		"permissions": []PermissionStatus{{
			Name:     "accessibility",
			Granted:  false,
			Required: true,
			Message:  message,
		}},
	})
}

func darwinAccessibilityPermissionMessage() string {
	return fmt.Sprintf("Grant Accessibility permission to %q in System Settings > Privacy & Security > Accessibility to enable snapshots and synthetic input", darwinAccessibilityAppName())
}

func (b *darwinBackend) listWindowRecords() ([]darwinWindowRecord, error) {
	return b.listWindowRecordsWithOptions(darwinCGWindowListOptionOnScreenOnly | darwinCGWindowListExcludeDesktop)
}

func (b *darwinBackend) listAllWindowRecords() ([]darwinWindowRecord, error) {
	return b.listWindowRecordsWithOptions(darwinCGWindowListOptionAll | darwinCGWindowListExcludeDesktop)
}

func (b *darwinBackend) listWindowRecordsWithOptions(options uint32) ([]darwinWindowRecord, error) {
	initDarwinRuntime()
	if darwinCGWindowListCopyInfo == nil {
		return nil, NewError("backend_unavailable", "CoreGraphics window listing is unavailable", nil)
	}
	if darwinCFArrayGetCount == nil || darwinCFArrayGetValueAtIndex == nil {
		return nil, NewError("backend_unavailable", "CoreFoundation array access is unavailable", nil)
	}
	array := darwinCGWindowListCopyInfo(options, 0)
	if array == 0 {
		return nil, nil
	}
	defer darwinRelease(array)

	count := darwinCFArrayGetCount(array)
	records := make([]darwinWindowRecord, 0, count)
	for idx := int64(0); idx < count; idx++ {
		dict := darwinCFArrayGetValueAtIndex(array, idx)
		if dict == 0 {
			continue
		}
		record := darwinWindowInfoFromDictionary(dict)
		if !darwinIncludeWindowRecord(record) {
			continue
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil, nil
	}
	frontPID, frontTitle, frontBounds, ok := darwinFocusedWindowDescriptor()
	if ok {
		darwinMarkFocusedRecord(records, frontPID, frontTitle, frontBounds)
		return records, nil
	}
	darwinMarkFocusedRecord(records, 0, "", darwinRect{})
	return records, nil
}

func (b *darwinBackend) resolveWindowRecord(windowID string) (darwinWindowRecord, error) {
	records, err := b.listWindowRecords()
	if err != nil {
		return darwinWindowRecord{}, err
	}
	record, resolveErr := darwinResolveWindowRecordFromLists(windowID, records, nil)
	if resolveErr == nil || strings.TrimSpace(windowID) == "" {
		return record, resolveErr
	}
	runtimeErr, ok := resolveErr.(*RuntimeError)
	if !ok || runtimeErr.Code != "backend_unavailable" || runtimeErr.Message != "target window not found" {
		return darwinWindowRecord{}, resolveErr
	}
	allRecords, err := b.listAllWindowRecords()
	if err != nil {
		return darwinWindowRecord{}, err
	}
	return darwinResolveWindowRecordFromLists(windowID, records, allRecords)
}

func (b *darwinBackend) refreshWindowRecord(current darwinWindowRecord) (darwinWindowRecord, error) {
	records, err := b.listWindowRecords()
	if err != nil {
		return darwinWindowRecord{}, err
	}
	allRecords := records
	if strings.TrimSpace(current.ID) != "" {
		if _, ok := darwinFindWindowRecordByID(records, current.ID); !ok {
			allRecords, err = b.listAllWindowRecords()
			if err != nil {
				return darwinWindowRecord{}, err
			}
		}
	}
	return darwinRefreshWindowRecordFromLists(current, records, allRecords), nil
}

func darwinResolveWindowRecordFromLists(windowID string, records []darwinWindowRecord, allRecords []darwinWindowRecord) (darwinWindowRecord, error) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		if len(records) == 0 {
			return darwinWindowRecord{}, NewError("backend_unavailable", "no host windows available", nil)
		}
		for _, record := range records {
			if record.Focused {
				return record, nil
			}
		}
		return records[0], nil
	}
	if record, ok := darwinFindWindowRecordByID(records, windowID); ok {
		return record, nil
	}
	if record, ok := darwinFindWindowRecordByID(allRecords, windowID); ok {
		return record, nil
	}
	if len(records) == 0 && len(allRecords) == 0 {
		return darwinWindowRecord{}, NewError("backend_unavailable", "no host windows available", nil)
	}
	return darwinWindowRecord{}, NewError("backend_unavailable", "target window not found", map[string]interface{}{"window_id": windowID})
}

func darwinRefreshWindowRecordFromLists(current darwinWindowRecord, records []darwinWindowRecord, allRecords []darwinWindowRecord) darwinWindowRecord {
	if len(records) == 0 && len(allRecords) == 0 {
		return current
	}
	if record, ok := darwinFindWindowRecordByID(records, current.ID); ok {
		return record
	}
	if record, ok := darwinFindWindowRecordByID(allRecords, current.ID); ok {
		return record
	}
	best, ok := darwinBestWindowRecordSimilarity(current, records)
	if ok {
		return best
	}
	best, ok = darwinBestWindowRecordSimilarity(current, allRecords)
	if ok {
		return best
	}
	return current
}

func darwinFindWindowRecordByID(records []darwinWindowRecord, windowID string) (darwinWindowRecord, bool) {
	windowID = strings.TrimSpace(windowID)
	if windowID == "" {
		return darwinWindowRecord{}, false
	}
	for _, record := range records {
		if record.ID == windowID {
			return record, true
		}
	}
	return darwinWindowRecord{}, false
}

func darwinBestWindowRecordSimilarity(current darwinWindowRecord, records []darwinWindowRecord) (darwinWindowRecord, bool) {
	best := current
	bestScore := -1
	for _, record := range records {
		score := darwinWindowRecordSimilarityScore(current, record)
		if score > bestScore {
			best = record
			bestScore = score
		}
	}
	if bestScore < 0 {
		return darwinWindowRecord{}, false
	}
	return best, true
}

func darwinWindowRecordSimilarityScore(current darwinWindowRecord, candidate darwinWindowRecord) int {
	score := -1
	if current.PID > 0 && candidate.PID == current.PID {
		score = 2
	}
	if strings.TrimSpace(current.AppName) != "" && strings.EqualFold(strings.TrimSpace(current.AppName), strings.TrimSpace(candidate.AppName)) {
		if score < 0 {
			score = 0
		}
		score++
	}
	if strings.TrimSpace(current.Title) != "" && strings.EqualFold(strings.TrimSpace(current.Title), strings.TrimSpace(candidate.Title)) {
		if score < 0 {
			score = 0
		}
		score += 2
	}
	if candidate.Focused && current.PID > 0 && candidate.PID == current.PID {
		if score < 0 {
			score = 0
		}
		score++
	}
	if darwinRectDefined(current.Bounds) && darwinRectsClose(current.Bounds, candidate.Bounds) {
		if score < 0 {
			score = 0
		}
		score++
	}
	return score
}

func darwinRectDefined(rect darwinRect) bool {
	return rect.Size.Width != 0 || rect.Size.Height != 0 || rect.Origin.X != 0 || rect.Origin.Y != 0
}

func (b *darwinBackend) buildSnapshotNode(element uintptr, depth int, visited *int) *Node {
	if element == 0 || depth > darwinSnapshotMaxDepth || visited == nil || *visited >= darwinSnapshotMaxNodes {
		return nil
	}
	*visited = *visited + 1

	roleRaw := darwinCopyStringAttribute(element, "AXSubrole")
	if strings.TrimSpace(roleRaw) == "" {
		roleRaw = darwinCopyStringAttribute(element, "AXRole")
	}
	role := normalizeDarwinRole(roleRaw)
	title := darwinFirstNonEmpty(
		darwinCopyStringAttribute(element, "AXTitle"),
		darwinCopyStringAttribute(element, "AXLabel"),
		darwinCopyStringAttribute(element, "AXDescription"),
	)
	valueRef := darwinMustCopyAttributeValue(element, "AXValue")
	value := darwinCFTypeValueString(valueRef)
	darwinRelease(valueRef)
	description := darwinCopyStringAttribute(element, "AXDescription")
	actions := darwinCopyActionNamesForElement(element)
	valueSettable := darwinAttributeSettable(element, "AXValue")
	defaultAction := darwinDefaultActionLabel(actions)
	interactive := valueSettable || len(actions) > 0 || darwinRoleLikelyInteractive(role)

	node := &Node{
		Role:          role,
		Name:          title,
		Value:         value,
		Description:   description,
		DefaultAction: defaultAction,
		Interactive:   interactive,
	}
	if interactive || strings.TrimSpace(defaultAction) != "" {
		node.Token = b.storeSnapshotElement(element)
	}

	childrenRef, err := darwinCopyAttributeValue(element, "AXChildren")
	if err != nil || childrenRef == 0 || darwinCFArrayGetCount == nil || darwinCFArrayGetValueAtIndex == nil {
		return node
	}
	defer darwinRelease(childrenRef)

	childCount := darwinCFArrayGetCount(childrenRef)
	if childCount > darwinSnapshotMaxChildren {
		childCount = darwinSnapshotMaxChildren
	}
	for idx := int64(0); idx < childCount; idx++ {
		child := darwinCFArrayGetValueAtIndex(childrenRef, idx)
		if child == 0 {
			continue
		}
		if childNode := b.buildSnapshotNode(child, depth+1, visited); childNode != nil {
			node.Children = append(node.Children, childNode)
		}
	}
	return node
}

func (b *darwinBackend) resetSnapshotElements() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clearSnapshotElementsForWindowLocked("")
	b.nextRef = 0
	b.snapshotWindow = ""
	if b.elements == nil {
		b.elements = make(map[string]darwinElementRef)
	}
}

func (b *darwinBackend) prepareSnapshotElements(windowID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clearSnapshotElementsForWindowLocked(windowID)
	b.snapshotWindow = strings.TrimSpace(windowID)
	if b.elements == nil {
		b.elements = make(map[string]darwinElementRef)
	}
}

func (b *darwinBackend) finishSnapshotElementsBuild() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.snapshotWindow = ""
}

func (b *darwinBackend) resetSnapshotElementsForWindow(windowID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clearSnapshotElementsForWindowLocked(windowID)
	if strings.TrimSpace(windowID) == strings.TrimSpace(b.snapshotWindow) {
		b.snapshotWindow = ""
	}
	if b.elements == nil {
		b.elements = make(map[string]darwinElementRef)
	}
}

func (b *darwinBackend) clearSnapshotElementsForWindowLocked(windowID string) {
	prefix := ""
	if trimmed := strings.TrimSpace(windowID); trimmed != "" {
		prefix = trimmed + "|"
	}
	for token, ref := range b.elements {
		if prefix != "" && !strings.HasPrefix(token, prefix) {
			continue
		}
		if ref.Element != 0 {
			darwinRelease(ref.Element)
		}
		delete(b.elements, token)
	}
}

func (b *darwinBackend) storeSnapshotElement(element uintptr) string {
	if element == 0 {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.elements == nil {
		b.elements = make(map[string]darwinElementRef)
	}
	b.nextRef++
	token := fmt.Sprintf("darwin-%d", b.nextRef)
	if windowID := strings.TrimSpace(b.snapshotWindow); windowID != "" {
		token = fmt.Sprintf("%s|darwin-%d", windowID, b.nextRef)
	}
	if darwinCFRetain != nil {
		darwinCFRetain(element)
	}
	b.elements[token] = darwinElementRef{Element: element}
	return token
}

func (b *darwinBackend) lookupSnapshotElement(token string) (uintptr, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ref, ok := b.elements[strings.TrimSpace(token)]
	if !ok || ref.Element == 0 {
		return 0, false
	}
	return ref.Element, true
}

func darwinInspectActionMetadata(element uintptr) darwinActionMetadata {
	expanded, hasExpanded := darwinCopyBoolAttribute(element, "AXExpanded")
	actions := darwinCopyActionNamesForElement(element)
	meta := darwinActionMetadata{
		Role:             normalizeDarwinRole(darwinFirstNonEmpty(darwinCopyStringAttribute(element, "AXSubrole"), darwinCopyStringAttribute(element, "AXRole"))),
		DefaultAction:    darwinDefaultActionLabel(actions),
		AvailableActions: actions,
		ValueSettable:    darwinAttributeSettable(element, "AXValue"),
	}
	if hasExpanded {
		meta.Expanded = &expanded
	}
	return meta
}

func darwinExecuteInputFallback(element uintptr, fallback string, value string, holdMS int) (string, string, error) {
	switch fallback {
	case darwinInputFallbackClick:
		if err := darwinClickElement(element, darwinCGMouseButtonLeft, false, false, NormalizeHoldMS(holdMS)); err != nil {
			return "", "", err
		}
		return "input", "input_click", nil
	case darwinInputFallbackDoubleClick:
		if err := darwinClickElement(element, darwinCGMouseButtonLeft, true, false, NormalizeHoldMS(holdMS)); err != nil {
			return "", "", err
		}
		return "input", "input_double_click", nil
	case darwinInputFallbackRightClick:
		if err := darwinClickElement(element, darwinCGMouseButtonRight, false, false, NormalizeHoldMS(holdMS)); err != nil {
			return "", "", err
		}
		return "input", "input_right_click", nil
	case darwinInputFallbackClickHold:
		if err := darwinClickElement(element, darwinCGMouseButtonLeft, false, true, NormalizeHoldMS(holdMS)); err != nil {
			return "", "", err
		}
		return "input", "input_long_press", nil
	case darwinInputFallbackType:
		alreadyFocused, _ := darwinCopyBoolAttribute(element, "AXFocused")
		inputMethod := ""
		if err := darwinTypeWithFocusClickFallback(
			value,
			alreadyFocused,
			func() error {
				return darwinClickElement(element, darwinCGMouseButtonLeft, false, false, NormalizeHoldMS(holdMS))
			},
			func() {
				time.Sleep(darwinSyntheticTextFocusDelay)
			},
			func(value string) error {
				method, err := darwinSendTextWithClipboardFallback(value, darwinPasteTextFunc, darwinSendText)
				if method != "" {
					inputMethod = method
				}
				return err
			},
		); err != nil {
			return "", "", err
		}
		if inputMethod == "" {
			inputMethod = "input_type"
		}
		return "input", inputMethod, nil
	default:
		return "", "", NewError("unsupported_action", "input fallback is unavailable", map[string]interface{}{"fallback": fallback})
	}
}

func darwinPasteTextInput(text string) error {
	output, err := darwinCLIFallback.pasteTextWithTemporaryClipboard(nil, text)
	if err != nil {
		return fmt.Errorf("osascript clipboard paste failed: %s: %w", output, err)
	}
	return nil
}

func darwinClickElement(element uintptr, button uint32, doubleClick bool, hold bool, holdMS int) error {
	bounds, ok := darwinElementBounds(element)
	if !ok {
		return NewError("unsupported_action", "element bounds are unavailable for pointer fallback", nil)
	}
	return darwinClickPoint(darwinPoint{
		X: bounds.Origin.X + bounds.Size.Width/2,
		Y: bounds.Origin.Y + bounds.Size.Height/2,
	}, button, doubleClick, hold, holdMS)
}

func darwinClickPoint(center darwinPoint, button uint32, doubleClick bool, hold bool, holdMS int) error {
	if darwinCGEventCreateMouseEvent == nil || darwinCGEventPost == nil {
		return NewError("backend_unavailable", "pointer event injection is unavailable", nil)
	}
	if darwinCGWarpMouseCursorPosition != nil {
		darwinCGWarpMouseCursorPosition(center)
	}

	downType := darwinCGEventLeftMouseDown
	upType := darwinCGEventLeftMouseUp
	if button == darwinCGMouseButtonRight {
		downType = darwinCGEventRightMouseDown
		upType = darwinCGEventRightMouseUp
	}

	down := darwinCGEventCreateMouseEvent(0, uint32(downType), center, button)
	up := darwinCGEventCreateMouseEvent(0, uint32(upType), center, button)
	if down == 0 || up == 0 {
		darwinRelease(down)
		darwinRelease(up)
		return NewError("backend_unavailable", "CGEvent mouse creation failed", nil)
	}
	defer darwinRelease(down)
	defer darwinRelease(up)

	if doubleClick && darwinCGEventSetIntegerValueField != nil {
		darwinCGEventSetIntegerValueField(down, darwinCGMouseEventClickState, 2)
		darwinCGEventSetIntegerValueField(up, darwinCGMouseEventClickState, 2)
	}
	darwinCGEventPost(darwinCGHIDEventTap, down)
	if hold {
		time.Sleep(time.Duration(holdMS) * time.Millisecond)
	} else {
		time.Sleep(darwinSyntheticClickDelay)
	}
	darwinCGEventPost(darwinCGHIDEventTap, up)

	if doubleClick {
		time.Sleep(darwinSyntheticClickDelay)
		secondDown := darwinCGEventCreateMouseEvent(0, uint32(downType), center, button)
		secondUp := darwinCGEventCreateMouseEvent(0, uint32(upType), center, button)
		if secondDown == 0 || secondUp == 0 {
			darwinRelease(secondDown)
			darwinRelease(secondUp)
			return nil
		}
		defer darwinRelease(secondDown)
		defer darwinRelease(secondUp)
		if darwinCGEventSetIntegerValueField != nil {
			darwinCGEventSetIntegerValueField(secondDown, darwinCGMouseEventClickState, 2)
			darwinCGEventSetIntegerValueField(secondUp, darwinCGMouseEventClickState, 2)
		}
		darwinCGEventPost(darwinCGHIDEventTap, secondDown)
		time.Sleep(darwinSyntheticClickDelay)
		darwinCGEventPost(darwinCGHIDEventTap, secondUp)
	}
	return nil
}

func darwinSendKeySequence(keys []string, holdMS int) error {
	cleaned, handled, err := handleLiteralTextKeySequence(
		keys,
		darwinIsModifierKey,
		func(value string) bool {
			_, ok := darwinKeyCodeForName(value)
			return ok
		},
		func(value string) error {
			_, err := darwinSendTextWithClipboardFallback(value, darwinPasteTextFunc, darwinUnicodeTextInputFunc)
			return err
		},
	)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}

	modifiers := make([]string, 0, len(cleaned))
	primary := ""
	for _, key := range cleaned {
		if darwinIsModifierKey(key) {
			modifiers = append(modifiers, key)
			continue
		}
		primary = key
	}
	if primary == "" && len(cleaned) > 0 {
		primary = cleaned[len(cleaned)-1]
	}

	flags := darwinModifierFlags(cleaned)
	for _, modifier := range modifiers {
		keyCode, ok := darwinKeyCodeForName(modifier)
		if !ok {
			continue
		}
		if err := darwinPostKeyCodeEvent(keyCode, darwinModifierFlags([]string{modifier}), true); err != nil {
			return err
		}
	}
	if primary != "" {
		keyCode, ok := darwinKeyCodeForName(primary)
		if !ok && len([]rune(primary)) > 0 {
			if _, err := darwinSendTextWithClipboardFallback(primary, darwinPasteTextFunc, darwinUnicodeTextInputFunc); err != nil {
				return err
			}
		} else {
			if err := darwinPostKeyCodeEvent(keyCode, flags, true); err != nil {
				return err
			}
			time.Sleep(time.Duration(holdMS) * time.Millisecond)
			if err := darwinPostKeyCodeEvent(keyCode, flags, false); err != nil {
				return err
			}
		}
	}
	for idx := len(modifiers) - 1; idx >= 0; idx-- {
		modifier := modifiers[idx]
		keyCode, ok := darwinKeyCodeForName(modifier)
		if !ok {
			continue
		}
		if err := darwinPostKeyCodeEvent(keyCode, 0, false); err != nil {
			return err
		}
	}
	return nil
}

func darwinSendText(text string) error {
	if darwinCGEventCreateKeyboardEvent == nil || darwinCGEventKeyboardSetUnicodeString == nil || darwinCGEventPost == nil {
		return NewError("backend_unavailable", "text input is unavailable", nil)
	}
	for _, r := range text {
		chars := []uint16{uint16(r)}
		down := darwinCGEventCreateKeyboardEvent(0, 0, true)
		up := darwinCGEventCreateKeyboardEvent(0, 0, false)
		if down == 0 || up == 0 {
			darwinRelease(down)
			darwinRelease(up)
			return NewError("backend_unavailable", "keyboard event creation failed", nil)
		}
		darwinCGEventKeyboardSetUnicodeString(down, 1, &chars[0])
		darwinCGEventKeyboardSetUnicodeString(up, 1, &chars[0])
		darwinCGEventPost(darwinCGHIDEventTap, down)
		darwinCGEventPost(darwinCGHIDEventTap, up)
		darwinRelease(down)
		darwinRelease(up)
	}
	return nil
}

func darwinPostKeyCodeEvent(keyCode uint16, flags uint64, down bool) error {
	if darwinCGEventCreateKeyboardEvent == nil || darwinCGEventPost == nil {
		return NewError("backend_unavailable", "keyboard event injection is unavailable", nil)
	}
	event := darwinCGEventCreateKeyboardEvent(0, keyCode, down)
	if event == 0 {
		return NewError("backend_unavailable", "keyboard event creation failed", nil)
	}
	defer darwinRelease(event)
	if darwinCGEventSetFlags != nil {
		darwinCGEventSetFlags(event, flags)
	}
	darwinCGEventPost(darwinCGHIDEventTap, event)
	return nil
}

func darwinFindWindowElement(app uintptr, record darwinWindowRecord) uintptr {
	if app == 0 {
		return 0
	}
	if focused, err := darwinCopyAttributeValue(app, "AXFocusedWindow"); err == nil && focused != 0 {
		if darwinWindowElementMatches(focused, record) {
			return focused
		}
		darwinRelease(focused)
	}
	windowsRef, err := darwinCopyAttributeValue(app, "AXWindows")
	if err != nil || windowsRef == 0 {
		return 0
	}
	defer darwinRelease(windowsRef)
	if darwinCFArrayGetCount == nil || darwinCFArrayGetValueAtIndex == nil {
		return 0
	}

	count := darwinCFArrayGetCount(windowsRef)
	if count == 1 {
		window := darwinCFArrayGetValueAtIndex(windowsRef, 0)
		if window != 0 && darwinCFRetain != nil {
			darwinCFRetain(window)
		}
		return window
	}
	best := uintptr(0)
	for idx := int64(0); idx < count; idx++ {
		window := darwinCFArrayGetValueAtIndex(windowsRef, idx)
		if window == 0 {
			continue
		}
		if darwinWindowElementMatches(window, record) {
			if darwinCFRetain != nil {
				darwinCFRetain(window)
			}
			return window
		}
		if best == 0 {
			best = window
		}
	}
	if best != 0 && darwinCFRetain != nil {
		darwinCFRetain(best)
	}
	return best
}

func darwinWindowElementMatches(element uintptr, record darwinWindowRecord) bool {
	title := darwinCopyStringAttribute(element, "AXTitle")
	if strings.TrimSpace(record.Title) != "" && strings.EqualFold(strings.TrimSpace(title), strings.TrimSpace(record.Title)) {
		return true
	}
	bounds, ok := darwinElementBounds(element)
	return ok && darwinRectsClose(bounds, record.Bounds)
}

func darwinFocusedWindowDescriptor() (int, string, darwinRect, bool) {
	if !darwinAccessibilityGrantedProbe() || darwinAXUIElementCreateSystemWide == nil {
		return 0, "", darwinRect{}, false
	}
	system := darwinAXUIElementCreateSystemWide()
	if system == 0 {
		return 0, "", darwinRect{}, false
	}
	defer darwinRelease(system)

	app, err := darwinCopyAttributeValue(system, "AXFocusedApplication")
	if err != nil || app == 0 {
		return 0, "", darwinRect{}, false
	}
	defer darwinRelease(app)

	var pid int32
	if darwinAXUIElementGetPid != nil {
		if darwinAXUIElementGetPid(app, &pid) != darwinAXErrorSuccess {
			pid = 0
		}
	}
	window, err := darwinCopyAttributeValue(app, "AXFocusedWindow")
	if err != nil || window == 0 {
		return int(pid), "", darwinRect{}, pid > 0
	}
	defer darwinRelease(window)
	title := darwinCopyStringAttribute(window, "AXTitle")
	bounds, _ := darwinElementBounds(window)
	return int(pid), title, bounds, pid > 0
}

func darwinActivateApp(appName string) error {
	return darwinCLIFallback.activateApp(appName)
}

func darwinEscapeAppleScript(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func darwinAccessibilityAppName() string {
	exe, err := os.Executable()
	if err == nil && strings.Contains(exe, ".app/Contents/MacOS/") {
		return "Blue"
	}
	return "Terminal"
}

func darwinCopyAttributeValue(element uintptr, attribute string) (uintptr, error) {
	initDarwinRuntime()
	if element == 0 || darwinAXUIElementCopyAttributeValue == nil {
		return 0, NewError("backend_unavailable", "AX attribute access is unavailable", map[string]interface{}{"attribute": attribute})
	}
	attrRef := darwinCFStringRef(attribute)
	if attrRef == 0 {
		return 0, NewError("backend_unavailable", "AX attribute access is unavailable", map[string]interface{}{"attribute": attribute})
	}
	defer darwinRelease(attrRef)
	var value uintptr
	if code := darwinAXUIElementCopyAttributeValue(element, attrRef, &value); code != darwinAXErrorSuccess {
		return 0, fmt.Errorf("AX error %d for %s", code, attribute)
	}
	return value, nil
}

func darwinMustCopyAttributeValue(element uintptr, attribute string) uintptr {
	value, err := darwinCopyAttributeValue(element, attribute)
	if err != nil {
		return 0
	}
	return value
}

func darwinCopyActionNamesForElement(element uintptr) []string {
	initDarwinRuntime()
	if element == 0 || darwinAXUIElementCopyActionNames == nil || darwinCFArrayGetCount == nil || darwinCFArrayGetValueAtIndex == nil {
		return nil
	}
	var actionsRef uintptr
	if code := darwinAXUIElementCopyActionNames(element, &actionsRef); code != darwinAXErrorSuccess || actionsRef == 0 {
		return nil
	}
	defer darwinRelease(actionsRef)
	count := darwinCFArrayGetCount(actionsRef)
	names := make([]string, 0, count)
	for idx := int64(0); idx < count; idx++ {
		raw := darwinCFArrayGetValueAtIndex(actionsRef, idx)
		if name := darwinCFStringValue(raw); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func darwinPerformAXAction(element uintptr, action string) error {
	initDarwinRuntime()
	if element == 0 || darwinAXUIElementPerformAction == nil {
		return NewError("backend_unavailable", "AX actions are unavailable", map[string]interface{}{"action": action})
	}
	actionRef := darwinCFStringRef(action)
	if actionRef == 0 {
		return NewError("backend_unavailable", "AX actions are unavailable", map[string]interface{}{"action": action})
	}
	defer darwinRelease(actionRef)
	if code := darwinAXUIElementPerformAction(element, actionRef); code != darwinAXErrorSuccess {
		return fmt.Errorf("AX action %s failed: %d", action, code)
	}
	return nil
}

func darwinSetStringAttribute(element uintptr, attribute string, value string) error {
	initDarwinRuntime()
	if element == 0 || darwinAXUIElementSetAttributeValue == nil {
		return NewError("backend_unavailable", "AX attribute mutation is unavailable", map[string]interface{}{"attribute": attribute})
	}
	attrRef := darwinCFStringRef(attribute)
	valueRef := darwinCFStringRef(value)
	if attrRef == 0 || valueRef == 0 {
		darwinRelease(attrRef)
		darwinRelease(valueRef)
		return NewError("backend_unavailable", "AX attribute mutation is unavailable", map[string]interface{}{"attribute": attribute})
	}
	defer darwinRelease(attrRef)
	defer darwinRelease(valueRef)
	if code := darwinAXUIElementSetAttributeValue(element, attrRef, valueRef); code != darwinAXErrorSuccess {
		return fmt.Errorf("AX set %s failed: %d", attribute, code)
	}
	return nil
}

func darwinAttributeSettable(element uintptr, attribute string) bool {
	initDarwinRuntime()
	if element == 0 || darwinAXUIElementIsAttributeSettable == nil {
		return false
	}
	attrRef := darwinCFStringRef(attribute)
	if attrRef == 0 {
		return false
	}
	defer darwinRelease(attrRef)
	var settable bool
	return darwinAXUIElementIsAttributeSettable(element, attrRef, &settable) == darwinAXErrorSuccess && settable
}

func darwinCopyStringAttribute(element uintptr, attribute string) string {
	value, err := darwinCopyAttributeValue(element, attribute)
	if err != nil || value == 0 {
		return ""
	}
	defer darwinRelease(value)
	return darwinCFTypeValueString(value)
}

func darwinCopyBoolAttribute(element uintptr, attribute string) (bool, bool) {
	value, err := darwinCopyAttributeValue(element, attribute)
	if err != nil || value == 0 {
		return false, false
	}
	defer darwinRelease(value)
	if darwinCFGetTypeID == nil || darwinCFBooleanGetTypeID == nil || darwinCFBooleanGetValue == nil {
		return false, false
	}
	if darwinCFGetTypeID(value) != darwinCFBooleanGetTypeID() {
		return false, false
	}
	return darwinCFBooleanGetValue(value), true
}

func darwinCFTypeValueString(ref uintptr) string {
	if ref == 0 || darwinCFGetTypeID == nil {
		return ""
	}
	typeID := darwinCFGetTypeID(ref)
	switch {
	case darwinCFStringGetTypeID != nil && typeID == darwinCFStringGetTypeID():
		return darwinCFStringValue(ref)
	case darwinCFNumberGetTypeID != nil && typeID == darwinCFNumberGetTypeID():
		return fmt.Sprintf("%d", darwinCFNumberValue(ref))
	case darwinCFBooleanGetTypeID != nil && typeID == darwinCFBooleanGetTypeID() && darwinCFBooleanGetValue != nil:
		if darwinCFBooleanGetValue(ref) {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func darwinElementBounds(element uintptr) (darwinRect, bool) {
	positionValue, err := darwinCopyAttributeValue(element, "AXPosition")
	if err != nil || positionValue == 0 {
		return darwinRect{}, false
	}
	defer darwinRelease(positionValue)

	sizeValue, err := darwinCopyAttributeValue(element, "AXSize")
	if err != nil || sizeValue == 0 {
		return darwinRect{}, false
	}
	defer darwinRelease(sizeValue)

	var point darwinPoint
	var size darwinSize
	if darwinAXValueGetValue == nil {
		return darwinRect{}, false
	}
	if !darwinAXValueGetValue(positionValue, darwinAXValueCGPointType, unsafe.Pointer(&point)) {
		return darwinRect{}, false
	}
	if !darwinAXValueGetValue(sizeValue, darwinAXValueCGSizeType, unsafe.Pointer(&size)) {
		return darwinRect{}, false
	}
	return darwinRect{Origin: point, Size: size}, true
}

func darwinRectsClose(a darwinRect, b darwinRect) bool {
	return darwinAbs(a.Origin.X-b.Origin.X) <= 3 &&
		darwinAbs(a.Origin.Y-b.Origin.Y) <= 3 &&
		darwinAbs(a.Size.Width-b.Size.Width) <= 3 &&
		darwinAbs(a.Size.Height-b.Size.Height) <= 3
}

func darwinDefaultActionLabel(actions []string) string {
	for _, action := range actions {
		switch strings.TrimSpace(strings.ToLower(action)) {
		case "axpress":
			return "press"
		case "axconfirm":
			return "confirm"
		case "axraise":
			return "raise"
		case "axshowmenu":
			return "show_menu"
		}
	}
	return ""
}

func darwinRoleLikelyInteractive(role string) bool {
	return containsAny(role, "button", "text_field", "text_area", "check_box", "checkbox", "switch", "slider", "link", "radio", "tab", "menu", "row", "cell", "scroll", "list")
}

func darwinFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func darwinRelease(ref uintptr) {
	if ref != 0 && darwinCFRelease != nil {
		darwinCFRelease(ref)
	}
}

func darwinAbs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
