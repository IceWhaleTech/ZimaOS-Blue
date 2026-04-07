package providerpool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ModelDiscovery handles model discovery and caching
type ModelDiscovery struct {
	registry     *Registry
	storage      Storage
	oauthManager interface {
		GetCopilotAccessToken(providerID string) (string, error)
		GetCopilotEndpoint() string
	}
	client          *http.Client
	insecureClient  *http.Client
	cache           map[string][]*Model
	cacheTTL        time.Duration
	cacheAt         map[string]time.Time
	mu              sync.RWMutex
	fetching        map[string]bool // tracks in-flight async fetches
	onModelsChanged func(providerID string)
}

// NewModelDiscovery creates a new ModelDiscovery
func NewModelDiscovery(registry *Registry, storage Storage, cacheTTL time.Duration) *ModelDiscovery {
	return &ModelDiscovery{
		registry: registry,
		storage:  storage,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		insecureClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user-opted skip for self-signed certs
			},
		},
		cache:    make(map[string][]*Model),
		cacheTTL: cacheTTL,
		cacheAt:  make(map[string]time.Time),
		fetching: make(map[string]bool),
	}
}

// SetModelsChangedHook registers a callback that runs after fresh model data is
// written into discovery cache/storage. Callers typically use this to rebuild
// router snapshots so empty-model routing doesn't keep serving stale candidates.
func (d *ModelDiscovery) SetModelsChangedHook(hook func(providerID string)) {
	if d == nil {
		return
	}
	d.onModelsChanged = hook
}

func (d *ModelDiscovery) notifyModelsChanged(providerID string) {
	if d == nil || d.onModelsChanged == nil {
		return
	}
	d.onModelsChanged(providerID)
}

// SetOAuthManager sets the OAuth manager for Copilot token exchange
func (d *ModelDiscovery) SetOAuthManager(m interface {
	GetCopilotAccessToken(providerID string) (string, error)
	GetCopilotEndpoint() string
}) {
	d.oauthManager = m
}

// clientFor returns the appropriate HTTP client for the provider
func (d *ModelDiscovery) clientFor(provider *Provider) *http.Client {
	if provider.SkipTLSVerify {
		return d.insecureClient
	}
	return d.client
}

func (d *ModelDiscovery) storeResolvedModels(providerID string, models []*Model) {
	d.mu.Lock()
	d.cache[providerID] = models
	d.cacheAt[providerID] = timeutil.NowTime()
	d.mu.Unlock()

	if err := d.storage.SaveModels(providerID, models); err != nil {
		// Best effort only.
	}
	d.notifyModelsChanged(providerID)
}

func (d *ModelDiscovery) promoteProviderAfterSuccessfulFetch(providerID string) {
	if d == nil || d.registry == nil {
		return
	}

	provider, err := d.registry.Get(providerID)
	if err != nil || provider == nil || !provider.Enabled {
		return
	}
	if provider.Status == ProviderStatusActive && strings.TrimSpace(provider.LastError) == "" {
		return
	}

	// Best-effort status promotion so UI/runtime consumers stop treating a
	// successfully discovered provider as unavailable.
	_ = d.registry.UpdateStatus(providerID, ProviderStatusActive, "")
}

// FetchModels fetches models from a provider
func (d *ModelDiscovery) FetchModels(ctx context.Context, providerID string) ([]*Model, error) {
	provider, err := d.registry.Get(providerID)
	if err != nil {
		return nil, err
	}

	if isCatalogMetadataProvider(provider) {
		models := sortModelsByPreference(GetBuiltinModels(providerID))
		if len(models) == 0 {
			return nil, ErrModelNotFound
		}
		d.storeResolvedModels(providerID, models)
		return models, nil
	}

	// Try to fetch from API
	models, err := d.fetchFromAPI(ctx, provider)
	if err != nil {
		// Fall back to built-in models
		builtinModels := GetBuiltinModels(providerID)
		if builtinModels != nil {
			return sortModelsByPreference(builtinModels), nil
		}
		return nil, err
	}
	models = sortModelsByPreference(models)

	d.storeResolvedModels(providerID, models)
	d.promoteProviderAfterSuccessfulFetch(providerID)

	return models, nil
}

// GetModels returns cached models for a provider
func (d *ModelDiscovery) GetModels(providerID string) ([]*Model, error) {
	provider, providerErr := d.registry.Get(providerID)
	if providerErr == nil && isCatalogMetadataProvider(provider) {
		d.mu.RLock()
		models, exists := d.cache[providerID]
		cacheTime := d.cacheAt[providerID]
		d.mu.RUnlock()
		if exists && len(models) > 0 && timeutil.SinceTime(cacheTime) < d.cacheTTL {
			return models, nil
		}

		storedModels, err := d.storage.LoadModels(providerID)
		if err == nil && len(storedModels) > 0 {
			mergeBuiltinPricing(providerID, storedModels)
			mergeBuiltinMetadata(providerID, storedModels)
			d.mu.Lock()
			d.cache[providerID] = storedModels
			d.cacheAt[providerID] = timeutil.NowTime()
			d.mu.Unlock()
			return storedModels, nil
		}

		models = sortModelsByPreference(GetBuiltinModels(providerID))
		if len(models) == 0 {
			return nil, ErrModelNotFound
		}
		d.storeResolvedModels(providerID, models)
		return models, nil
	}

	// Trial providers fetch models from API like other providers
	d.mu.RLock()
	models, exists := d.cache[providerID]
	cacheTime := d.cacheAt[providerID]
	d.mu.RUnlock()

	// Check if cache is valid
	if exists && len(models) > 0 && timeutil.SinceTime(cacheTime) < d.cacheTTL {
		return models, nil
	}

	// Try to load from storage
	storedModels, err := d.storage.LoadModels(providerID)
	if err == nil && len(storedModels) > 0 {
		// Merge pricing info from built-in models
		mergeBuiltinPricing(providerID, storedModels)

		d.mu.Lock()
		d.cache[providerID] = storedModels
		d.cacheAt[providerID] = timeutil.NowTime()
		d.mu.Unlock()
		return storedModels, nil
	}

	// Fall back to built-in models
	builtinModels := GetBuiltinModels(providerID)
	if builtinModels != nil && len(builtinModels) > 0 {
		return builtinModels, nil
	}

	// No cached/stored/builtin models — trigger async fetch instead of blocking.
	// Pool.Start() also fetches in background; this covers the case where
	// ListProviders is called before Start() finishes.
	d.triggerAsyncFetch(providerID)

	return nil, ErrModelNotFound
}

// triggerAsyncFetch starts a background goroutine to fetch models for a provider.
// Deduplicates concurrent requests — only one fetch per provider at a time.
func (d *ModelDiscovery) triggerAsyncFetch(providerID string) {
	d.mu.Lock()
	if d.fetching[providerID] {
		d.mu.Unlock()
		return
	}
	d.fetching[providerID] = true
	d.mu.Unlock()

	go func() {
		defer func() {
			d.mu.Lock()
			delete(d.fetching, providerID)
			d.mu.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		d.FetchModels(ctx, providerID) //nolint:errcheck // best-effort background fetch
	}()
}

// GetFilteredModels returns models for a provider, filtered by AllowedModels if set
func (d *ModelDiscovery) GetFilteredModels(providerID string) ([]*Model, error) {
	models, err := d.GetModels(providerID)
	if err != nil {
		return nil, err
	}

	// Filter trial provider models to well-known models only
	if IsTrialProvider(providerID) {
		models = filterTrialModels(models)
	}

	// Get provider to check AllowedModels
	provider, err := d.registry.Get(providerID)
	if err != nil {
		return models, nil // Return all models if provider not found
	}

	allowedModels := effectiveAllowedModels(provider)
	if len(allowedModels) == 0 {
		return sortModelsByPreference(models), nil
	}

	// Filter models by AllowedModels while preserving the user's configured order.
	return filterModelsByAllowed(models, allowedModels), nil
}

// filterModelsByAllowed filters models to only include those in the allowed list
func filterModelsByAllowed(models []*Model, allowedModels []string) []*Model {
	allowedModels = normalizeAllowedModels(allowedModels)
	if len(allowedModels) == 0 {
		return models
	}

	byID := make(map[string]*Model, len(models))
	byName := make(map[string]*Model, len(models))
	for _, model := range models {
		if model == nil {
			continue
		}
		if id := strings.TrimSpace(model.ID); id != "" {
			byID[id] = model
		}
		if name := strings.TrimSpace(model.Name); name != "" {
			byName[name] = model
		}
	}

	filtered := make([]*Model, 0, len(allowedModels))
	seen := make(map[string]struct{}, len(allowedModels))
	appendUnique := func(model *Model) {
		if model == nil {
			return
		}
		key := preferredModelID(model)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		filtered = append(filtered, model)
	}

	for _, allowed := range allowedModels {
		if model := byID[allowed]; model != nil {
			appendUnique(model)
			continue
		}
		appendUnique(byName[allowed])
	}

	return filtered
}

// filterTrialModels filters trial provider models to only include well-known models.
func filterTrialModels(models []*Model) []*Model {
	filtered := make([]*Model, 0, len(models))
	for _, m := range models {
		lower := strings.ToLower(m.ID)
		if strings.Contains(lower, "claude") ||
			strings.Contains(lower, "opus") ||
			strings.Contains(lower, "haiku") ||
			strings.Contains(lower, "sonnet") ||
			strings.Contains(lower, "glm") ||
			strings.Contains(lower, "qwen") ||
			strings.Contains(lower, "k2.5") ||
			strings.Contains(lower, "m2.5") {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// GetAllModels returns all models from all providers (including built-in models from disabled providers)
func (d *ModelDiscovery) GetAllModels() []*Model {
	var allModels []*Model
	seenProviders := make(map[string]bool)

	// First, get models from all enabled providers (custom or built-in)
	enabledProviders := d.registry.ListEnabled()
	for _, provider := range enabledProviders {
		models, err := d.GetModels(provider.ID)
		if err != nil {
			continue
		}
		// Filter trial provider models to well-known models only
		if IsTrialProvider(provider.ID) {
			models = filterTrialModels(models)
		}
		allModels = append(allModels, models...)
		seenProviders[provider.ID] = true
	}

	// Then, add built-in models from all built-in providers (even if disabled)
	// This allows users to see all available models with pricing before enabling providers
	builtinProviders := BuiltinProviders()
	for _, builtinProvider := range builtinProviders {
		// Skip if we already got models from this provider (it's enabled)
		if seenProviders[builtinProvider.ID] {
			continue
		}

		// Get built-in models for this provider
		builtinModels := GetBuiltinModels(builtinProvider.ID)
		if builtinModels != nil {
			allModels = append(allModels, builtinModels...)
		}
	}

	return sortModelsByPreference(allModels)
}

// RefreshAll refreshes models for all enabled providers
func (d *ModelDiscovery) RefreshAll(ctx context.Context) error {
	providers := d.registry.ListEnabled()
	var lastErr error

	for _, provider := range providers {
		if _, err := d.FetchModels(ctx, provider.ID); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// GetModel returns a specific model by ID
func (d *ModelDiscovery) GetModel(providerID, modelID string) (*Model, error) {
	models, err := d.GetModels(providerID)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.ID == modelID || model.Name == modelID {
			return model, nil
		}
	}

	return nil, ErrModelNotFound
}

// FindModel searches for a model across all providers
func (d *ModelDiscovery) FindModel(modelID string) (*Model, *Provider, error) {
	providers := d.registry.ListEnabled()

	for _, provider := range providers {
		models, err := d.GetModels(provider.ID)
		if err != nil {
			continue
		}

		for _, model := range models {
			if model.ID == modelID || model.Name == modelID {
				return model, provider, nil
			}
		}
	}

	return nil, nil, ErrModelNotFound
}

// FindModelsByCapability finds models with specific capabilities
func (d *ModelDiscovery) FindModelsByCapability(cap ModelCapabilities) []*Model {
	allModels := d.GetAllModels()
	var matching []*Model

	for _, model := range allModels {
		if matchesCapabilities(model.Capabilities, cap) {
			matching = append(matching, model)
		}
	}

	return matching
}

// matchesCapabilities checks if model capabilities match required capabilities
func matchesCapabilities(model, required ModelCapabilities) bool {
	if required.Chat && !model.Chat {
		return false
	}
	if required.Vision && !model.Vision {
		return false
	}
	if required.FunctionCall && !model.FunctionCall {
		return false
	}
	if required.Streaming && !model.Streaming {
		return false
	}
	if required.Thinking && !model.Thinking {
		return false
	}
	if required.JSON && !model.JSON {
		return false
	}
	if required.SystemPrompt && !model.SystemPrompt {
		return false
	}
	return true
}

// fetchFromAPI fetches models from provider API
func (d *ModelDiscovery) fetchFromAPI(ctx context.Context, provider *Provider) ([]*Model, error) {
	if isCatalogMetadataProvider(provider) {
		return GetBuiltinModels(provider.ID), nil
	}

	if provider.BaseURL == "" {
		return nil, fmt.Errorf("no base URL configured")
	}

	// Get API key (optional for some providers like Ollama)
	apiKey, _ := d.registry.GetAPIKey(provider.ID)

	// Build request based on provider type
	var models []*Model
	var fetchErr error

	switch provider.ID {
	case "openai", "deepseek", "moonshot", "openrouter", "openrouter-free", "aihubmix", "codex":
		if apiKey == nil {
			return nil, ErrNoAPIKey
		}
		models, fetchErr = d.fetchOpenAIModels(ctx, provider, apiKey)
		// Set API format for built-in OpenAI-compatible providers
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatOpenAI
		}
	case "minimax":
		// MiniMax doesn't have a models endpoint, use built-in
		models = GetBuiltinModels("minimax")
		// Set API format for MiniMax
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatOpenAI
		}
	case "anthropic":
		// Anthropic doesn't have a models endpoint, use built-in
		models = GetBuiltinModels("anthropic")
		// Set API format for Anthropic
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatAnthropic
		}
	case "google":
		if apiKey == nil {
			return nil, ErrNoAPIKey
		}
		models, fetchErr = d.fetchGoogleModels(ctx, provider, apiKey)
		// Set API format for Google
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatGoogle
		}
	case "ollama":
		// Ollama doesn't require API key
		models, fetchErr = d.fetchOllamaModels(ctx, provider)
		// Set API format for Ollama
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatOllama
		}
	default:
		// For custom providers, try multiple API formats
		models, fetchErr = d.fetchCustomProviderModels(ctx, provider, apiKey)
	}

	if fetchErr != nil {
		return nil, fetchErr
	}

	return models, nil
}

// fetchCustomProviderModels tries multiple API formats for custom providers
// It also detects and updates the provider's APIFormat if not already set
func (d *ModelDiscovery) fetchCustomProviderModels(ctx context.Context, provider *Provider, apiKey *APIKey) ([]*Model, error) {
	baseURL := strings.TrimSuffix(provider.BaseURL, "/")
	var errors []string

	// Use a per-strategy timeout so we don't spend 30s on each failed attempt
	strategyTimeout := 8 * time.Second

	// Strategy 1: Try OpenAI-compatible endpoints
	s1Ctx, s1Cancel := context.WithTimeout(ctx, strategyTimeout)
	models, err := d.fetchOpenAIModels(s1Ctx, provider, apiKey)
	s1Cancel()
	if err == nil && len(models) > 0 {
		// Auto-detect API format
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatOpenAI
			d.registry.Update(provider) // Save detected format
		}
		return models, nil
	}
	if err != nil {
		errors = append(errors, fmt.Sprintf("OpenAI: %v", err))
	}

	// Strategy 2: Try Ollama-style endpoint (/api/tags)
	s2Ctx, s2Cancel := context.WithTimeout(ctx, strategyTimeout)
	models, err = d.tryOllamaStyleEndpoint(s2Ctx, baseURL, provider)
	s2Cancel()
	if err == nil && len(models) > 0 {
		// Mark models with provider ID
		for _, m := range models {
			m.ProviderID = provider.ID
		}
		// Auto-detect API format
		if provider.APIFormat == "" {
			provider.APIFormat = APIFormatOllama
			d.registry.Update(provider) // Save detected format
		}
		return models, nil
	}
	if err != nil {
		errors = append(errors, fmt.Sprintf("Ollama: %v", err))
	}

	// Strategy 3: Try LiteLLM-style endpoint (/model/info)
	s3Ctx, s3Cancel := context.WithTimeout(ctx, strategyTimeout)
	models, err = d.tryLiteLLMStyleEndpoint(s3Ctx, baseURL, apiKey, provider)
	s3Cancel()
	if err == nil && len(models) > 0 {
		for _, m := range models {
			m.ProviderID = provider.ID
		}
		return models, nil
	}
	if err != nil {
		if err == ErrAuthRequired {
			return nil, err
		}
		errors = append(errors, fmt.Sprintf("LiteLLM: %v", err))
	}

	// Return detailed error with all attempts
	if len(errors) > 0 {
		return nil, fmt.Errorf("failed to fetch models: %s", strings.Join(errors, "; "))
	}
	return nil, fmt.Errorf("no models found from any endpoint format")
}

// tryOllamaStyleEndpoint tries to fetch models using Ollama API format
func (d *ModelDiscovery) tryOllamaStyleEndpoint(ctx context.Context, baseURL string, provider *Provider) ([]*Model, error) {
	// Try multiple path combinations
	// baseURL might be "https://example.com" or "https://example.com/v1"
	var urls []string
	if strings.HasSuffix(baseURL, "/v1") {
		// Strip /v1 and try Ollama paths
		base := strings.TrimSuffix(baseURL, "/v1")
		urls = []string{
			base + "/api/tags",
			base + "/tags",
		}
	} else {
		urls = []string{
			baseURL + "/api/tags",
			baseURL + "/tags",
		}
	}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		resp, err := d.clientFor(provider).Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		// Skip HTML responses
		if len(body) > 0 && body[0] == '<' {
			continue
		}

		var result struct {
			Models []struct {
				Name       string `json:"name"`
				ModifiedAt string `json:"modified_at"`
				Size       int64  `json:"size"`
			} `json:"models"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		if len(result.Models) == 0 {
			continue
		}

		models := make([]*Model, 0, len(result.Models))
		for _, m := range result.Models {
			model := &Model{
				ID:          m.Name,
				Name:        m.Name,
				DisplayName: formatModelName(m.Name),
				Enabled:     true,
				Capabilities: ModelCapabilities{
					Chat:         true,
					FunctionCall: true,
					Streaming:    true,
					SystemPrompt: true,
				},
			}
			inferOllamaCapabilities(model)
			applyDiscoveredModelTypeHeuristics(model)
			models = append(models, model)
		}
		return models, nil
	}

	return nil, fmt.Errorf("ollama-style endpoint not available")
}

// tryLiteLLMStyleEndpoint tries to fetch models using LiteLLM API format
func (d *ModelDiscovery) tryLiteLLMStyleEndpoint(ctx context.Context, baseURL string, apiKey *APIKey, provider *Provider) ([]*Model, error) {
	// LiteLLM uses /model/info or /models/info
	var urls []string
	if strings.HasSuffix(baseURL, "/v1") {
		base := strings.TrimSuffix(baseURL, "/v1")
		urls = []string{
			base + "/model/info",
			base + "/models/info",
			baseURL + "/model/info",
		}
	} else {
		urls = []string{
			baseURL + "/model/info",
			baseURL + "/models/info",
			baseURL + "/v1/model/info",
		}
	}

	for _, url := range urls {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			continue
		}

		if apiKey != nil && apiKey.Key != "" {
			req.Header.Set("Authorization", "Bearer "+apiKey.Key)
		}

		resp, err := d.clientFor(provider).Do(req)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		// Check for authentication errors
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			return nil, ErrAuthRequired
		}

		if resp.StatusCode != http.StatusOK {
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		// Skip HTML responses
		if len(body) > 0 && body[0] == '<' {
			continue
		}

		// LiteLLM returns a map of model_name -> model_info
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err != nil {
			continue
		}

		// Check if it's the expected format (has "data" key with model info)
		if data, ok := result["data"].(map[string]interface{}); ok {
			models := make([]*Model, 0, len(data))
			for modelName := range data {
				model := &Model{
					ID:          modelName,
					Name:        modelName,
					DisplayName: formatModelName(modelName),
					Enabled:     true,
					Capabilities: ModelCapabilities{
						Chat:         true,
						FunctionCall: true,
						Streaming:    true,
						SystemPrompt: true,
					},
				}
				inferCapabilities(model)
				applyDiscoveredModelTypeHeuristics(model)
				models = append(models, model)
			}
			if len(models) > 0 {
				return models, nil
			}
		}
	}

	return nil, fmt.Errorf("litellm-style endpoint not available")
}

// fetchOpenAIModels fetches models from OpenAI-compatible API
func (d *ModelDiscovery) fetchOpenAIModels(ctx context.Context, provider *Provider, apiKey *APIKey) ([]*Model, error) {
	// Build list of URLs to try.
	// Different providers use different paths: /models, /v1/models.
	// Responses-style relays may store BaseURL as a concrete responses endpoint
	// (for example /v1/responses); strip that suffix before discovering /models.
	baseURL := strings.TrimSuffix(provider.BaseURL, "/")
	if provider != nil && provider.APIFormat == APIFormatResponses && isResponsesEndpointBaseURL(baseURL) {
		switch {
		case strings.HasSuffix(baseURL, "/v1/responses"):
			baseURL = strings.TrimSuffix(baseURL, "/v1/responses")
		case strings.HasSuffix(baseURL, "/responses"):
			baseURL = strings.TrimSuffix(baseURL, "/responses")
		}
	}
	var urls []string

	if strings.HasSuffix(baseURL, "/v1") {
		// Base URL already includes /v1, just append /models
		urls = []string{baseURL + "/models"}
	} else {
		// Try /v1/models first, then /models
		urls = []string{
			baseURL + "/v1/models",
			baseURL + "/models",
		}
	}

	var errors []string
	for _, url := range urls {
		// Try with API key first (returns key-scoped model list if provider supports it)
		result, err := d.tryFetchModels(ctx, url, apiKey, provider)
		if err != nil {
			// If auth failed with a key, retry without key to get the full model list
			if err == ErrAuthRequired && apiKey != nil && apiKey.Key != "" {
				result, err = d.tryFetchModels(ctx, url, nil, provider)
			}
			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", url, err))
				continue
			}
		}
		return d.parseOpenAIModelsResponse(result, provider)
	}

	if len(errors) > 0 {
		return nil, fmt.Errorf("tried endpoints: %s", strings.Join(errors, "; "))
	}
	return nil, fmt.Errorf("failed to fetch models from any endpoint")
}

// tryFetchModels attempts to fetch models from a single URL
func (d *ModelDiscovery) tryFetchModels(ctx context.Context, url string, apiKey *APIKey, provider *Provider) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Use OAuth token for Copilot
	if provider.APIFormat == APIFormatCopilot && d.oauthManager != nil {
		if token, err := d.oauthManager.GetCopilotAccessToken(provider.ID); err == nil && token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Editor-Version", "vscode/1.85.0")
		}
	} else if apiKey != nil && apiKey.Key != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey.Key)
	}

	resp, err := d.clientFor(provider).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check for authentication errors - these should stop all retries
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, ErrAuthRequired
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if response is HTML (error page) instead of JSON
	if len(body) > 0 && body[0] == '<' {
		return nil, fmt.Errorf("provider returned HTML instead of JSON - check base URL and API key")
	}

	return body, nil
}

// parseOpenAIModelsResponse parses the OpenAI models API response
func (d *ModelDiscovery) parseOpenAIModelsResponse(body []byte, provider *Provider) ([]*Model, error) {
	var result struct {
		Data []struct {
			ID      string                 `json:"id"`
			Object  string                 `json:"object"`
			Created int64                  `json:"created"`
			OwnedBy string                 `json:"owned_by"`
			Pricing map[string]interface{} `json:"pricing"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Get built-in models for this provider to merge pricing info
	builtinModels := GetBuiltinModels(provider.ID)
	builtinMap := make(map[string]*Model)
	for _, bm := range builtinModels {
		builtinMap[bm.ID] = bm
	}

	models := make([]*Model, 0, len(result.Data))
	isOpenRouterFreeProfile := provider.ID == "openrouter-free"
	for _, m := range result.Data {
		if isOpenRouterFreeProfile && !isOpenRouterFreeModel(m.ID, m.Pricing) {
			continue
		}

		model := &Model{
			ID:          m.ID,
			ProviderID:  provider.ID,
			Name:        m.ID,
			DisplayName: formatModelName(m.ID),
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:         true,
				FunctionCall: true,
				Streaming:    true,
				SystemPrompt: true,
			},
		}

		// Merge info from built-in model if available
		if builtin, ok := builtinMap[m.ID]; ok {
			model.DisplayName = builtin.DisplayName
			model.InputPrice = builtin.InputPrice
			model.OutputPrice = builtin.OutputPrice
			model.CachePrice = builtin.CachePrice
			model.ContextWindow = builtin.ContextWindow
			model.MaxOutput = builtin.MaxOutput
			model.Capabilities = builtin.Capabilities
		} else {
			// Infer capabilities from model name
			inferCapabilities(model)
		}
		applyDiscoveredModelTypeHeuristics(model)

		models = append(models, model)
	}

	return models, nil
}

func isOpenRouterFreeModel(modelID string, pricing map[string]interface{}) bool {
	// OpenRouter generally tags free-tier models with ":free".
	if strings.Contains(strings.ToLower(modelID), ":free") {
		return true
	}

	// Fallback: treat models with all parsed pricing dimensions == 0 as free.
	return openRouterPricingIsFree(pricing)
}

func openRouterPricingIsFree(pricing map[string]interface{}) bool {
	if len(pricing) == 0 {
		return false
	}

	hasParsedPrice := false
	for _, raw := range pricing {
		value, ok := parsePricingValue(raw)
		if !ok {
			continue
		}
		hasParsedPrice = true
		if value > 0 {
			return false
		}
	}

	return hasParsedPrice
}

func parsePricingValue(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return 0, false
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	case json.Number:
		n, err := v.Float64()
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// fetchGoogleModels fetches models from Google Gemini API
func (d *ModelDiscovery) fetchGoogleModels(ctx context.Context, provider *Provider, apiKey *APIKey) ([]*Model, error) {
	url := strings.TrimSuffix(provider.BaseURL, "/") + "/v1beta/models"
	if apiKey != nil && apiKey.Key != "" {
		url += "?key=" + apiKey.Key
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.clientFor(provider).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if response is HTML (error page) instead of JSON
	if len(body) > 0 && body[0] == '<' {
		return nil, fmt.Errorf("provider returned HTML instead of JSON - check base URL and API key")
	}

	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Get built-in models for this provider to merge pricing info
	builtinModels := GetBuiltinModels(provider.ID)
	builtinMap := make(map[string]*Model)
	for _, bm := range builtinModels {
		builtinMap[bm.ID] = bm
	}

	models := make([]*Model, 0, len(result.Models))
	for _, m := range result.Models {
		// Extract model ID from name (e.g., "models/gemini-pro" -> "gemini-pro")
		modelID := m.Name
		if strings.HasPrefix(modelID, "models/") {
			modelID = strings.TrimPrefix(modelID, "models/")
		}

		model := &Model{
			ID:            modelID,
			ProviderID:    provider.ID,
			Name:          modelID,
			DisplayName:   m.DisplayName,
			Description:   m.Description,
			Enabled:       true,
			ContextWindow: m.InputTokenLimit,
			MaxOutput:     m.OutputTokenLimit,
			Capabilities: ModelCapabilities{
				Chat:         contains(m.SupportedGenerationMethods, "generateContent"),
				Streaming:    true,
				SystemPrompt: true,
			},
		}

		// Merge pricing and capabilities from built-in model if available
		if builtin, ok := builtinMap[modelID]; ok {
			model.InputPrice = builtin.InputPrice
			model.OutputPrice = builtin.OutputPrice
			model.CachePrice = builtin.CachePrice
			if builtin.ContextWindow > 0 {
				model.ContextWindow = builtin.ContextWindow
			}
			if builtin.MaxOutput > 0 {
				model.MaxOutput = builtin.MaxOutput
			}
			model.Capabilities = builtin.Capabilities
		} else {
			// Infer additional capabilities
			if strings.Contains(modelID, "vision") || strings.Contains(modelID, "pro") {
				model.Capabilities.Vision = true
			}
			if strings.Contains(modelID, "pro") || strings.Contains(modelID, "ultra") {
				model.Capabilities.FunctionCall = true
			}
		}
		applyDiscoveredModelTypeHeuristics(model)

		models = append(models, model)
	}

	return models, nil
}

// fetchOllamaModels fetches models from Ollama API
func (d *ModelDiscovery) fetchOllamaModels(ctx context.Context, provider *Provider) ([]*Model, error) {
	url := strings.TrimSuffix(provider.BaseURL, "/") + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.clientFor(provider).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Check if response is HTML (error page) instead of JSON
	if len(body) > 0 && body[0] == '<' {
		return nil, fmt.Errorf("provider returned HTML instead of JSON - check base URL")
	}

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Get built-in models for this provider to merge pricing info
	builtinModels := GetBuiltinModels(provider.ID)
	builtinMap := make(map[string]*Model)
	for _, bm := range builtinModels {
		builtinMap[bm.ID] = bm
	}

	models := make([]*Model, 0, len(result.Models))
	for _, m := range result.Models {
		model := &Model{
			ID:          m.Name,
			ProviderID:  provider.ID,
			Name:        m.Name,
			DisplayName: formatModelName(m.Name),
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:         true,
				FunctionCall: true,
				Streaming:    true,
				SystemPrompt: true,
			},
		}

		// Merge info from built-in model if available
		if builtin, ok := builtinMap[m.Name]; ok {
			model.DisplayName = builtin.DisplayName
			model.InputPrice = builtin.InputPrice
			model.OutputPrice = builtin.OutputPrice
			model.CachePrice = builtin.CachePrice
			model.ContextWindow = builtin.ContextWindow
			model.MaxOutput = builtin.MaxOutput
			model.Capabilities = builtin.Capabilities
		} else {
			// Infer capabilities from model name
			inferOllamaCapabilities(model)
		}
		applyDiscoveredModelTypeHeuristics(model)

		models = append(models, model)
	}

	return models, nil
}

// formatModelName formats a model ID into a display name
func formatModelName(id string) string {
	// Remove common prefixes
	name := id
	prefixes := []string{"models/", "gpt-", "claude-", "gemini-"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(name), prefix) {
			name = name[len(prefix):]
			break
		}
	}

	// Replace separators with spaces and capitalize
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")

	// Capitalize first letter of each word
	words := strings.Fields(name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}

	return strings.Join(words, " ")
}

// inferCapabilities infers model capabilities from model name
func inferCapabilities(model *Model) {
	name := strings.ToLower(model.Name)

	// Vision capability
	if strings.Contains(name, "vision") || strings.Contains(name, "4o") ||
		strings.Contains(name, "gpt-4-turbo") || strings.Contains(name, "claude-3") {
		model.Capabilities.Vision = true
	}

	// Function calling
	if strings.Contains(name, "gpt-4") || strings.Contains(name, "gpt-3.5-turbo") ||
		strings.Contains(name, "claude") || strings.Contains(name, "gemini") {
		model.Capabilities.FunctionCall = true
	}

	// Thinking/reasoning
	if strings.Contains(name, "o1") || strings.Contains(name, "thinking") ||
		strings.Contains(name, "reasoner") {
		model.Capabilities.Thinking = true
	}

	// JSON mode
	if strings.Contains(name, "gpt-4") || strings.Contains(name, "gpt-3.5-turbo") ||
		strings.Contains(name, "claude") {
		model.Capabilities.JSON = true
	}
}

// inferOllamaCapabilities infers capabilities for Ollama models
func inferOllamaCapabilities(model *Model) {
	name := strings.ToLower(model.Name)

	// Vision capability
	if strings.Contains(name, "llava") || strings.Contains(name, "vision") ||
		strings.Contains(name, "bakllava") {
		model.Capabilities.Vision = true
	}

	// Most Ollama models don't have native function calling
	model.Capabilities.FunctionCall = false

	// Code models
	if strings.Contains(name, "code") || strings.Contains(name, "coder") ||
		strings.Contains(name, "starcoder") || strings.Contains(name, "codellama") {
		model.Capabilities.Completion = true
	}
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// mergeBuiltinPricing merges pricing info from built-in models into the given models
func mergeBuiltinPricing(providerID string, models []*Model) {
	builtinModels := GetBuiltinModels(providerID)
	if builtinModels == nil {
		return
	}

	builtinMap := make(map[string]*Model)
	for _, bm := range builtinModels {
		builtinMap[bm.ID] = bm
	}

	for _, model := range models {
		if builtin, ok := builtinMap[model.ID]; ok {
			// Only merge if model doesn't have pricing set
			if model.InputPrice == 0 && builtin.InputPrice > 0 {
				model.InputPrice = builtin.InputPrice
			}
			if model.OutputPrice == 0 && builtin.OutputPrice > 0 {
				model.OutputPrice = builtin.OutputPrice
			}
			if model.CachePrice == 0 && builtin.CachePrice > 0 {
				model.CachePrice = builtin.CachePrice
			}
			// Also merge context window and max output if not set
			if model.ContextWindow == 0 && builtin.ContextWindow > 0 {
				model.ContextWindow = builtin.ContextWindow
			}
			if model.MaxOutput == 0 && builtin.MaxOutput > 0 {
				model.MaxOutput = builtin.MaxOutput
			}
		}
	}
}

func mergeBuiltinMetadata(providerID string, models []*Model) {
	builtinModels := GetBuiltinModels(providerID)
	if builtinModels == nil {
		return
	}

	builtinMap := make(map[string]*Model, len(builtinModels))
	for _, builtin := range builtinModels {
		if builtin == nil {
			continue
		}
		builtinMap[builtin.ID] = builtin
	}

	for _, model := range models {
		if model == nil {
			continue
		}
		builtin := builtinMap[model.ID]
		if builtin == nil {
			continue
		}
		model.ProviderID = providerID
		model.Name = builtin.Name
		model.DisplayName = builtin.DisplayName
		model.Description = builtin.Description
		model.ContextWindow = builtin.ContextWindow
		model.MaxOutput = builtin.MaxOutput
		model.Capabilities = builtin.Capabilities
	}
}

// ProbeResult holds the result of probing a single model.
type ProbeResult struct {
	ModelID    string        `json:"model_id"`
	Available  bool          `json:"available"`
	StatusCode int           `json:"status_code"`
	Error      string        `json:"error,omitempty"`
	Latency    time.Duration `json:"latency"`
}

// ProbeModels sends a minimal chat completion request to each model in parallel
// to determine which ones are actually configured and usable. Models that return
// a "not configured" error (HTTP 400/404 with model-not-found patterns) are
// marked as Enabled=false. Returns the probe results for all models.
//
// concurrency controls how many probes run in parallel (0 = len(models)).
func (d *ModelDiscovery) ProbeModels(ctx context.Context, providerID string, concurrency int) ([]ProbeResult, error) {
	provider, err := d.registry.Get(providerID)
	if err != nil {
		return nil, err
	}
	if isCatalogMetadataProvider(provider) {
		return nil, fmt.Errorf("model probing is not supported for catalog providers")
	}

	models, err := d.GetModels(providerID)
	if err != nil {
		return nil, err
	}

	apiKey, _ := d.registry.GetAPIKey(providerID)

	if concurrency <= 0 || concurrency > len(models) {
		concurrency = len(models)
	}

	results := make([]ProbeResult, len(models))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, model := range models {
		wg.Add(1)
		go func(idx int, m *Model) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result := d.probeModel(ctx, provider, apiKey, m)
			results[idx] = result

			if !result.Available {
				m.Enabled = false
			}
		}(i, model)
	}

	wg.Wait()

	// Update cache with probed models
	d.mu.Lock()
	d.cache[providerID] = models
	d.cacheAt[providerID] = timeutil.NowTime()
	d.mu.Unlock()

	// Persist to storage
	_ = d.storage.SaveModels(providerID, models)
	d.notifyModelsChanged(providerID)

	// Log summary
	available := 0
	for _, r := range results {
		if r.Available {
			available++
		}
	}
	slog.Info("[model-probe] probe complete",
		"provider", providerID,
		"total", len(models),
		"available", available,
		"unavailable", len(models)-available)

	return results, nil
}

// probeModel sends a minimal chat completion request to test if a model is configured.
func (d *ModelDiscovery) probeModel(ctx context.Context, provider *Provider, apiKey *APIKey, model *Model) ProbeResult {
	result := ProbeResult{ModelID: model.ID}
	start := timeutil.NowTime()

	baseURL := strings.TrimSuffix(provider.BaseURL, "/")
	var endpoint string
	if strings.HasSuffix(baseURL, "/v1") {
		endpoint = baseURL + "/chat/completions"
	} else {
		endpoint = baseURL + "/v1/chat/completions"
	}

	// Per-probe timeout: 10s is enough for a single model check
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Minimal request body — max_tokens:1 to minimize cost
	body := []byte(`{"model":"` + model.ID + `","messages":[{"role":"user","content":"hi"}],"max_tokens":1}`)

	req, err := http.NewRequestWithContext(probeCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		result.Latency = timeutil.SinceTime(start)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != nil && apiKey.Key != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey.Key)
	}

	resp, err := d.clientFor(provider).Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Latency = timeutil.SinceTime(start)
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Latency = timeutil.SinceTime(start)

	if resp.StatusCode == http.StatusOK {
		io.Copy(io.Discard, resp.Body)
		result.Available = true
		return result
	}

	respBody, _ := io.ReadAll(resp.Body)
	errMsg := string(respBody)
	result.Error = errMsg

	result.Available = !isModelNotConfiguredError(resp.StatusCode, errMsg)
	return result
}

// isModelNotConfiguredError checks if the error indicates the model is not configured
// on this provider endpoint (as opposed to a transient error like rate limiting).
func isModelNotConfiguredError(statusCode int, errMsg string) bool {
	lower := strings.ToLower(errMsg)

	if statusCode == http.StatusBadRequest || statusCode == http.StatusNotFound {
		patterns := []string{
			"未配置模型",
			"not configured",
			"model not found",
			"does not exist",
			"not available",
			"invalid model",
			"unknown model",
			"unsupported model",
			"no such model",
		}
		for _, p := range patterns {
			if strings.Contains(lower, p) {
				return true
			}
		}
	}

	return false
}
