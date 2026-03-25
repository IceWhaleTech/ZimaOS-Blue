package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

func TestHybridCapabilityBrowserBackendPrefersLightpandaForReadOnlyNavigate(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})
	if !backend.lightpandaPreferredNavigate(ctx, "https://example.com", "") {
		t.Fatal("expected read-only navigate to prefer lightpanda")
	}

	visionCtx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
		Vision:         true,
		RequiresImage:  true,
	})
	if backend.lightpandaPreferredNavigate(visionCtx, "https://example.com", "") {
		t.Fatal("expected vision navigate to stay off lightpanda")
	}
}

func TestHybridCapabilityBrowserBackendKeepsLightpandaTargetOnUnsupportedScreenshot(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Lightpanda</title></head><body><h1>Read only</h1><a href="/next">Next</a></body></html>`))
	}))
	defer pageServer.Close()

	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	lp := browser.NewLightpandaService(cfg)
	nav, err := lp.Navigate(context.Background(), &browser.NavigateRequest{URL: pageServer.URL})
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, lp, nil, nil, nil)
	if got := backend.targetEngine(context.Background(), nav.TargetID); got != browser.SessionEngineLightpanda {
		t.Fatalf("targetEngine() = %q, want %q", got, browser.SessionEngineLightpanda)
	}
	if _, err := backend.ScreenshotTab(context.Background(), nav.TargetID); !errors.Is(err, browser.ErrLightpandaUnsupportedCapability) {
		t.Fatalf("ScreenshotTab() error = %v, want unsupported capability", err)
	}
}

func TestHybridCapabilityBrowserBackendKeepsExistingLightpandaSessionOnTargetAffinity(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/first":
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>First</title></head><body><main><h1>First</h1></main></body></html>`))
		case "/second":
			_, _ = w.Write([]byte(`<!doctype html><html><head><title>Second</title></head><body><main><h1>Second</h1></main></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer pageServer.Close()

	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	lp := browser.NewLightpandaService(cfg)
	backend := NewHybridCapabilityBrowserBackend(cfg, lp, nil, nil, nil)

	createCtx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})
	firstNav, err := backend.Navigate(createCtx, pageServer.URL+"/first", "")
	if err != nil {
		t.Fatalf("first Navigate() error = %v", err)
	}
	if got := backend.targetEngine(context.Background(), firstNav.TargetID); got != browser.SessionEngineLightpanda {
		t.Fatalf("targetEngine() = %q, want %q", got, browser.SessionEngineLightpanda)
	}

	affinityCtx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
		Vision:         true,
		RequiresImage:  true,
	})
	secondNav, err := backend.Navigate(affinityCtx, pageServer.URL+"/second", firstNav.TargetID)
	if err != nil {
		t.Fatalf("second Navigate() error = %v", err)
	}
	if secondNav.TargetID != firstNav.TargetID {
		t.Fatalf("TargetID drifted from %q to %q", firstNav.TargetID, secondNav.TargetID)
	}
	if got := backend.targetEngine(context.Background(), secondNav.TargetID); got != browser.SessionEngineLightpanda {
		t.Fatalf("targetEngine() after affinity navigate = %q, want %q", got, browser.SessionEngineLightpanda)
	}
}

func TestHybridCapabilityBrowserBackendRecognizesEscalationErrors(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil)
	for _, err := range []error{
		browser.ErrLightpandaUnsupportedCapability,
		browser.ErrLightpandaDOMUnstable,
		browser.ErrLightpandaEmptyTree,
		browser.ErrLightpandaNavigationBlocked,
		browser.ErrLightpandaJSRequired,
	} {
		if !backend.isLightpandaEscalationError(err) {
			t.Fatalf("isLightpandaEscalationError(%v) = false, want true", err)
		}
	}
}
