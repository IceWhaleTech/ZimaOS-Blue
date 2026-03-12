package tools

import (
	"context"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

func TestRodBrowserBackendGetForTargetUsesVisibleModeHint(t *testing.T) {
	defaultSvc := &browser.RodService{}
	visibleSvc := &browser.RodService{}
	backend := NewModeAwareRodBrowserBackend(func() *browser.RodService { return defaultSvc }, func() *browser.RodService { return visibleSvc })

	ctx := WithBrowserLaunchMode(context.Background(), BrowserLaunchModeVisible)
	got, err := backend.getForTarget(ctx, "")
	if err != nil {
		t.Fatalf("getForTarget() error = %v", err)
	}
	if got != visibleSvc {
		t.Fatalf("getForTarget() = %p, want visible %p", got, visibleSvc)
	}
}

func TestRodBrowserBackendGetForTargetFallsBackToDefaultWhenVisibleUnavailable(t *testing.T) {
	defaultSvc := &browser.RodService{}
	backend := NewModeAwareRodBrowserBackend(func() *browser.RodService { return defaultSvc }, nil)

	ctx := WithBrowserLaunchMode(context.Background(), BrowserLaunchModeVisible)
	got, err := backend.getForTarget(ctx, "")
	if err != nil {
		t.Fatalf("getForTarget() error = %v", err)
	}
	if got != defaultSvc {
		t.Fatalf("getForTarget() = %p, want default %p", got, defaultSvc)
	}
}
