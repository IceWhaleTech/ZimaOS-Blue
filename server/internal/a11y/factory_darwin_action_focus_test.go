//go:build darwin

package a11y

import (
	"context"
	"testing"
)

func TestDarwinResolveWindowForAction_SkipsFocusWhenAlreadyFocused(t *testing.T) {
	prevResolve := darwinResolveWindowRecordForAction
	prevFocus := darwinFocusWindowForHostAction
	prevActivate := darwinActivateAppFunc
	t.Cleanup(func() {
		darwinResolveWindowRecordForAction = prevResolve
		darwinFocusWindowForHostAction = prevFocus
		darwinActivateAppFunc = prevActivate
	})

	darwinResolveWindowRecordForAction = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, Focused: true}, nil
	}
	activateCalls := 0
	darwinActivateAppFunc = func(string) error {
		activateCalls++
		return nil
	}
	focusCalls := 0
	darwinFocusWindowForHostAction = func(ctx context.Context, b *darwinBackend, windowID string) (ActionResult, error) {
		focusCalls++
		return ActionResult{HostOS: b.HostOS(), WindowID: windowID}, nil
	}

	b := &darwinBackend{}
	resolved, err := b.resolveWindowForAction(context.Background(), "win-1")
	if err != nil {
		t.Fatalf("resolveWindowForAction() error = %v", err)
	}
	if resolved != "win-1" {
		t.Fatalf("resolved = %q, want win-1", resolved)
	}
	if focusCalls != 0 {
		t.Fatalf("focusCalls = %d, want 0", focusCalls)
	}
	if activateCalls != 0 {
		t.Fatalf("activateCalls = %d, want 0", activateCalls)
	}
}

func TestDarwinResolveWindowForAction_ActivatesAppWhenNotFocused(t *testing.T) {
	prevResolve := darwinResolveWindowRecordForAction
	prevFocus := darwinFocusWindowForHostAction
	prevActivate := darwinActivateAppFunc
	t.Cleanup(func() {
		darwinResolveWindowRecordForAction = prevResolve
		darwinFocusWindowForHostAction = prevFocus
		darwinActivateAppFunc = prevActivate
	})

	darwinResolveWindowRecordForAction = func(_ *darwinBackend, windowID string) (darwinWindowRecord, error) {
		return darwinWindowRecord{ID: windowID, AppName: "Feishu", Focused: false}, nil
	}
	activateCalls := 0
	darwinActivateAppFunc = func(string) error {
		activateCalls++
		return nil
	}
	focusCalls := 0
	darwinFocusWindowForHostAction = func(ctx context.Context, b *darwinBackend, windowID string) (ActionResult, error) {
		focusCalls++
		return ActionResult{HostOS: b.HostOS(), WindowID: windowID}, nil
	}

	b := &darwinBackend{}
	resolved, err := b.resolveWindowForAction(context.Background(), "win-2")
	if err != nil {
		t.Fatalf("resolveWindowForAction() error = %v", err)
	}
	if resolved != "win-2" {
		t.Fatalf("resolved = %q, want win-2", resolved)
	}
	if focusCalls != 0 {
		t.Fatalf("focusCalls = %d, want 0", focusCalls)
	}
	if activateCalls != 1 {
		t.Fatalf("activateCalls = %d, want 1", activateCalls)
	}
}
