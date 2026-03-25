package browser

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/labstack/echo/v4"
)

// Handler handles browser automation HTTP requests.
type Handler struct {
	service        Service
	serviceFactory func() Service
	lazy           *reclaim.Managed[Service]
	relayInfo      func() RelayInfo
	sessionRoutes  SessionRouteProvider
	tasks          map[string]*BrowserTask
	tasksMu        sync.RWMutex
}

const (
	browserSessionCreateTimeout   = 60 * time.Second
	browserSessionNavigateTimeout = 60 * time.Second
)

// BrowserTask represents a browser automation task.
type BrowserTask struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Status      string      `json:"status"`
	Steps       interface{} `json:"steps"`
	CreatedAt   string      `json:"created_at"`
	StartedAt   string      `json:"started_at,omitempty"`
	CompletedAt string      `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
	Result      interface{} `json:"result,omitempty"`
}

type browserViewportScreenshoter interface {
	ScreenshotViewport(ctx context.Context, targetID string) (string, error)
}

type browserTabScreenshoter interface {
	ScreenshotTab(ctx context.Context, targetID string) (string, error)
}

type browserSessionScreenshotHistoryProvider interface {
	SessionScreenshotHistory(targetID string) []SessionScreenshot
}

// NewHandler creates a new browser handler.
func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
		tasks:   make(map[string]*BrowserTask),
	}
}

// NewLazyHandler creates a browser handler whose service is initialized on demand.
func NewLazyHandler(factory func() Service) *Handler {
	h := &Handler{
		serviceFactory: factory,
		tasks:          make(map[string]*BrowserTask),
	}
	h.lazy = reclaim.NewManaged[Service](0, func() (Service, error) {
		if h.serviceFactory == nil {
			return nil, nil
		}
		svc := h.serviceFactory()
		if svc == nil {
			return nil, ErrBrowserNotAvailable
		}
		return svc, nil
	}, func(_ context.Context, svc Service) error {
		if svc == nil {
			return nil
		}
		return svc.Close()
	})
	return h
}

func (h *Handler) getService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Get()
	return svc
}

// GetService returns the current service, creating it if needed.
func (h *Handler) GetService() Service {
	return h.getService()
}

// PeekService returns the current browser service without creating it.
func (h *Handler) PeekService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, ok := h.lazy.Peek()
	if !ok {
		return nil
	}
	return svc
}

// SetIdleReclaim configures idle reclaim for lazily created browser services.
func (h *Handler) SetIdleReclaim(idleAfter time.Duration) {
	if h == nil || h.lazy == nil {
		return
	}
	h.lazy.SetIdleAfter(idleAfter)
}

func (h *Handler) peekService() Service {
	if h == nil {
		return nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service
	}
	svc, _ := h.lazy.Peek()
	return svc
}

func (h *Handler) acquireService() (Service, func(), error) {
	if h == nil {
		return nil, nil, nil
	}
	if h.service != nil || h.lazy == nil {
		return h.service, func() {}, nil
	}
	svc, release, err := h.lazy.Acquire()
	if err != nil {
		return nil, nil, err
	}
	return svc, release, nil
}

func (h *Handler) acquireStartedService(ctx context.Context) (Service, func(), error) {
	service, release, err := h.acquireService()
	if err != nil {
		return nil, nil, err
	}
	if service == nil {
		return nil, release, nil
	}
	if err := service.Start(ctx); err != nil {
		release()
		return nil, nil, err
	}
	return service, release, nil
}

// SetRelayInfoProvider configures how relay runtime info is exposed over HTTP.
func (h *Handler) SetRelayInfoProvider(provider func() RelayInfo) {
	h.relayInfo = provider
}

// SetSessionRouteProvider configures a multi-engine session provider for
// session and monitor HTTP routes.
func (h *Handler) SetSessionRouteProvider(provider SessionRouteProvider) {
	h.sessionRoutes = provider
}

// RegisterRoutes registers the browser routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Status and control
	g.GET("/relay/info", h.RelayInfo)
	g.GET("/status", h.Status)
	g.POST("/start", h.Start)
	g.POST("/stop", h.Stop)

	// Tab management
	g.GET("/tabs", h.Tabs)
	g.POST("/tabs", h.OpenTab)
	g.POST("/tabs/:id/focus", h.FocusTab)
	g.DELETE("/tabs/:id", h.CloseTab)

	// Navigation
	g.POST("/navigate", h.Navigate)

	// Content capture
	g.POST("/screenshot", h.Screenshot)
	g.POST("/pdf", h.PDF)
	g.POST("/snapshot", h.Snapshot)

	// Data extraction
	g.POST("/scrape", h.Scrape)

	// Actions
	g.POST("/act", h.Act)
	g.POST("/automate", h.Automate)

	// Console
	g.GET("/console", h.Console)

	// Recipes
	g.GET("/recipes", h.ListRecipes)
	g.POST("/recipe", h.ExecuteRecipe)

	// Task management (stub endpoints for frontend compatibility)
	g.GET("/tasks", h.ListTasks)
	g.POST("/tasks", h.CreateTask)
	g.GET("/tasks/:id", h.GetTask)
	g.POST("/tasks/:id/run", h.RunTask)
	g.POST("/tasks/:id/cancel", h.CancelTask)
	g.DELETE("/tasks/:id", h.DeleteTask)

	// Session management (stub endpoints for frontend compatibility)
	g.GET("/sessions", h.ListSessions)
	g.POST("/sessions", h.CreateSession)
	g.GET("/sessions/:id", h.GetSession)
	g.DELETE("/sessions/:id", h.CloseSession)
	g.POST("/sessions/:id/monitor", h.SessionMonitor)
	g.POST("/sessions/:id/screenshot", h.SessionScreenshot)
	g.POST("/sessions/:id/navigate", h.SessionNavigate)
	g.POST("/sessions/:id/execute", h.SessionExecute)

	// Security configuration (stub endpoints for frontend compatibility)
	g.GET("/security", h.GetSecurityConfig)
	g.PUT("/security", h.UpdateSecurityConfig)
	g.POST("/security/allowed", h.AddAllowedDomain)
	g.DELETE("/security/allowed/:domain", h.RemoveAllowedDomain)
	g.POST("/security/blocked", h.AddBlockedDomain)
	g.DELETE("/security/blocked/:domain", h.RemoveBlockedDomain)
	g.POST("/security/test", h.TestURL)
}

// RelayInfo returns built-in browser relay runtime details.
func (h *Handler) RelayInfo(c echo.Context) error {
	if h.relayInfo == nil {
		return c.JSON(http.StatusOK, RelayInfo{})
	}
	return c.JSON(http.StatusOK, h.relayInfo())
}

// ListTasks returns all browser automation tasks.
func (h *Handler) ListTasks(c echo.Context) error {
	h.tasksMu.RLock()
	defer h.tasksMu.RUnlock()

	tasks := make([]*BrowserTask, 0, len(h.tasks))
	for _, task := range h.tasks {
		tasks = append(tasks, task)
	}
	return c.JSON(http.StatusOK, tasks)
}

// CreateTask creates a new browser automation task.
func (h *Handler) CreateTask(c echo.Context) error {
	var req struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Steps       interface{} `json:"steps"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Generate unique ID
	id := timeutil.NowTime().Format("20060102150405") + "-" + randomString(6)

	task := &BrowserTask{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Status:      "pending",
		Steps:       req.Steps,
		CreatedAt:   timeutil.NowTime().Format(time.RFC3339),
	}

	h.tasksMu.Lock()
	h.tasks[id] = task
	h.tasksMu.Unlock()

	return c.JSON(http.StatusOK, task)
}

// randomString generates a random string of given length.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0180789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[timeutil.NowNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

func newBrowserSessionResponse(tab *Tab, now time.Time) SessionInfo {
	session := SessionInfo{
		CreatedAt:    now.Format(time.RFC3339),
		LastActivity: now.Format(time.RFC3339),
		Engine:       SessionEngineChromiumManaged,
		MonitorKind:  SessionMonitorKindImage,
	}
	if tab == nil {
		session.Status = "idle"
		return session
	}
	session.ID = tab.TargetID
	session.CurrentURL = tab.URL
	session.PageTitle = tab.Title
	if tab.Active {
		session.Status = "active"
	} else {
		session.Status = "idle"
	}
	return session
}

func browserSessionResponsesFromTabs(tabs []*Tab, now time.Time) []SessionInfo {
	sessions := make([]SessionInfo, 0, len(tabs))
	for _, tab := range tabs {
		if tab == nil {
			continue
		}
		sessions = append(sessions, newBrowserSessionResponse(tab, now))
	}
	return sessions
}

func findTabByTargetID(tabs []*Tab, id string) *Tab {
	for _, tab := range tabs {
		if tab != nil && tab.TargetID == id {
			return tab
		}
	}
	return nil
}

// GetTask returns a specific task.
func (h *Handler) GetTask(c echo.Context) error {
	id := c.Param("id")

	h.tasksMu.RLock()
	task, exists := h.tasks[id]
	h.tasksMu.RUnlock()

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	return c.JSON(http.StatusOK, task)
}

// RunTask runs a task.
func (h *Handler) RunTask(c echo.Context) error {
	id := c.Param("id")

	h.tasksMu.Lock()
	task, exists := h.tasks[id]
	if exists {
		task.Status = "running"
		task.StartedAt = timeutil.NowTime().Format(time.RFC3339)
	}
	h.tasksMu.Unlock()

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}

	// Simulate task completion after a short delay (in production, this would be async)
	go func() {
		time.Sleep(2 * time.Second)
		h.tasksMu.Lock()
		if t, ok := h.tasks[id]; ok {
			if t.Status == "running" {
				t.Status = "completed"
				t.CompletedAt = timeutil.NowTime().Format(time.RFC3339)
			}
		}
		h.tasksMu.Unlock()
	}()

	return c.JSON(http.StatusOK, map[string]string{"status": "started"})
}

// CancelTask cancels a running task.
func (h *Handler) CancelTask(c echo.Context) error {
	id := c.Param("id")

	h.tasksMu.Lock()
	task, exists := h.tasks[id]
	if exists && (task.Status == "pending" || task.Status == "running") {
		task.Status = "cancelled"
		task.CompletedAt = timeutil.NowTime().Format(time.RFC3339)
	}
	h.tasksMu.Unlock()

	if !exists {
		return echo.NewHTTPError(http.StatusNotFound, "task not found")
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "cancelled"})
}

// DeleteTask deletes a task.
func (h *Handler) DeleteTask(c echo.Context) error {
	id := c.Param("id")

	h.tasksMu.Lock()
	delete(h.tasks, id)
	h.tasksMu.Unlock()

	return c.NoContent(http.StatusNoContent)
}

// ListSessions returns all browser sessions.
func (h *Handler) ListSessions(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
		defer cancel()
		sessions, err := h.sessionRoutes.ListBrowserSessions(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if sessions == nil {
			sessions = []SessionInfo{}
		}
		return c.JSON(http.StatusOK, sessions)
	}

	service := h.peekService()
	if service == nil {
		return c.JSON(http.StatusOK, []SessionInfo{})
	}

	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()

	status, err := service.Status(ctx)
	if err != nil || status == nil || !status.Running {
		return c.JSON(http.StatusOK, []SessionInfo{})
	}

	// Return tabs as sessions for compatibility
	tabs, err := service.Tabs(ctx)
	if err != nil {
		return c.JSON(http.StatusOK, []SessionInfo{})
	}
	return c.JSON(http.StatusOK, browserSessionResponsesFromTabs(tabs, timeutil.NowTime()))
}

// CreateSession creates a new browser session.
func (h *Handler) CreateSession(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionCreateTimeout)
		defer cancel()
		session, err := h.sessionRoutes.CreateBrowserSession(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if session == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "browser session unavailable")
		}
		return c.JSON(http.StatusOK, session)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":            "session-" + timeutil.NowTime().Format("20060102150405"),
			"status":        "idle",
			"current_url":   "",
			"page_title":    "",
			"created_at":    timeutil.NowTime().Format(time.RFC3339),
			"last_activity": timeutil.NowTime().Format(time.RFC3339),
		})
	}

	// Create a context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	// Try to create a new tab
	tab, err := service.OpenTab(ctx, "about:blank")
	if err != nil {
		// Return a stub session if browser not available or timeout
		return c.JSON(http.StatusOK, map[string]interface{}{
			"id":            "session-" + timeutil.NowTime().Format("20060102150405"),
			"status":        "idle",
			"current_url":   "",
			"page_title":    "",
			"created_at":    timeutil.NowTime().Format(time.RFC3339),
			"last_activity": timeutil.NowTime().Format(time.RFC3339),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":            tab.TargetID,
		"status":        "active",
		"current_url":   tab.URL,
		"page_title":    tab.Title,
		"created_at":    timeutil.NowTime().Format(time.RFC3339),
		"last_activity": timeutil.NowTime().Format(time.RFC3339),
	})
}

// GetSession returns a specific session.
func (h *Handler) GetSession(c echo.Context) error {
	id := c.Param("id")
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		session, err := h.sessionRoutes.GetBrowserSession(ctx, id)
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, session)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()

	tabs, err := service.Tabs(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	tab := findTabByTargetID(tabs, id)
	if tab == nil {
		return echo.NewHTTPError(http.StatusNotFound, "session not found")
	}

	return c.JSON(http.StatusOK, newBrowserSessionResponse(tab, timeutil.NowTime()))
}

// CloseSession closes a browser session.
func (h *Handler) CloseSession(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		if err := h.sessionRoutes.CloseBrowserSession(ctx, c.Param("id")); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.NoContent(http.StatusNoContent)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return c.NoContent(http.StatusNoContent)
	}

	id := c.Param("id")
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(c.Request().Context(), 3*time.Second)
	defer cancel()
	// Try to close the tab
	_ = service.CloseTab(ctx, id)
	return c.NoContent(http.StatusNoContent)
}

// SessionMonitor returns the unified text/image monitor payload for a session.
func (h *Handler) SessionMonitor(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		payload, err := h.sessionRoutes.CaptureBrowserSessionMonitor(ctx, c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, payload)
	}

	screenshotPayload, err := h.captureLegacySessionScreenshot(c, c.Param("id"))
	if err != nil {
		return err
	}
	updatedAt := ""
	if len(screenshotPayload.History) > 0 {
		updatedAt = screenshotPayload.History[0].CapturedAt
	}
	return c.JSON(http.StatusOK, &SessionMonitorResponse{
		Kind: SessionMonitorKindImage,
		Image: &SessionImageMonitor{
			Screenshot: screenshotPayload.Screenshot,
			History:    screenshotPayload.History,
			UpdatedAt:  updatedAt,
			Status:     "active",
		},
		Error: screenshotPayload.Error,
	})
}

func (h *Handler) SessionScreenshot(c echo.Context) error {
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()
		payload, err := h.sessionRoutes.CaptureBrowserSessionScreenshot(ctx, c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, payload)
	}
	payload, err := h.captureLegacySessionScreenshot(c, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *Handler) captureLegacySessionScreenshot(c echo.Context, targetID string) (*SessionScreenshotResponse, error) {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return &SessionScreenshotResponse{
			Error: "browser service unavailable",
		}, nil
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
	defer cancel()

	var captureErrors []string

	if screenshoter, ok := service.(browserViewportScreenshoter); ok {
		screenshot, err := screenshoter.ScreenshotViewport(ctx, targetID)
		if err == nil && strings.TrimSpace(screenshot) != "" {
			return &SessionScreenshotResponse{
				Screenshot: screenshot,
				History:    sessionScreenshotHistory(service, targetID),
			}, nil
		}
		if err != nil {
			captureErrors = append(captureErrors, err.Error())
		}
	}

	if screenshoter, ok := service.(browserTabScreenshoter); ok {
		screenshot, err := screenshoter.ScreenshotTab(ctx, targetID)
		if err == nil && strings.TrimSpace(screenshot) != "" {
			return &SessionScreenshotResponse{
				Screenshot: screenshot,
				History:    sessionScreenshotHistory(service, targetID),
			}, nil
		}
		if err != nil {
			captureErrors = append(captureErrors, err.Error())
		}
	}

	history := sessionScreenshotHistory(service, targetID)
	latest := ""
	if len(history) > 0 {
		latest = history[0].Data
	}

	errorMessage := "preview unavailable"
	if len(captureErrors) > 0 {
		errorMessage = strings.Join(captureErrors, "; ")
	}

	return &SessionScreenshotResponse{
		Screenshot: latest,
		History:    history,
		Error:      errorMessage,
	}, nil
}

func sessionScreenshotHistory(service Service, targetID string) []SessionScreenshot {
	provider, ok := service.(browserSessionScreenshotHistoryProvider)
	if !ok || provider == nil {
		return nil
	}
	return provider.SessionScreenshotHistory(targetID)
}

func (h *Handler) SessionNavigate(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if h.sessionRoutes != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionNavigateTimeout)
		defer cancel()
		resp, err := h.sessionRoutes.NavigateBrowserSession(ctx, c.Param("id"), req.URL)
		if err != nil {
			if errors.Is(err, ErrTabNotFound) {
				return echo.NewHTTPError(http.StatusNotFound, "session not found")
			}
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
		if resp == nil {
			return c.JSON(http.StatusOK, map[string]string{"status": "navigated"})
		}
		return c.JSON(http.StatusOK, resp)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	ctx, cancel := context.WithTimeout(c.Request().Context(), browserSessionNavigateTimeout)
	defer cancel()

	resp, err := service.Navigate(ctx, &NavigateRequest{
		URL:      req.URL,
		TargetID: c.Param("id"),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if resp == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "navigated"})
	}
	return c.JSON(http.StatusOK, resp)
}

// SessionExecute executes a step in a session (stub).
func (h *Handler) SessionExecute(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"screenshot":     "",
		"extracted_data": nil,
		"element_found":  true,
		"page_title":     "",
		"page_url":       "",
	})
}

// Status returns the browser status.
func (h *Handler) Status(c echo.Context) error {
	service := h.peekService()
	if service == nil {
		return c.JSON(http.StatusOK, &StatusResponse{Running: false})
	}

	status, err := service.Status(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, status)
}

// Start starts the browser.
func (h *Handler) Start(c echo.Context) error {
	service, release, err := h.acquireService()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.Start(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "started"})
}

// Stop stops the browser.
func (h *Handler) Stop(c echo.Context) error {
	service, release, err := h.acquireService()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
	}

	err = service.Stop(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}

// Tabs returns all open tabs.
func (h *Handler) Tabs(c echo.Context) error {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	tabs, err := service.Tabs(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, tabs)
}

// OpenTab opens a new tab.
func (h *Handler) OpenTab(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	tab, err := service.OpenTab(c.Request().Context(), req.URL)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, tab)
}

// FocusTab focuses a tab.
func (h *Handler) FocusTab(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tab id is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.FocusTab(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "focused"})
}

// CloseTab closes a tab.
func (h *Handler) CloseTab(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "tab id is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	err = service.CloseTab(c.Request().Context(), id)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "closed"})
}

// Navigate navigates to a URL.
func (h *Handler) Navigate(c echo.Context) error {
	var req NavigateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Navigate(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Screenshot captures a screenshot.
func (h *Handler) Screenshot(c echo.Context) error {
	var req ScreenshotRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Screenshot(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// PDF generates a PDF.
func (h *Handler) PDF(c echo.Context) error {
	var req PDFRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.PDF(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Snapshot returns a page snapshot.
func (h *Handler) Snapshot(c echo.Context) error {
	var req SnapshotRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Snapshot(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Scrape extracts data from a page.
func (h *Handler) Scrape(c echo.Context) error {
	var req ScrapeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if len(req.Selectors) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "selectors is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Scrape(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Act performs an action on the page.
func (h *Handler) Act(c echo.Context) error {
	var req ActRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Kind == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "kind is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Act(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Automate runs an automation task.
func (h *Handler) Automate(c echo.Context) error {
	var req AutomateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	if len(req.Steps) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "steps is required")
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Automate(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// Console returns console messages.
func (h *Handler) Console(c echo.Context) error {
	var req ConsoleRequest
	req.TargetID = c.QueryParam("target_id")
	req.Level = c.QueryParam("level")
	req.Clear = c.QueryParam("clear") == "true"

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.Console(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
	}
	return c.JSON(http.StatusOK, resp)
}

// ListRecipes returns all available recipes.
func (h *Handler) ListRecipes(c echo.Context) error {
	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	recipes := service.Recipes()
	return c.JSON(http.StatusOK, recipes.List())
}

// ExecuteRecipe runs a browser recipe.
func (h *Handler) ExecuteRecipe(c echo.Context) error {
	var req RecipeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Recipe == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "recipe is required")
	}
	if req.Params == nil {
		req.Params = make(map[string]string)
	}

	service, release, err := h.acquireStartedService(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if release != nil {
		defer release()
	}
	if service == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "browser service unavailable")
	}

	resp, err := service.ExecuteRecipe(c.Request().Context(), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return c.JSON(http.StatusOK, resp)
}

// mapError maps domain errors to HTTP errors.
func mapError(err error) *echo.HTTPError {
	switch err {
	case ErrBrowserNotAvailable, ErrBrowserNotRunning:
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	case ErrPageNotFound, ErrTabNotFound, ErrElementNotFound:
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case ErrURLNotAllowed, ErrURLBlocked:
		return echo.NewHTTPError(http.StatusForbidden, err.Error())
	case ErrTimeout:
		return echo.NewHTTPError(http.StatusGatewayTimeout, err.Error())
	case ErrResourceLimitExceeded:
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	case ErrInvalidSelector, ErrInvalidAction:
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
}

// BrowserSecurityConfig represents the browser security configuration.
type BrowserSecurityConfig struct {
	AllowedDomains []string `json:"allowed_domains"`
	BlockedDomains []string `json:"blocked_domains"`
}

// In-memory security config (stub - should be persisted in production)
var securityConfig = BrowserSecurityConfig{
	AllowedDomains: []string{},
	BlockedDomains: []string{},
}

// GetSecurityConfig returns the browser security configuration (stub).
func (h *Handler) GetSecurityConfig(c echo.Context) error {
	return c.JSON(http.StatusOK, securityConfig)
}

// UpdateSecurityConfig updates the browser security configuration (stub).
func (h *Handler) UpdateSecurityConfig(c echo.Context) error {
	var config BrowserSecurityConfig
	if err := c.Bind(&config); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	securityConfig = config
	return c.JSON(http.StatusOK, securityConfig)
}

// AddAllowedDomain adds a domain to the allowed list (stub).
func (h *Handler) AddAllowedDomain(c echo.Context) error {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	// Check if already exists
	for _, d := range securityConfig.AllowedDomains {
		if d == req.Domain {
			return c.JSON(http.StatusOK, securityConfig)
		}
	}
	securityConfig.AllowedDomains = append(securityConfig.AllowedDomains, req.Domain)
	return c.JSON(http.StatusOK, securityConfig)
}

// RemoveAllowedDomain removes a domain from the allowed list (stub).
func (h *Handler) RemoveAllowedDomain(c echo.Context) error {
	domain := c.Param("domain")
	if domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	newList := make([]string, 0, len(securityConfig.AllowedDomains))
	for _, d := range securityConfig.AllowedDomains {
		if d != domain {
			newList = append(newList, d)
		}
	}
	securityConfig.AllowedDomains = newList
	return c.JSON(http.StatusOK, securityConfig)
}

// AddBlockedDomain adds a domain to the blocked list (stub).
func (h *Handler) AddBlockedDomain(c echo.Context) error {
	var req struct {
		Domain string `json:"domain"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.Domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	// Check if already exists
	for _, d := range securityConfig.BlockedDomains {
		if d == req.Domain {
			return c.JSON(http.StatusOK, securityConfig)
		}
	}
	securityConfig.BlockedDomains = append(securityConfig.BlockedDomains, req.Domain)
	return c.JSON(http.StatusOK, securityConfig)
}

// RemoveBlockedDomain removes a domain from the blocked list (stub).
func (h *Handler) RemoveBlockedDomain(c echo.Context) error {
	domain := c.Param("domain")
	if domain == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "domain is required")
	}
	newList := make([]string, 0, len(securityConfig.BlockedDomains))
	for _, d := range securityConfig.BlockedDomains {
		if d != domain {
			newList = append(newList, d)
		}
	}
	securityConfig.BlockedDomains = newList
	return c.JSON(http.StatusOK, securityConfig)
}

// TestURL tests if a URL is allowed (stub).
func (h *Handler) TestURL(c echo.Context) error {
	var req struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if req.URL == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "url is required")
	}
	// Simple stub - always allow
	return c.JSON(http.StatusOK, map[string]interface{}{
		"allowed": true,
		"reason":  "",
	})
}
