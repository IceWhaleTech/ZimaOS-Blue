package metrics

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/cache"
)

// CacheStatsProvider provides cache statistics
type CacheStatsProvider interface {
	Stats() map[string]interface{}
}

// Handler handles metrics API endpoints.
type Handler struct {
	writer        *MetricsWriter
	cacheProvider CacheStatsProvider

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data (using ecache2 generic cache)
	metricsCache *cache.GenericCache[string]
}

// NewHandler creates a new metrics handler.
func NewHandler(writer *MetricsWriter) *Handler {
	return &Handler{
		writer: writer,
		metricsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    100,
			DefaultTTL: 3 * time.Second,
		}, "metrics"),
	}
}

// SetCacheProvider sets the cache stats provider
func (h *Handler) SetCacheProvider(provider CacheStatsProvider) {
	h.cacheProvider = provider
}

// GetCallStats handles GET /api/v1/metrics/calls
func (h *Handler) GetCallStats(c echo.Context) error {
	period := c.QueryParam("period")
	if period == "" {
		period = PeriodDaily
	}

	stats := h.writer.callCollector.GetCallStatsByPeriod(period)
	hourlyStats := h.writer.callCollector.GetHourlyStats()

	response := &CallStatsResponse{
		Period: period,
		Stats:  stats,
		ByHour: hourlyStats,
	}

	return c.JSON(http.StatusOK, response)
}

// GetModelStats handles GET /api/v1/metrics/models
func (h *Handler) GetModelStats(c echo.Context) error {
	period := c.QueryParam("period")
	if period == "" {
		period = PeriodDaily
	}

	modelStats := h.writer.GetModelStats()

	// Calculate summary
	var totalCalls, totalTokens int64
	var totalCost float64
	for _, ms := range modelStats {
		totalCalls += ms.Calls
		totalTokens += ms.TotalTokens
		totalCost += ms.EstimatedCost
	}

	response := &ModelStatsResponse{
		Period: period,
		Models: modelStats,
		Summary: &StatsSummary{
			TotalCalls:  totalCalls,
			TotalTokens: totalTokens,
			TotalCost:   totalCost,
		},
	}

	return c.JSON(http.StatusOK, response)
}

// GetModelStatsByName handles GET /api/v1/metrics/models/:model
func (h *Handler) GetModelStatsByName(c echo.Context) error {
	model := c.Param("model")
	if model == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "model name is required")
	}

	stats := h.writer.callCollector.GetModelStatsByName(model)
	if stats == nil {
		return echo.NewHTTPError(http.StatusNotFound, "model not found")
	}

	return c.JSON(http.StatusOK, stats)
}

// GetTokenUsage handles GET /api/v1/metrics/tokens
func (h *Handler) GetTokenUsage(c echo.Context) error {
	period := c.QueryParam("period")
	if period == "" {
		period = PeriodDaily
	}

	usage := h.writer.GetTokenUsage()
	byModel := h.writer.GetAllModelUsage()

	response := &TokenUsageResponse{
		Period:  period,
		Usage:   usage,
		ByModel: byModel,
	}

	return c.JSON(http.StatusOK, response)
}

// GetLatencyStats handles GET /api/v1/metrics/latency
func (h *Handler) GetLatencyStats(c echo.Context) error {
	model := c.QueryParam("model")

	var stats *LatencyStats
	if model != "" {
		stats = h.writer.latencyTracker.GetLatencyStatsByModel(model)
		if stats == nil {
			return echo.NewHTTPError(http.StatusNotFound, "model not found")
		}
	} else {
		stats = h.writer.GetLatencyStats()
	}

	return c.JSON(http.StatusOK, stats)
}

// GetSpeedStats handles GET /api/v1/metrics/speed
func (h *Handler) GetSpeedStats(c echo.Context) error {
	model := c.QueryParam("model")

	if model != "" {
		speed := h.writer.latencyTracker.GetSpeedStatsByModel(model)
		if speed == nil {
			return echo.NewHTTPError(http.StatusNotFound, "model not found")
		}

		percentiles := h.writer.latencyTracker.GetSpeedPercentilesByModel(model)

		response := &SpeedResponse{
			Current: &SpeedStats{
				TokensPerSecond:    speed.TokensPerSecond,
				TimeToFirstTokenMs: speed.TimeToFirstToken,
				DecodeSpeed:        speed.DecodeSpeed,
			},
			Average: &SpeedStats{
				TokensPerSecond:    speed.AvgTokensPerSecond,
				TimeToFirstTokenMs: speed.AvgTimeToFirstToken,
			},
			Percentiles: percentiles,
		}

		return c.JSON(http.StatusOK, response)
	}

	speed := h.writer.GetSpeedStats()
	percentiles := h.writer.latencyTracker.GetSpeedPercentiles()

	response := &SpeedResponse{
		Current: &SpeedStats{
			TokensPerSecond:    speed.TokensPerSecond,
			TimeToFirstTokenMs: speed.TimeToFirstToken,
			DecodeSpeed:        speed.DecodeSpeed,
		},
		Average: &SpeedStats{
			TokensPerSecond:    speed.AvgTokensPerSecond,
			TimeToFirstTokenMs: speed.AvgTimeToFirstToken,
		},
		Percentiles: percentiles,
	}

	return c.JSON(http.StatusOK, response)
}

// GetModelSpeedStats handles GET /api/v1/metrics/speed/models
func (h *Handler) GetModelSpeedStats(c echo.Context) error {
	responses := h.writer.latencyTracker.GetModelSpeedResponses()
	return c.JSON(http.StatusOK, responses)
}

// GetSystemMetrics handles GET /api/v1/metrics/system
func (h *Handler) GetSystemMetrics(c echo.Context) error {
	metrics := h.writer.GetSystemMetrics()
	if metrics == nil {
		// Return empty object instead of error
		return c.JSON(http.StatusOK, &SystemResourceMetrics{})
	}

	return c.JSON(http.StatusOK, metrics)
}

// GetResourceHistory handles GET /api/v1/metrics/system/history
func (h *Handler) GetResourceHistory(c echo.Context) error {
	history := h.writer.GetResourceHistory()
	if history == nil {
		return c.JSON(http.StatusOK, []ResourceHistory{})
	}

	return c.JSON(http.StatusOK, history)
}

// GetProcessMetrics handles GET /api/v1/metrics/process
func (h *Handler) GetProcessMetrics(c echo.Context) error {
	metrics, err := h.writer.GetCurrentProcessMetrics()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get process metrics")
	}
	if metrics == nil {
		// Return empty object instead of error
		return c.JSON(http.StatusOK, &ProcessMetrics{})
	}

	return c.JSON(http.StatusOK, metrics)
}

// GetPricing handles GET /api/v1/metrics/pricing
func (h *Handler) GetPricing(c echo.Context) error {
	pricing := h.writer.tokenTracker.GetPricing()
	return c.JSON(http.StatusOK, pricing)
}

// GetPricingForModel handles GET /api/v1/metrics/pricing/:model
func (h *Handler) GetPricingForModel(c echo.Context) error {
	model := c.Param("model")
	if model == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "model name is required")
	}

	pricing := h.writer.tokenTracker.GetPricingForModel(model)
	return c.JSON(http.StatusOK, pricing)
}

// GetCacheStats handles GET /api/v1/metrics/cache
func (h *Handler) GetCacheStats(c echo.Context) error {
	if h.cacheProvider == nil {
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

	return c.JSON(http.StatusOK, h.cacheProvider.Stats())
}

// GetSummary handles GET /api/v1/metrics/summary
func (h *Handler) GetSummary(c echo.Context) error {
	// Try cache first
	cacheKey := "metrics_summary"
	if cached, ok := h.metricsCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent requests
	result, _, _ := h.sfGroup.Do("get_summary", func() (interface{}, error) {
		callStats := h.writer.GetCallStats()
		tokenUsage := h.writer.GetTokenUsage()
		latencyStats := h.writer.GetLatencyStats()
		speedStats := h.writer.GetSpeedStats()
		systemMetrics := h.writer.GetSystemMetrics()

		summary := map[string]interface{}{
			"calls":   callStats,
			"tokens":  tokenUsage,
			"latency": latencyStats,
			"speed":   speedStats,
		}

		if systemMetrics != nil {
			summary["system"] = systemMetrics
		}

		// Add cache stats if available
		if h.cacheProvider != nil {
			summary["cache"] = h.cacheProvider.Stats()
		}

		// Cache the result
		h.metricsCache.Put(cacheKey, summary)
		return summary, nil
	})

	return c.JSON(http.StatusOK, result)
}

// GetAll handles GET /api/v1/metrics/all - aggregated metrics endpoint
func (h *Handler) GetAll(c echo.Context) error {
	// Call stats
	callStats := h.writer.GetCallStats()
	hourlyStats := h.writer.callCollector.GetHourlyStats()
	if hourlyStats == nil {
		hourlyStats = []HourlyStats{}
	}

	// Model stats
	modelStats := h.writer.GetModelStats()
	if modelStats == nil {
		modelStats = []ModelStats{}
	}

	// Calculate model summary
	var totalCalls, totalTokens int64
	var totalCost float64
	for _, ms := range modelStats {
		totalCalls += ms.Calls
		totalTokens += ms.TotalTokens
		totalCost += ms.EstimatedCost
	}

	// Token usage
	tokenUsage := h.writer.GetTokenUsage()
	byModel := h.writer.GetAllModelUsage()
	if byModel == nil {
		byModel = []ModelTokenUsage{}
	}

	// Latency stats
	latencyStats := h.writer.GetLatencyStats()

	// Speed stats
	speed := h.writer.GetSpeedStats()
	percentiles := h.writer.latencyTracker.GetSpeedPercentiles()

	// System metrics
	systemMetrics := h.writer.GetSystemMetrics()

	// Resource history
	resourceHistory := h.writer.GetResourceHistory()
	var resourceHistoryResponse interface{}
	if resourceHistory == nil {
		resourceHistoryResponse = []ResourceHistory{}
	} else {
		resourceHistoryResponse = resourceHistory
	}

	// Process metrics
	processMetrics, _ := h.writer.GetCurrentProcessMetrics()

	// Pricing
	pricing := h.writer.tokenTracker.GetPricing()
	if pricing == nil {
		pricing = []TokenPricing{}
	}

	response := map[string]interface{}{
		"calls": map[string]interface{}{
			"period":  PeriodDaily,
			"stats":   callStats,
			"by_hour": hourlyStats,
		},
		"models": map[string]interface{}{
			"period": PeriodDaily,
			"models": modelStats,
			"summary": map[string]interface{}{
				"total_calls":  totalCalls,
				"total_tokens": totalTokens,
				"total_cost":   totalCost,
			},
		},
		"tokens": map[string]interface{}{
			"period":   PeriodDaily,
			"usage":    tokenUsage,
			"by_model": byModel,
		},
		"latency": latencyStats,
		"speed": map[string]interface{}{
			"current": map[string]interface{}{
				"tokens_per_second":      speed.TokensPerSecond,
				"time_to_first_token_ms": speed.TimeToFirstToken,
				"decode_speed":           speed.DecodeSpeed,
			},
			"average": map[string]interface{}{
				"tokens_per_second":      speed.AvgTokensPerSecond,
				"time_to_first_token_ms": speed.AvgTimeToFirstToken,
			},
			"percentiles": percentiles,
		},
		"system":           systemMetrics,
		"resource_history": resourceHistoryResponse,
		"process":          processMetrics,
		"pricing":          pricing,
	}

	// Add cache stats if available
	if h.cacheProvider != nil {
		response["cache"] = h.cacheProvider.Stats()
	}

	return c.JSON(http.StatusOK, response)
}

// ResetMetrics handles POST /api/v1/metrics/reset
func (h *Handler) ResetMetrics(c echo.Context) error {
	h.writer.Reset()
	return c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
		"message": "metrics reset successfully",
	})
}

// RegisterRoutes registers the metrics routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Summary and aggregated
	g.GET("/summary", h.GetSummary)
	g.GET("/all", h.GetAll)

	// Call statistics
	g.GET("/calls", h.GetCallStats)

	// Model statistics
	g.GET("/models", h.GetModelStats)
	g.GET("/models/:model", h.GetModelStatsByName)

	// Token usage
	g.GET("/tokens", h.GetTokenUsage)

	// Latency
	g.GET("/latency", h.GetLatencyStats)

	// Speed
	g.GET("/speed", h.GetSpeedStats)
	g.GET("/speed/models", h.GetModelSpeedStats)

	// System metrics
	g.GET("/system", h.GetSystemMetrics)
	g.GET("/system/history", h.GetResourceHistory)

	// Process metrics
	g.GET("/process", h.GetProcessMetrics)

	// Pricing
	g.GET("/pricing", h.GetPricing)
	g.GET("/pricing/:model", h.GetPricingForModel)

	// Cache (cc-cache)
	g.GET("/cache", h.GetCacheStats)

	// Admin
	g.POST("/reset", h.ResetMetrics)
}
