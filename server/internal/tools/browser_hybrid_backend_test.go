package tools

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
)

type stubSessionBrowserBackend struct {
	startCalls int

	navigateFn          func(ctx context.Context, url string, targetID string) (BrowserNavResult, error)
	tabsFn              func(ctx context.Context) ([]BrowserTabResult, error)
	a11yFn              func(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error)
	interactiveFn       func(ctx context.Context, targetID string) (BrowserInteractiveResult, error)
	countInteractiveFn  func(ctx context.Context, targetID string) (int, error)
	closeFn             func(ctx context.Context, targetID string) error
	listSessionInfosFn  func(ctx context.Context) ([]browser.SessionInfo, error)
	getSessionInfoFn    func(ctx context.Context, targetID string) (*browser.SessionInfo, error)
	captureMonitorFn    func(ctx context.Context, targetID string) (*browser.SessionMonitorResponse, error)
	captureScreenshotFn func(ctx context.Context, targetID string) (*browser.SessionScreenshotResponse, error)
}

func (s *stubSessionBrowserBackend) Start(context.Context) error {
	s.startCalls++
	return nil
}

func (s *stubSessionBrowserBackend) Navigate(ctx context.Context, url string, targetID string) (BrowserNavResult, error) {
	if s.navigateFn == nil {
		return BrowserNavResult{}, browser.ErrBrowserNotAvailable
	}
	return s.navigateFn(ctx, url, targetID)
}

func (*stubSessionBrowserBackend) CookieHeader(context.Context, string, string) (string, error) {
	return "", browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) ObserveNetwork(context.Context, string, int, bool) (BrowserObservedNetworkResult, error) {
	return BrowserObservedNetworkResult{}, browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) WaitNetworkIdle(context.Context, string, int, int) error {
	return browser.ErrLightpandaUnsupportedCapability
}

func (s *stubSessionBrowserBackend) AccessibilityTree(ctx context.Context, targetID string, maxDepth int) (BrowserA11yTreeResult, error) {
	if s.a11yFn == nil {
		return BrowserA11yTreeResult{}, browser.ErrTabNotFound
	}
	return s.a11yFn(ctx, targetID, maxDepth)
}

func (s *stubSessionBrowserBackend) InteractiveElements(ctx context.Context, targetID string) (BrowserInteractiveResult, error) {
	if s.interactiveFn == nil {
		return BrowserInteractiveResult{}, browser.ErrTabNotFound
	}
	return s.interactiveFn(ctx, targetID)
}

func (s *stubSessionBrowserBackend) CountInteractiveElements(ctx context.Context, targetID string) (int, error) {
	if s.countInteractiveFn == nil {
		return 0, browser.ErrTabNotFound
	}
	return s.countInteractiveFn(ctx, targetID)
}

func (*stubSessionBrowserBackend) ActByRef(context.Context, string, int, map[int]int, string, string) error {
	return browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) ActByInteractiveRef(context.Context, string, int, map[int]string, string, string) error {
	return browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) Screenshot(context.Context, string) (string, error) {
	return "", browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) ScreenshotTab(context.Context, string) (string, error) {
	return "", browser.ErrLightpandaUnsupportedCapability
}

func (s *stubSessionBrowserBackend) CloseTab(ctx context.Context, targetID string) error {
	if s.closeFn == nil {
		return nil
	}
	return s.closeFn(ctx, targetID)
}

func (s *stubSessionBrowserBackend) Tabs(ctx context.Context) ([]BrowserTabResult, error) {
	if s.tabsFn == nil {
		return nil, nil
	}
	return s.tabsFn(ctx)
}

func (*stubSessionBrowserBackend) ExecuteRecipe(context.Context, string, map[string]string) (BrowserRecipeResult, error) {
	return BrowserRecipeResult{}, browser.ErrLightpandaUnsupportedCapability
}

func (*stubSessionBrowserBackend) ListRecipes(context.Context) []BrowserRecipeInfo {
	return nil
}

func (s *stubSessionBrowserBackend) ListSessionInfos(ctx context.Context) ([]browser.SessionInfo, error) {
	if s.listSessionInfosFn == nil {
		return nil, nil
	}
	return s.listSessionInfosFn(ctx)
}

func (s *stubSessionBrowserBackend) GetSessionInfo(ctx context.Context, targetID string) (*browser.SessionInfo, error) {
	if s.getSessionInfoFn == nil {
		return nil, browser.ErrTabNotFound
	}
	return s.getSessionInfoFn(ctx, targetID)
}

func (s *stubSessionBrowserBackend) CaptureSessionMonitor(ctx context.Context, targetID string) (*browser.SessionMonitorResponse, error) {
	if s.captureMonitorFn == nil {
		return nil, browser.ErrTabNotFound
	}
	return s.captureMonitorFn(ctx, targetID)
}

func (s *stubSessionBrowserBackend) CaptureSessionScreenshot(ctx context.Context, targetID string) (*browser.SessionScreenshotResponse, error) {
	if s.captureScreenshotFn == nil {
		return nil, browser.ErrTabNotFound
	}
	return s.captureScreenshotFn(ctx, targetID)
}

func makeHybridLightpandaConfig(t *testing.T) *browser.Config {
	t.Helper()
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true
	binaryPath := filepath.Join(t.TempDir(), "lightpanda")
	if err := os.WriteFile(binaryPath, []byte("stub"), 0o755); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", binaryPath, err)
	}
	cfg.Lightpanda.BinaryPath = binaryPath
	return cfg
}

func TestHybridCapabilityBrowserBackendPrefersLightpandaForReadOnlyNavigate(t *testing.T) {
	cfg := makeHybridLightpandaConfig(t)

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil, nil, nil)
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

func TestHybridCapabilityBrowserBackendKeepsGitHubOffLightpanda(t *testing.T) {
	cfg := makeHybridLightpandaConfig(t)

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})
	if backend.lightpandaPreferredNavigate(ctx, "https://github.com/search?q=openclaw&type=repositories", "") {
		t.Fatal("expected github.com read-only navigate to stay on a full browser engine")
	}
}

func TestHybridCapabilityBrowserBackendPrefersWarmManagedChromiumOverLightpanda(t *testing.T) {
	cfg := makeHybridLightpandaConfig(t)

	managedInfo := browser.SessionInfo{
		ID:           "chrome-managed-1",
		Status:       "active",
		CurrentURL:   "https://example.com/already-open",
		PageTitle:    "Warm Chromium",
		CreatedAt:    "2026-03-26T12:00:00Z",
		LastActivity: "2026-03-26T12:05:00Z",
		Engine:       browser.SessionEngineChromiumManaged,
		EngineDetail: browser.SessionEngineDetailChromiumManaged,
		SessionLayer: browser.SessionLayerFullBrowser,
		MonitorKind:  browser.SessionMonitorKindImage,
	}
	managed := &stubSessionBrowserBackend{
		navigateFn: func(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
			if targetID != managedInfo.ID {
				t.Fatalf("Navigate() target = %q, want warm managed target %q", targetID, managedInfo.ID)
			}
			return BrowserNavResult{URL: url, Title: "Warm Chromium", TargetID: managedInfo.ID}, nil
		},
		listSessionInfosFn: func(context.Context) ([]browser.SessionInfo, error) {
			return []browser.SessionInfo{managedInfo}, nil
		},
		getSessionInfoFn: func(_ context.Context, targetID string) (*browser.SessionInfo, error) {
			if targetID != managedInfo.ID {
				return nil, browser.ErrTabNotFound
			}
			info := managedInfo
			return &info, nil
		},
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, managed, nil, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})

	if backend.lightpandaPreferredNavigate(ctx, "https://example.com", "") {
		t.Fatal("expected warm managed Chromium to suppress lightpanda-first routing")
	}

	nav, err := backend.Navigate(ctx, "https://example.com", "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}
	if nav.TargetID != managedInfo.ID {
		t.Fatalf("Navigate() target = %q, want %q", nav.TargetID, managedInfo.ID)
	}
	if got := backend.targetDetail(context.Background(), nav.TargetID); got != browser.SessionEngineDetailChromiumManaged {
		t.Fatalf("targetDetail() = %q, want %q", got, browser.SessionEngineDetailChromiumManaged)
	}
}

func TestHybridCapabilityBrowserBackendPrefersWarmRelayChromiumOverColdManaged(t *testing.T) {
	cfg := makeHybridLightpandaConfig(t)

	relayInfo := browser.SessionInfo{
		ID:           "chrome-relay-1",
		Status:       "active",
		CurrentURL:   "https://example.com/already-open",
		PageTitle:    "Warm Relay",
		CreatedAt:    "2026-03-26T12:00:00Z",
		LastActivity: "2026-03-26T12:05:00Z",
		Engine:       browser.SessionEngineChromiumRelay,
		EngineDetail: browser.SessionEngineDetailChromiumRelay,
		SessionLayer: browser.SessionLayerFullBrowser,
		MonitorKind:  browser.SessionMonitorKindImage,
	}
	managed := &stubSessionBrowserBackend{
		navigateFn: func(context.Context, string, string) (BrowserNavResult, error) {
			t.Fatal("managed chromium should not be selected when relay already has a warm session")
			return BrowserNavResult{}, nil
		},
	}
	relay := &stubSessionBrowserBackend{
		navigateFn: func(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
			if targetID != relayInfo.ID {
				t.Fatalf("Navigate() target = %q, want warm relay target %q", targetID, relayInfo.ID)
			}
			return BrowserNavResult{URL: url, Title: "Warm Relay", TargetID: relayInfo.ID}, nil
		},
		listSessionInfosFn: func(context.Context) ([]browser.SessionInfo, error) {
			return []browser.SessionInfo{relayInfo}, nil
		},
		getSessionInfoFn: func(_ context.Context, targetID string) (*browser.SessionInfo, error) {
			if targetID != relayInfo.ID {
				return nil, browser.ErrTabNotFound
			}
			info := relayInfo
			return &info, nil
		},
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, managed, relay, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})

	nav, err := backend.Navigate(ctx, "https://example.com", "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}
	if nav.TargetID != relayInfo.ID {
		t.Fatalf("Navigate() target = %q, want %q", nav.TargetID, relayInfo.ID)
	}
	if !backend.UsesRelayFor(ctx, "", "https://example.com") {
		t.Fatal("expected UsesRelayFor() to report relay when a warm relay Chromium session is reused")
	}
}

func TestHybridCapabilityBrowserBackendNavigateUsesExplicitExecutionPlanForNewTarget(t *testing.T) {
	cfg := makeHybridLightpandaConfig(t)

	managedCalls := 0
	relayCalls := 0
	managed := &stubSessionBrowserBackend{
		navigateFn: func(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
			managedCalls++
			return BrowserNavResult{URL: url, Title: "Managed", TargetID: "managed-tab"}, nil
		},
	}
	relay := &stubSessionBrowserBackend{
		navigateFn: func(_ context.Context, url string, targetID string) (BrowserNavResult, error) {
			relayCalls++
			return BrowserNavResult{URL: url, Title: "Relay", TargetID: "relay-tab"}, nil
		},
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, managed, relay, nil, func(context.Context) bool { return true })
	ctx := WithWebExecutionPlan(context.Background(), ExecutionPlan{
		Kind:           WebTaskKindOperate,
		PrimaryRuntime: string(browser.SessionEngineDetailChromiumRelay),
		Steps: []PlanStep{{
			Kind:    WebTaskKindOperate,
			Runtime: string(browser.SessionEngineDetailChromiumRelay),
			Reason:  "planner selected relay runtime",
		}},
	})

	nav, err := backend.Navigate(ctx, "https://example.com/explicit-plan", "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}
	if nav.TargetID != "relay-tab" {
		t.Fatalf("target = %q, want relay-tab", nav.TargetID)
	}
	if relayCalls != 1 {
		t.Fatalf("relay navigate calls = %d, want 1", relayCalls)
	}
	if managedCalls != 0 {
		t.Fatalf("managed navigate calls = %d, want 0", managedCalls)
	}
}

func TestHybridCapabilityBrowserBackendKeepsLightpandaTargetOnUnsupportedScreenshot(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Lightpanda</title></head><body><h1>Read only</h1><a href="/next">Next</a></body></html>`))
	}))
	defer pageServer.Close()

	cfg := makeHybridLightpandaConfig(t)

	lp := browser.NewLightpandaService(cfg)
	nav, err := lp.Navigate(context.Background(), &browser.NavigateRequest{URL: pageServer.URL})
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, lp, nil, nil, nil, nil, nil)
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

	cfg := makeHybridLightpandaConfig(t)

	lp := browser.NewLightpandaService(cfg)
	backend := NewHybridCapabilityBrowserBackend(cfg, lp, nil, nil, nil, nil, nil)

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
	cfg := makeHybridLightpandaConfig(t)

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil, nil, nil)
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

func TestHybridCapabilityBrowserBackendAllowsLightpandaShimWithoutReadyBinary(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), nil, nil, nil, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})
	if !backend.lightpandaPreferredNavigate(ctx, "https://example.com", "") {
		t.Fatal("expected read-layer Lightpanda shim to stay available even before the browser-lite binary is ready")
	}
}

func TestHybridCapabilityBrowserBackendExposesReadSessionMetadata(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Docs</title></head><body><main><h1>Read Layer</h1></main></body></html>`))
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

	backend := NewHybridCapabilityBrowserBackend(cfg, lp, nil, nil, nil, nil, nil)
	info, err := backend.GetBrowserSession(context.Background(), nav.TargetID)
	if err != nil {
		t.Fatalf("GetBrowserSession() error = %v", err)
	}
	if info.Engine != browser.SessionEngineLightpanda {
		t.Fatalf("Engine = %q, want %q", info.Engine, browser.SessionEngineLightpanda)
	}
	if info.EngineDetail != browser.SessionEngineDetailLightpandaShim {
		t.Fatalf("EngineDetail = %q, want %q", info.EngineDetail, browser.SessionEngineDetailLightpandaShim)
	}
	if info.SessionLayer != browser.SessionLayerRead {
		t.Fatalf("SessionLayer = %q, want %q", info.SessionLayer, browser.SessionLayerRead)
	}
}

func TestTabToSessionInfoExposesFullBrowserMetadata(t *testing.T) {
	now := time.Date(2026, 3, 25, 12, 0, 0, 0, time.UTC)
	info := tabToSessionInfo(&browser.Tab{
		TargetID: "tab-1",
		URL:      "https://example.com",
		Title:    "Example",
		Active:   true,
	}, browser.SessionEngineChromiumRelay, now)
	if info.EngineDetail != browser.SessionEngineDetailChromiumRelay {
		t.Fatalf("EngineDetail = %q, want %q", info.EngineDetail, browser.SessionEngineDetailChromiumRelay)
	}
	if info.SessionLayer != browser.SessionLayerFullBrowser {
		t.Fatalf("SessionLayer = %q, want %q", info.SessionLayer, browser.SessionLayerFullBrowser)
	}
}

func TestHybridCapabilityBrowserBackendEscalatesShimToBrowserLiteBeforeChromium(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><body><div id="root"></div><script>1</script><script>2</script><script>3</script></body></html>`))
	}))
	defer pageServer.Close()

	cfg := makeHybridLightpandaConfig(t)
	lp := browser.NewLightpandaService(cfg)

	browserLiteInfo := browser.SessionInfo{
		ID:           "lp-binary-1",
		Status:       "active",
		CurrentURL:   pageServer.URL,
		PageTitle:    "Binary Docs",
		CreatedAt:    "2026-03-26T12:00:00Z",
		LastActivity: "2026-03-26T12:05:00Z",
		Engine:       browser.SessionEngineLightpanda,
		EngineDetail: browser.SessionEngineDetailLightpandaBinary,
		SessionLayer: browser.SessionLayerBrowserLite,
		MonitorKind:  browser.SessionMonitorKindText,
	}
	browserLite := &stubSessionBrowserBackend{
		navigateFn: func(context.Context, string, string) (BrowserNavResult, error) {
			return BrowserNavResult{URL: pageServer.URL, Title: "Binary Docs", TargetID: "lp-binary-1"}, nil
		},
		getSessionInfoFn: func(context.Context, string) (*browser.SessionInfo, error) {
			info := browserLiteInfo
			return &info, nil
		},
	}
	chromium := &stubSessionBrowserBackend{
		navigateFn: func(context.Context, string, string) (BrowserNavResult, error) {
			return BrowserNavResult{URL: pageServer.URL, Title: "Chromium", TargetID: "chrome-1"}, nil
		},
	}

	backend := NewHybridCapabilityBrowserBackend(cfg, lp, browserLite, chromium, nil, nil, nil)
	ctx := WithBrowserRouteHint(context.Background(), BrowserRouteHint{
		Action:         "navigate",
		FollowupAction: "snapshot_auto",
	})
	nav, err := backend.Navigate(ctx, pageServer.URL, "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}
	if nav.TargetID != "lp-binary-1" {
		t.Fatalf("Navigate() target = %q, want %q", nav.TargetID, "lp-binary-1")
	}
	if got := backend.targetDetail(context.Background(), nav.TargetID); got != browser.SessionEngineDetailLightpandaBinary {
		t.Fatalf("targetDetail() = %q, want %q", got, browser.SessionEngineDetailLightpandaBinary)
	}
	if chromium.startCalls != 0 {
		t.Fatalf("chromium start calls = %d, want 0", chromium.startCalls)
	}
}

func TestHybridCapabilityBrowserBackendExposesBrowserLiteSessionMetadataAndMonitor(t *testing.T) {
	info := browser.SessionInfo{
		ID:           "lp-binary-1",
		Status:       "active",
		CurrentURL:   "https://example.com/docs",
		PageTitle:    "Docs",
		CreatedAt:    "2026-03-26T12:00:00Z",
		LastActivity: "2026-03-26T12:05:00Z",
		Engine:       browser.SessionEngineLightpanda,
		EngineDetail: browser.SessionEngineDetailLightpandaBinary,
		SessionLayer: browser.SessionLayerBrowserLite,
		MonitorKind:  browser.SessionMonitorKindText,
	}
	monitor := &browser.SessionMonitorResponse{
		Kind: browser.SessionMonitorKindText,
		Text: &browser.SessionTextMonitor{
			Title:            "Docs",
			URL:              "https://example.com/docs",
			TreePreview:      "[document] \"Docs\"",
			InteractiveCount: 3,
			UpdatedAt:        "2026-03-26T12:05:00Z",
			Status:           "active",
		},
	}
	screenshot := &browser.SessionScreenshotResponse{
		Error: "unsupported",
	}
	browserLite := &stubSessionBrowserBackend{
		listSessionInfosFn: func(context.Context) ([]browser.SessionInfo, error) {
			return []browser.SessionInfo{info}, nil
		},
		getSessionInfoFn: func(context.Context, string) (*browser.SessionInfo, error) {
			copy := info
			return &copy, nil
		},
		captureMonitorFn: func(context.Context, string) (*browser.SessionMonitorResponse, error) {
			return monitor, nil
		},
		captureScreenshotFn: func(context.Context, string) (*browser.SessionScreenshotResponse, error) {
			return screenshot, nil
		},
	}

	cfg := makeHybridLightpandaConfig(t)
	backend := NewHybridCapabilityBrowserBackend(cfg, browser.NewLightpandaService(cfg), browserLite, nil, nil, nil, nil)

	session, err := backend.GetBrowserSession(context.Background(), "lp-binary-1")
	if err != nil {
		t.Fatalf("GetBrowserSession() error = %v", err)
	}
	if session.EngineDetail != browser.SessionEngineDetailLightpandaBinary {
		t.Fatalf("EngineDetail = %q, want %q", session.EngineDetail, browser.SessionEngineDetailLightpandaBinary)
	}
	if session.SessionLayer != browser.SessionLayerBrowserLite {
		t.Fatalf("SessionLayer = %q, want %q", session.SessionLayer, browser.SessionLayerBrowserLite)
	}
	if session.MonitorKind != browser.SessionMonitorKindText {
		t.Fatalf("MonitorKind = %q, want %q", session.MonitorKind, browser.SessionMonitorKindText)
	}

	monitorPayload, err := backend.CaptureBrowserSessionMonitor(context.Background(), "lp-binary-1")
	if err != nil {
		t.Fatalf("CaptureBrowserSessionMonitor() error = %v", err)
	}
	if monitorPayload.Kind != browser.SessionMonitorKindText {
		t.Fatalf("monitor kind = %q, want %q", monitorPayload.Kind, browser.SessionMonitorKindText)
	}

	screenshotPayload, err := backend.CaptureBrowserSessionScreenshot(context.Background(), "lp-binary-1")
	if err != nil {
		t.Fatalf("CaptureBrowserSessionScreenshot() error = %v", err)
	}
	if screenshotPayload.Error == "" {
		t.Fatal("expected browser-lite screenshot payload to explain unsupported capture")
	}
}

func TestHybridCapabilityBrowserBackendUsesManagedWhenRelayIsNotReady(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = false

	managed := &RodBrowserBackend{}
	relay := &RodBrowserBackend{}
	backend := NewHybridCapabilityBrowserBackend(
		cfg,
		nil,
		nil,
		managed,
		relay,
		nil,
		func(context.Context) bool { return false },
	)
	if backend.UsesRelayFor(context.Background(), "", "https://example.com") {
		t.Fatal("expected managed Chromium when relay/local Chrome is unavailable")
	}
}

func TestHybridCapabilityBrowserBackendUsesRelayWhenReady(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.Strategy = browser.BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = false

	managed := &RodBrowserBackend{}
	relay := &RodBrowserBackend{}
	backend := NewHybridCapabilityBrowserBackend(
		cfg,
		nil,
		nil,
		managed,
		relay,
		nil,
		func(context.Context) bool { return true },
	)
	if !backend.UsesRelayFor(context.Background(), "", "https://example.com") {
		t.Fatal("expected relay/local Chrome when it is actually reachable")
	}
}
