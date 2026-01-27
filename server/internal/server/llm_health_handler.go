package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/llm"
)

// LLMHealthHandler handles LLM health-related endpoints.
type LLMHealthHandler struct {
	chainManager *llm.ModelChainManager
	registry     *llm.ProviderRegistry
}

// NewLLMHealthHandler creates a new LLMHealthHandler.
func NewLLMHealthHandler(chainManager *llm.ModelChainManager, registry *llm.ProviderRegistry) *LLMHealthHandler {
	return &LLMHealthHandler{
		chainManager: chainManager,
		registry:     registry,
	}
}

// RegisterRoutes registers LLM health routes.
func (h *LLMHealthHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/llm/health", h.GetHealth)
	g.GET("/llm/providers", h.ListProviders)
	g.GET("/llm/chains", h.ListChains)
	g.GET("/llm/chains/:name", h.GetChain)
	g.GET("/llm/stats", h.GetStats)
}

// LLMHealthResponse represents overall LLM health.
type LLMHealthResponse struct {
	Status          string                   `json:"status"`
	HealthyCount    int                      `json:"healthy_count"`
	TotalCount      int                      `json:"total_count"`
	Chains          map[string]ChainHealth   `json:"chains"`
	LastHealthCheck time.Time                `json:"last_health_check"`
}

// ChainHealth represents a chain's health status.
type ChainHealth struct {
	Name         string             `json:"name"`
	Description  string             `json:"description"`
	Status       string             `json:"status"`
	HealthyCount int                `json:"healthy_count"`
	TotalCount   int                `json:"total_count"`
	Models       []ModelHealthInfo  `json:"models"`
}

// ModelHealthInfo represents a model's health information.
type ModelHealthInfo struct {
	Provider     string    `json:"provider"`
	Model        string    `json:"model"`
	Priority     int       `json:"priority"`
	Status       string    `json:"status"`
	SuccessCount int64     `json:"success_count"`
	FailureCount int64     `json:"failure_count"`
	AvgLatencyMs int64     `json:"avg_latency_ms"`
	LastSuccess  time.Time `json:"last_success,omitempty"`
	LastFailure  time.Time `json:"last_failure,omitempty"`
}

// GetHealth returns the overall LLM health status.
func (h *LLMHealthHandler) GetHealth(c echo.Context) error {
	if h.chainManager == nil {
		return c.JSON(http.StatusOK, LLMHealthResponse{
			Status:          "unknown",
			HealthyCount:    0,
			TotalCount:      0,
			Chains:          make(map[string]ChainHealth),
			LastHealthCheck: time.Now(),
		})
	}

	stats := h.chainManager.HealthCheck(c.Request().Context())

	response := LLMHealthResponse{
		Chains:          make(map[string]ChainHealth),
		LastHealthCheck: time.Now(),
	}

	totalHealthy := 0
	totalModels := 0

	for chainName, modelStats := range stats {
		chain, _ := h.chainManager.GetChain(chainName)
		chainHealth := ChainHealth{
			Name:   chainName,
			Models: make([]ModelHealthInfo, 0, len(modelStats)),
		}
		if chain != nil {
			chainHealth.Description = chain.Description()
		}

		chainHealthy := 0
		for _, ms := range modelStats {
			status := "unhealthy"
			if ms.Healthy {
				status = "healthy"
				chainHealthy++
				totalHealthy++
			}
			totalModels++

			chainHealth.Models = append(chainHealth.Models, ModelHealthInfo{
				Provider:     ms.Provider,
				Model:        ms.Model,
				Priority:     ms.Priority,
				Status:       status,
				SuccessCount: ms.SuccessCount,
				FailureCount: ms.FailureCount,
				AvgLatencyMs: ms.AvgLatency.Milliseconds(),
				LastSuccess:  ms.LastSuccess,
				LastFailure:  ms.LastFailure,
			})
		}

		chainHealth.HealthyCount = chainHealthy
		chainHealth.TotalCount = len(modelStats)
		if chainHealthy == len(modelStats) {
			chainHealth.Status = "healthy"
		} else if chainHealthy > 0 {
			chainHealth.Status = "degraded"
		} else {
			chainHealth.Status = "unhealthy"
		}

		response.Chains[chainName] = chainHealth
	}

	response.HealthyCount = totalHealthy
	response.TotalCount = totalModels

	if totalHealthy == totalModels && totalModels > 0 {
		response.Status = "healthy"
	} else if totalHealthy > 0 {
		response.Status = "degraded"
	} else {
		response.Status = "unhealthy"
	}

	return c.JSON(http.StatusOK, response)
}

// LLMProviderInfo represents provider information.
type LLMProviderInfo struct {
	Name   string   `json:"name"`
	Models []string `json:"models"`
}

// ListProviders returns all registered providers.
func (h *LLMHealthHandler) ListProviders(c echo.Context) error {
	if h.registry == nil {
		return c.JSON(http.StatusOK, []LLMProviderInfo{})
	}

	providers := h.registry.List()
	result := make([]LLMProviderInfo, 0, len(providers))

	for _, name := range providers {
		provider := h.registry.Get(name)
		if provider == nil {
			continue
		}
		result = append(result, LLMProviderInfo{
			Name:   name,
			Models: provider.Models(),
		})
	}

	return c.JSON(http.StatusOK, result)
}

// ChainInfo represents chain information.
type ChainInfo struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Default     bool              `json:"default"`
	Models      []ChainModelInfo  `json:"models"`
}

// ChainModelInfo represents a model in a chain.
type ChainModelInfo struct {
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	Priority   int    `json:"priority"`
	Weight     int    `json:"weight"`
	MaxRetries int    `json:"max_retries"`
	TimeoutMs  int64  `json:"timeout_ms"`
}

// ListChains returns all configured chains.
func (h *LLMHealthHandler) ListChains(c echo.Context) error {
	if h.chainManager == nil {
		return c.JSON(http.StatusOK, []ChainInfo{})
	}

	chainNames := h.chainManager.ListChains()
	defaultChain := h.chainManager.GetDefaultChain()

	result := make([]ChainInfo, 0, len(chainNames))
	for _, name := range chainNames {
		chain, ok := h.chainManager.GetChain(name)
		if !ok {
			continue
		}

		info := ChainInfo{
			Name:        chain.Name(),
			Description: chain.Description(),
			Default:     defaultChain != nil && chain.Name() == defaultChain.Name(),
			Models:      make([]ChainModelInfo, 0),
		}

		for _, m := range chain.GetModels() {
			info.Models = append(info.Models, ChainModelInfo{
				Provider:   m.Provider,
				Model:      m.Model,
				Priority:   m.Priority,
				Weight:     m.Weight,
				MaxRetries: m.MaxRetries,
				TimeoutMs:  m.Timeout.Milliseconds(),
			})
		}

		result = append(result, info)
	}

	return c.JSON(http.StatusOK, result)
}

// GetChain returns a specific chain's details.
func (h *LLMHealthHandler) GetChain(c echo.Context) error {
	name := c.Param("name")
	if h.chainManager == nil {
		return echo.NewHTTPError(http.StatusNotFound, "chain not found")
	}

	chain, ok := h.chainManager.GetChain(name)
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "chain not found")
	}

	defaultChain := h.chainManager.GetDefaultChain()
	info := ChainInfo{
		Name:        chain.Name(),
		Description: chain.Description(),
		Default:     defaultChain != nil && chain.Name() == defaultChain.Name(),
		Models:      make([]ChainModelInfo, 0),
	}

	for _, m := range chain.GetModels() {
		info.Models = append(info.Models, ChainModelInfo{
			Provider:   m.Provider,
			Model:      m.Model,
			Priority:   m.Priority,
			Weight:     m.Weight,
			MaxRetries: m.MaxRetries,
			TimeoutMs:  m.Timeout.Milliseconds(),
		})
	}

	return c.JSON(http.StatusOK, info)
}

// StatsResponse represents overall statistics.
type StatsResponse struct {
	TotalRequests   int64             `json:"total_requests"`
	TotalSuccesses  int64             `json:"total_successes"`
	TotalFailures   int64             `json:"total_failures"`
	SuccessRate     float64           `json:"success_rate"`
	AvgLatencyMs    int64             `json:"avg_latency_ms"`
	ChainStats      map[string]ChainStats `json:"chain_stats"`
}

// ChainStats represents statistics for a chain.
type ChainStats struct {
	TotalRequests  int64   `json:"total_requests"`
	TotalSuccesses int64   `json:"total_successes"`
	TotalFailures  int64   `json:"total_failures"`
	SuccessRate    float64 `json:"success_rate"`
	AvgLatencyMs   int64   `json:"avg_latency_ms"`
}

// GetStats returns overall LLM statistics.
func (h *LLMHealthHandler) GetStats(c echo.Context) error {
	if h.chainManager == nil {
		return c.JSON(http.StatusOK, StatsResponse{
			ChainStats: make(map[string]ChainStats),
		})
	}

	stats := h.chainManager.HealthCheck(c.Request().Context())

	response := StatsResponse{
		ChainStats: make(map[string]ChainStats),
	}

	var totalLatency int64
	var latencyCount int64

	for chainName, modelStats := range stats {
		chainStat := ChainStats{}
		var chainLatency int64
		var chainLatencyCount int64

		for _, ms := range modelStats {
			chainStat.TotalSuccesses += ms.SuccessCount
			chainStat.TotalFailures += ms.FailureCount
			if ms.SuccessCount > 0 {
				chainLatency += ms.AvgLatency.Milliseconds() * ms.SuccessCount
				chainLatencyCount += ms.SuccessCount
			}
		}

		chainStat.TotalRequests = chainStat.TotalSuccesses + chainStat.TotalFailures
		if chainStat.TotalRequests > 0 {
			chainStat.SuccessRate = float64(chainStat.TotalSuccesses) / float64(chainStat.TotalRequests)
		}
		if chainLatencyCount > 0 {
			chainStat.AvgLatencyMs = chainLatency / chainLatencyCount
		}

		response.ChainStats[chainName] = chainStat
		response.TotalSuccesses += chainStat.TotalSuccesses
		response.TotalFailures += chainStat.TotalFailures
		totalLatency += chainLatency
		latencyCount += chainLatencyCount
	}

	response.TotalRequests = response.TotalSuccesses + response.TotalFailures
	if response.TotalRequests > 0 {
		response.SuccessRate = float64(response.TotalSuccesses) / float64(response.TotalRequests)
	}
	if latencyCount > 0 {
		response.AvgLatencyMs = totalLatency / latencyCount
	}

	return c.JSON(http.StatusOK, response)
}
