package proxy

import (
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Provider represents an upstream provider
type Provider struct {
	Config      *ProviderConfig
	Healthy     bool
	LastCheck   time.Time
	LastError   error
	LastLatency time.Duration
}

// Router handles provider selection
type Router struct {
	config    *RouteConfig
	providers map[string]*Provider
	mu        sync.RWMutex

	// Round-robin state
	rrIndex uint64
}

// NewRouter creates a new router
func NewRouter(config *RouteConfig) *Router {
	r := &Router{
		config:    config,
		providers: make(map[string]*Provider),
	}

	// Initialize providers
	for _, pc := range config.Providers {
		r.providers[pc.Name] = &Provider{
			Config:  pc,
			Healthy: true, // Assume healthy until proven otherwise
		}
	}

	return r
}

// SelectProvider selects the best available provider
func (r *Router) SelectProvider(req *http.Request) (*Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Get enabled and healthy providers
	available := r.getAvailableProviders()
	if len(available) == 0 {
		return nil, ErrNoAvailableProvider
	}

	// Select based on load balancing strategy
	switch r.config.LoadBalancing {
	case "round-robin":
		return r.selectRoundRobin(available), nil
	case "weighted":
		return r.selectWeighted(available), nil
	default: // "priority"
		return r.selectByPriority(available), nil
	}
}

// GetProvider returns a provider by name
func (r *Router) GetProvider(name string) (*Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// GetAllProviders returns all providers
func (r *Router) GetAllProviders() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]*Provider, 0, len(r.providers))
	for _, p := range r.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetAvailableProviders returns enabled and healthy providers
func (r *Router) GetAvailableProviders() []*Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getAvailableProviders()
}

// getAvailableProviders returns enabled and healthy providers (internal, no lock)
func (r *Router) getAvailableProviders() []*Provider {
	available := make([]*Provider, 0)
	for _, p := range r.providers {
		if p.Config.Enabled && p.Healthy {
			available = append(available, p)
		}
	}
	return available
}

// selectByPriority selects provider with highest priority (lowest number)
func (r *Router) selectByPriority(providers []*Provider) *Provider {
	if len(providers) == 0 {
		return nil
	}

	sorted := make([]*Provider, len(providers))
	copy(sorted, providers)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Config.Priority < sorted[j].Config.Priority
	})

	return sorted[0]
}

// selectRoundRobin selects provider using round-robin
func (r *Router) selectRoundRobin(providers []*Provider) *Provider {
	if len(providers) == 0 {
		return nil
	}

	idx := atomic.AddUint64(&r.rrIndex, 1)
	return providers[idx%uint64(len(providers))]
}

// selectWeighted selects provider using weighted random selection
func (r *Router) selectWeighted(providers []*Provider) *Provider {
	if len(providers) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, p := range providers {
		weight := p.Config.Weight
		if weight <= 0 {
			weight = 1
		}
		totalWeight += weight
	}

	// Simple weighted selection using round-robin index as pseudo-random
	idx := atomic.AddUint64(&r.rrIndex, 1)
	target := int(idx % uint64(totalWeight))

	cumulative := 0
	for _, p := range providers {
		weight := p.Config.Weight
		if weight <= 0 {
			weight = 1
		}
		cumulative += weight
		if target < cumulative {
			return p
		}
	}

	return providers[0]
}

// UpdateHealth updates provider health status
func (r *Router) UpdateHealth(name string, healthy bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if provider, ok := r.providers[name]; ok {
		provider.Healthy = healthy
		provider.LastCheck = timeutil.NowTime()
		provider.LastError = err
	}
}

// UpdateHealthWithLatency updates provider health status with latency
func (r *Router) UpdateHealthWithLatency(name string, healthy bool, err error, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if provider, ok := r.providers[name]; ok {
		provider.Healthy = healthy
		provider.LastCheck = timeutil.NowTime()
		provider.LastError = err
		provider.LastLatency = latency
	}
}

// SetProviderEnabled enables or disables a provider
func (r *Router) SetProviderEnabled(name string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	provider, ok := r.providers[name]
	if !ok {
		return ErrProviderNotFound
	}

	provider.Config.Enabled = enabled
	return nil
}

// AddProvider adds a new provider
func (r *Router) AddProvider(config *ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[config.Name] = &Provider{
		Config:  config,
		Healthy: true,
	}
	r.config.Providers = append(r.config.Providers, config)
}

// RemoveProvider removes a provider
func (r *Router) RemoveProvider(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return ErrProviderNotFound
	}

	delete(r.providers, name)

	// Remove from config
	for i, p := range r.config.Providers {
		if p.Name == name {
			r.config.Providers = append(r.config.Providers[:i], r.config.Providers[i+1:]...)
			break
		}
	}

	return nil
}

// Stats returns router statistics
func (r *Router) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerStats := make([]map[string]interface{}, 0)
	for _, p := range r.providers {
		ps := map[string]interface{}{
			"name":         p.Config.Name,
			"endpoint":     p.Config.Endpoint,
			"priority":     p.Config.Priority,
			"weight":       p.Config.Weight,
			"enabled":      p.Config.Enabled,
			"healthy":      p.Healthy,
			"last_check":   p.LastCheck,
			"last_latency": p.LastLatency.Milliseconds(),
		}
		if p.LastError != nil {
			ps["last_error"] = p.LastError.Error()
		}
		providerStats = append(providerStats, ps)
	}

	return map[string]interface{}{
		"default_provider": r.config.DefaultProvider,
		"load_balancing":   r.config.LoadBalancing,
		"providers":        providerStats,
		"available_count":  len(r.getAvailableProviders()),
		"total_count":      len(r.providers),
	}
}
