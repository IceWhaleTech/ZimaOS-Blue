package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
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

func TestBrowserListSessionsMapsTabsToSessions(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{},
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

	rec := performBrowserJSONRequest(e, http.MethodGet, "/browser/sessions", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var sessions []browserSessionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("decode sessions failed: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions)=%d, want 2", len(sessions))
	}
	if sessions[0].ID != "tab-1" {
		t.Fatalf("sessions[0].ID=%q, want %q", sessions[0].ID, "tab-1")
	}
	if sessions[0].Status != "active" {
		t.Fatalf("sessions[0].Status=%q, want %q", sessions[0].Status, "active")
	}
	if sessions[1].Status != "idle" {
		t.Fatalf("sessions[1].Status=%q, want %q", sessions[1].Status, "idle")
	}
}

func TestBrowserSessionScreenshotUsesViewportCapture(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{},
		screenshot:         "base64-png",
	}
	h := NewHandler(service)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodPost, "/browser/sessions/tab-42/screenshot", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var payload browserSessionScreenshotResponse
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
		stubBrowserService: &stubBrowserService{},
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

	var payload browserSessionScreenshotResponse
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

func TestBrowserSessionNavigateUsesSessionTargetID(t *testing.T) {
	service := &sessionAwareStubBrowserService{
		stubBrowserService: &stubBrowserService{},
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
