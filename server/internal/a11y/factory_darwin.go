//go:build darwin

package a11y

import (
	"context"
	"sync"
)

type darwinBackend struct {
	mediaDir       string
	mu             sync.Mutex
	nextRef        int
	snapshotWindow string
	elements       map[string]darwinElementRef
	snapshots      *SnapshotStore
}

func DefaultHostBackend(mediaDir string) Backend {
	return &darwinBackend{
		mediaDir:  mediaDir,
		elements:  make(map[string]darwinElementRef),
		snapshots: NewSnapshotStore(),
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

func (b *darwinBackend) ListAllWindows(ctx context.Context) ([]WindowInfo, error) {
	return b.listAllWindows(ctx)
}

func (b *darwinBackend) FocusWindow(ctx context.Context, windowID string) (ActionResult, error) {
	return b.focusWindow(ctx, windowID)
}

func (b *darwinBackend) ActivateApp(_ context.Context, appName string) (ActionResult, error) {
	if err := darwinActivateApp(appName); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{
		HostOS:        b.HostOS(),
		ExecutionMode: "automation",
		Message:       "Application activated",
	}, nil
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

func (b *darwinBackend) ClickWindowPoint(ctx context.Context, windowID string, point NormalizedPoint, holdMS int) (ActionResult, error) {
	return b.clickWindowPoint(ctx, windowID, point, holdMS)
}

func (b *darwinBackend) ClickWindowPixel(ctx context.Context, windowID string, x int, y int, holdMS int) (ActionResult, error) {
	return b.clickWindowPixel(ctx, windowID, x, y, holdMS)
}

func (b *darwinBackend) Key(ctx context.Context, windowID string, keys []string, holdMS int) (ActionResult, error) {
	return b.key(ctx, windowID, keys, holdMS)
}

func (b *darwinBackend) TypeFocusedText(ctx context.Context, windowID string, value string, holdMS int) (ActionResult, error) {
	return b.typeFocusedText(ctx, windowID, value, holdMS)
}

func (b *darwinBackend) Screenshot(ctx context.Context, windowID string) (ScreenshotResult, error) {
	return b.screenshot(ctx, windowID)
}

func (b *darwinBackend) ScreenshotForGrounding(ctx context.Context, windowID string) (ScreenshotResult, error) {
	return b.screenshotForGrounding(ctx, windowID)
}
