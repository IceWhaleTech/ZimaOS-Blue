package providerpool

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"
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
		defaultStrategy: defaultStrategy,
	}
}

// Route selects the best provider for a model request
func (r *Router) Route(req *RouteRequest) (*RouteResult, error) {
	if req.Strategy == "" {
		req.Strategy = r.defaultStrategy
	}

	// Find all candidates that have the requested model
	candidates, err := r.findCandidates(req)
	if err != nil {
		return nil, err
	}

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

	// Build exclusion set
	excludeSet := make(map[string]bool)
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

		// Get models for this provider
		models, err := r.discovery.GetModels(provider.ID)
		if err != nil {
			continue
		}

		// Find matching model
		for _, model := range models {
			if !model.Enabled {
				continue
			}

			// Check if model matches
			if model.ID != req.ModelID && model.Name != req.ModelID {
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
	}

	return candidates, nil
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

// RouteWithFallback attempts to route with automatic fallback on failure
func (r *Router) RouteWithFallback(ctx context.Context, req *RouteRequest, execute func(*RouteResult) error) error {
	result, err := r.Route(req)
	if err != nil {
		return err
	}

	// Try primary
	start := time.Now()
	err = execute(result)
	latency := time.Since(start)

	if err == nil {
		r.UpdateLatency(result.Provider.ID, latency)
		return nil
	}

	// Update latency even on failure (with penalty)
	r.UpdateLatency(result.Provider.ID, latency*2)

	// Try fallbacks
	for _, fallback := range result.Fallbacks {
		// Check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		fallbackResult := &RouteResult{
			Provider: fallback.Provider,
			Model:    fallback.Model,
		}

		// Get API key
		if apiKey, err := r.registry.GetAPIKey(fallback.Provider.ID); err == nil {
			fallbackResult.APIKey = apiKey
		}

		start := time.Now()
		err = execute(fallbackResult)
		latency := time.Since(start)

		if err == nil {
			r.UpdateLatency(fallback.Provider.ID, latency)
			return nil
		}

		r.UpdateLatency(fallback.Provider.ID, latency*2)
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
