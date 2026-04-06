package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

type sessionAwareStubBrowserService struct {
	*stubBrowserService
	tabs             []*Tab
	screenshot       string
	history          []SessionScreenshot
	lastNavigateReq  *NavigateRequest
	lastScreenshotID string
}

func (s *sessionAwareStubBrowserService) Tabs(context.Context) ([]*Tab, error) {
	return s.tabs, nil
}

func (s *sessionAwareStubBrowserService) Navigate(
	_ context.Context,
	req *NavigateRequest,
) (*NavigateResponse, error) {
	copyReq := *req
	s.lastNavigateReq = &copyReq
	return &NavigateResponse{
		URL:      req.URL,
		Title:    "Example",
		TargetID: req.TargetID,
	}, nil
}

func (s *sessionAwareStubBrowserService) ScreenshotViewport(
	_ context.Context,
	targetID string,
) (string, error) {
	s.lastScreenshotID = targetID
	return s.screenshot, nil
}

func (s *sessionAwareStubBrowserService) SessionScreenshotHistory(
	targetID string,
) []SessionScreenshot {
	s.lastScreenshotID = targetID
	if len(s.history) == 0 {
		return nil
	}
	out := make([]SessionScreenshot, len(s.history))
	copy(out, s.history)
	return out
}

type stubSessionRouteProvider struct {
	sessions           []SessionInfo
	monitor            *SessionMonitorResponse
	screenshot         *SessionScreenshotResponse
	lastNavigateID     string
	lastNavigateURL    string
	lastScreenshotID   string
	lastMonitorID      string
	lastGetSessionID   string
	lastCloseSessionID string
}

func (s *stubSessionRouteProvider) ListBrowserSessions(context.Context) ([]SessionInfo, error) {
	return s.sessions, nil
}

func (s *stubSessionRouteProvider) CreateBrowserSession(context.Context) (*SessionInfo, error) {
	if len(s.sessions) == 0 {
		return nil, nil
	}
	session := s.sessions[0]
	return &session, nil
}

func (s *stubSessionRouteProvider) GetBrowserSession(_ context.Context, id string) (*SessionInfo, error) {
	s.lastGetSessionID = id
	for i := range s.sessions {
		if s.sessions[i].ID == id {
			session := s.sessions[i]
			return &session, nil
		}
	}
	return nil, ErrTabNotFound
}

func (s *stubSessionRouteProvider) CloseBrowserSession(_ context.Context, id string) error {
	s.lastCloseSessionID = id
	return nil
}

func (s *stubSessionRouteProvider) NavigateBrowserSession(_ context.Context, id string, rawURL string) (*NavigateResponse, error) {
	s.lastNavigateID = id
	s.lastNavigateURL = rawURL
	return &NavigateResponse{URL: rawURL, Title: "Example", TargetID: id}, nil
}

func (s *stubSessionRouteProvider) CaptureBrowserSessionMonitor(_ context.Context, id string) (*SessionMonitorResponse, error) {
	s.lastMonitorID = id
	return s.monitor, nil
}

func (s *stubSessionRouteProvider) CaptureBrowserSessionScreenshot(_ context.Context, id string) (*SessionScreenshotResponse, error) {
	s.lastScreenshotID = id
	return s.screenshot, nil
}

type stubBrowserTaskProjectionService struct {
	list      func(ctx context.Context, query BrowserOverviewTaskQuery) ([]map[string]any, error)
	lastQuery BrowserOverviewTaskQuery
}

func (s *stubBrowserTaskProjectionService) List(
	ctx context.Context,
	query BrowserOverviewTaskQuery,
) ([]map[string]any, error) {
	s.lastQuery = query
	if s.list == nil {
		return nil, nil
	}
	return s.list(ctx, query)
}

func TestBrowserOverviewMapsTabsToSessions(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{running: true},
		tabs: []*Tab{
			{
				TargetID: "tab-1",
				URL:      "https://example.com",
				Title:    "Example",
				Active:   true,
			},
			{
				TargetID: "tab-2",
				URL:      "https://example.org",
				Title:    "Example Org",
			},
		},
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodGet, "/browser/overview", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload browserOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode overview failed: %v", err)
	}
	sessions := payload.Sessions
	if len(sessions) != 2 {
		t.Fatalf("len(sessions)=%d, want 2", len(sessions))
	}
	if sessions[0].ID != "tab-1" {
		t.Fatalf("sessions[0].ID=%q, want %q", sessions[0].ID, "tab-1")
	}
	if sessions[0].Status != "active" {
		t.Fatalf("sessions[0].Status=%q, want %q", sessions[0].Status, "active")
	}
	if sessions[0].Engine != SessionEngineChromiumManaged {
		t.Fatalf("sessions[0].Engine=%q, want %q", sessions[0].Engine, SessionEngineChromiumManaged)
	}
	if sessions[0].MonitorKind != SessionMonitorKindImage {
		t.Fatalf("sessions[0].MonitorKind=%q, want %q", sessions[0].MonitorKind, SessionMonitorKindImage)
	}
	if sessions[1].Status != "idle" {
		t.Fatalf("sessions[1].Status=%q, want %q", sessions[1].Status, "idle")
	}
}

func TestBrowserStandaloneSessionReadRoutesRemoved(t *testing.T) {
	h := NewHandler(&stubBrowserService{running: true})
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	listRec := performBrowserJSONRequest(e, http.MethodGet, "/browser/sessions", "")
	if listRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /browser/sessions status=%d, want %d", listRec.Code, http.StatusMethodNotAllowed)
	}

	detailRec := performBrowserJSONRequest(e, http.MethodGet, "/browser/sessions/tab-1", "")
	if detailRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /browser/sessions/:id status=%d, want %d", detailRec.Code, http.StatusMethodNotAllowed)
	}
}

func TestBrowserSessionScreenshotUsesViewportCapture(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{running: true},
		screenshot:         "base64-png",
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/tab-42/screenshot", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload SessionScreenshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode screenshot failed: %v", err)
	}
	if payload.Screenshot != "base64-png" {
		t.Fatalf("screenshot=%q, want %q", payload.Screenshot, "base64-png")
	}
	if service.lastScreenshotID != "tab-42" {
		t.Fatalf("lastScreenshotID=%q, want %q", service.lastScreenshotID, "tab-42")
	}
}

func TestBrowserSessionScreenshotFallsBackToHistory(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{running: true},
		history: []SessionScreenshot{
			{
				Data:       "base64-history",
				CapturedAt: "2026-03-23T00:00:00Z",
				URL:        "https://example.com",
				Title:      "Example",
			},
		},
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/tab-42/screenshot", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload SessionScreenshotResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode screenshot failed: %v", err)
	}
	if payload.Screenshot != "base64-history" {
		t.Fatalf("screenshot=%q, want %q", payload.Screenshot, "base64-history")
	}
	if len(payload.History) != 1 {
		t.Fatalf("len(history)=%d, want 1", len(payload.History))
	}
	if payload.History[0].Data != "base64-history" {
		t.Fatalf("history[0].Data=%q, want %q", payload.History[0].Data, "base64-history")
	}
	if payload.Error == "" {
		t.Fatal("expected fallback response to include an error message")
	}
}

func TestBrowserSessionMonitorWrapsLegacyImagePreview(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{running: true},
		history: []SessionScreenshot{
			{
				Data:       "base64-history",
				CapturedAt: "2026-03-23T00:00:00Z",
				URL:        "https://example.com",
				Title:      "Example",
			},
		},
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/tab-42/monitor", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload SessionMonitorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode monitor failed: %v", err)
	}
	if payload.Kind != SessionMonitorKindImage {
		t.Fatalf("payload.Kind=%q, want %q", payload.Kind, SessionMonitorKindImage)
	}
	if payload.Image == nil || payload.Image.History[0].Data != "base64-history" {
		t.Fatalf("payload.Image=%#v, want image history", payload.Image)
	}
}

func TestBrowserOverviewAggregatesSessionsAndTaskProjections(t *testing.T) {
	provider := &stubSessionRouteProvider{
		sessions: []SessionInfo{{
			ID:           "session-1",
			Status:       "active",
			CurrentURL:   "https://example.com",
			PageTitle:    "Example",
			CreatedAt:    "2026-03-23T00:00:00Z",
			LastActivity: "2026-03-23T00:00:00Z",
			Engine:       SessionEngineChromiumManaged,
			MonitorKind:  SessionMonitorKindImage,
		}},
	}
	taskService := &stubBrowserTaskProjectionService{
		list: func(_ context.Context, query BrowserOverviewTaskQuery) ([]map[string]any, error) {
			return []map[string]any{{
				"id":              "task-1",
				"kind":            "research",
				"conversation_id": query.ConversationID,
				"scope":           query.Scope,
				"title":           "Research task",
				"status":          "running",
				"stage":           "working",
				"progress":        55,
				"actions":         map[string]any{"items": []any{}},
				"updated_at":      "2026-03-23T00:00:00Z",
			}}, nil
		},
	}

	h := NewHandler(nil)
	h.SetSessionRouteProvider(provider)
	h.SetTaskProjectionService(taskService)

	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	req := httptest.NewRequest(http.MethodGet, "/browser/overview?scope=all&conversation_id=conv-1&limit=12", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Overview(c); err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Tasks    []map[string]any `json:"tasks"`
		Sessions []SessionInfo    `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode overview failed: %v", err)
	}
	if len(payload.Tasks) != 1 || payload.Tasks[0]["id"] != "task-1" {
		t.Fatalf("unexpected tasks payload: %#v", payload.Tasks)
	}
	if len(payload.Sessions) != 1 || payload.Sessions[0].ID != "session-1" {
		t.Fatalf("unexpected sessions payload: %#v", payload.Sessions)
	}
	if taskService.lastQuery.UserID != "user-1" || taskService.lastQuery.Scope != "all" || taskService.lastQuery.ConversationID != "conv-1" || taskService.lastQuery.Limit != 12 {
		t.Fatalf("unexpected task query: %#v", taskService.lastQuery)
	}
}

func TestBrowserSessionNavigateUsesSessionTargetID(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{running: true},
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(
		e,
		http.MethodPost,
		"/browser/sessions/tab-99/navigate",
		`{"url":"https://example.com/path"}`,
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	if service.lastNavigateReq == nil {
		t.Fatal("expected navigate request to be recorded")
	}
	if service.lastNavigateReq.TargetID != "tab-99" {
		t.Fatalf("TargetID=%q, want %q", service.lastNavigateReq.TargetID, "tab-99")
	}
	if service.lastNavigateReq.URL != "https://example.com/path" {
		t.Fatalf("URL=%q, want %q", service.lastNavigateReq.URL, "https://example.com/path")
	}
}

func TestBrowserSessionRoutesUseProviderWhenConfigured(t *testing.T) {
	provider := &stubSessionRouteProvider{
		sessions: []SessionInfo{
			{
				ID:           "lp-1",
				Status:       "active",
				CurrentURL:   "https://example.com",
				PageTitle:    "Example",
				CreatedAt:    "2026-03-23T00:00:00Z",
				LastActivity: "2026-03-23T00:00:00Z",
				Engine:       SessionEngineLightpanda,
				MonitorKind:  SessionMonitorKindText,
			},
		},
		monitor: &SessionMonitorResponse{
			Kind: SessionMonitorKindText,
			Text: &SessionTextMonitor{
				Title:            "Example",
				URL:              "https://example.com",
				Summary:          "summary",
				TreePreview:      "[document] \"Example\"",
				InteractiveCount: 2,
				UpdatedAt:        "2026-03-23T00:00:00Z",
				Status:           "active",
			},
		},
		screenshot: &SessionScreenshotResponse{
			Error: "unsupported_capability",
		},
	}
	h := NewHandler(&stubBrowserService{running: true})
	h.SetSessionRouteProvider(provider)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	overviewRec := performBrowserJSONRequest(e, http.MethodGet, "/browser/overview", "")
	if overviewRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", overviewRec.Code, overviewRec.Body.String())
	}

	var payload browserOverviewResponse
	if err := json.Unmarshal(overviewRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode overview failed: %v", err)
	}
	sessions := payload.Sessions
	if len(sessions) != 1 || sessions[0].Engine != SessionEngineLightpanda {
		t.Fatalf("sessions=%#v, want lightpanda provider session", sessions)
	}

	monitorRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/lp-1/monitor", "")
	if monitorRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", monitorRec.Code, monitorRec.Body.String())
	}
	if provider.lastMonitorID != "lp-1" {
		t.Fatalf("lastMonitorID=%q, want lp-1", provider.lastMonitorID)
	}

	screenshotRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/lp-1/screenshot", "")
	if screenshotRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", screenshotRec.Code, screenshotRec.Body.String())
	}
	if provider.lastScreenshotID != "lp-1" {
		t.Fatalf("lastScreenshotID=%q, want lp-1", provider.lastScreenshotID)
	}

	navigateRec := performBrowserJSONRequest(
		e,
		http.MethodPost,
		"/browser/sessions/lp-1/navigate",
		`{"url":"https://example.com/path"}`,
	)
	if navigateRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", navigateRec.Code, navigateRec.Body.String())
	}
	if provider.lastNavigateID != "lp-1" || provider.lastNavigateURL != "https://example.com/path" {
		t.Fatalf("provider navigate = (%q, %q), want (lp-1, https://example.com/path)", provider.lastNavigateID, provider.lastNavigateURL)
	}
}

func TestLightpandaSessionMonitorAndScreenshotCompatibility(t *testing.T) {
	pageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Docs</title></head><body><main><h1>Hybrid routing</h1><p>Readable text monitor payload.</p><a href="/next">Next</a></main></body></html>`))
	}))
	defer pageServer.Close()

	cfg := DefaultConfig()
	cfg.Strategy = BrowserStrategyHybridCapability
	cfg.Lightpanda.Enabled = true

	lp := NewLightpandaService(cfg)
	nav, err := lp.Navigate(context.Background(), &NavigateRequest{URL: pageServer.URL})
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	provider := &stubSessionRouteProvider{
		sessions: []SessionInfo{
			{
				ID:           nav.TargetID,
				Status:       "active",
				CurrentURL:   nav.URL,
				PageTitle:    nav.Title,
				CreatedAt:    "2026-03-23T00:00:00Z",
				LastActivity: "2026-03-23T00:00:00Z",
				Engine:       SessionEngineLightpanda,
				MonitorKind:  SessionMonitorKindText,
			},
		},
	}
	provider.monitor, err = lp.CaptureMonitor(nav.TargetID)
	if err != nil {
		t.Fatalf("CaptureMonitor() error = %v", err)
	}
	provider.screenshot, err = lp.CaptureScreenshot(nav.TargetID)
	if err != nil {
		t.Fatalf("CaptureScreenshot() error = %v", err)
	}

	h := NewHandler(&stubBrowserService{running: true})
	h.SetSessionRouteProvider(provider)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	monitorRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/"+nav.TargetID+"/monitor", "")
	if monitorRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", monitorRec.Code, monitorRec.Body.String())
	}

	var monitorPayload SessionMonitorResponse
	if err := json.Unmarshal(monitorRec.Body.Bytes(), &monitorPayload); err != nil {
		t.Fatalf("decode monitor failed: %v", err)
	}
	if monitorPayload.Kind != SessionMonitorKindText {
		t.Fatalf("monitor kind=%q, want %q", monitorPayload.Kind, SessionMonitorKindText)
	}
	if monitorPayload.Text == nil || monitorPayload.Text.Summary == "" || monitorPayload.Text.TreePreview == "" {
		t.Fatalf("monitor text payload=%#v, want non-empty summary/tree", monitorPayload.Text)
	}

	screenshotRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/"+nav.TargetID+"/screenshot", "")
	if screenshotRec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", screenshotRec.Code, screenshotRec.Body.String())
	}
	var screenshotPayload SessionScreenshotResponse
	if err := json.Unmarshal(screenshotRec.Body.Bytes(), &screenshotPayload); err != nil {
		t.Fatalf("decode screenshot failed: %v", err)
	}
	if screenshotPayload.Screenshot != "" {
		t.Fatalf("screenshot=%q, want empty for lightpanda", screenshotPayload.Screenshot)
	}
	if screenshotPayload.Error == "" {
		t.Fatal("expected compatibility screenshot to include text-only error")
	}
}
