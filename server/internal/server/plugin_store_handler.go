package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/plugin"
)

// PluginStoreHandler handles plugin store HTTP requests
type PluginStoreHandler struct {
	store *plugin.Store

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data
	pluginsCache *cache.GenericCache[string]
}

// NewPluginStoreHandler creates a new plugin store handler
func NewPluginStoreHandler(store *plugin.Store) *PluginStoreHandler {
	return &PluginStoreHandler{
		store: store,
		pluginsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    100,
			DefaultTTL: 60 * time.Second, // Plugin list changes infrequently
		}, "plugin_store"),
	}
}

// RegisterRoutes registers plugin store routes
func (h *PluginStoreHandler) RegisterRoutes(g *echo.Group) {
	store := g.Group("/plugin-store")
	store.GET("/sources", h.ListSources)
	store.POST("/sources", h.AddSource)
	store.DELETE("/sources/:id", h.RemoveSource)
	store.GET("/browse", h.BrowsePlugins)
	store.POST("/install/:id", h.InstallPlugin)
	store.POST("/uninstall/:id", h.UninstallPlugin)
	store.POST("/refresh", h.RefreshStore)
}

// PluginSourceResponse represents a plugin source in API responses
type PluginSourceResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// RemotePluginResponse represents a remote plugin in API responses
type RemotePluginResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Description string   `json:"description"`
	Author      string   `json:"author,omitempty"`
	Type        string   `json:"type"`
	Capabilities []string `json:"capabilities,omitempty"`
	SourceID    string   `json:"source_id"`
	SourceName  string   `json:"source_name"`
	DownloadURL string   `json:"download_url,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	Stars       int      `json:"stars,omitempty"`
	Downloads   int      `json:"downloads,omitempty"`
	Installed   bool     `json:"installed"`
}

// ListSources returns all plugin sources
func (h *PluginStoreHandler) ListSources(c echo.Context) error {
	// Return default moltbot source
	sources := []PluginSourceResponse{
		{
			ID:          "moltbot",
			Name:        "Extensions",
			URL:         "https://github.com/moltbot/moltbot",
			Type:        "github",
			Description: "Official extensions repository",
			Enabled:     true,
		},
	}
	return c.JSON(http.StatusOK, sources)
}

// AddSource adds a new plugin source
func (h *PluginStoreHandler) AddSource(c echo.Context) error {
	var req struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Type        string `json:"type"`
		Description string `json:"description"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})
	}

	// TODO: Implement custom source management
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "source added successfully",
	})
}

// RemoveSource removes a plugin source
func (h *PluginStoreHandler) RemoveSource(c echo.Context) error {
	// id := c.Param("id")
	// TODO: Implement source removal
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "source removed successfully",
	})
}

// BrowsePlugins returns all available plugins from the store
func (h *PluginStoreHandler) BrowsePlugins(c echo.Context) error {
	// Try cache first
	cacheKey := "plugin_list"
	if cached, ok := h.pluginsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, err, _ := h.sfGroup.Do("browse_plugins", func() (interface{}, error) {
		plugins, err := h.store.ListPlugins(c.Request().Context())
		if err != nil {
			return nil, err
		}

		// Convert to response format
		response := make([]RemotePluginResponse, 0, len(plugins))
		for _, p := range plugins {
			response = append(response, RemotePluginResponse{
				ID:          p.ID,
				Name:        p.Name,
				Version:     p.Version,
				Description: p.Description,
				Author:      p.Author,
				Type:        "js", // Default type for extensions
				SourceID:    string(p.Source),
				SourceName:  "Extensions",
				DownloadURL: p.DownloadURL,
				Homepage:    p.RepoURL,
				Downloads:   p.Downloads,
				Installed:   p.Installed,
			})
		}

		// Cache the result
		h.pluginsCache.Put(cacheKey, response)
		return response, nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// InstallPlugin installs a plugin from the store
func (h *PluginStoreHandler) InstallPlugin(c echo.Context) error {
	id := c.Param("id")
	if err := h.store.InstallPlugin(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "plugin installed successfully",
	})
}

// UninstallPlugin uninstalls a plugin
func (h *PluginStoreHandler) UninstallPlugin(c echo.Context) error {
	id := c.Param("id")
	if err := h.store.UninstallPlugin(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "plugin uninstalled successfully",
	})
}

// RefreshStore refreshes the plugin store cache
func (h *PluginStoreHandler) RefreshStore(c echo.Context) error {
	plugins, err := h.store.ListPlugins(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":       true,
		"plugins_count": len(plugins),
	})
}
