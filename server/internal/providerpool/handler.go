package providerpool

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/ide"
)

// Pool is the main entry point for the provider pool functionality
type Pool struct {
	Registry          *Registry
	Discovery         *ModelDiscovery
	Router            *Router
	IDEDiscovery      *ide.Discovery
	UsageTracker      *UsageTracker
	PricingManager    *PricingManager
	Storage           Storage
	Config            *PoolConfig
	TrialQuotaManager *TrialQuotaManager
	pricingUpdater    *PricingUpdater
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
func NewPool(dataPath string, opts ...PoolOption) (*Pool, error) {
	// Create storage (no encryption)
	storage, err := NewFileStorage(dataPath)
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

	// Wire provider changes to rebuild the static candidate list.
	// Must be async: the callback fires inside Registry.mu.Lock(),
	// and RebuildCandidates needs Registry.mu.RLock() — sync call deadlocks.
	registry.onProviderChange = func(provider *Provider, action string) {
		go router.RebuildCandidates()
	}

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
			DefaultRoutingMode:       RoutingModeAuto,
			HealthCheckEnabled:       true,
			HealthCheckInterval:      60 * time.Second,
			HealthCheckTimeout:       10 * time.Second,
			IDEDiscoveryEnabled:      true,
			IDEDiscoveryScanInterval: 5 * time.Minute,
			UsageTrackingEnabled:     true,
			UsageRetentionDays:       30,
		},
		TrialQuotaManager: NewTrialQuotaManager(registry, dataPath, GetTrialLicense()),
	}

	for _, opt := range opts {
		opt(pool)
	}

	// Initialize built-in providers synchronously (required for chat to work immediately)
	pool.initBuiltinProviders()

	// Start background pricing updater (fetches remote model_pricing.json via GitHub/jsdelivr)
	pool.pricingUpdater = NewPricingUpdater(nil)
	pool.pricingUpdater.Start()

	// Remove trial provider if quota is already exhausted (e.g. zero quota, expired, tampered)
	if pool.TrialQuotaManager != nil && pool.TrialQuotaManager.IsExhausted() {
		pool.TrialQuotaManager.DeleteTrialProvider()
		fmt.Printf("[Pool] Trial provider removed at startup (reason: %s)\n",
			pool.TrialQuotaManager.GetStatus().ExhaustedReason)
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

	// Refresh models for all enabled providers (especially important for trial provider)
	// This runs in background to not block startup
	go func() {
		refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		providers := p.Registry.ListEnabled()
		successCount := 0
		failedProviders := []string{}

		for _, provider := range providers {
			if _, err := p.Discovery.FetchModels(refreshCtx, provider.ID); err != nil {
				failedProviders = append(failedProviders, provider.Name)
			} else {
				successCount++
			}
		}

		if len(failedProviders) > 0 {
			if successCount == 0 {
				// All providers failed - likely need configuration
				fmt.Printf("[Provider Pool] Note: Providers need configuration (API keys) to fetch models. Models will be fetched on demand when providers are properly configured.\n")
			} else {
				// Some providers succeeded, some failed
				fmt.Printf("[Provider Pool] Successfully refreshed %d provider(s). %d provider(s) need configuration: %v\n",
					successCount, len(failedProviders), failedProviders)
			}
		} else {
			fmt.Printf("[Provider Pool] Successfully refreshed models for all %d configured provider(s)\n", successCount)
		}

		// Rebuild candidate list after models are refreshed
		p.Router.RebuildCandidates()
	}()
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
// and updates existing builtin providers with new metadata (icon, description, etc.)
func (p *Pool) initBuiltinProviders() {
	existing := p.Registry.List()
	existingMap := make(map[string]*Provider)
	for _, provider := range existing {
		existingMap[provider.ID] = provider
	}

	builtins := BuiltinProviders()

	for _, builtin := range builtins {
		if existingProvider, exists := existingMap[builtin.ID]; exists {
			// Update existing builtin provider's metadata (but preserve user settings)
			needsUpdate := false

			// Update icon if changed
			if existingProvider.Icon != builtin.Icon {
				existingProvider.Icon = builtin.Icon
				needsUpdate = true
			}

			// Update description if changed
			if existingProvider.Description != builtin.Description {
				existingProvider.Description = builtin.Description
				needsUpdate = true
			}

			// Update name if changed
			if existingProvider.Name != builtin.Name {
				existingProvider.Name = builtin.Name
				needsUpdate = true
			}

			// Update base URL only if it was empty (user hasn't configured it)
			if existingProvider.BaseURL == "" && builtin.BaseURL != "" {
				existingProvider.BaseURL = builtin.BaseURL
				needsUpdate = true
			}

			// Update API version if changed
			if existingProvider.APIVersion != builtin.APIVersion {
				existingProvider.APIVersion = builtin.APIVersion
				needsUpdate = true
			}

			// Update API format if changed (important for trial provider)
			if existingProvider.APIFormat != builtin.APIFormat {
				existingProvider.APIFormat = builtin.APIFormat
				needsUpdate = true
			}

			// Update API keys if builtin has keys (always update for trial provider to get latest key)
			if len(builtin.APIKeys) > 0 {
				// Check if keys are different
				keysChanged := len(existingProvider.APIKeys) != len(builtin.APIKeys)
				if !keysChanged && len(builtin.APIKeys) > 0 {
					// Compare first key
					if len(existingProvider.APIKeys) == 0 || existingProvider.APIKeys[0].Key != builtin.APIKeys[0].Key {
						keysChanged = true
					}
				}
				if keysChanged {
					existingProvider.APIKeys = builtin.APIKeys
					needsUpdate = true
				}
			}

			// Update website if changed
			if existingProvider.Website != builtin.Website {
				existingProvider.Website = builtin.Website
				needsUpdate = true
			}

			// Update API key URL if changed
			if existingProvider.APIKeyURL != builtin.APIKeyURL {
				existingProvider.APIKeyURL = builtin.APIKeyURL
				needsUpdate = true
			}

			if needsUpdate {
				existingProvider.UpdatedAt = builtin.UpdatedAt
				p.Registry.Update(existingProvider)
			}
		} else {
			// Register new builtin provider
			p.Registry.Register(builtin)
		}
	}
}

// Handler provides HTTP handlers for the provider pool API
type Handler struct {
	pool *Pool

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for frequently accessed data (using ecache2 generic cache)
	modelsCache *cache.GenericCache[string]
	ideCache    *cache.GenericCache[string]
}

// NewHandler creates a new Handler
func NewHandler(pool *Pool) *Handler {
	return &Handler{
		pool: pool,
		modelsCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    100,
			DefaultTTL: 30 * time.Second,
		}, "providerpool_models"),
		ideCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    50,
			DefaultTTL: 10 * time.Second,
		}, "providerpool_ide"),
	}
}

// poolNotAvailable returns a standard error response when pool is nil
func (h *Handler) poolNotAvailable(c echo.Context) error {
	return c.JSON(http.StatusServiceUnavailable, map[string]string{
		"error": "provider pool not available",
	})
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
	g.PUT("/:id/allowed-models", h.UpdateAllowedModels)
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

	// Trial quota endpoint
	g.GET("/trial/quota", h.GetTrialQuota)
}

// RegisterModelRoutes registers model routes on a separate group
func (h *Handler) RegisterModelRoutes(g *echo.Group) {
	g.GET("", h.ListAllModels)
}

// RegisterIDERoutes registers IDE discovery routes on a separate group
func (h *Handler) RegisterIDERoutes(g *echo.Group) {
	g.GET("/scan", h.ScanIDEs)
	g.POST("/:type/connect", h.ConnectIDE)
	g.GET("/importable", h.GetImportableConfigs)
	g.POST("/import/:type", h.ImportIDEConfig)
	g.POST("/import-ext/:type", h.ImportExtensionConfig)
	g.POST("/import-cc-switch", h.ImportFromCCSwitch)
	g.GET("/env-hints", h.GetEnvHints)
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

// providerResponse is a slim JSON representation for the list endpoint.
// Omits internal/default-value fields like timestamps, detected format, health check details.
type providerResponse struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Type     ProviderType     `json:"type"`
	Location ProviderLocation `json:"location"`
	Enabled  bool             `json:"enabled"`
	Status   ProviderStatus   `json:"status"`

	BaseURL   string    `json:"base_url,omitempty"`
	APIFormat APIFormat `json:"api_format,omitempty"`

	APIKeys []APIKey `json:"api_keys,omitempty"`

	Priority      int      `json:"priority"`
	AllowedModels []string `json:"allowed_models,omitempty"`

	Icon        string `json:"icon,omitempty"`
	CustomIcon  string `json:"custom_icon,omitempty"`
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
	APIKeyURL   string `json:"api_key_url,omitempty"`

	Models []*modelResponse `json:"models"`
}

// modelResponse is a slim JSON representation for models with pricing.
type modelResponse struct {
	ID           string   `json:"id"`
	ProviderID   string   `json:"provider_id"`
	Name         string   `json:"name"`
	DisplayName  string   `json:"display_name"`
	Enabled      bool     `json:"enabled"`
	Capabilities []string `json:"capabilities"`

	InputPrice  float64 `json:"input_price"`
	OutputPrice float64 `json:"output_price"`
	CachePrice  float64 `json:"cache_price"`

	ContextWindow int `json:"context_window,omitempty"`
	MaxOutput     int `json:"max_output,omitempty"`
}

// capabilitiesToStrings converts ModelCapabilities to a list of enabled capability names.
func capabilitiesToStrings(c ModelCapabilities) []string {
	var caps []string
	if c.Chat {
		caps = append(caps, "chat")
	}
	if c.Completion {
		caps = append(caps, "completion")
	}
	if c.Vision {
		caps = append(caps, "vision")
	}
	if c.FunctionCall {
		caps = append(caps, "function_call")
	}
	if c.Streaming {
		caps = append(caps, "streaming")
	}
	if c.Thinking {
		caps = append(caps, "thinking")
	}
	if c.JSON {
		caps = append(caps, "json")
	}
	if c.SystemPrompt {
		caps = append(caps, "system_prompt")
	}
	if caps == nil {
		caps = []string{}
	}
	return caps
}

func toProviderResponse(p *Provider, models []*Model, pm *PricingManager) *providerResponse {
	mr := make([]*modelResponse, len(models))
	for i, m := range models {
		mr[i] = &modelResponse{
			ID:            m.ID,
			ProviderID:    m.ProviderID,
			Name:          m.Name,
			DisplayName:   m.DisplayName,
			Enabled:       m.Enabled,
			Capabilities:  capabilitiesToStrings(m.Capabilities),
			InputPrice:    m.InputPrice,
			OutputPrice:   m.OutputPrice,
			CachePrice:    m.CachePrice,
			ContextWindow: m.ContextWindow,
			MaxOutput:     m.MaxOutput,
		}
		// Enrich with pricing from PricingManager if model has no price set
		if mr[i].InputPrice == 0 && mr[i].OutputPrice == 0 && pm != nil {
			if pricing := pm.GetModelPricingWithHeuristics(m.ID, p.ID); pricing != nil {
				mr[i].InputPrice = pricing.InputPrice
				mr[i].OutputPrice = pricing.OutputPrice
				mr[i].CachePrice = pricing.CachePrice
			}
		}
	}
	return &providerResponse{
		ID:            p.ID,
		Name:          p.Name,
		Type:          p.Type,
		Location:      p.Location,
		Enabled:       p.Enabled,
		Status:        p.Status,
		BaseURL:       p.BaseURL,
		APIFormat:     p.APIFormat,
		APIKeys:       p.APIKeys,
		Priority:      p.Priority,
		AllowedModels: p.AllowedModels,
		Icon:          p.Icon,
		CustomIcon:    p.CustomIcon,
		Description:   p.Description,
		Website:       p.Website,
		APIKeyURL:     p.APIKeyURL,
		Models:        mr,
	}
}

// toModelResponses converts a slice of Model to modelResponse (capabilities as string array).
// If pm is non-nil, enriches pricing from PricingManager for models with no price set.
func toModelResponses(models []*Model, pm *PricingManager) []*modelResponse {
	result := make([]*modelResponse, len(models))
	for i, m := range models {
		result[i] = &modelResponse{
			ID:            m.ID,
			ProviderID:    m.ProviderID,
			Name:          m.Name,
			DisplayName:   m.DisplayName,
			Enabled:       m.Enabled,
			Capabilities:  capabilitiesToStrings(m.Capabilities),
			InputPrice:    m.InputPrice,
			OutputPrice:   m.OutputPrice,
			CachePrice:    m.CachePrice,
			ContextWindow: m.ContextWindow,
			MaxOutput:     m.MaxOutput,
		}
		if result[i].InputPrice == 0 && result[i].OutputPrice == 0 && pm != nil {
			if pricing := pm.GetModelPricingWithHeuristics(m.ID, m.ProviderID); pricing != nil {
				result[i].InputPrice = pricing.InputPrice
				result[i].OutputPrice = pricing.OutputPrice
				result[i].CachePrice = pricing.CachePrice
			}
		}
	}
	return result
}

// ListProviders returns all providers with their models inlined.
func (h *Handler) ListProviders(c echo.Context) error {
	if h.pool == nil || h.pool.Registry == nil {
		builtinProviders := BuiltinProviders()
		sanitizedProviders := sanitizeTrialProviders(builtinProviders)
		result := make([]*providerResponse, len(sanitizedProviders))
		for i, p := range sanitizedProviders {
			models := GetBuiltinModels(p.ID)
			if models == nil {
				models = []*Model{}
			}
			result[i] = toProviderResponse(p, models, nil)
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"providers": result,
			"total":     len(result),
		})
	}

	providers := h.pool.Registry.List()
	sanitizedProviders := sanitizeTrialProviders(providers)

	// Fetch models per provider with 1s total timeout.
	// GetFilteredModels reads cache/storage/builtin — normally instant.
	ctx, cancel := context.WithTimeout(c.Request().Context(), time.Second)
	defer cancel()

	result := make([]*providerResponse, len(sanitizedProviders))
	for i, p := range sanitizedProviders {
		var models []*Model
		if ctx.Err() == nil {
			if m, err := h.pool.Discovery.GetFilteredModels(p.ID); err == nil {
				models = m
			}
		}
		if models == nil {
			models = GetBuiltinModels(p.ID)
		}
		if models == nil {
			models = []*Model{}
		}
		result[i] = toProviderResponse(p, models, h.pool.PricingManager)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers": result,
		"total":     len(result),
	})
}

// sanitizeTrialProviders removes sensitive information from trial providers
func sanitizeTrialProviders(providers []*Provider) []*Provider {
	result := make([]*Provider, len(providers))
	for i, p := range providers {
		if p.Type == ProviderTypeTrial {
			// Create a copy with sensitive fields hidden
			sanitized := *p
			sanitized.BaseURL = ""
			sanitized.APIKeys = nil
			result[i] = &sanitized
		} else {
			result[i] = p
		}
	}
	return result
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

	// Set location (default to cloud if not specified)
	if provider.Location == "" {
		provider.Location = ProviderLocationCloud
	} else if provider.Location != ProviderLocationCloud && provider.Location != ProviderLocationLocal {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "location must be 'cloud' or 'local'"})
	}

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
	if updates.Location != "" {
		if updates.Location != ProviderLocationCloud && updates.Location != ProviderLocationLocal {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "location must be 'cloud' or 'local'"})
		}
		existing.Location = updates.Location
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

// TestProvider tests a provider connection, optionally with a specific API key.
func (h *Handler) TestProvider(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Optional: test with a specific API key
	var body struct {
		KeyID string `json:"key_id"`
	}
	_ = c.Bind(&body) // ignore bind errors — key_id is optional

	// Build a provider copy scoped to the target key
	testProvider := *provider // shallow copy
	var targetKey *APIKey
	if body.KeyID != "" {
		for i := range provider.APIKeys {
			if provider.APIKeys[i].ID == body.KeyID {
				targetKey = &provider.APIKeys[i]
				break
			}
		}
		if targetKey == nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "api key not found"})
		}
		testProvider.APIKeys = []APIKey{*targetKey}
	}

	// Use singleflight to deduplicate concurrent test requests for the same provider+key
	sfKey := fmt.Sprintf("test_provider:%s:%s", id, body.KeyID)
	result, err, _ := h.sfGroup.Do(sfKey, func() (interface{}, error) {
		checker := NewHTTPHealthChecker(10 * time.Second)
		return checker.Check(c.Request().Context(), &testProvider), nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	healthResult := result.(*HealthCheckResult)
	if targetKey != nil {
		healthResult.KeyID = targetKey.ID
		healthResult.KeyHash = targetKey.KeyHash
	}
	h.pool.Registry.SetHealth(id, healthResult)

	return c.JSON(http.StatusOK, healthResult)
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

// UpdateAllowedModels updates the allowed models for a provider
func (h *Handler) UpdateAllowedModels(c echo.Context) error {
	id := c.Param("id")

	provider, err := h.pool.Registry.Get(id)
	if err != nil {
		if err == ErrProviderNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "provider not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	var req struct {
		AllowedModels []string `json:"allowed_models"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	provider.AllowedModels = req.AllowedModels
	provider.UpdatedAt = time.Now()

	if err := h.pool.Registry.Update(provider); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":        "allowed models updated",
		"allowed_models": provider.AllowedModels,
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

	models, err := h.pool.Discovery.GetFilteredModels(id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	resp := toModelResponses(models, h.pool.PricingManager)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": resp,
		"total":  len(resp),
	})
}

// FetchProviderModels fetches models from a provider
func (h *Handler) FetchProviderModels(c echo.Context) error {
	id := c.Param("id")

	// Use singleflight to deduplicate concurrent requests for the same provider
	key := fmt.Sprintf("fetch_models:%s", id)
	result, err, _ := h.sfGroup.Do(key, func() (interface{}, error) {
		return h.pool.Discovery.FetchModels(c.Request().Context(), id)
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	models := result.([]*Model)
	resp := toModelResponses(models, h.pool.PricingManager)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": resp,
		"total":  len(resp),
	})
}

// ListAllModels returns all models from all providers
func (h *Handler) ListAllModels(c echo.Context) error {
	// Handle nil pool gracefully - return built-in models
	if h.pool == nil || h.pool.Discovery == nil {
		var allModels []*Model

		// Get models from all built-in providers
		for _, provider := range BuiltinProviders() {
			builtinModels := GetBuiltinModels(provider.ID)
			if builtinModels != nil {
				allModels = append(allModels, builtinModels...)
			}
		}

		resp := toModelResponses(allModels, nil)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"models": resp,
			"total":  len(resp),
		})
	}

	models := h.pool.Discovery.GetAllModels()
	resp := toModelResponses(models, h.pool.PricingManager)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"models": resp,
		"total":  len(resp),
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

// ScanIDEs scans for available IDEs and returns all results (including not found)
func (h *Handler) ScanIDEs(c echo.Context) error {
	// Handle nil pool gracefully
	if h.pool == nil || h.pool.IDEDiscovery == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"ides":         []interface{}{},
			"scan_results": []interface{}{},
			"total":        0,
		})
	}

	ctx := c.Request().Context()

	// Try cache first
	cacheKey := "ide_scan_results"
	if cached, ok := h.ideCache.Get(cacheKey); ok {
		return c.JSON(http.StatusOK, cached)
	}

	// Use singleflight to deduplicate concurrent scan requests
	result, err, _ := h.sfGroup.Do("scan_ides", func() (interface{}, error) {
		results, err := h.pool.IDEDiscovery.ScanAll(ctx)
		if err != nil {
			return nil, err
		}

		ides := h.pool.IDEDiscovery.GetDiscovered()

		response := map[string]interface{}{
			"ides":         ides,
			"scan_results": results,
			"total":        len(ides),
		}

		// Cache the result
		h.ideCache.Put(cacheKey, response)

		return response, nil
	})

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, result)
}

// ConnectIDE connects to an IDE
func (h *Handler) ConnectIDE(c echo.Context) error {
	if h.pool == nil || h.pool.IDEDiscovery == nil {
		return h.poolNotAvailable(c)
	}

	ideType := ide.IDEType(c.Param("type"))

	info, err := h.pool.IDEDiscovery.Connect(c.Request().Context(), ideType)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, info)
}

// GetImportableConfigs returns all IDE configurations that can be imported
func (h *Handler) GetImportableConfigs(c echo.Context) error {
	if h.pool == nil || h.pool.IDEDiscovery == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"configs": []interface{}{},
			"total":   0,
		})
	}

	configs, err := h.pool.IDEDiscovery.GetImportableConfigs(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Collect all existing API key hashes from the registry
	existingKeyHashes := make(map[string]bool)
	if h.pool.Registry != nil {
		for _, provider := range h.pool.Registry.List() {
			for _, apiKey := range provider.APIKeys {
				if apiKey.KeyHash != "" {
					existingKeyHashes[apiKey.KeyHash] = true
				}
			}
		}
	}

	// Mark configs whose API key already exists in the provider pool
	for _, cfg := range configs {
		if !cfg.CanImport {
			continue
		}
		realKey, err := h.pool.IDEDiscovery.GetRealAPIKey(cfg.IDEType)
		if err != nil || realKey == "" {
			continue
		}
		keyHash := HashAPIKey(realKey)
		if existingKeyHashes[keyHash] {
			cfg.AlreadyImported = true
			cfg.CanImport = false
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"configs": configs,
		"total":   len(configs),
	})
}

// ImportIDEConfig imports configuration from a specific IDE
func (h *Handler) ImportIDEConfig(c echo.Context) error {
	ideType := ide.IDEType(c.Param("type"))

	// Get the real API key
	apiKey, err := h.pool.IDEDiscovery.GetRealAPIKey(ideType)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// Get the base URL (from env var or config file)
	baseURL := h.pool.IDEDiscovery.GetRealBaseURL(ideType)

	// Determine provider ID: if a custom base URL is set, import as custom provider
	providerID := getProviderIDForIDE(ideType)
	isCustomURL := baseURL != ""
	if isCustomURL {
		providerID = "custom-" + string(ideType)
	}

	// Add API key to the provider
	key := &APIKey{
		Key:     apiKey,
		Label:   "Imported from " + string(ideType),
		KeyHash: HashAPIKey(apiKey),
		Enabled: true,
	}

	if err := h.pool.Registry.AddAPIKey(providerID, key); err != nil {
		if err == ErrProviderNotFound {
			if isCustomURL {
				// Create a new custom provider for the third-party API
				provider := &Provider{
					ID:        providerID,
					Name:      string(ideType) + " (Custom API)",
					Type:      ProviderTypeCustom,
					Location:  ProviderLocationCloud,
					Enabled:   true,
					Status:    ProviderStatusActive,
					BaseURL:   baseURL,
					APIFormat: getAPIFormatForIDE(ideType),
					Priority:  45,
					Icon:      getIconForIDE(ideType),
					APIKeys:   []APIKey{*key},
				}
				if regErr := h.pool.Registry.Register(provider); regErr != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": regErr.Error()})
				}
			} else {
				// Try to enable the builtin provider
				if enableErr := h.pool.Registry.Enable(providerID); enableErr != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "provider not found: " + providerID})
				}
				if err = h.pool.Registry.AddAPIKey(providerID, key); err != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
				}
			}
		} else {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	} else if isCustomURL {
		// Key added successfully, update base URL on existing custom provider
		if provider, err := h.pool.Registry.Get(providerID); err == nil {
			provider.BaseURL = baseURL
			h.pool.Registry.Update(provider)
		}
	}

	// Enable the provider
	h.pool.Registry.Enable(providerID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "configuration imported successfully",
		"provider_id": providerID,
		"ide_type":    ideType,
		"base_url":    baseURL,
	})
}

// ImportExtensionConfig imports Claude Code extension config from an IDE's settings.json
func (h *Handler) ImportExtensionConfig(c echo.Context) error {
	ideType := ide.IDEType(c.Param("type"))

	// Get the real (unmasked) extension env vars
	envVars, err := h.pool.IDEDiscovery.GetRealExtensionConfig(ideType)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	if len(envVars) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no Claude Code extension config found"})
	}

	// Group env vars by provider
	type providerConfig struct {
		apiKey  string
		baseURL string
	}
	providers := make(map[string]*providerConfig)

	for _, ev := range envVars {
		info, ok := ide.GetClaudeCodeEnvVarProvider(ev.Name)
		if !ok {
			continue
		}
		if providers[info.ProviderID] == nil {
			providers[info.ProviderID] = &providerConfig{}
		}
		switch info.FieldType {
		case "api_key", "auth_token":
			providers[info.ProviderID].apiKey = ev.Value
		case "base_url":
			providers[info.ProviderID].baseURL = ev.Value
		}
	}

	imported := []string{}
	for providerID, cfg := range providers {
		if cfg.apiKey == "" {
			continue
		}

		// If a custom base URL is set, create a custom provider instead of the builtin one
		targetID := providerID
		isCustomURL := cfg.baseURL != ""
		if isCustomURL {
			targetID = "custom-" + providerID + "-" + string(ideType)
		}

		key := &APIKey{
			Key:     cfg.apiKey,
			Label:   fmt.Sprintf("ide_import:%s", ideType),
			KeyHash: HashAPIKey(cfg.apiKey),
			Enabled: true,
		}

		if err := h.pool.Registry.AddAPIKey(targetID, key); err != nil {
			if err == ErrProviderNotFound {
				if isCustomURL {
					provider := &Provider{
						ID:        targetID,
						Name:      string(ideType) + " (Custom API)",
						Type:      ProviderTypeCustom,
						Location:  ProviderLocationCloud,
						Enabled:   true,
						Status:    ProviderStatusActive,
						BaseURL:   cfg.baseURL,
						APIFormat: getAPIFormatForProvider(providerID),
						Priority:  45,
						Icon:      getIconForIDE(ideType),
						APIKeys:   []APIKey{*key},
					}
					if regErr := h.pool.Registry.Register(provider); regErr != nil {
						continue
					}
				} else {
					if enableErr := h.pool.Registry.Enable(targetID); enableErr != nil {
						continue
					}
					if err = h.pool.Registry.AddAPIKey(targetID, key); err != nil {
						continue
					}
				}
			} else {
				continue
			}
		} else if isCustomURL {
			if provider, err := h.pool.Registry.Get(targetID); err == nil {
				provider.BaseURL = cfg.baseURL
				h.pool.Registry.Update(provider)
			}
		}

		h.pool.Registry.Enable(targetID)
		imported = append(imported, targetID)
	}

	if len(imported) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no valid provider configs found to import"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   fmt.Sprintf("imported %d provider(s) from Claude Code extension config", len(imported)),
		"providers": imported,
		"ide_type":  ideType,
	})
}

// ImportFromCCSwitch imports configuration from Claude Code's cc switch output
func (h *Handler) ImportFromCCSwitch(c echo.Context) error {
	var req struct {
		Output string `json:"output"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if req.Output == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "output is required"})
	}

	config, err := h.pool.IDEDiscovery.ImportFromClaudeCodeSwitch(req.Output)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"config":  config,
		"message": "configuration parsed successfully",
	})
}

// GetEnvHints returns environment variable hints for IDE configuration
func (h *Handler) GetEnvHints(c echo.Context) error {
	claudeKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeClaudeCode)
	cursorKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeCursor)
	windsurfKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeWindsurf)
	qoderKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeQoder)
	traeKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeTRAE)
	antigravityKey, _ := ide.GetAPIKeyFromEnv(ide.IDETypeAntigravity)

	hints := []map[string]interface{}{
		{
			"ide":      "claude-code",
			"name":     "Claude Code",
			"env_vars": []string{"ANTHROPIC_API_KEY", "CLAUDE_API_KEY"},
			"provider": "anthropic",
			"detected": claudeKey != "",
		},
		{
			"ide":      "cursor",
			"name":     "Cursor",
			"env_vars": []string{"CURSOR_API_KEY", "OPENAI_API_KEY"},
			"provider": "openai",
			"detected": cursorKey != "",
		},
		{
			"ide":      "windsurf",
			"name":     "Windsurf",
			"env_vars": []string{"WINDSURF_API_KEY", "OPENAI_API_KEY"},
			"provider": "openai",
			"detected": windsurfKey != "",
		},
		{
			"ide":      "qoder",
			"name":     "Qoder",
			"env_vars": []string{"QODER_API_KEY", "OPENAI_API_KEY"},
			"provider": "openai",
			"detected": qoderKey != "",
		},
		{
			"ide":      "trae",
			"name":     "TRAE",
			"env_vars": []string{"TRAE_API_KEY"},
			"provider": "custom",
			"detected": traeKey != "",
		},
		{
			"ide":      "antigravity",
			"name":     "Antigravity",
			"env_vars": []string{"ANTIGRAVITY_API_KEY", "GOOGLE_API_KEY"},
			"provider": "google",
			"detected": antigravityKey != "",
		},
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"hints": hints,
	})
}

// getProviderIDForIDE returns the provider ID for an IDE type
func getProviderIDForIDE(ideType ide.IDEType) string {
	switch ideType {
	case ide.IDETypeClaudeCode:
		return "anthropic"
	case ide.IDETypeCursor, ide.IDETypeWindsurf, ide.IDETypeQoder, ide.IDETypeKiro:
		return "openai"
	case ide.IDETypeAntigravity:
		return "google"
	case ide.IDETypeTRAE:
		return "custom"
	case ide.IDETypeCopilot:
		return "github"
	}
	return "openai"
}

// getAPIFormatForIDE returns the API format for an IDE type
func getAPIFormatForIDE(ideType ide.IDEType) APIFormat {
	switch ideType {
	case ide.IDETypeClaudeCode:
		return APIFormatAnthropic
	case ide.IDETypeAntigravity:
		return APIFormatGoogle
	default:
		return APIFormatOpenAI
	}
}

// getAPIFormatForProvider returns the API format based on provider ID
func getAPIFormatForProvider(providerID string) APIFormat {
	switch providerID {
	case "anthropic":
		return APIFormatAnthropic
	case "google":
		return APIFormatGoogle
	default:
		return APIFormatOpenAI
	}
}

// getIconForIDE returns the icon name for an IDE type
func getIconForIDE(ideType ide.IDEType) string {
	switch ideType {
	case ide.IDETypeClaudeCode:
		return "anthropic"
	case ide.IDETypeCursor:
		return "cursor"
	case ide.IDETypeWindsurf:
		return "windsurf"
	case ide.IDETypeAntigravity:
		return "google"
	default:
		return "custom"
	}
}

// Pricing handlers

// GetPricingConfig returns the current pricing configuration
func (h *Handler) GetPricingConfig(c echo.Context) error {
	if h.pool == nil || h.pool.PricingManager == nil {
		// Return built-in pricing when pool is unavailable
		config := GetBuiltinPricingConfig()
		return c.JSON(http.StatusOK, config)
	}

	// Get custom pricing from storage
	config := h.pool.PricingManager.GetPricingConfig()

	// Merge with built-in pricing (built-in as baseline, custom overrides)
	builtinConfig := GetBuiltinPricingConfig()

	// Start with built-in pricing
	for modelID, builtinPricing := range builtinConfig.CustomPricing {
		// Only add if not already in custom pricing
		if _, exists := config.CustomPricing[modelID]; !exists {
			config.CustomPricing[modelID] = builtinPricing
		}
	}

	return c.JSON(http.StatusOK, config)
}

// SetDefaultPricing updates the default pricing for unknown models
func (h *Handler) SetDefaultPricing(c echo.Context) error {
	if h.pool == nil || h.pool.PricingManager == nil {
		return h.poolNotAvailable(c)
	}

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
	if h.pool == nil || h.pool.PricingManager == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"pricing": []interface{}{},
			"total":   0,
		})
	}

	pricing := h.pool.PricingManager.ListCustomPricing()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"pricing": pricing,
		"total":   len(pricing),
	})
}

// SetModelPricing sets custom pricing for a specific model
func (h *Handler) SetModelPricing(c echo.Context) error {
	if h.pool == nil || h.pool.PricingManager == nil {
		return h.poolNotAvailable(c)
	}

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
	if h.pool == nil || h.pool.PricingManager == nil {
		return h.poolNotAvailable(c)
	}

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
		"period":            period,
		"records_processed": recalculatedCount,
		"old_total_cost":    totalOldCost,
		"new_total_cost":    totalNewCost,
		"cost_difference":   totalNewCost - totalOldCost,
		"message":           "costs recalculated (view only, historical records not modified)",
	})
}

// RegisterConfigRoutes registers configuration routes on a separate group
func (h *Handler) RegisterConfigRoutes(g *echo.Group) {
	g.GET("/routing-mode", h.GetRoutingMode)
	g.PUT("/routing-mode", h.SetRoutingMode)
	g.GET("/location-stats", h.GetLocationStats)
}

// GetRoutingMode returns the current routing mode
func (h *Handler) GetRoutingMode(c echo.Context) error {
	// Handle nil pool gracefully
	if h.pool == nil || h.pool.Config == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"mode": "auto",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"mode": h.pool.Config.DefaultRoutingMode,
	})
}

// SetRoutingMode sets the routing mode
func (h *Handler) SetRoutingMode(c echo.Context) error {
	var req struct {
		Mode RoutingMode `json:"mode"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Validate mode
	if req.Mode != RoutingModeAuto && req.Mode != RoutingModeCloud && req.Mode != RoutingModeLocal {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid routing mode"})
	}

	h.pool.Config.DefaultRoutingMode = req.Mode

	// Save config to storage
	if err := h.pool.Storage.SaveConfig(h.pool.Config); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"mode": h.pool.Config.DefaultRoutingMode,
	})
}

// GetLocationStats returns statistics about provider locations
func (h *Handler) GetLocationStats(c echo.Context) error {
	if h.pool == nil || h.pool.Registry == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"cloud_count":     0,
			"local_count":     0,
			"cloud_providers": []string{},
			"local_providers": []string{},
			"has_cloud":       false,
			"has_local":       false,
		})
	}

	providers := h.pool.Registry.ListEnabled()

	var cloudCount, localCount int
	var cloudProviders, localProviders []string

	for _, p := range providers {
		if p.Location == ProviderLocationCloud {
			cloudCount++
			cloudProviders = append(cloudProviders, p.ID)
		} else if p.Location == ProviderLocationLocal {
			localCount++
			localProviders = append(localProviders, p.ID)
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"cloud_count":     cloudCount,
		"local_count":     localCount,
		"cloud_providers": cloudProviders,
		"local_providers": localProviders,
		"has_cloud":       cloudCount > 0,
		"has_local":       localCount > 0,
	})
}

// GetTrialQuota returns the current trial quota status
func (h *Handler) GetTrialQuota(c echo.Context) error {
	if h.pool == nil || h.pool.TrialQuotaManager == nil {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"tokens_used":      0,
			"tokens_remaining": 0,
			"token_limit":      0,
			"exhausted":        true,
		})
	}

	status := h.pool.TrialQuotaManager.GetStatus()
	return c.JSON(http.StatusOK, status)
}
