package server

import (
	"net/http"
	"sort"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
	"github.com/labstack/echo/v4"
)

// PluginHandler handles installed plugin management HTTP requests
type PluginHandler struct {
	registry *plugin.Registry
}

// NewPluginHandler creates a new plugin handler
func NewPluginHandler(registry *plugin.Registry) *PluginHandler {
	return &PluginHandler{
		registry: registry,
	}
}

// RegisterRoutes registers plugin management routes
func (h *PluginHandler) RegisterRoutes(g *echo.Group) {
	plugins := g.Group("/plugins")
	plugins.GET("", h.ListPlugins)
	plugins.GET("/:id", h.GetPlugin)
	plugins.POST("/:id/enable", h.EnablePlugin)
	plugins.POST("/:id/disable", h.DisablePlugin)
	plugins.PUT("/:id/config", h.UpdateConfig)
	plugins.GET("/:id/logs", h.GetLogs)
	plugins.POST("/:id/reload", h.ReloadPlugin)
}

// PluginResponse represents a plugin in API responses
type PluginResponse struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Author       string                 `json:"author,omitempty"`
	Type         string                 `json:"type"`
	Status       string                 `json:"status"`
	Enabled      bool                   `json:"enabled"`
	ConfigSchema map[string]interface{} `json:"config_schema,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Capabilities []string               `json:"capabilities,omitempty"`
}

// getPluginType returns the plugin type based on origin
func getPluginType(p *plugin.PluginInfo) string {
	if p.IsNative {
		return "native"
	}
	return "js"
}

// ListPlugins returns all installed plugins
func (h *PluginHandler) ListPlugins(c echo.Context) error {
	plugins := h.registry.ListPlugins()

	response := make([]PluginResponse, 0, len(plugins))
	for _, p := range plugins {
		if p.Manifest == nil {
			continue
		}
		response = append(response, PluginResponse{
			ID:           p.Manifest.ID,
			Name:         p.Manifest.Name,
			Description:  p.Manifest.Description,
			Version:      p.Manifest.Version,
			Author:       p.Manifest.Author,
			Type:         getPluginType(p),
			Status:       string(p.Status),
			Enabled:      p.Status == plugin.StatusLoaded,
			ConfigSchema: p.Manifest.ConfigSchema,
			Config:       p.Config,
			Error:        p.Error,
			Capabilities: p.Manifest.Skills,
		})
	}

	// Sort by name for stable ordering
	sort.Slice(response, func(i, j int) bool {
		return response[i].Name < response[j].Name
	})

	return c.JSON(http.StatusOK, response)
}

// GetPlugin returns a specific installed plugin
func (h *PluginHandler) GetPlugin(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil || p.Manifest == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	return c.JSON(http.StatusOK, PluginResponse{
		ID:           p.Manifest.ID,
		Name:         p.Manifest.Name,
		Description:  p.Manifest.Description,
		Version:      p.Manifest.Version,
		Author:       p.Manifest.Author,
		Type:         getPluginType(p),
		Status:       string(p.Status),
		Enabled:      p.Status == plugin.StatusLoaded,
		ConfigSchema: p.Manifest.ConfigSchema,
		Config:       p.Config,
		Error:        p.Error,
		Capabilities: p.Manifest.Skills,
	})
}

// EnablePlugin enables a plugin
func (h *PluginHandler) EnablePlugin(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	// Only start native plugins (JavaScript plugins don't have a running state)
	if p.IsNative {
		if err := h.registry.StartPlugin(c.Request().Context(), id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}
	} else {
		// For JavaScript plugins, just update the status to loaded
		if err := h.registry.SetPluginStatus(id, plugin.StatusLoaded); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "plugin enabled",
	})
}

// DisablePlugin disables a plugin
func (h *PluginHandler) DisablePlugin(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	// Only stop native plugins (JavaScript plugins don't have a running state)
	if p.IsNative {
		if err := h.registry.StopPlugin(c.Request().Context(), id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}
	} else {
		// For JavaScript plugins, just update the status to disabled
		if err := h.registry.SetPluginStatus(id, plugin.StatusDisabled); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "plugin disabled",
	})
}

// UpdateConfig updates plugin configuration
func (h *PluginHandler) UpdateConfig(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	var config map[string]interface{}
	if err := c.Bind(&config); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	// TODO: Implement config update and validation
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "configuration updated",
	})
}

// PluginLog represents a plugin log entry
type PluginLog struct {
	Timestamp string `json:"timestamp"`
	Level     string `json:"level"`
	Message   string `json:"message"`
}

// GetLogs returns plugin logs
func (h *PluginHandler) GetLogs(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	// TODO: Implement plugin-specific log retrieval
	// For now, return empty logs
	return c.JSON(http.StatusOK, []PluginLog{})
}

// ReloadPlugin reloads a plugin
func (h *PluginHandler) ReloadPlugin(c echo.Context) error {
	id := c.Param("id")
	p := h.registry.GetPlugin(id)
	if p == nil {
		return c.JSON(http.StatusNotFound, map[string]string{
			"error": "plugin not found",
		})
	}

	// TODO: Implement plugin reload logic
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "plugin reloaded",
	})
}
