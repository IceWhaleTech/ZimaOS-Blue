package providerpool

import (
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Registry manages provider registration and lifecycle
type Registry struct {
	storage   Storage
	providers map[string]*Provider
	mu        sync.RWMutex

	// Callbacks
	onProviderChange func(provider *Provider, action string)
	onStatusChange   func(providerID string, oldStatus, newStatus ProviderStatus) // status transition
}

// RegistryOption configures the Registry
type RegistryOption func(*Registry)

// WithProviderChangeCallback sets a callback for provider changes
func WithProviderChangeCallback(cb func(provider *Provider, action string)) RegistryOption {
	return func(r *Registry) {
		r.onProviderChange = cb
	}
}

// AddProviderChangeListener chains an additional callback onto the existing
// onProviderChange handler. The new listener fires after the original callback.
func (r *Registry) AddProviderChangeListener(cb func(provider *Provider, action string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	prev := r.onProviderChange
	if prev == nil {
		r.onProviderChange = cb
	} else {
		r.onProviderChange = func(provider *Provider, action string) {
			prev(provider, action)
			cb(provider, action)
		}
	}
}

// SetOnStatusChange sets a callback invoked when a provider's status transitions
// (e.g. active → error or error → active).
func (r *Registry) SetOnStatusChange(cb func(providerID string, oldStatus, newStatus ProviderStatus)) {
	r.mu.Lock()
	r.onStatusChange = cb
	r.mu.Unlock()
}

// NewRegistry creates a new Registry
func NewRegistry(storage Storage, opts ...RegistryOption) (*Registry, error) {
	r := &Registry{
		storage:   storage,
		providers: make(map[string]*Provider),
	}

	for _, opt := range opts {
		opt(r)
	}

	// Load existing providers
	if err := r.loadProviders(); err != nil {
		return nil, err
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

	// Get builtin providers to merge AlternateBaseURLs for existing providers
	builtinProviders := BuiltinProviders()
	builtinMap := make(map[string]*Provider)
	for _, bp := range builtinProviders {
		builtinMap[bp.ID] = bp
	}

	for _, p := range providers {
		// Merge AlternateBaseURLs from builtin provider if missing (for old data migration)
		if p.Type == ProviderTypeBuiltin && len(p.AlternateBaseURLs) == 0 {
			if bp, ok := builtinMap[p.ID]; ok {
				p.AlternateBaseURLs = bp.AlternateBaseURLs
			}
		}
		r.providers[p.ID] = p
	}

	return nil
}

// Register adds a new provider to the registry
func (r *Registry) Register(provider *Provider) error {
	if provider == nil {
		return ErrInvalidConfig
	}
	validatedID, err := ValidateProviderID(provider.ID)
	if err != nil {
		return err
	}
	provider.ID = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	normalizeProviderAPIKeys(provider)

	if _, exists := r.providers[provider.ID]; exists {
		return ErrProviderExists
	}

	// Set defaults
	if provider.CreatedAt.IsZero() {
		provider.CreatedAt = timeutil.NowTime()
	}
	provider.UpdatedAt = timeutil.NowTime()
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
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return err
	}
	id = validatedID

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

	if r.onProviderChange != nil {
		r.onProviderChange(provider, "unregister")
	}

	return nil
}

// Get returns a provider by ID
func (r *Registry) Get(id string) (*Provider, error) {
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return nil, err
	}
	id = validatedID

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
	if provider == nil {
		return ErrInvalidConfig
	}
	validatedID, err := ValidateProviderID(provider.ID)
	if err != nil {
		return err
	}
	provider.ID = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[provider.ID]; !exists {
		return ErrProviderNotFound
	}

	provider.UpdatedAt = timeutil.NowTime()

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
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return err
	}
	id = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Enabled = true
	provider.UpdatedAt = timeutil.NowTime()

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
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return err
	}
	id = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Enabled = false
	provider.Status = ProviderStatusInactive
	provider.UpdatedAt = timeutil.NowTime()

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
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return err
	}
	id = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	provider.Status = status
	provider.UpdatedAt = timeutil.NowTime()
	if lastError != "" {
		provider.LastError = lastError
		provider.LastErrorTime = timeutil.NowTime()
	}

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	return nil
}

// ClearError clears the error status of a provider, allowing manual retry
func (r *Registry) ClearError(id string) error {
	validatedID, err := ValidateProviderID(id)
	if err != nil {
		return err
	}
	id = validatedID

	r.mu.Lock()
	defer r.mu.Unlock()

	provider, exists := r.providers[id]
	if !exists {
		return ErrProviderNotFound
	}

	// Clear error state
	provider.LastError = ""
	provider.LastErrorTime = time.Time{}
	provider.Status = ProviderStatusActive

	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}

	return nil
}

// AddAPIKey adds an API key to a provider
func (r *Registry) AddAPIKey(providerID string, key *APIKey) error {
	validatedID, err := ValidateProviderID(providerID)
	if err != nil {
		return err
	}
	providerID = validatedID

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
		key.CreatedAt = timeutil.NowTime()
	}
	normalizeAPIKeyMetadata(key)

	// Deduplicate: skip if a key with the same hash already exists
	for _, existing := range provider.APIKeys {
		if apiKeyFingerprint(&existing) != "" && apiKeyFingerprint(&existing) == apiKeyFingerprint(key) {
			return nil // already exists, no-op
		}
	}

	provider.APIKeys = append(provider.APIKeys, *key)
	provider.UpdatedAt = timeutil.NowTime()
	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}
	if r.onProviderChange != nil {
		r.onProviderChange(provider, "add_api_key")
	}
	return nil
}

// RemoveAPIKey removes an API key from a provider
func (r *Registry) RemoveAPIKey(providerID, keyID string) error {
	validatedID, err := ValidateProviderID(providerID)
	if err != nil {
		return err
	}
	providerID = validatedID

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
	provider.UpdatedAt = timeutil.NowTime()
	if err := r.storage.SaveProvider(provider); err != nil {
		return err
	}
	if r.onProviderChange != nil {
		r.onProviderChange(provider, "remove_api_key")
	}
	return nil
}

// GetAPIKey returns an API key for a provider
func (r *Registry) GetAPIKey(providerID string) (*APIKey, error) {
	validatedID, err := ValidateProviderID(providerID)
	if err != nil {
		return nil, err
	}
	providerID = validatedID

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

// GetOAuthConfig returns the OAuth configuration for a provider, if connected.
func (r *Registry) GetOAuthConfig(providerID string) (*OAuthConfig, error) {
	validatedID, err := ValidateProviderID(providerID)
	if err != nil {
		return nil, err
	}
	providerID = validatedID

	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerID]
	if !exists {
		return nil, ErrProviderNotFound
	}

	if provider.OAuth == nil || !provider.OAuth.Connected {
		return nil, fmt.Errorf("no oauth configured for provider %s", providerID)
	}

	return provider.OAuth, nil
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
	pm.config.UpdatedAt = timeutil.NowTime()

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
	pricing.UpdatedAt = timeutil.NowTime()

	// Use model_id as key, or provider_id:model_id if provider-specific
	key := pricing.ModelID
	if pricing.ProviderID != "" {
		key = pricing.ProviderID + ":" + pricing.ModelID
	}

	pm.config.CustomPricing[key] = pricing
	pm.config.UpdatedAt = timeutil.NowTime()

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
	pm.config.UpdatedAt = timeutil.NowTime()

	if err := pm.storage.SavePricingConfig(pm.config); err != nil {
		return err
	}

	if pm.onPricingChange != nil {
		go pm.onPricingChange()
	}

	return nil
}

// GetModelPricing returns the pricing for a specific model
// It checks in order: provider-specific pricing, model pricing, heuristic match, default pricing
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

	// 3. Try heuristic matching against builtin pricing database
	if matched := MatchModelPricing(modelID); matched != nil {
		matched.ProviderID = providerID
		return matched
	}

	// 4. Return default pricing for truly unknown models
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
