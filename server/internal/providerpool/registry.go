package providerpool

import (
	"context"
	"sync"
	"time"
)

// Registry manages provider registration and lifecycle
type Registry struct {
	storage   Storage
	providers map[string]*Provider
	health    map[string]*HealthCheckResult
	mu        sync.RWMutex

	// Health check
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	healthCheckCancel   context.CancelFunc

	// Callbacks
	onProviderChange func(provider *Provider, action string)
}

// RegistryOption configures the Registry
type RegistryOption func(*Registry)

// WithHealthCheck enables periodic health checking
func WithHealthCheck(interval, timeout time.Duration) RegistryOption {
	return func(r *Registry) {
		r.healthCheckInterval = interval
		r.healthCheckTimeout = timeout
	}
}

// WithProviderChangeCallback sets a callback for provider changes
func WithProviderChangeCallback(cb func(provider *Provider, action string)) RegistryOption {
	return func(r *Registry) {
		r.onProviderChange = cb
	}
}

// NewRegistry creates a new Registry
func NewRegistry(storage Storage, opts ...RegistryOption) (*Registry, error) {
	r := &Registry{
		storage:             storage,
		providers:           make(map[string]*Provider),
		health:              make(map[string]*HealthCheckResult),
		healthCheckInterval: 60 * time.Second,
		healthCheckTimeout:  10 * time.Second,
	}

	for _, opt := range opts {
		opt(r)
	}

	// Load existing providers
	if err := r.loadProviders(); err != nil {
		return nil, err
	}

	// Load health status
	if healthStatus, err := storage.LoadHealthStatus(); err == nil {
		r.health = healthStatus
	}

	return r, nil
}

// loadProviders loads all providers from storage
func (r *Registry) loadProviders() error {
	providers, err := r.storage.LoadAllProviders()
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for _, p := range providers {
		r.providers[p.ID] = p
	}

	return nil
}

// Register adds a new provider to the registry
func (r *Registry) Register(provider *Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[provider.ID]; exists {
		return ErrProviderExists
	}

	// Set defaults
	if provider.CreatedAt.IsZero() {
		provider.CreatedAt = time.Now()
	}
	provider.UpdatedAt = time.Now()
	if provider.Status == "" {
		provider.Status = ProviderStatusInactive
	}

	// Save to storage
	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	r.providers[provider.ID] = provider

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "register")
	}

	return nil
}

// Unregister removes a provider from the registry
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	// Delete from storage
	if err := r.storage.DeleteProvider(id); err != nil {
		return err
	}

	delete(r.providers, id)
	delete(r.health, id)

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "unregister")
	}

	return nil
}

// Get returns a provider by ID
func (r *Registry) Get(id string) (*Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[id]
	if !exists {
		return nil, ErrProviderNotFound
	}

	return provider, nil
}

// List returns all registered providers
func (r *Registry) List() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}

	return providers
}

// ListEnabled returns all enabled providers
func (r *Registry) ListEnabled() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*Provider, 0)
	for _, p := range r.providers {
		if p.Enabled {
			providers = append(providers, p)
		}
	}

	return providers
}

// ListHealthy returns all healthy providers
func (r *Registry) ListHealthy() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*Provider, 0)
	for _, p := range r.providers {
		if p.Enabled && p.Status == ProviderStatusActive {
			providers = append(providers, p)
		}
	}

	return providers
}

// Update updates a provider's configuration
func (r *Registry) Update(provider *Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[provider.ID]; !exists {
		return ErrProviderNotFound
	}

	provider.UpdatedAt = time.Now()

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	r.providers[provider.ID] = provider

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "update")
	}

	return nil
}

// Enable enables a provider
func (r *Registry) Enable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Enabled = true
	provider.UpdatedAt = time.Now()

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "enable")
	}

	return nil
}

// Disable disables a provider
func (r *Registry) Disable(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Enabled = false
	provider.Status = ProviderStatusInactive
	provider.UpdatedAt = time.Now()

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "disable")
	}

	return nil
}

// UpdateStatus updates a provider's status
func (r *Registry) UpdateStatus(id string, status ProviderStatus, lastError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Status = status
	provider.UpdatedAt = time.Now()
	if lastError != "" {
		provider.LastError = lastError
		provider.LastErrorTime = time.Now()
	}

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	return nil
}

// GetHealth returns the health status of a provider
func (r *Registry) GetHealth(id string) (*HealthCheckResult, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result, exists := r.health[id]
	return result, exists
}

// SetHealth updates the health status of a provider
func (r *Registry) SetHealth(id string, result *HealthCheckResult) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.health[id] = result

	// Update provider status based on health
	if provider, exists := r.providers[id]; exists && provider.Enabled {
		if result.Healthy {
			provider.Status = ProviderStatusActive
		} else {
			provider.Status = ProviderStatusError
			provider.LastError = result.Error
			provider.LastErrorTime = result.CheckedAt
		}
		provider.LastHealthCheck = result.CheckedAt
	}
}

// StartHealthCheck starts periodic health checking
func (r *Registry) StartHealthCheck(ctx context.Context, checker HealthChecker) {
	if r.healthCheckInterval <= 0 {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	r.healthCheckCancel = cancel

	go func() {
		ticker := time.NewTicker(r.healthCheckInterval)
		defer ticker.Stop()

		// Run immediately
		r.runHealthCheck(ctx, checker)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.runHealthCheck(ctx, checker)
			}
		}
	}()
}

// StopHealthCheck stops periodic health checking
func (r *Registry) StopHealthCheck() {
	if r.healthCheckCancel != nil {
		r.healthCheckCancel()
		r.healthCheckCancel = nil
	}
}

// runHealthCheck runs health check for all enabled providers
func (r *Registry) runHealthCheck(ctx context.Context, checker HealthChecker) {
	providers := r.ListEnabled()

	for _, provider := range providers {
		checkCtx, cancel := context.WithTimeout(ctx, r.healthCheckTimeout)
		result := checker.Check(checkCtx, provider)
		cancel()

		r.SetHealth(provider.ID, result)
	}

	// Save health status
	r.mu.RLock()
	healthCopy := make(map[string]*HealthCheckResult)
	for k, v := range r.health {
		healthCopy[k] = v
	}
	r.mu.RUnlock()

	r.storage.SaveHealthStatus(healthCopy)
}

// AddAPIKey adds an API key to a provider
func (r *Registry) AddAPIKey(providerID string, key *APIKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return ErrProviderNotFound
	}

	// Generate ID if not set
	if key.ID == "" {
		key.ID = GenerateID("key")
	}
	if key.CreatedAt.IsZero() {
		key.CreatedAt = time.Now()
	}
	if key.KeyHash == "" {
		key.KeyHash = HashAPIKey(key.Key)
	}

	provider.APIKeys = append(provider.APIKeys, *key)
	provider.UpdatedAt = time.Now()

	return r.storage.SaveProvider(provider)
}

// RemoveAPIKey removes an API key from a provider
func (r *Registry) RemoveAPIKey(providerID, keyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return ErrProviderNotFound
	}

	found := false
	newKeys := make([]APIKey, 0, len(provider.APIKeys))
	for _, k := range provider.APIKeys {
		if k.ID != keyID {
			newKeys = append(newKeys, k)
		} else {
			found = true
		}
	}

	if !found {
		return ErrAPIKeyNotFound
	}

	provider.APIKeys = newKeys
	provider.UpdatedAt = time.Now()

	return r.storage.SaveProvider(provider)
}

// GetAPIKey returns an API key for a provider
func (r *Registry) GetAPIKey(providerID string) (*APIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return nil, ErrProviderNotFound
	}

	// Return first enabled key
	for i := range provider.APIKeys {
		if provider.APIKeys[i].Enabled {
			return &provider.APIKeys[i], nil
		}
	}

	if len(provider.APIKeys) > 0 {
		return &provider.APIKeys[0], nil
	}

	return nil, ErrNoAPIKey
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	Check(ctx context.Context, provider *Provider) *HealthCheckResult
}

// DefaultHealthChecker provides a basic health check implementation
type DefaultHealthChecker struct{}

// Check performs a basic health check
func (c *DefaultHealthChecker) Check(ctx context.Context, provider *Provider) *HealthCheckResult {
	start := time.Now()
	result := &HealthCheckResult{
		ProviderID: provider.ID,
		CheckedAt:  start,
	}

	// For now, just check if provider has API keys configured
	// Real implementation would make an HTTP request to the provider
	if len(provider.APIKeys) == 0 && provider.OAuth == nil {
		result.Healthy = false
		result.Error = "no credentials configured"
		return result
	}

	result.Healthy = true
	result.Latency = time.Since(start)
	return result
}

// PricingManager manages model pricing configuration
type PricingManager struct {
	storage Storage
	config  *PricingConfig
	mu      sync.RWMutex

	// Callback when pricing changes (for recalculating costs)
	onPricingChange func()
}

// NewPricingManager creates a new pricing manager
func NewPricingManager(storage Storage) (*PricingManager, error) {
	pm := &PricingManager{
		storage: storage,
	}

	// Load existing pricing config
	config, err := storage.LoadPricingConfig()
	if err != nil {
		// Use default config if not found
		config = DefaultPricingConfig()
	}
	pm.config = config

	return pm, nil
}

// SetPricingChangeCallback sets a callback for pricing changes
func (pm *PricingManager) SetPricingChangeCallback(cb func()) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.onPricingChange = cb
}

// GetPricingConfig returns the current pricing configuration
func (pm *PricingManager) GetPricingConfig() *PricingConfig {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// Return a copy to prevent external modification
	configCopy := &PricingConfig{
		DefaultInputPrice:  pm.config.DefaultInputPrice,
		DefaultOutputPrice: pm.config.DefaultOutputPrice,
		DefaultCachePrice:  pm.config.DefaultCachePrice,
		CustomPricing:      make(map[string]*ModelPricing),
		UpdatedAt:          pm.config.UpdatedAt,
	}
	for k, v := range pm.config.CustomPricing {
		pricingCopy := *v
		configCopy.CustomPricing[k] = &pricingCopy
	}
	return configCopy
}

// SetDefaultPricing updates the default pricing for unknown models
func (pm *PricingManager) SetDefaultPricing(inputPrice, outputPrice, cachePrice float64) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.config.DefaultInputPrice = inputPrice
	pm.config.DefaultOutputPrice = outputPrice
	pm.config.DefaultCachePrice = cachePrice
	pm.config.UpdatedAt = time.Now()

	if err := pm.storage.SavePricingConfig(pm.config); err != nil {
		return err
	}

	if pm.onPricingChange != nil {
		go pm.onPricingChange()
	}

	return nil
}

// SetModelPricing sets custom pricing for a specific model
func (pm *PricingManager) SetModelPricing(pricing *ModelPricing) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pricing.IsCustom = true
	pricing.UpdatedAt = time.Now()

	// Use model_id as key, or provider_id:model_id if provider-specific
	key := pricing.ModelID
	if pricing.ProviderID != "" {
		key = pricing.ProviderID + ":" + pricing.ModelID
	}

	pm.config.CustomPricing[key] = pricing
	pm.config.UpdatedAt = time.Now()

	if err := pm.storage.SavePricingConfig(pm.config); err != nil {
		return err
	}

	if pm.onPricingChange != nil {
		go pm.onPricingChange()
	}

	return nil
}

// RemoveModelPricing removes custom pricing for a model (reverts to default)
func (pm *PricingManager) RemoveModelPricing(modelID, providerID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	key := modelID
	if providerID != "" {
		key = providerID + ":" + modelID
	}

	if _, exists := pm.config.CustomPricing[key]; !exists {
		return ErrPricingNotFound
	}

	delete(pm.config.CustomPricing, key)
	pm.config.UpdatedAt = time.Now()

	if err := pm.storage.SavePricingConfig(pm.config); err != nil {
		return err
	}

	if pm.onPricingChange != nil {
		go pm.onPricingChange()
	}

	return nil
}

// GetModelPricing returns the pricing for a specific model
// It checks in order: provider-specific pricing, model pricing, default pricing
func (pm *PricingManager) GetModelPricing(modelID, providerID string) *ModelPricing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 1. Check provider-specific pricing
	if providerID != "" {
		key := providerID + ":" + modelID
		if pricing, exists := pm.config.CustomPricing[key]; exists {
			return pricing
		}
	}

	// 2. Check model-specific pricing (any provider)
	if pricing, exists := pm.config.CustomPricing[modelID]; exists {
		return pricing
	}

	// 3. Return default pricing for unknown models
	return &ModelPricing{
		ModelID:     modelID,
		ProviderID:  providerID,
		InputPrice:  pm.config.DefaultInputPrice,
		OutputPrice: pm.config.DefaultOutputPrice,
		CachePrice:  pm.config.DefaultCachePrice,
		IsCustom:    false,
		UpdatedAt:   pm.config.UpdatedAt,
	}
}

// CalculateCost calculates the cost for a usage record using current pricing
func (pm *PricingManager) CalculateCost(modelID, providerID string, inputTokens, outputTokens, cacheTokens int64) float64 {
	pricing := pm.GetModelPricing(modelID, providerID)

	// Prices are per 1M tokens
	inputCost := float64(inputTokens) * pricing.InputPrice / 1_000_000
	outputCost := float64(outputTokens) * pricing.OutputPrice / 1_000_000
	cacheCost := float64(cacheTokens) * pricing.CachePrice / 1_000_000

	return inputCost + outputCost + cacheCost
}

// ListCustomPricing returns all custom pricing configurations
func (pm *PricingManager) ListCustomPricing() []*ModelPricing {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*ModelPricing, 0, len(pm.config.CustomPricing))
	for _, pricing := range pm.config.CustomPricing {
		pricingCopy := *pricing
		result = append(result, &pricingCopy)
	}
	return result
}
