package zimaos

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
)

// Handler provides HTTP endpoints for ZimaOS integration.
type Handler struct {
	integration *Integration

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data
	appsCache   *cache.GenericCache[string]
	systemCache *cache.GenericCache[string]
}

// NewHandler creates a new ZimaOS handler.
func NewHandler(integration *Integration) *Handler {
	return &Handler{
		integration: integration,
		appsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    100,
			DefaultTTL: 30 * time.Second, // Apps list changes infrequently
		}, "zimaos_apps"),
		systemCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    10,
			DefaultTTL: 60 * time.Second, // System info is relatively static
		}, "zimaos_system"),
	}
}

// RegisterRoutes registers ZimaOS routes.
func (h *Handler) RegisterRoutes(e *echo.Echo) {
	g := e.Group("/api/zimaos")
	g.GET("/status", h.GetStatus)
	g.GET("/system", h.GetSystemInfo)
	g.GET("/apps", h.ListApps)
	g.GET("/apps/:id", h.GetApp)
	g.GET("/files", h.ListFiles)
	g.POST("/notify", h.SendNotification)
}

// GetStatus returns the ZimaOS integration status.
func (h *Handler) GetStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"running_on_zimaos": h.integration.IsRunningOnZimaOS(),
		"data_path":         h.integration.GetDataPath(),
		"config_path":       h.integration.GetConfigPath(),
	})
}

// GetSystemInfo returns ZimaOS system information.
func (h *Handler) GetSystemInfo(c echo.Context) error {
	// Try cache first
	cacheKey := "system_info"
	if cached, ok := h.systemCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, _, _ := h.sfGroup.Do("get_system_info", func() (interface{}, error) {
		info, err := h.integration.GetSystemInfo(c.Request().Context())
		if err != nil {
			return nil, err
		}
		// Cache the result
		h.systemCache.Put(cacheKey, info)
		return info, nil
	})

	if result == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get system info",
		})
	}

	return c.JSON(http.StatusOK, result)
}

// ListApps returns installed ZimaOS applications.
func (h *Handler) ListApps(c echo.Context) error {
	// Try cache first
	cacheKey := "apps_list"
	if cached, ok := h.appsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, _, _ := h.sfGroup.Do("list_apps", func() (interface{}, error) {
		apps, err := h.integration.ListApps(c.Request().Context())
		if err != nil {
			return nil, err
		}
		// Cache the result
		h.appsCache.Put(cacheKey, apps)
		return apps, nil
	})

	if result == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to list apps",
		})
	}

	return c.JSON(http.StatusOK, result)
}

// GetApp returns information about a specific app.
func (h *Handler) GetApp(c echo.Context) error {
	appID := c.Param("id")

	// Try cache first
	cacheKey := "app_" + appID
	if cached, ok := h.appsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, _, _ := h.sfGroup.Do("get_app_"+appID, func() (interface{}, error) {
		app, err := h.integration.GetApp(c.Request().Context(), appID)
		if err != nil {
			return nil, err
		}
		// Cache the result
		h.appsCache.Put(cacheKey, app)
		return app, nil
	})

	if result == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to get app",
		})
	}

	return c.JSON(http.StatusOK, result)
}

// ListFiles returns files in a directory.
func (h *Handler) ListFiles(c echo.Context) error {
	path := c.QueryParam("path")
	if path == "" {
		path = h.integration.GetDataPath()
	}

	files, err := h.integration.ListFiles(c.Request().Context(), path)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, files)
}

// SendNotification sends a notification through ZimaOS.
func (h *Handler) SendNotification(c echo.Context) error {
	var req struct {
		Title   string `json:"title"`
		Message string `json:"message"`
		Level   string `json:"level"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid request body",
		})
	}

	if req.Level == "" {
		req.Level = "info"
	}

	if err := h.integration.SendNotification(c.Request().Context(), req.Title, req.Message, req.Level); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "sent",
	})
}
