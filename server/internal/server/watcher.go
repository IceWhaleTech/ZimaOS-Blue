package server

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/watcher"
)

// WatcherHandler handles watcher-related API endpoints.
type WatcherHandler struct {
	watcher *watcher.Watcher
}

// NewWatcherHandler creates a new watcher handler.
func NewWatcherHandler(w *watcher.Watcher) *WatcherHandler {
	return &WatcherHandler{
		watcher: w,
	}
}

// RegisterWatcherRoutes registers watcher routes.
func (h *WatcherHandler) RegisterWatcherRoutes(e *echo.Echo) {
	g := e.Group("/api/v1/watcher")
	g.GET("/status", h.getStatus)
	g.GET("/paths", h.getPaths)
	g.POST("/paths", h.addPath)
	g.DELETE("/paths", h.removePath)
}

// WatcherStatusResponse represents the watcher status response.
type WatcherStatusResponse struct {
	Enabled        bool     `json:"enabled"`
	Recursive      bool     `json:"recursive"`
	WatchedPaths   []string `json:"watched_paths"`
	WatchedCount   int      `json:"watched_count"`
	DebounceMs     int      `json:"debounce_ms"`
	Events         []string `json:"events"`
	IgnorePatterns []string `json:"ignore_patterns"`
}

// getStatus returns the watcher status.
func (h *WatcherHandler) getStatus(c echo.Context) error {
	if h.watcher == nil {
		return c.JSON(http.StatusOK, WatcherStatusResponse{
			Enabled: false,
		})
	}

	config := h.watcher.Config()
	events := make([]string, len(config.Events))
	for i, e := range config.Events {
		events[i] = string(e)
	}

	return c.JSON(http.StatusOK, WatcherStatusResponse{
		Enabled:        config.Enabled,
		Recursive:      config.Recursive,
		WatchedPaths:   h.watcher.WatchedPaths(),
		WatchedCount:   h.watcher.WatchedDirCount(),
		DebounceMs:     config.DebounceMs,
		Events:         events,
		IgnorePatterns: config.IgnorePatterns,
	})
}

// getPaths returns the list of watched paths.
func (h *WatcherHandler) getPaths(c echo.Context) error {
	if h.watcher == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"paths": []string{},
			"count": 0,
		})
	}

	paths := h.watcher.WatchedPaths()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"paths": paths,
		"count": len(paths),
	})
}

// AddPathRequest represents a request to add a watch path.
type AddPathRequest struct {
	Path string `json:"path" validate:"required"`
}

// addPath adds a path to watch.
func (h *WatcherHandler) addPath(c echo.Context) error {
	if h.watcher == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "watcher is not enabled",
		})
	}

	var req AddPathRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Path == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "path is required",
		})
	}

	if err := h.watcher.AddPath(req.Path); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "path added successfully",
		"path":    req.Path,
		"count":   h.watcher.WatchedDirCount(),
	})
}

// RemovePathRequest represents a request to remove a watch path.
type RemovePathRequest struct {
	Path string `json:"path" validate:"required"`
}

// removePath removes a path from watching.
func (h *WatcherHandler) removePath(c echo.Context) error {
	if h.watcher == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{
			"error": "watcher is not enabled",
		})
	}

	var req RemovePathRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	if req.Path == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "path is required",
		})
	}

	if err := h.watcher.RemovePath(req.Path); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "path removed successfully",
		"path":    req.Path,
		"count":   h.watcher.WatchedDirCount(),
	})
}
