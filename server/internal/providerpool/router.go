package providerpool

import (
	"context"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// candidateSnapshot holds a precomputed candidate list for all known models.
// Stored in atomic.Value for lock-free reads on the hot path.
type candidateSnapshot struct {
	// byModel maps model ID → sorted candidates (by default strategy)
	byModel map[string][]*RouteCandidate
	// allCandidates is the full list of all provider+model pairs (for empty model requests)
	allCandidates []*RouteCandidate
	// builtAt is when this snapshot was created
	builtAt time.Time
}

// Router handles intelligent model routing and provider selection
type Router struct {
	registry  *Registry
	discovery *ModelDiscovery

	// Static candidate cache — rebuilt on provider changes, read lock-free
	candidates atomic.Value // *candidateSnapshot

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
	r := &Router{
		registry:        registry,
		discovery:       discovery,
		rrIndex:         make(map[string]*uint64),
		latencies:       make(map[string]time.Duration),
		cooldowns:       make(map[string]*CooldownEntry),
		cooldownCfg:     DefaultCooldownConfig(),
		defaultStrategy: defaultStrategy,
	}
	r.RebuildCandidates()
	return r
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

// RebuildCandidates rebuilds the static candidate snapshot from current provider/model state.
// Call this when providers are added/removed/updated or models change.
// The snapshot is swapped atomically — zero contention on the read path.
func (r *Router) RebuildCandidates() {
	providers := r.registry.ListEnabled()
	snap := &candidateSnapshot{
		byModel: make(map[string][]*RouteCandidate),
		builtAt: time.Now(),
	}

	for _, provider := range providers {
		// Skip cloud providers without a usable API key
		if provider.Location == ProviderLocationCloud {
			if key, err := r.registry.GetAPIKey(provider.ID); err != nil || key == nil || key.Key == "" {
				continue
			}
		}

		models, err := r.discovery.GetModels(provider.ID)
		if err != nil {
			continue
		}

		for _, model := range models {
			if !model.Enabled {
				continue
			}
			c := &RouteCandidate{
				Provider: provider,
				Model:    model,
			}
			snap.byModel[model.ID] = append(snap.byModel[model.ID], c)
			snap.byModel[model.Name] = append(snap.byModel[model.Name], c)
			snap.allCandidates = append(snap.allCandidates, c)
		}
	}

	r.candidates.Store(snap)
}

// Route selects the best provider for a model request.
// Uses the precomputed candidate snapshot for O(1) lookup, falls back to dynamic search
// for alias matching or when the snapshot doesn't have the model.
func (r *Router) Route(req *RouteRequest) (*RouteResult, error) {
	if req.Strategy == "" {
		req.Strategy = r.defaultStrategy
	}

	candidates := r.getCandidatesFromSnapshot(req)

	// Fallback to dynamic search if snapshot miss (alias matching, etc.)
	if len(candidates) == 0 {
		var err error
		candidates, err = r.findCandidates(req)
		if err != nil {
			return nil, err
		}
	}

	if len(candidates) == 0 {
		return nil, ErrNoAvailableProvider
	}

	r.sortByStrategy(candidates, req.Strategy)

	result := &RouteResult{
		Provider: candidates[0].Provider,
		Model:    candidates[0].Model,
	}

	if apiKey, err := r.registry.GetAPIKey(candidates[0].Provider.ID); err == nil {
		result.APIKey = apiKey
	}

	if len(candidates) > 1 {
		result.Fallbacks = candidates[1:]
	}

	return result, nil
}

// getCandidatesFromSnapshot returns candidates from the precomputed snapshot.
// Returns a fresh copy so callers can sort/filter without affecting the snapshot.
// Applies runtime filters (cooldown, exclude, routing mode) that can't be precomputed.
func (r *Router) getCandidatesFromSnapshot(req *RouteRequest) []*RouteCandidate {
	snapVal := r.candidates.Load()
	if snapVal == nil {
		return nil
	}
	snap := snapVal.(*candidateSnapshot)

	var source []*RouteCandidate
	if req.ModelID == "" {
		source = snap.allCandidates
	} else {
		source = snap.byModel[req.ModelID]
	}
	if len(source) == 0 {
		return nil
	}

	// Build exclusion set
	excludeSet := make(map[string]bool, len(req.Exclude))
	for _, id := range req.Exclude {
		excludeSet[id] = true
	}

	// Copy + filter (runtime state: cooldown, exclude, routing mode, status)
	result := make([]*RouteCandidate, 0, len(source))
	for _, c := range source {
		if excludeSet[c.Provider.ID] {
			continue
		}
		if c.Provider.Status == ProviderStatusError {
			continue
		}
		if r.IsInCooldown(c.Provider.ID) {
			continue
		}
		if req.Mode != "" && req.Mode != RoutingModeAuto {
			if req.Mode == RoutingModeCloud && c.Provider.Location != ProviderLocationCloud {
				continue
			}
			if req.Mode == RoutingModeLocal && c.Provider.Location != ProviderLocationLocal {
				continue
			}
		}
		if req.RequireCap != nil && !matchesCapabilities(c.Model.Capabilities, *req.RequireCap) {
			continue
		}
		// Copy the candidate so sorting doesn't mutate the snapshot
		copy := *c
		result = append(result, &copy)
	}
	return result
}

// findCandidates finds all providers that can serve the requested model
func (r *Router) findCandidates(req *RouteRequest) ([]*RouteCandidate, error) {
	var candidates []*RouteCandidate

	providers := r.registry.ListEnabled()

	// Build exclusion set
	excludeSet := make(map[string]bool, len(req.Exclude))
	for _, id := range req.Exclude {
		excludeSet[id] = true
	}

	for _, provider := range providers {
		// Skip excluded providers
		if excludeSet[provider.ID] {
			continue
		}

		// Skip unhealthy providers
		if provider.Status == ProviderStatusError {
			continue
		}

		// Skip providers in cooldown
		if r.IsInCooldown(provider.ID) {
			continue
		}

		// Skip cloud providers without a usable API key
		if provider.Location == ProviderLocationCloud {
			if key, err := r.registry.GetAPIKey(provider.ID); err != nil || key == nil || key.Key == "" {
				continue
			}
		}

		// Filter by routing mode
		if req.Mode != "" && req.Mode != RoutingModeAuto {
			if req.Mode == RoutingModeCloud && provider.Location != ProviderLocationCloud {
				continue
			}
			if req.Mode == RoutingModeLocal && provider.Location != ProviderLocationLocal {
				continue
			}
		}

		// Get models for this provider
		models, err := r.discovery.GetModels(provider.ID)
		if err != nil {
			continue
		}

		// If no model specified, use first available enabled model
		if req.ModelID == "" {
			for _, model := range models {
				if !model.Enabled {
					continue
				}
				if req.RequireCap != nil && !matchesCapabilities(model.Capabilities, *req.RequireCap) {
					continue
				}
				candidates = append(candidates, &RouteCandidate{
					Provider: provider,
					Model:    model,
				})
				break
			}
			continue
		}

		// Find matching model
		for _, model := range models {
			if !model.Enabled {
				continue
			}

			if !modelMatches(model.ID, model.Name, req.ModelID) {
				continue
			}

			if req.RequireCap != nil && !matchesCapabilities(model.Capabilities, *req.RequireCap) {
				continue
			}

			candidates = append(candidates, &RouteCandidate{
				Provider: provider,
				Model:    model,
			})
		}
	}

	return candidates, nil
}

// Pre-computed Claude model aliases for O(1) lookup
var claudeModelAliases = map[string][]string{
	"claude-haiku-4-5":  {"claude-3-5-haiku", "claude-haiku-4-5"},
	"claude-sonnet-4-5": {"claude-sonnet-4-5", "claude-3-5-sonnet"},
	"claude-opus-4-5":   {"claude-opus-4-5"},
	"claude-3-5-haiku":  {"claude-haiku-4-5", "claude-3-5-haiku"},
	"claude-3-5-sonnet": {"claude-sonnet-4-5", "claude-3-5-sonnet"},
}

// modelMatches checks if a model matches the requested model ID.
func modelMatches(modelID, modelName, requestedID string) bool {
	// Exact match (most common case)
	if modelID == requestedID || modelName == requestedID {
		return true
	}

	// Prefix match for versioned models
	if strings.HasPrefix(requestedID, modelID+"-") || strings.HasPrefix(requestedID, modelName+"-") ||
		strings.HasPrefix(modelID, requestedID+"-") || strings.HasPrefix(modelName, requestedID+"-") {
		return true
	}

	// Claude alias matching
	if aliases, ok := claudeModelAliases[requestedID]; ok {
		for _, alias := range aliases {
			if strings.HasPrefix(modelID, alias) || strings.HasPrefix(modelName, alias) {
				return true
			}
		}
	}

	// Reverse alias matching
	for alias, variants := range claudeModelAliases {
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
		// Same provider: keep together (maintain order)
		if candidates[i].Provider.ID == candidates[j].Provider.ID {
			return false
		}
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
	// Only allocate failover tracking when a callback is registered
	var failoverResult *FailoverResult
	if r.failoverCallback != nil {
		failoverResult = &FailoverResult{
			RequestID: uuid.New().String(),
			StartTime: time.Now(),
		}
		defer func() {
			failoverResult.EndTime = time.Now()
			r.failoverCallback(failoverResult)
		}()
	}

	result, err := r.Route(req)
	if err != nil {
		if failoverResult != nil {
			failoverResult.FinalError = err.Error()
		}
		return err
	}

	// Try primary
	if failoverResult != nil {
		failoverResult.TotalAttempts++
	}
	start := time.Now()
	err = execute(result)
	latency := time.Since(start)

	if err == nil {
		r.UpdateLatency(result.Provider.ID, latency)
		r.RecordSuccess(result.Provider.ID)
		if failoverResult != nil {
			failoverResult.SuccessProvider = result.Provider.ID
			failoverResult.SuccessModel = result.Model.ID
		}
		return nil
	}

	// Record failure for primary
	r.UpdateLatency(result.Provider.ID, latency*2)
	r.RecordFailure(result.Provider.ID, err)

	// Track the failed attempt
	if failoverResult != nil {
		failoverResult.FailedAttempts = append(failoverResult.FailedAttempts, &FailoverRecord{
			Timestamp:    start,
			ProviderID:   result.Provider.ID,
			ProviderName: result.Provider.Name,
			ModelID:      result.Model.ID,
			Reason:       classifyError(err),
			Error:        err.Error(),
			Latency:      latency,
		})
	}

	// Try fallbacks
	for i, fallback := range result.Fallbacks {
		// Check context
		if ctx.Err() != nil {
			if failoverResult != nil {
				failoverResult.FinalError = ctx.Err().Error()
			}
			return ctx.Err()
		}

		// Update previous record with next provider info
		if failoverResult != nil && len(failoverResult.FailedAttempts) > 0 {
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

		if failoverResult != nil {
			failoverResult.TotalAttempts++
		}
		start := time.Now()
		err = execute(fallbackResult)
		latency := time.Since(start)

		if err == nil {
			r.UpdateLatency(fallback.Provider.ID, latency)
			r.RecordSuccess(fallback.Provider.ID)
			if failoverResult != nil {
				failoverResult.SuccessProvider = fallback.Provider.ID
				failoverResult.SuccessModel = fallback.Model.ID
			}
			return nil
		}

		// Record failure
		r.UpdateLatency(fallback.Provider.ID, latency*2)
		r.RecordFailure(fallback.Provider.ID, err)

		if failoverResult != nil {
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
	}

	if failoverResult != nil {
		failoverResult.FinalError = err.Error()
	}
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
