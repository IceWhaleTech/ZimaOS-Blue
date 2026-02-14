package proxy

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

// CacheAPIHandler provides HTTP handlers for cache management
type CacheAPIHandler struct {
	cache  *CCCache
	config *CacheConfig
}

// NewCacheAPIHandler creates a new cache API handler
func NewCacheAPIHandler(cache *CCCache, config *CacheConfig) *CacheAPIHandler {
	return &CacheAPIHandler{
		cache:  cache,
		config: config,
	}
}

// RegisterRoutes registers cache API routes
func (h *CacheAPIHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/stats", h.GetStats)
	g.GET("/config", h.GetConfig)
	g.PUT("/config", h.UpdateConfig)
	g.POST("/clear", h.ClearCache)
	g.POST("/warmup", h.TriggerWarmup)
	g.DELETE("/entry/:key", h.DeleteEntry)
}

// GetStats returns cache statistics
// GET /api/v1/proxy/cache/stats
func (h *CacheAPIHandler) GetStats(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled":     false,
			"entries":     0,
			"max_entries": 0,
			"hits":        0,
			"misses":      0,
			"evictions":   0,
			"bypasses":    0,
			"hit_rate":    0,
		})
	}

	return c.JSON(http.StatusOK, h.cache.Stats())
}

// GetConfig returns cache configuration
// GET /api/v1/proxy/cache/config
func (h *CacheAPIHandler) GetConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusOK, DefaultCacheConfig())
	}
	return c.JSON(http.StatusOK, h.config)
}

// UpdateConfig updates cache configuration (partial update)
// PUT /api/v1/proxy/cache/config
func (h *CacheAPIHandler) UpdateConfig(c echo.Context) error {
	if h.config == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cache not initialized",
		})
	}

	var update struct {
		Enabled      *bool   `json:"enabled"`
		MaxSize      *int    `json:"max_size"`
		MaxEntrySize *int    `json:"max_entry_size"`
		TTLSeconds   *int    `json:"ttl_seconds"`
	}

	if err := c.Bind(&update); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
	}

	// Apply updates
	if update.Enabled != nil {
		h.config.Enabled = *update.Enabled
	}
	if update.MaxSize != nil {
		h.config.MaxSize = *update.MaxSize
	}
	if update.MaxEntrySize != nil {
		h.config.MaxEntrySize = *update.MaxEntrySize
	}
	if update.TTLSeconds != nil {
		h.config.TTL = time.Duration(*update.TTLSeconds) * time.Second
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"config":  h.config,
	})
}

// ClearCache clears all cache entries
// POST /api/v1/proxy/cache/clear
func (h *CacheAPIHandler) ClearCache(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cache not initialized",
		})
	}

	h.cache.Clear()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "cache cleared",
	})
}

// DeleteEntry deletes a specific cache entry
// DELETE /api/v1/proxy/cache/entry/:key
func (h *CacheAPIHandler) DeleteEntry(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cache not initialized",
		})
	}

	key := c.Param("key")
	if key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "key is required",
		})
	}

	// Delete from L1
	h.cache.entries.Delete(key)

	// Delete from L2 disk
	if disk := h.cache.GetDisk(); disk != nil {
		disk.Delete(key)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"key":     key,
	})
}

// TriggerWarmup manually triggers cache warmup (load top-N from L2 disk to L1 memory)
// POST /api/v1/proxy/cache/warmup
func (h *CacheAPIHandler) TriggerWarmup(c echo.Context) error {
	if h.cache == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "cache not initialized",
		})
	}

	topN := 1000
	if h.config != nil && h.config.Warming.MaxRequests > 0 {
		topN = h.config.Warming.MaxRequests
	}

	loaded := h.cache.Warmup(topN)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success":        true,
		"entries_loaded": loaded,
	})
}
