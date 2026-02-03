package providerpool

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Router handles intelligent model routing and provider selection
type Router struct {
	registry  *Registry
	discovery *ModelDiscovery

	// Round-robin state
	rrIndex map[string]*uint64
	rrMu    sync.RWMutex

	// Latency tracking for latency-based routing
	latencies map[string]time.Duration
	latencyMu sync.RWMutex

	// Cooldown management
	cooldowns  map[string]*CooldownEntry
	cooldownMu sync.RWMutex
	cooldownCfg *CooldownConfig

	// Failover callback for external logging/tracking
	failoverCallback func(*FailoverResult)

	// Default strategy
	defaultStrategy RoutingStrategy
}

// NewRouter creates a new Router
func NewRouter(registry *Registry, discovery *ModelDiscovery, defaultStrategy RoutingStrategy) *Router {
	return &Router{
		registry:        registry,
		discovery:       discovery,
		rrIndex:         make(map[string]*uint64),
		latencies:       make(map[string]time.Duration),
		cooldowns:       make(map[string]*CooldownEntry),
		cooldownCfg:     DefaultCooldownConfig(),
		defaultStrategy: defaultStrategy,
	}
}

// SetCooldownConfig sets the cooldown configuration
func (r *Router) SetCooldownConfig(cfg *CooldownConfig) {
	r.cooldownMu.Lock()
	defer r.cooldownMu.Unlock()
	r.cooldownCfg = cfg
}

// SetFailoverCallback sets a callback to be called after each failover operation
func (r *Router) SetFailoverCallback(cb func(*FailoverResult)) {
	r.failoverCallback = cb
}

// Route selects the best provider for a model request
func (r *Router) Route(req *RouteRequest) (*RouteResult, error) {
	if req.Strategy == "" {
		req.Strategy = r.defaultStrategy
	}

	fmt.Printf("[Router] Route: modelID=%s, mode=%s\n", req.ModelID, req.Mode)

	// Find all candidates that have the requested model
	candidates, err := r.findCandidates(req)
	if err != nil {
		fmt.Printf("[Router] Route: findCandidates error: %v\n", err)
		return nil, err
	}

	fmt.Printf("[Router] Route: found %d candidates\n", len(candidates))

	if len(candidates) == 0 {
		return nil, ErrNoAvailableProvider
	}

	// Sort candidates by strategy
	r.sortByStrategy(candidates, req.Strategy)

	// Return the best candidate with fallbacks
	result := &RouteResult{
		Provider: candidates[0].Provider,
		Model:    candidates[0].Model,
	}

	// Get API key for the selected provider
	if apiKey, err := r.registry.GetAPIKey(candidates[0].Provider.ID); err == nil {
		result.APIKey = apiKey
	}

	// Add fallbacks
	if len(candidates) > 1 {
		result.Fallbacks = candidates[1:]
	}

	return result, nil
}

// findCandidates finds all providers that can serve the requested model
func (r *Router) findCandidates(req *RouteRequest) ([]*RouteCandidate, error) {
	var candidates []*RouteCandidate

	// Get all enabled providers
	providers := r.registry.ListEnabled()
	fmt.Printf("[Router] findCandidates: %d enabled providers\n", len(providers))

	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, id := range req.Exclude {
		excludeSet[id] = true
	}

	for _, provider := range providers {
		fmt.Printf("[Router] findCandidates: checking provider %s (status=%s, location=%s)\n", provider.ID, provider.Status, provider.Location)

		// Skip excluded providers
		if excludeSet[provider.ID] {
			fmt.Printf("[Router] findCandidates: provider %s excluded\n", provider.ID)
			continue
		}

		// Skip unhealthy providers
		if provider.Status == ProviderStatusError {
			fmt.Printf("[Router] findCandidates: provider %s has error status\n", provider.ID)
			continue
		}

		// Skip providers in cooldown
		if r.IsInCooldown(provider.ID) {
			fmt.Printf("[Router] findCandidates: provider %s in cooldown\n", provider.ID)
			continue
		}

		// Filter by routing mode (location preference)
		if req.Mode != "" && req.Mode != RoutingModeAuto {
			if req.Mode == RoutingModeCloud && provider.Location != ProviderLocationCloud {
				fmt.Printf("[Router] findCandidates: provider %s not cloud (location=%s)\n", provider.ID, provider.Location)
				continue
			}
			if req.Mode == RoutingModeLocal && provider.Location != ProviderLocationLocal {
				fmt.Printf("[Router] findCandidates: provider %s not local (location=%s)\n", provider.ID, provider.Location)
				continue
			}
		}

		// Get models for this provider
		models, err := r.discovery.GetModels(provider.ID)
		if err != nil {
			fmt.Printf("[Router] findCandidates: provider %s GetModels error: %v\n", provider.ID, err)
			continue
		}
		fmt.Printf("[Router] findCandidates: provider %s has %d models\n", provider.ID, len(models))

		// If no model specified, use first available enabled model
		if req.ModelID == "" {
			for _, model := range models {
				if !model.Enabled {
					continue
				}
				// Check capabilities if required
				if req.RequireCap != nil && !matchesCapabilities(model.Capabilities, *req.RequireCap) {
					continue
				}
				candidates = append(candidates, &RouteCandidate{
					Provider: provider,
					Model:    model,
				})
				break // Only one model per provider
			}
			continue
		}

		// Find matching model
		for _, model := range models {
			fmt.Printf("[Router] findCandidates: checking model %s (name=%s) against requested %s\n", model.ID, model.Name, req.ModelID)
			if !model.Enabled {
				fmt.Printf("[Router] findCandidates: model %s is disabled\n", model.ID)
				continue
			}

			// Check if model matches (exact match or prefix match for versioned model names)
			// e.g., "claude-sonnet-4-5-20250929" should match "claude-sonnet-4-5"
			if !modelMatches(model.ID, model.Name, req.ModelID) {
				continue
			}

			fmt.Printf("[Router] findCandidates: model %s matched!\n", model.ID)

			// Check capabilities if required
			if req.RequireCap != nil && !matchesCapabilities(model.Capabilities, *req.RequireCap) {
				continue
			}

			candidates = append(candidates, &RouteCandidate{
				Provider: provider,
				Model:    model,
			})
			break // Only one model per provider
		}
	}

	return candidates, nil
}

// modelMatches checks if a model matches the requested model ID.
// Supports exact match, prefix match, and Claude model alias matching.
// e.g., "claude-sonnet-4-5-20250929" matches "claude-sonnet-4-5"
// e.g., "claude-3-5-haiku-20241022" matches "claude-haiku-4-5"
func modelMatches(modelID, modelName, requestedID string) bool {
	// Exact match
	if modelID == requestedID || modelName == requestedID {
		return true
	}

	// Prefix match: requested ID starts with model ID/name
	// e.g., "claude-sonnet-4-5-20250929" starts with "claude-sonnet-4-5"
	if strings.HasPrefix(requestedID, modelID+"-") || strings.HasPrefix(requestedID, modelName+"-") {
		return true
	}

	// Reverse prefix match: model ID/name starts with requested ID
	if strings.HasPrefix(modelID, requestedID+"-") || strings.HasPrefix(modelName, requestedID+"-") {
		return true
	}

	// Claude model alias matching
	// CC CLI uses: claude-haiku-4-5, claude-sonnet-4-5, claude-opus-4-5
	// API uses: claude-3-5-haiku-20241022, claude-sonnet-4-5-20250929, claude-opus-4-5-20251101
	claudeAliases := map[string][]string{
		"claude-haiku-4-5":  {"claude-3-5-haiku", "claude-haiku-4-5"},
		"claude-sonnet-4-5": {"claude-sonnet-4-5", "claude-3-5-sonnet"},
		"claude-opus-4-5":   {"claude-opus-4-5", "claude-opus-4-5"},
		// Also support the reverse lookup
		"claude-3-5-haiku":  {"claude-haiku-4-5", "claude-3-5-haiku"},
		"claude-3-5-sonnet": {"claude-sonnet-4-5", "claude-3-5-sonnet"},
	}

	// Check if requestedID is a Claude alias
	if aliases, ok := claudeAliases[requestedID]; ok {
		for _, alias := range aliases {
			if strings.HasPrefix(modelID, alias) || strings.HasPrefix(modelName, alias) {
				return true
			}
		}
	}

	// Check if modelID/modelName contains a Claude alias that matches requestedID
	for alias, variants := range claudeAliases {
		if strings.HasPrefix(modelID, alias) || strings.HasPrefix(modelName, alias) {
			for _, variant := range variants {
				if requestedID == variant || strings.HasPrefix(requestedID, variant) {
					return true
				}
			}
		}
	}

	return false
}

// sortByStrategy sorts candidates according to the routing strategy
func (r *Router) sortByStrategy(candidates []*RouteCandidate, strategy RoutingStrategy) {
	switch strategy {
	case RoutingStrategyPriority:
		r.sortByPriority(candidates)
	case RoutingStrategyCost:
		r.sortByCost(candidates)
	case RoutingStrategyLatency:
		r.sortByLatency(candidates)
	case RoutingStrategyRoundRobin:
		r.sortByRoundRobin(candidates)
	default:
		r.sortByPriority(candidates)
	}
}

// sortByPriority sorts by provider priority (higher first)
func (r *Router) sortByPriority(candidates []*RouteCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		// Higher priority first
		if candidates[i].Provider.Priority != candidates[j].Provider.Priority {
			return candidates[i].Provider.Priority > candidates[j].Provider.Priority
		}
		// Then by health status
		return candidates[i].Provider.Status == ProviderStatusActive &&
			candidates[j].Provider.Status != ProviderStatusActive
	})

	// Calculate scores
	for i, c := range candidates {
		c.Score = float64(len(candidates) - i)
	}
}

// sortByCost sorts by model cost (lower first)
func (r *Router) sortByCost(candidates []*RouteCandidate) {
	sort.Slice(candidates, func(i, j int) bool {
		// Calculate total cost (input + output)
		costI := candidates[i].Model.InputPrice + candidates[i].Model.OutputPrice
		costJ := candidates[j].Model.InputPrice + candidates[j].Model.OutputPrice
		return costI < costJ
	})

	// Calculate scores (inverse of cost)
	for _, c := range candidates {
		totalCost := c.Model.InputPrice + c.Model.OutputPrice
		if totalCost > 0 {
			c.Score = 1.0 / totalCost
		} else {
			c.Score = 1000 // Free models get high score
		}
	}
}

// sortByLatency sorts by provider latency (lower first)
func (r *Router) sortByLatency(candidates []*RouteCandidate) {
	r.latencyMu.RLock()
	defer r.latencyMu.RUnlock()

	sort.Slice(candidates, func(i, j int) bool {
		latI := r.latencies[candidates[i].Provider.ID]
		latJ := r.latencies[candidates[j].Provider.ID]

		// Providers without latency data go last
		if latI == 0 && latJ == 0 {
			return candidates[i].Provider.Priority > candidates[j].Provider.Priority
		}
		if latI == 0 {
			return false
		}
		if latJ == 0 {
			return true
		}

		return latI < latJ
	})

	// Calculate scores (inverse of latency)
	for _, c := range candidates {
		lat := r.latencies[c.Provider.ID]
		if lat > 0 {
			c.Score = 1.0 / float64(lat.Milliseconds())
		} else {
			c.Score = 0
		}
	}
}

// sortByRoundRobin rotates through providers
func (r *Router) sortByRoundRobin(candidates []*RouteCandidate) {
	if len(candidates) <= 1 {
		return
	}

	// First, sort by provider ID for stable ordering
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Provider.ID < candidates[j].Provider.ID
	})

	// Get or create round-robin index for this model
	modelID := candidates[0].Model.ID
	r.rrMu.Lock()
	if r.rrIndex[modelID] == nil {
		var idx uint64
		r.rrIndex[modelID] = &idx
	}
	idx := r.rrIndex[modelID]
	r.rrMu.Unlock()

	// Get current index and increment
	current := atomic.AddUint64(idx, 1) - 1
	offset := int(current % uint64(len(candidates)))

	// Rotate the slice
	rotated := make([]*RouteCandidate, len(candidates))
	for i := range candidates {
		rotated[i] = candidates[(i+offset)%len(candidates)]
	}
	copy(candidates, rotated)

	// Set scores
	for i, c := range candidates {
		c.Score = float64(len(candidates) - i)
	}
}

// UpdateLatency updates the latency for a provider
func (r *Router) UpdateLatency(providerID string, latency time.Duration) {
	r.latencyMu.Lock()
	defer r.latencyMu.Unlock()

	// Use exponential moving average
	existing := r.latencies[providerID]
	if existing == 0 {
		r.latencies[providerID] = latency
	} else {
		// EMA with alpha = 0.3
		r.latencies[providerID] = time.Duration(float64(existing)*0.7 + float64(latency)*0.3)
	}
}

// GetLatency returns the current latency for a provider
func (r *Router) GetLatency(providerID string) time.Duration {
	r.latencyMu.RLock()
	defer r.latencyMu.RUnlock()
	return r.latencies[providerID]
}

// IsInCooldown checks if a provider is currently in cooldown
func (r *Router) IsInCooldown(providerID string) bool {
	r.cooldownMu.RLock()
	defer r.cooldownMu.RUnlock()
	entry, exists := r.cooldowns[providerID]
	if !exists {
		return false
	}
	return time.Now().Before(entry.CooldownUntil)
}

// GetCooldownEntry returns the cooldown entry for a provider
func (r *Router) GetCooldownEntry(providerID string) *CooldownEntry {
	r.cooldownMu.RLock()
	defer r.cooldownMu.RUnlock()
	if entry, exists := r.cooldowns[providerID]; exists {
		// Return a copy
		entryCopy := *entry
		return &entryCopy
	}
	return nil
}

// RecordFailure records a failure for a provider and potentially puts it in cooldown
func (r *Router) RecordFailure(providerID string, err error) {
	r.cooldownMu.Lock()
	defer r.cooldownMu.Unlock()

	entry, exists := r.cooldowns[providerID]
	if !exists {
		entry = &CooldownEntry{
			ProviderID: providerID,
		}
		r.cooldowns[providerID] = entry
	}

	entry.FailureCount++
	entry.LastFailure = time.Now()
	if err != nil {
		entry.LastError = err.Error()
	}

	// Check if we should enter cooldown
	if entry.FailureCount >= r.cooldownCfg.FailureThreshold {
		// Calculate cooldown duration with exponential backoff
		cooldownDuration := r.cooldownCfg.InitialCooldown
		multiplier := 1.0
		for i := r.cooldownCfg.FailureThreshold; i < entry.FailureCount; i++ {
			multiplier *= r.cooldownCfg.CooldownMultiplier
		}
		cooldownDuration = time.Duration(float64(cooldownDuration) * multiplier)
		if cooldownDuration > r.cooldownCfg.MaxCooldown {
			cooldownDuration = r.cooldownCfg.MaxCooldown
		}
		entry.CooldownUntil = time.Now().Add(cooldownDuration)
	}
}

// RecordSuccess records a successful request and potentially resets cooldown
func (r *Router) RecordSuccess(providerID string) {
	r.cooldownMu.Lock()
	defer r.cooldownMu.Unlock()

	entry, exists := r.cooldowns[providerID]
	if !exists {
		return
	}

	// Reset failure count on success
	entry.FailureCount = 0
	entry.CooldownUntil = time.Time{}
}

// ClearCooldown manually clears cooldown for a provider
func (r *Router) ClearCooldown(providerID string) {
	r.cooldownMu.Lock()
	defer r.cooldownMu.Unlock()
	delete(r.cooldowns, providerID)
}

// ListCooldowns returns all providers currently in cooldown
func (r *Router) ListCooldowns() []*CooldownEntry {
	r.cooldownMu.RLock()
	defer r.cooldownMu.RUnlock()

	var result []*CooldownEntry
	now := time.Now()
	for _, entry := range r.cooldowns {
		if now.Before(entry.CooldownUntil) {
			entryCopy := *entry
			result = append(result, &entryCopy)
		}
	}
	return result
}

// classifyError determines the failover reason from an error
func classifyError(err error) FailoverReason {
	if err == nil {
		return FailoverReasonUnknown
	}
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded") {
		return FailoverReasonTimeout
	}
	if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") || strings.Contains(errStr, "too many requests") {
		return FailoverReasonRateLimit
	}
	if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401") || strings.Contains(errStr, "invalid api key") || strings.Contains(errStr, "authentication") {
		return FailoverReasonAuthError
	}
	if strings.Contains(errStr, "model not found") || strings.Contains(errStr, "404") || strings.Contains(errStr, "does not exist") {
		return FailoverReasonModelNotFound
	}
	return FailoverReasonAPIError
}

// RouteWithFallback attempts to route with automatic fallback on failure
func (r *Router) RouteWithFallback(ctx context.Context, req *RouteRequest, execute func(*RouteResult) error) error {
	// Initialize failover tracking
	failoverResult := &FailoverResult{
		RequestID: uuid.New().String(),
		StartTime: time.Now(),
	}
	defer func() {
		failoverResult.EndTime = time.Now()
		if r.failoverCallback != nil {
			r.failoverCallback(failoverResult)
		}
	}()

	result, err := r.Route(req)
	if err != nil {
		failoverResult.FinalError = err.Error()
		return err
	}

	// Try primary
	failoverResult.TotalAttempts++
	start := time.Now()
	err = execute(result)
	latency := time.Since(start)

	if err == nil {
		r.UpdateLatency(result.Provider.ID, latency)
		r.RecordSuccess(result.Provider.ID)
		failoverResult.SuccessProvider = result.Provider.ID
		failoverResult.SuccessModel = result.Model.ID
		return nil
	}

	// Record failure for primary
	r.UpdateLatency(result.Provider.ID, latency*2)
	r.RecordFailure(result.Provider.ID, err)

	// Track the failed attempt
	failoverResult.FailedAttempts = append(failoverResult.FailedAttempts, &FailoverRecord{
		Timestamp:    start,
		ProviderID:   result.Provider.ID,
		ProviderName: result.Provider.Name,
		ModelID:      result.Model.ID,
		Reason:       classifyError(err),
		Error:        err.Error(),
		Latency:      latency,
	})

	// Try fallbacks
	for i, fallback := range result.Fallbacks {
		// Check context
		if ctx.Err() != nil {
			failoverResult.FinalError = ctx.Err().Error()
			return ctx.Err()
		}

		// Update previous record with next provider info
		if len(failoverResult.FailedAttempts) > 0 {
			failoverResult.FailedAttempts[len(failoverResult.FailedAttempts)-1].NextProviderID = fallback.Provider.ID
		}

		fallbackResult := &RouteResult{
			Provider: fallback.Provider,
			Model:    fallback.Model,
		}

		// Get API key
		if apiKey, keyErr := r.registry.GetAPIKey(fallback.Provider.ID); keyErr == nil {
			fallbackResult.APIKey = apiKey
		}

		failoverResult.TotalAttempts++
		start := time.Now()
		err = execute(fallbackResult)
		latency := time.Since(start)

		if err == nil {
			r.UpdateLatency(fallback.Provider.ID, latency)
			r.RecordSuccess(fallback.Provider.ID)
			failoverResult.SuccessProvider = fallback.Provider.ID
			failoverResult.SuccessModel = fallback.Model.ID
			return nil
		}

		// Record failure
		r.UpdateLatency(fallback.Provider.ID, latency*2)
		r.RecordFailure(fallback.Provider.ID, err)

		nextProviderID := ""
		if i+1 < len(result.Fallbacks) {
			nextProviderID = result.Fallbacks[i+1].Provider.ID
		}

		failoverResult.FailedAttempts = append(failoverResult.FailedAttempts, &FailoverRecord{
			Timestamp:      start,
			ProviderID:     fallback.Provider.ID,
			ProviderName:   fallback.Provider.Name,
			ModelID:        fallback.Model.ID,
			Reason:         classifyError(err),
			Error:          err.Error(),
			Latency:        latency,
			NextProviderID: nextProviderID,
		})
	}

	failoverResult.FinalError = err.Error()
	return err
}

// FindBestProvider finds the best provider for a model without executing
func (r *Router) FindBestProvider(modelID string) (*Provider, *Model, error) {
	result, err := r.Route(&RouteRequest{
		ModelID:  modelID,
		Strategy: r.defaultStrategy,
	})
	if err != nil {
		return nil, nil, err
	}
	return result.Provider, result.Model, nil
}

// ListAvailableModels returns all models that can be routed
func (r *Router) ListAvailableModels() []*Model {
	return r.discovery.GetAllModels()
}

// GetProviderForModel returns the provider that would be selected for a model
func (r *Router) GetProviderForModel(modelID string, strategy RoutingStrategy) (*Provider, error) {
	result, err := r.Route(&RouteRequest{
		ModelID:  modelID,
		Strategy: strategy,
	})
	if err != nil {
		return nil, err
	}
	return result.Provider, nil
}
