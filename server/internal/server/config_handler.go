package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/labstack/echo/v4"
)

// ConfigHandler handles configuration-related API endpoints
type ConfigHandler struct {
	hotReloader *config.HotReloader
	configStore *config.ConfigStore
}

// NewConfigHandler creates a new config handler
func NewConfigHandler(hr *config.HotReloader) *ConfigHandler {
	return &ConfigHandler{
		hotReloader: hr,
	}
}

// SetConfigStore sets the kvstore-backed config store for section-level API.
func (h *ConfigHandler) SetConfigStore(cs *config.ConfigStore) {
	h.configStore = cs
}

// RegisterRoutes registers config routes
func (h *ConfigHandler) RegisterRoutes(g *echo.Group) {
	g.POST("/config/reload", h.Reload)
	g.GET("/config/status", h.Status)
	g.GET("/config/sections/:name", h.GetSection)
	g.PUT("/config/sections/:name", h.SetSection)
}

// ReloadRequest represents a reload request
type ReloadRequest struct {
	Force bool `json:"force"`
}

// ReloadResponse represents a reload response
type ReloadResponse struct {
	Success     bool      `json:"success"`
	Message     string    `json:"message"`
	ReloadedAt  time.Time `json:"reloaded_at,omitempty"`
	ReloadCount int64     `json:"reload_count"`
	Error       string    `json:"error,omitempty"`
}

// Reload handles POST /api/v1/config/reload
// @Summary Reload configuration
// @Description Triggers a manual configuration reload
// @Tags config
// @Accept json
// @Produce json
// @Param request body ReloadRequest false "Reload options"
// @Success 200 {object} ReloadResponse
// @Failure 500 {object} ReloadResponse
// @Router /api/v1/config/reload [post]
func (h *ConfigHandler) Reload(c echo.Context) error {
	if h.hotReloader == nil {
		return c.JSON(http.StatusServiceUnavailable, ReloadResponse{
			Success: false,
			Message: "Hot reload is not enabled",
			Error:   "hot_reload_disabled",
		})
	}

	err := h.hotReloader.Reload()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ReloadResponse{
			Success: false,
			Message: "Failed to reload configuration",
			Error:   err.Error(),
		})
	}

	stats := h.hotReloader.Stats()
	reloadCount, _ := stats["reload_count"].(int64)
	lastReload, _ := stats["last_reload"].(time.Time)

	return c.JSON(http.StatusOK, ReloadResponse{
		Success:     true,
		Message:     "Configuration reloaded successfully",
		ReloadedAt:  lastReload,
		ReloadCount: reloadCount,
	})
}

// ConfigStatusResponse represents the config status response
type ConfigStatusResponse struct {
	Enabled     bool      `json:"enabled"`
	ConfigPath  string    `json:"config_path"`
	LastReload  time.Time `json:"last_reload"`
	ReloadCount int64     `json:"reload_count"`
	IsReloading bool      `json:"is_reloading"`
	LastError   string    `json:"last_error,omitempty"`
}

// Status handles GET /api/v1/config/status
// @Summary Get configuration status
// @Description Returns the current configuration reload status
// @Tags config
// @Produce json
// @Success 200 {object} ConfigStatusResponse
// @Router /api/v1/config/status [get]
func (h *ConfigHandler) Status(c echo.Context) error {
	if h.hotReloader == nil {
		return c.JSON(http.StatusOK, ConfigStatusResponse{
			Enabled: false,
		})
	}

	stats := h.hotReloader.Stats()

	response := ConfigStatusResponse{
		Enabled:     stats["enabled"].(bool),
		ConfigPath:  stats["config_path"].(string),
		IsReloading: stats["is_reloading"].(bool),
	}

	if lastReload, ok := stats["last_reload"].(time.Time); ok {
		response.LastReload = lastReload
	}

	if reloadCount, ok := stats["reload_count"].(int64); ok {
		response.ReloadCount = reloadCount
	}

	if lastError, ok := stats["last_error"].(string); ok {
		response.LastError = lastError
	}

	return c.JSON(http.StatusOK, response)
}

// GetSection handles GET /api/v1/config/sections/:name
func (h *ConfigHandler) GetSection(c echo.Context) error {
	if h.configStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "config store not initialized"})
	}
	name := c.Param("name")
	data, err := h.configStore.GetSection(name)
	if err != nil {
		if errors.Is(err, config.ErrInvalidSection) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid config section", "section": name})
		}
		return c.JSON(http.StatusNotFound, map[string]string{"error": "section not found", "section": name})
	}
	return c.JSONBlob(http.StatusOK, data)
}

// SetSection handles PUT /api/v1/config/sections/:name
func (h *ConfigHandler) SetSection(c echo.Context) error {
	if h.configStore == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "config store not initialized"})
	}
	name := c.Param("name")
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to read body"})
	}
	// Validate JSON
	if !json.Valid(body) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
	}
	if err := h.configStore.SetSection(name, body); err != nil {
		if errors.Is(err, config.ErrInvalidSection) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid config section", "section": name})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"success": true, "section": name})
}
