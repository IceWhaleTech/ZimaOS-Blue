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
	lease, err := backend.acquireForTarget(ctx, "")
	if err != nil {
		t.Fatalf("acquireForTarget() error = %v", err)
	}
	defer lease.close()
	if lease.svc != visibleSvc {
		t.Fatalf("acquireForTarget() = %p, want visible %p", lease.svc, visibleSvc)
	}
}

func TestRodBrowserBackendGetForTargetFallsBackToDefaultWhenVisibleUnavailable(t *testing.T) {
	defaultSvc := &browser.RodService{}
	backend := NewModeAwareRodBrowserBackend(func() *browser.RodService { return defaultSvc }, nil)

	ctx := WithBrowserLaunchMode(context.Background(), BrowserLaunchModeVisible)
	lease, err := backend.acquireForTarget(ctx, "")
	if err != nil {
		t.Fatalf("acquireForTarget() error = %v", err)
	}
	defer lease.close()
	if lease.svc != defaultSvc {
		t.Fatalf("acquireForTarget() = %p, want default %p", lease.svc, defaultSvc)
	}
}

func TestLazyRodBrowserBackendDoesNotCacheResolvedService(t *testing.T) {
	svcs := []*browser.RodService{{}, {}}
	resolveCalls := 0
	backend := NewLazyRodBrowserBackend(func() *browser.RodService {
		defer func() { resolveCalls++ }()
		return svcs[resolveCalls]
	})

	first, err := backend.acquireDefault()
	if err != nil {
		t.Fatalf("acquireDefault() error = %v", err)
	}
	defer first.close()
	if first.svc != svcs[0] {
		t.Fatalf("first acquireDefault() = %p, want %p", first.svc, svcs[0])
	}

	second, err := backend.acquireDefault()
	if err != nil {
		t.Fatalf("acquireDefault() second error = %v", err)
	}
	defer second.close()
	if second.svc != svcs[1] {
		t.Fatalf("second acquireDefault() = %p, want %p", second.svc, svcs[1])
	}
}

func TestLeaseAwareRodBrowserBackendReleasesUnusedService(t *testing.T) {
	defaultSvc := &browser.RodService{}
	visibleSvc := &browser.RodService{}
	defaultReleases := 0
	visibleReleases := 0

	backend := NewLeaseAwareRodBrowserBackend(
		func() (*browser.RodService, func(), error) {
			return defaultSvc, func() { defaultReleases++ }, nil
		},
		func() (*browser.RodService, func(), error) {
			return visibleSvc, func() { visibleReleases++ }, nil
		},
	)

	ctx := WithBrowserLaunchMode(context.Background(), BrowserLaunchModeVisible)
	lease, err := backend.acquireForTarget(ctx, "")
	if err != nil {
		t.Fatalf("acquireForTarget() error = %v", err)
	}
	if lease.svc != visibleSvc {
		t.Fatalf("acquireForTarget() = %p, want visible %p", lease.svc, visibleSvc)
	}
	if defaultReleases != 1 {
		t.Fatalf("default release count = %d, want 1", defaultReleases)
	}
	if visibleReleases != 0 {
		t.Fatalf("visible release count before close = %d, want 0", visibleReleases)
	}

	lease.close()
	if visibleReleases != 1 {
		t.Fatalf("visible release count after close = %d, want 1", visibleReleases)
	}
}

func TestLeaseAwareRodBrowserBackendDoesNotWarmVisibleServiceForDefaultMode(t *testing.T) {
	defaultSvc := &browser.RodService{}
	defaultCalls := 0
	visibleCalls := 0

	backend := NewLeaseAwareRodBrowserBackend(
		func() (*browser.RodService, func(), error) {
			defaultCalls++
			return defaultSvc, func() {}, nil
		},
		func() (*browser.RodService, func(), error) {
			visibleCalls++
			return &browser.RodService{}, func() {}, nil
		},
	)

	lease, err := backend.acquireForTarget(context.Background(), "")
	if err != nil {
		t.Fatalf("acquireForTarget() error = %v", err)
	}
	defer lease.close()
	if lease.svc != defaultSvc {
		t.Fatalf("acquireForTarget() = %p, want default %p", lease.svc, defaultSvc)
	}
	if defaultCalls != 1 {
		t.Fatalf("default acquire count = %d, want 1", defaultCalls)
	}
	if visibleCalls != 0 {
		t.Fatalf("visible acquire count = %d, want 0", visibleCalls)
	}
}
