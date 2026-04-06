package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type stubBrowserService struct {
	closeCount int
	startCount int
	running    bool
}

func (s *stubBrowserService) Status(context.Context) (*StatusResponse, error) {
	return &StatusResponse{Running: s.running}, nil
}

func (s *stubBrowserService) Start(context.Context) error {
	s.startCount++
	s.running = true
	return nil
}
func (s *stubBrowserService) Stop(context.Context) error { return nil }
func (s *stubBrowserService) Tabs(context.Context) ([]*Tab, error) {
	return nil, nil
}
func (s *stubBrowserService) OpenTab(context.Context, string) (*Tab, error) {
	return &Tab{}, nil
}
func (s *stubBrowserService) FocusTab(context.Context, string) error { return nil }
func (s *stubBrowserService) CloseTab(context.Context, string) error { return nil }
func (s *stubBrowserService) Navigate(context.Context, *NavigateRequest) (*NavigateResponse, error) {
	return &NavigateResponse{}, nil
}
func (s *stubBrowserService) Screenshot(context.Context, *ScreenshotRequest) (*ScreenshotResponse, error) {
	return &ScreenshotResponse{}, nil
}
func (s *stubBrowserService) PDF(context.Context, *PDFRequest) (*PDFResponse, error) {
	return &PDFResponse{}, nil
}
func (s *stubBrowserService) Snapshot(context.Context, *SnapshotRequest) (*SnapshotResponse, error) {
	return &SnapshotResponse{}, nil
}
func (s *stubBrowserService) Scrape(context.Context, *ScrapeRequest) (*ScrapeResponse, error) {
	return &ScrapeResponse{}, nil
}
func (s *stubBrowserService) Act(context.Context, *ActRequest) (*ActResponse, error) {
	return &ActResponse{}, nil
}
func (s *stubBrowserService) Automate(context.Context, *AutomateRequest) (*AutomateResponse, error) {
	return &AutomateResponse{}, nil
}
func (s *stubBrowserService) Console(context.Context, *ConsoleRequest) (*ConsoleResponse, error) {
	return &ConsoleResponse{}, nil
}
func (s *stubBrowserService) ExecuteRecipe(context.Context, *RecipeRequest) (*RecipeResponse, error) {
	return &RecipeResponse{}, nil
}
func (s *stubBrowserService) Recipes() *RecipeRegistry { return NewRecipeRegistry() }
func (s *stubBrowserService) Close() error {
	s.closeCount++
	return nil
}

func TestLazyHandlerStatusDoesNotInitializeService(t *testing.T) {
	var created int
	h := NewLazyHandler(func() Service {
		created++
		return &stubBrowserService{running: true}
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Status(c); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if created != 0 {
		t.Fatalf("created = %d, want 0", created)
	}
}

func TestLazyHandlerIdleReclaimsAndRecreatesService(t *testing.T) {
	var (
		created int
		seen    []*stubBrowserService
	)
	h := NewLazyHandler(func() Service {
		created++
		svc := &stubBrowserService{running: true}
		seen = append(seen, svc)
		return svc
	})
	h.SetIdleReclaim(20 * time.Millisecond)

	first := h.GetService()
	if first == nil {
		t.Fatal("GetService() returned nil")
	}
	if created != 1 {
		t.Fatalf("created after first GetService = %d, want 1", created)
	}

	time.Sleep(80 * time.Millisecond)

	second := h.GetService()
	if second == nil {
		t.Fatal("GetService() second call returned nil")
	}
	if created != 2 {
		t.Fatalf("created after idle reclaim = %d, want 2", created)
	}
	if first == second {
		t.Fatal("expected a new browser service after idle reclaim")
	}
	if seen[0].closeCount != 1 {
		t.Fatalf("first service closeCount = %d, want 1", seen[0].closeCount)
	}
}

func TestLazyHandlerOverviewDoesNotInitializeService(t *testing.T) {
	var created int
	h := NewLazyHandler(func() Service {
		created++
		return &stubBrowserService{}
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Overview(c); err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if created != 0 {
		t.Fatalf("created = %d, want 0", created)
	}
}

func TestOverviewDoesNotStartIdleService(t *testing.T) {
	service := &stubBrowserService{}
	h := NewHandler(service)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Overview(c); err != nil {
		t.Fatalf("Overview() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}
	if service.startCount != 0 {
		t.Fatalf("startCount = %d, want 0", service.startCount)
	}
}
