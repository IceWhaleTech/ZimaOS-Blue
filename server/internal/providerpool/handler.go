package providerpool

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/providerpool/ide"
)

// Pool is the main entry point for the provider pool functionality
type Pool struct {
	Registry       *Registry
	Discovery      *ModelDiscovery
	Router         *Router
	IDEDiscovery   *ide.Discovery
	UsageTracker   *UsageTracker
	PricingManager *PricingManager
	Storage        Storage
	Config         *PoolConfig
}

// PoolOption configures the Pool
type PoolOption func(*Pool)

// WithConfig sets the pool configuration
func WithConfig(config *PoolConfig) PoolOption {
	return func(p *Pool) {
		p.Config = config
	}
}

// NewPool creates a new Pool with all components
func NewPool(dataPath string, encryptionKey string, opts ...PoolOption) (*Pool, error) {
	// Create encryptor
	var encryptor *Encryptor
	if encryptionKey != "" {
		var err error
		encryptor, err = NewEncryptor(encryptionKey)
		if err != nil {
			return nil, err
		}
	}

	// Create storage
	storage, err := NewFileStorage(dataPath, encryptor)
	if err != nil {
		return nil, err
	}

	// Create registry
	registry, err := NewRegistry(storage)
	if err != nil {
		return nil, err
	}

	// Create discovery
	discovery := NewModelDiscovery(registry, storage, time.Hour)

	// Create router
	router := NewRouter(registry, discovery, RoutingStrategyPriority)

	// Create IDE discovery
	ideDiscovery := ide.NewDiscovery(10 * time.Second)

	// Create usage tracker
	usageTracker := NewUsageTracker(storage)

	// Create pricing manager
	pricingManager, err := NewPricingManager(storage)
	if err != nil {
		return nil, err
	}

	// Link pricing manager to usage tracker for automatic cost calculation
	usageTracker.SetPricingManager(pricingManager)

	pool := &Pool{
		Registry:       registry,
		Discovery:      discovery,
		Router:         router,
		IDEDiscovery:   ideDiscovery,
		UsageTracker:   usageTracker,
		PricingManager: pricingManager,
		Storage:        storage,
		Config: &PoolConfig{
			DefaultStrategy:          RoutingStrategyPriority,
			HealthCheckEnabled:       true,
			HealthCheckInterval:      60 * time.Second,
			HealthCheckTimeout:       10 * time.Second,
			IDEDiscoveryEnabled:      true,
			IDEDiscoveryScanInterval: 5 * time.Minute,
			UsageTrackingEnabled:     true,
			UsageRetentionDays:       30,
		},
	}

	for _, opt := range opts {
		opt(pool)
	}

	return pool, nil
}

// Start starts background services
func (p *Pool) Start(ctx context.Context) {
	// Start usage tracker
	if p.Config.UsageTrackingEnabled {
		p.UsageTracker.Start()
	}

	// Start health checking
	if p.Config.HealthCheckEnabled {
		checker := NewHTTPHealthChecker(p.Config.HealthCheckTimeout)
		p.Registry.StartHealthCheck(ctx, checker)
	}

	// Fix provider types for non-builtin providers that were incorrectly marked as builtin
	p.fixProviderTypes()

	// Deduplicate providers (merge duplicates from historical data)
	p.deduplicateProviders()

	// Initialize built-in providers
	p.initBuiltinProviders()
}

// Stop stops background services
func (p *Pool) Stop() {
	p.Registry.StopHealthCheck()
	p.UsageTracker.Stop()
}

// fixProviderTypes fixes providers that were incorrectly marked as builtin
// This handles migration issues where custom providers like "claude-code" or "custom"
// were saved with type "builtin" instead of "custom"
func (p *Pool) fixProviderTypes() {
	// Build a set of valid builtin provider IDs
	builtinIDs := make(map[string]bool)
	for _, builtin := range BuiltinProviders() {
		builtinIDs[builtin.ID] = true
	}

	// Check all existing providers
	existing := p.Registry.List()
	for _, provider := range existing {
		// If provider is marked as builtin but is not in the builtin list, fix it
		if provider.Type == ProviderTypeBuiltin && !builtinIDs[provider.ID] {
			provider.Type = ProviderTypeCustom
			p.Registry.Update(provider)
		}
	}
}

// deduplicateProviders merges duplicate custom providers from historical data
func (p *Pool) deduplicateProviders() {
	// Call the deduplication function from migration.go
	DeduplicateProviders(p)
}

// initBuiltinProviders initializes built-in providers if not already registered
func (p *Pool) initBuiltinProviders() {
	existing := p.Registry.List()
	existingIDs := make(map[string]bool)
	for _, provider := range existing {
		existingIDs[provider.ID] = true
	}

	for _, builtin := range BuiltinProviders() {
		if !existingIDs[builtin.ID] {
			p.Registry.Register(builtin)
		}
	}
}

// Handler provides HTTP handlers for the provider pool API
type Handler struct {
	pool *Pool
}

// NewHandler creates a new Handler
func NewHandler(pool *Pool) *Handler {
	return &Handler{pool: pool}
}

// RegisterRoutes registers all API routes on an Echo group
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Provider endpoints
	g.GET("", h.ListProviders)
	g.POST("", h.AddProvider)
	g.GET("/:id", h.GetProvider)
	g.PUT("/:id", h.UpdateProvider)
	g.DELETE("/:id", h.DeleteProvider)
	g.POST("/:id/enable", h.EnableProvider)
	g.POST("/:id/disable", h.DisableProvider)
	g.POST("/:id/test", h.TestProvider)
	g.PUT("/:id/params", h.UpdateModelParams)
	g.POST("/:id/detect", h.DetectCapabilities)
	g.PUT("/:id/icon", h.UpdateProviderIcon)
	g.DELETE("/:id/icon", h.DeleteProviderIcon)

	// Model endpoints
	g.GET("/:id/models", h.ListProviderModels)
	g.POST("/:id/models/fetch", h.FetchProviderModels)

	// API Key endpoints
	g.POST("/:id/keys", h.AddAPIKey)
	g.DELETE("/:id/keys/:keyId", h.RemoveAPIKey)

	// Usage endpoints
	g.GET("/usage", h.GetUsageStats)
	g.GET("/:id/usage", h.GetProviderUsage)
}

// RegisterModelRoutes registers model routes on a separate group
func (h *Handler) RegisterModelRoutes(g *echo.Group) {
	g.GET("", h.ListAllModels)
}

// RegisterIDERoutes registers IDE discovery routes on a separate group
func (h *Handler) RegisterIDERoutes(g *echo.Group) {
	g.GET("/scan", h.ScanIDEs)
	g.POST("/:type/connect", h.ConnectIDE)
}

// RegisterPricingRoutes registers pricing management routes on a separate group
func (h *Handler) RegisterPricingRoutes(g *echo.Group) {
	g.GET("", h.GetPricingConfig)
	g.PUT("/default", h.SetDefaultPricing)
	g.GET("/models", h.ListModelPricing)
	g.PUT("/models/:modelId", h.SetModelPricing)
	g.DELETE("/models/:modelId", h.RemoveModelPricing)
	g.POST("/recalculate", h.RecalculateCosts)
}

// Provider handlers

// ListProviders returns all providers
func (h *Handler) ListProviders(c echo.Context) error {
	providers := h.pool.Registry.List()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers": providers,
		"total":     len(providers),
	})
}

// AddProvider adds a new custom provider
func (h *Handler) AddProvider(c echo.Context) error {
	var provider Provider
	if err := c.Bind(&provider); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate
	if provider.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if provider.BaseURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "base_url is required"})
	}

	// Generate ID if not provided
	if provider.ID == "" {
		provider.ID = GenerateID("prov")
	}

	// Set defaults
	provider.Type = ProviderTypeCustom
	provider.Status = ProviderStatusInactive

	if err := h.pool.Registry.Register(&provider); err != nil {
		if err == ErrProviderExists {
			return c.JSON(http.StatusConflict, map[string]string{"error": "provider already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, provider)
}

// GetProvider returns a specific provider
func (h *Handler) GetProvider(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Get health status
	health, _ := h.pool.Registry.GetHealth(id)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider": provider,
		"health":   health,
	})
}

// UpdateProvider updates a provider
func (h *Handler) UpdateProvider(c echo.Context) error {
	id := c.Param("id")

	existing, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var updates Provider
	if err := c.Bind(&updates); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Apply updates
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.BaseURL != "" {
		existing.BaseURL = updates.BaseURL
	}
	if updates.Priority != 0 {
		existing.Priority = updates.Priority
	}
	if updates.Headers != nil {
		existing.Headers = updates.Headers
	}

	if err := h.pool.Registry.Update(existing); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, existing)
}

// DeleteProvider removes a provider
func (h *Handler) DeleteProvider(c echo.Context) error {
	id := c.Param("id")

	if err := h.pool.Registry.Unregister(id); err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// EnableProvider enables a provider
func (h *Handler) EnableProvider(c echo.Context) error {
	id := c.Param("id")

	if err := h.pool.Registry.Enable(id); err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "enabled"})
}

// DisableProvider disables a provider
func (h *Handler) DisableProvider(c echo.Context) error {
	id := c.Param("id")

	if err := h.pool.Registry.Disable(id); err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "disabled"})
}

// TestProvider tests a provider connection
func (h *Handler) TestProvider(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	checker := NewHTTPHealthChecker(10 * time.Second)
	result := checker.Check(c.Request().Context(), provider)

	h.pool.Registry.SetHealth(id, result)

	return c.JSON(http.StatusOK, result)
}

// UpdateModelParams updates model parameters for a provider
func (h *Handler) UpdateModelParams(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var params ModelParams
	if err := c.Bind(&params); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate temperature
	if params.Temperature != nil && (*params.Temperature < 0 || *params.Temperature > 2) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "temperature must be between 0 and 2"})
	}

	// Validate top_p
	if params.TopP != nil && (*params.TopP < 0 || *params.TopP > 1) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "top_p must be between 0 and 1"})
	}

	// Validate max_tokens
	if params.MaxTokens != nil && *params.MaxTokens < 1 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "max_tokens must be at least 1"})
	}

	// Preserve detected values if they exist
	if provider.ModelParams != nil {
		if params.DetectedMaxTokens == nil {
			params.DetectedMaxTokens = provider.ModelParams.DetectedMaxTokens
		}
		if params.DetectedAt == nil {
			params.DetectedAt = provider.ModelParams.DetectedAt
		}
	}

	provider.ModelParams = &params
	provider.UpdatedAt = time.Now()

	if err := h.pool.Registry.Update(provider); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":      "model parameters updated",
		"model_params": provider.ModelParams,
	})
}

// DetectCapabilities detects server capabilities for a provider
func (h *Handler) DetectCapabilities(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Get models for this provider to determine max tokens
	models, err := h.pool.Discovery.GetModels(id)
	if err != nil || len(models) == 0 {
		// Try to fetch models first
		models, err = h.pool.Discovery.FetchModels(c.Request().Context(), id)
		if err != nil {
			// If we can't fetch models, use default values based on provider type
			// This is not a fatal error for custom providers
			models = nil
		}
	}

	// Find the maximum output tokens from all models
	var detectedMaxTokens int
	for _, model := range models {
		if model.MaxOutput > detectedMaxTokens {
			detectedMaxTokens = model.MaxOutput
		}
	}

	// If no max output found from models, use a reasonable default based on provider
	if detectedMaxTokens == 0 {
		switch provider.ID {
		case "openai":
			detectedMaxTokens = 16384
		case "anthropic":
			detectedMaxTokens = 8192
		case "google":
			detectedMaxTokens = 8192
		case "deepseek":
			detectedMaxTokens = 8192
		default:
			detectedMaxTokens = 4096
		}
	}

	// Update provider with detected capabilities
	now := time.Now().Unix()
	if provider.ModelParams == nil {
		provider.ModelParams = &ModelParams{}
	}
	provider.ModelParams.DetectedMaxTokens = &detectedMaxTokens
	provider.ModelParams.DetectedAt = &now
	provider.UpdatedAt = time.Now()

	if err := h.pool.Registry.Update(provider); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":             "capabilities detected",
		"detected_max_tokens": detectedMaxTokens,
		"detected_at":         now,
	})
}

// UpdateProviderIcon updates the custom icon for a provider
func (h *Handler) UpdateProviderIcon(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Only allow custom icon for custom providers
	if provider.Type != ProviderTypeCustom {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "custom icon only allowed for custom providers"})
	}

	var req struct {
		Icon string `json:"icon"` // Base64 data URL (e.g., "data:image/png;base64,...")
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Icon == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "icon is required"})
	}

	// Validate icon format (should be a data URL or empty)
	if len(req.Icon) > 0 && len(req.Icon) > 500*1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "icon too large (max 500KB)"})
	}

	provider.CustomIcon = req.Icon
	provider.UpdatedAt = time.Now()

	if err := h.pool.Registry.Update(provider); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "icon updated",
		"custom_icon": provider.CustomIcon,
	})
}

// DeleteProviderIcon removes the custom icon for a provider
func (h *Handler) DeleteProviderIcon(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	provider.CustomIcon = ""
	provider.UpdatedAt = time.Now()

	if err := h.pool.Registry.Update(provider); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// Model handlers

// ListProviderModels returns models for a provider
func (h *Handler) ListProviderModels(c echo.Context) error {
	id := c.Param("id")

	models, err := h.pool.Discovery.GetModels(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
		"total":  len(models),
	})
}

// FetchProviderModels fetches models from a provider
func (h *Handler) FetchProviderModels(c echo.Context) error {
	id := c.Param("id")

	models, err := h.pool.Discovery.FetchModels(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
		"total":  len(models),
	})
}

// ListAllModels returns all models from all providers
func (h *Handler) ListAllModels(c echo.Context) error {
	models := h.pool.Discovery.GetAllModels()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": models,
		"total":  len(models),
	})
}

// API Key handlers

// AddAPIKey adds an API key to a provider
func (h *Handler) AddAPIKey(c echo.Context) error {
	id := c.Param("id")

	var req struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Key == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "key is required"})
	}

	apiKey := &APIKey{
		Key:     req.Key,
		Label:   req.Label,
		KeyHash: HashAPIKey(req.Key),
		Enabled: true,
	}

	if err := h.pool.Registry.AddAPIKey(id, apiKey); err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Return without the actual key
	apiKey.Key = ""
	return c.JSON(http.StatusCreated, apiKey)
}

// RemoveAPIKey removes an API key from a provider
func (h *Handler) RemoveAPIKey(c echo.Context) error {
	id := c.Param("id")
	keyID := c.Param("keyId")

	if err := h.pool.Registry.RemoveAPIKey(id, keyID); err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		if err == ErrAPIKeyNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "api key not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// Usage handlers

// GetUsageStats returns overall usage statistics
func (h *Handler) GetUsageStats(c echo.Context) error {
	// Get current stats
	currentStats := h.pool.UsageTracker.GetCurrentStats()

	// Get period from query
	period := c.QueryParam("period")
	if period == "" {
		period = "week"
	}

	var start, end time.Time
	end = time.Now()

	switch period {
	case "day":
		start = end.AddDate(0, 0, -1)
	case "week":
		start = end.AddDate(0, 0, -7)
	case "month":
		start = end.AddDate(0, -1, 0)
	default:
		start = end.AddDate(0, 0, -7)
	}

	providerStats, _ := h.pool.UsageTracker.GetProviderStats(start, end)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"current":   currentStats,
		"period":    period,
		"providers": providerStats,
	})
}

// GetProviderUsage returns usage for a specific provider
func (h *Handler) GetProviderUsage(c echo.Context) error {
	id := c.Param("id")

	summary, err := h.pool.UsageTracker.GetWeeklySummary(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	modelStats, _ := h.pool.UsageTracker.GetModelStats(id, time.Now().AddDate(0, 0, -7), time.Now())

	return c.JSON(http.StatusOK, map[string]interface{}{
		"summary": summary,
		"models":  modelStats,
	})
}

// IDE handlers

// ScanIDEs scans for available IDEs
func (h *Handler) ScanIDEs(c echo.Context) error {
	ides, err := h.pool.IDEDiscovery.Scan(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"ides":  ides,
		"total": len(ides),
	})
}

// ConnectIDE connects to an IDE
func (h *Handler) ConnectIDE(c echo.Context) error {
	ideType := ide.IDEType(c.Param("type"))

	info, err := h.pool.IDEDiscovery.Connect(c.Request().Context(), ideType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, info)
}

// Pricing handlers

// GetPricingConfig returns the current pricing configuration
func (h *Handler) GetPricingConfig(c echo.Context) error {
	config := h.pool.PricingManager.GetPricingConfig()
	return c.JSON(http.StatusOK, config)
}

// SetDefaultPricing updates the default pricing for unknown models
func (h *Handler) SetDefaultPricing(c echo.Context) error {
	var req struct {
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
		CachePrice  float64 `json:"cache_price"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.InputPrice < 0 || req.OutputPrice < 0 || req.CachePrice < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prices cannot be negative"})
	}

	if err := h.pool.PricingManager.SetDefaultPricing(req.InputPrice, req.OutputPrice, req.CachePrice); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":      "default pricing updated",
		"input_price":  req.InputPrice,
		"output_price": req.OutputPrice,
		"cache_price":  req.CachePrice,
	})
}

// ListModelPricing returns all custom model pricing configurations
func (h *Handler) ListModelPricing(c echo.Context) error {
	pricing := h.pool.PricingManager.ListCustomPricing()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"pricing": pricing,
		"total":   len(pricing),
	})
}

// SetModelPricing sets custom pricing for a specific model
func (h *Handler) SetModelPricing(c echo.Context) error {
	modelID := c.Param("modelId")

	var req struct {
		ProviderID  string  `json:"provider_id"`
		InputPrice  float64 `json:"input_price"`
		OutputPrice float64 `json:"output_price"`
		CachePrice  float64 `json:"cache_price"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.InputPrice < 0 || req.OutputPrice < 0 || req.CachePrice < 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "prices cannot be negative"})
	}

	pricing := &ModelPricing{
		ModelID:     modelID,
		ProviderID:  req.ProviderID,
		InputPrice:  req.InputPrice,
		OutputPrice: req.OutputPrice,
		CachePrice:  req.CachePrice,
	}

	if err := h.pool.PricingManager.SetModelPricing(pricing); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, pricing)
}

// RemoveModelPricing removes custom pricing for a model (reverts to default)
func (h *Handler) RemoveModelPricing(c echo.Context) error {
	modelID := c.Param("modelId")
	providerID := c.QueryParam("provider_id")

	if err := h.pool.PricingManager.RemoveModelPricing(modelID, providerID); err != nil {
		if err == ErrPricingNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "pricing not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.NoContent(http.StatusNoContent)
}

// RecalculateCosts recalculates costs for historical usage records
func (h *Handler) RecalculateCosts(c echo.Context) error {
	// Get time range from query params
	period := c.QueryParam("period")
	if period == "" {
		period = "week"
	}

	var start, end time.Time
	end = time.Now()

	switch period {
	case "day":
		start = end.AddDate(0, 0, -1)
	case "week":
		start = end.AddDate(0, 0, -7)
	case "month":
		start = end.AddDate(0, -1, 0)
	case "all":
		start = end.AddDate(-1, 0, 0) // Last year
	default:
		start = end.AddDate(0, 0, -7)
	}

	// Load usage records
	records, err := h.pool.Storage.LoadUsage("", start, end)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Recalculate costs
	var totalOldCost, totalNewCost float64
	recalculatedCount := 0

	for _, record := range records {
		totalOldCost += record.EstimatedCost

		newCost := h.pool.PricingManager.CalculateCost(
			record.ModelID,
			record.ProviderID,
			record.InputTokens,
			record.OutputTokens,
			record.CacheReadTokens,
		)
		totalNewCost += newCost
		recalculatedCount++
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"period":             period,
		"records_processed":  recalculatedCount,
		"old_total_cost":     totalOldCost,
		"new_total_cost":     totalNewCost,
		"cost_difference":    totalNewCost - totalOldCost,
		"message":            "costs recalculated (view only, historical records not modified)",
	})
}
