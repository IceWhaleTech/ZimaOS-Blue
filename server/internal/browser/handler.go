package browser

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// Handler handles browser automation HTTP requests.
type Handler struct {
	service Service
}

// NewHandler creates a new browser handler.
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the browser routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Status and control
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

	// Task management (stub endpoints for frontend compatibility)
	g.GET("/tasks", h.ListTasks)
	g.GET("/sessions", h.ListSessions)
}

// ListTasks returns all browser automation tasks (stub).
func (h *Handler) ListTasks(c echo.Context) error {
	// Return empty array for now - task management not yet implemented
	return c.JSON(http.StatusOK, []interface{}{})
}

// ListSessions returns all browser sessions (stub).
func (h *Handler) ListSessions(c echo.Context) error {
	// Return tabs as sessions for compatibility
	tabs, err := h.service.Tabs(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusOK, []interface{}{})
	}
	return c.JSON(http.StatusOK, tabs)
}

// Status returns the browser status.
func (h *Handler) Status(c echo.Context) error {
	status, err := h.service.Status(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, status)
}

// Start starts the browser.
func (h *Handler) Start(c echo.Context) error {
	err := h.service.Start(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "started"})
}

// Stop stops the browser.
func (h *Handler) Stop(c echo.Context) error {
	err := h.service.Stop(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]string{"status": "stopped"})
}

// Tabs returns all open tabs.
func (h *Handler) Tabs(c echo.Context) error {
	tabs, err := h.service.Tabs(c.Request().Context())
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

	tab, err := h.service.OpenTab(c.Request().Context(), req.URL)
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

	err := h.service.FocusTab(c.Request().Context(), id)
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

	err := h.service.CloseTab(c.Request().Context(), id)
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

	resp, err := h.service.Navigate(c.Request().Context(), &req)
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

	resp, err := h.service.Screenshot(c.Request().Context(), &req)
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

	resp, err := h.service.PDF(c.Request().Context(), &req)
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

	resp, err := h.service.Snapshot(c.Request().Context(), &req)
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

	resp, err := h.service.Scrape(c.Request().Context(), &req)
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

	resp, err := h.service.Act(c.Request().Context(), &req)
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

	resp, err := h.service.Automate(c.Request().Context(), &req)
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

	resp, err := h.service.Console(c.Request().Context(), &req)
	if err != nil {
		return mapError(err)
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
