//go:build darwin

package a11y

import (
	"context"
	"fmt"
	"strings"
)

func (b *darwinBackend) Capture(ctx context.Context, scope CaptureScope) (RawTree, error) {
	record, err := b.semanticCaptureWindowRecord(ctx, scope)
	if err != nil {
		return RawTree{}, err
	}
	if err := b.ensureAccessibilityPermission(); err != nil {
		return RawTree{}, err
	}
	if darwinAXUIElementCreateApplication == nil {
		return RawTree{}, NewError("backend_unavailable", "AX application lookup is unavailable", map[string]interface{}{"window_id": record.ID})
	}
	app := darwinAXUIElementCreateApplication(int32(record.PID))
	if app == 0 {
		return RawTree{}, NewError("backend_unavailable", "AX application lookup failed", map[string]interface{}{"window_id": record.ID})
	}
	defer darwinRelease(app)

	record, window := b.findSnapshotWindowElement(app, record)
	if window == 0 {
		return RawTree{}, NewError("backend_unavailable", "AX window lookup failed", darwinWindowRecordDiagnostics(record))
	}
	defer darwinRelease(window)

	b.prepareSnapshotElements(record.ID)
	defer b.finishSnapshotElementsBuild()
	defer func() {
		if r := recover(); r != nil {
			b.resetSnapshotElementsForWindow(record.ID)
			panic(r)
		}
	}()

	visited := 0
	root := b.buildSnapshotNode(window, 0, &visited)
	if root == nil {
		b.resetSnapshotElementsForWindow(record.ID)
		return RawTree{}, NewError("backend_unavailable", "AX snapshot is empty", map[string]interface{}{"window_id": record.ID})
	}
	return RawTree{
		WindowID: record.ID,
		Title:    record.Title,
		Mode:     "ax",
		Root:     root,
	}, nil
}

func (b *darwinBackend) Execute(ctx context.Context, windowID string, node SemanticNode, action Action) (ActionResult, error) {
	if err := b.ensureAccessibilityPermission(); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	token := strings.TrimSpace(node.BackendToken)
	if token == "" {
		return ActionResult{HostOS: b.HostOS()}, NewError("stale_ref", "semantic node has no backend token", map[string]interface{}{"node_id": node.StableID})
	}
	element, ok := b.lookupSnapshotElement(token)
	if !ok {
		return ActionResult{HostOS: b.HostOS()}, NewError("stale_ref", "semantic node token is no longer valid; capture a fresh tree first", map[string]interface{}{"node_id": node.StableID})
	}
	meta := darwinInspectActionMetadata(element)
	op := semanticNormalizeAction(action.Op)
	plan := planDarwinAction(op, meta)
	if plan.Unsupported {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", plan.UnsupportedReason, map[string]interface{}{"act_type": op})
	}
	if !plan.SetValue && plan.ExecutionMode != "semantic" {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", "semantic SDK does not allow input fallback for this action", map[string]interface{}{"act_type": op})
	}
	if plan.SetValue {
		if err := darwinSetStringAttribute(element, "AXValue", action.Value); err != nil {
			return ActionResult{HostOS: b.HostOS()}, err
		}
		b.UpdateSnapshotAfterAction(windowID, token, "type", action.Value)
		return ActionResult{
			HostOS:             b.HostOS(),
			WindowID:           strings.TrimSpace(windowID),
			ExecutionMode:      "semantic",
			TargetHit:          true,
			InputMethod:        "set_value",
			VerificationPassed: true,
			VerificationMethod: "ax_value",
			Message:            "Host action completed",
		}, nil
	}
	if strings.TrimSpace(plan.SemanticAction) == "" {
		return ActionResult{HostOS: b.HostOS()}, NewError("unsupported_action", "semantic action is unavailable", map[string]interface{}{"act_type": op})
	}
	if err := darwinPerformAXAction(element, plan.SemanticAction); err != nil {
		return ActionResult{HostOS: b.HostOS()}, err
	}
	b.UpdateSnapshotAfterAction(windowID, token, op, action.Value)
	return ActionResult{
		HostOS:             b.HostOS(),
		WindowID:           strings.TrimSpace(windowID),
		ExecutionMode:      "semantic",
		TargetHit:          true,
		VerificationPassed: true,
		VerificationMethod: "ax_action",
		Message:            "Host action completed",
	}, nil
}

func (b *darwinBackend) semanticCaptureWindowRecord(ctx context.Context, scope CaptureScope) (darwinWindowRecord, error) {
	if b == nil {
		return darwinWindowRecord{}, NewError("backend_unavailable", "darwin accessibility backend is unavailable", nil)
	}
	if windowID := strings.TrimSpace(scope.WindowID); windowID != "" {
		return b.resolveWindowRecord(windowID)
	}
	windows, err := b.listWindows(ctx)
	if err != nil {
		return darwinWindowRecord{}, err
	}
	appName := strings.TrimSpace(scope.AppName)
	var fallback WindowInfo
	for _, window := range windows {
		if appName != "" && !strings.EqualFold(strings.TrimSpace(window.AppName), appName) {
			continue
		}
		if fallback.ID == "" {
			fallback = window
		}
		if window.Focused {
			return b.resolveWindowRecord(window.ID)
		}
	}
	if strings.TrimSpace(fallback.ID) != "" {
		return b.resolveWindowRecord(fallback.ID)
	}
	return darwinWindowRecord{}, NewError("backend_unavailable", fmt.Sprintf("no capture window matched app %q", appName), nil)
}
