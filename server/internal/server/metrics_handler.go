package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MetricsHandler handles metrics-related API endpoints
type MetricsHandler struct {
	collector     *metrics.Collector
	cacheProvider ChatCacheFootprintProvider
}

// NewMetricsHandler creates a new metrics handler
func NewMetricsHandler(collector *metrics.Collector, cacheProvider ...ChatCacheFootprintProvider) *MetricsHandler {
	var provider ChatCacheFootprintProvider
	if len(cacheProvider) > 0 {
		provider = cacheProvider[0]
	}
	return &MetricsHandler{collector: collector, cacheProvider: provider}
}

// RegisterRoutes registers metrics routes on the given group
func (h *MetricsHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/system/metrics", h.GetCurrentMetrics)
	g.GET("/system/metrics/history", h.GetMetricsHistory)
}

// GetCurrentMetrics returns the current system metrics
func (h *MetricsHandler) GetCurrentMetrics(c echo.Context) error {
	snapshot := metrics.SystemMetrics{
		Timestamp: timeutil.NowTime(),
	}
	latest := h.collector.GetLatest()
	if latest != nil {
		snapshot = *latest
	}
	if h.cacheProvider != nil {
		footprint := h.cacheProvider.ChatCacheFootprint()
		snapshot.ConversationCacheEntries = footprint.ConversationCacheEntries
		snapshot.ConversationCacheBytes = footprint.ConversationCacheBytes
		snapshot.WarmupCacheEntries = footprint.WarmupCacheEntries
		snapshot.WarmupCacheBytes = footprint.WarmupCacheBytes
		snapshot.PromptToolSurfaceRefs = footprint.PromptToolSurfaceRefs
		snapshot.PromptToolSurfaceSharedEntries = footprint.PromptToolSurfaceSharedEntries
		snapshot.PromptToolSurfaceSharedBytes = footprint.PromptToolSurfaceSharedBytes
		snapshot.ProviderAffinityEntries = footprint.ProviderAffinityEntries
	}
	return c.JSON(http.StatusOK, snapshot)
}

// GetMetricsHistory returns historical metrics data
func (h *MetricsHandler) GetMetricsHistory(c echo.Context) error {
	durationStr := c.QueryParam("duration")
	if durationStr == "" {
		durationStr = "5m"
	}

	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid duration format. Use formats like '5m', '1h', '30s'",
		})
	}

	// Cap duration at 1 hour
	if duration > time.Hour {
		duration = time.Hour
	}

	history := h.collector.GetHistory(duration)
	return c.JSON(http.StatusOK, history)
}
