//go:build darwin

package a11y

import (
	"context"
	"sync"
)

type darwinBackend struct {
	mediaDir string
	mu       sync.Mutex
	nextRef  int
	elements map[string]darwinElementRef
}

func DefaultHostBackend(mediaDir string) Backend {
	return &darwinBackend{
		mediaDir: mediaDir,
		elements: make(map[string]darwinElementRef),
	}
}

func (b *darwinBackend) HostOS() string { return "darwin" }

func (b *darwinBackend) Capabilities(context.Context) (CapabilitiesResult, error) {
	granted := darwinAccessibilityGrantedProbe()
	message := ""
	permissionMessage := ""
	if !granted {
		permissionMessage = darwinAccessibilityPermissionMessage()
		message = permissionMessage
	}
	return CapabilitiesResult{
		HostOS:           b.HostOS(),
		SupportedActions: append([]string(nil), SupportedActions...),
		Permissions: []PermissionStatus{
			{Name: "accessibility", Granted: granted, Required: true, Message: permissionMessage},
		},
		Message: message,
	}, nil
}

func (b *darwinBackend) ListWindows(ctx context.Context) ([]WindowInfo, error) {
	return b.listWindows(ctx)
}

func (b *darwinBackend) FocusWindow(ctx context.Context, windowID string) (ActionResult, error) {
	return b.focusWindow(ctx, windowID)
}

func (b *darwinBackend) Snapshot(ctx context.Context, windowID string) (SnapshotResult, error) {
	return b.snapshot(ctx, windowID, false)
}

func (b *darwinBackend) SnapshotInteractive(ctx context.Context, windowID string) (SnapshotResult, error) {
	return b.snapshot(ctx, windowID, true)
}

func (b *darwinBackend) Act(ctx context.Context, windowID string, ref int, refMap map[int]string, actType string, value string, holdMS int) (ActionResult, error) {
	return b.act(ctx, windowID, ref, refMap, actType, value, holdMS)
}

func (b *darwinBackend) Scroll(ctx context.Context, windowID string, direction string, lines int) (ActionResult, error) {
	return b.scroll(ctx, windowID, direction, lines)
}

func (b *darwinBackend) PointerMove(ctx context.Context, x int, y int) (ActionResult, error) {
	return b.pointerMove(ctx, x, y)
}

func (b *darwinBackend) Key(ctx context.Context, windowID string, keys []string, holdMS int) (ActionResult, error) {
	return b.key(ctx, windowID, keys, holdMS)
}

func (b *darwinBackend) Screenshot(ctx context.Context, windowID string) (ScreenshotResult, error) {
	return b.screenshot(ctx, windowID)
}
