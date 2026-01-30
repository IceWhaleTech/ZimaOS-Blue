package providerpool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ModelDiscovery handles model discovery and caching
type ModelDiscovery struct {
	registry *Registry
	storage  Storage
	client   *http.Client
	cache    map[string][]*Model
	cacheTTL time.Duration
	cacheAt  map[string]time.Time
	mu       sync.RWMutex
}

// NewModelDiscovery creates a new ModelDiscovery
func NewModelDiscovery(registry *Registry, storage Storage, cacheTTL time.Duration) *ModelDiscovery {
	return &ModelDiscovery{
		registry: registry,
		storage:  storage,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:    make(map[string][]*Model),
		cacheTTL: cacheTTL,
		cacheAt:  make(map[string]time.Time),
	}
}

// FetchModels fetches models from a provider
func (d *ModelDiscovery) FetchModels(ctx context.Context, providerID string) ([]*Model, error) {
	provider, err := d.registry.Get(providerID)
	if err != nil {
		return nil, err
	}

	// Try to fetch from API
	models, err := d.fetchFromAPI(ctx, provider)
	if err != nil {
		// Fall back to built-in models
		builtinModels := GetBuiltinModels(providerID)
		if builtinModels != nil {
			return builtinModels, nil
		}
		return nil, err
	}

	// Cache the results
	d.mu.Lock()
	d.cache[providerID] = models
	d.cacheAt[providerID] = time.Now()
	d.mu.Unlock()

	// Save to storage
	if err := d.storage.SaveModels(providerID, models); err != nil {
		// Log but don't fail
	}

	return models, nil
}

// GetModels returns cached models for a provider
func (d *ModelDiscovery) GetModels(providerID string) ([]*Model, error) {
	d.mu.RLock()
	models, exists := d.cache[providerID]
	cacheTime := d.cacheAt[providerID]
	d.mu.RUnlock()

	// Check if cache is valid
	if exists && time.Since(cacheTime) < d.cacheTTL {
		return models, nil
	}

	// Try to load from storage
	storedModels, err := d.storage.LoadModels(providerID)
	if err == nil && len(storedModels) > 0 {
		d.mu.Lock()
		d.cache[providerID] = storedModels
		d.cacheAt[providerID] = time.Now()
		d.mu.Unlock()
		return storedModels, nil
	}

	// Fall back to built-in models
	builtinModels := GetBuiltinModels(providerID)
	if builtinModels != nil {
		return builtinModels, nil
	}

	return nil, ErrModelNotFound
}

// GetAllModels returns all models from all enabled providers
func (d *ModelDiscovery) GetAllModels() []*Model {
	providers := d.registry.ListEnabled()
	var allModels []*Model

	for _, provider := range providers {
		models, err := d.GetModels(provider.ID)
		if err != nil {
			continue
		}
		allModels = append(allModels, models...)
	}

	return allModels
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
	if provider.BaseURL == "" {
		return nil, fmt.Errorf("no base URL configured")
	}

	// Get API key (optional for some providers like Ollama)
	apiKey, _ := d.registry.GetAPIKey(provider.ID)

	// Build request based on provider type
	var models []*Model
	var fetchErr error

	switch provider.ID {
	case "openai", "deepseek", "moonshot", "openrouter", "aihubmix", "minimax", "codex":
		if apiKey == nil {
			return nil, ErrNoAPIKey
		}
		models, fetchErr = d.fetchOpenAIModels(ctx, provider, apiKey)
	case "anthropic":
		// Anthropic doesn't have a models endpoint, use built-in
		models = GetBuiltinModels("anthropic")
	case "google":
		if apiKey == nil {
			return nil, ErrNoAPIKey
		}
		models, fetchErr = d.fetchGoogleModels(ctx, provider, apiKey)
	case "ollama":
		// Ollama doesn't require API key
		models, fetchErr = d.fetchOllamaModels(ctx, provider)
	default:
		// Try OpenAI-compatible endpoint
		models, fetchErr = d.fetchOpenAIModels(ctx, provider, apiKey)
	}

	if fetchErr != nil {
		return nil, fetchErr
	}

	return models, nil
}

// fetchOpenAIModels fetches models from OpenAI-compatible API
func (d *ModelDiscovery) fetchOpenAIModels(ctx context.Context, provider *Provider, apiKey *APIKey) ([]*Model, error) {
	url := provider.BaseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if apiKey != nil && apiKey.Key != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey.Key)
	}

	resp, err := d.client.Do(req)
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
		Data []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	models := make([]*Model, 0, len(result.Data))
	for _, m := range result.Data {
		model := &Model{
			ID:          m.ID,
			ProviderID:  provider.ID,
			Name:        m.ID,
			DisplayName: formatModelName(m.ID),
			Enabled:     true,
			Capabilities: ModelCapabilities{
				Chat:         true,
				Streaming:    true,
				SystemPrompt: true,
			},
		}

		// Infer capabilities from model name
		inferCapabilities(model)

		models = append(models, model)
	}

	return models, nil
}

// fetchGoogleModels fetches models from Google Gemini API
func (d *ModelDiscovery) fetchGoogleModels(ctx context.Context, provider *Provider, apiKey *APIKey) ([]*Model, error) {
	url := provider.BaseURL + "/v1beta/models"
	if apiKey != nil && apiKey.Key != "" {
		url += "?key=" + apiKey.Key
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
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

		// Infer additional capabilities
		if strings.Contains(modelID, "vision") || strings.Contains(modelID, "pro") {
			model.Capabilities.Vision = true
		}
		if strings.Contains(modelID, "pro") || strings.Contains(modelID, "ultra") {
			model.Capabilities.FunctionCall = true
		}

		models = append(models, model)
	}

	return models, nil
}

// fetchOllamaModels fetches models from Ollama API
func (d *ModelDiscovery) fetchOllamaModels(ctx context.Context, provider *Provider) ([]*Model, error) {
	url := provider.BaseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := d.client.Do(req)
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
				Streaming:    true,
				SystemPrompt: true,
			},
		}

		// Infer capabilities from model name
		inferOllamaCapabilities(model)

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
