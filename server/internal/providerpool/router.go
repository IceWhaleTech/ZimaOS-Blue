package providerpool

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// candidateSnapshot holds a precomputed candidate list for all known models.
// Stored in atomic.Value for lock-free reads on the hot path.
type candidateSnapshot struct {
	// byModel maps model ID → sorted candidates (by default strategy)
	byModel map[string][]*RouteCandidate
	// allCandidates is the full list of all provider+model pairs. Empty-model
	// routing collapses this to one filtered candidate per provider at read time.
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
	cooldowns   map[string]*CooldownEntry
	cooldownMu  sync.RWMutex
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
	if discovery != nil {
		discovery.SetModelsChangedHook(func(_ string) {
			r.RebuildCandidates()
		})
	}
	r.RebuildCandidates()
	return r
}

// hasCredentials returns true if the provider has a usable API key or connected OAuth.
func (r *Router) hasCredentials(p *Provider) bool {
	if key, err := r.registry.GetAPIKey(p.ID); err == nil && key != nil && key.Key != "" {
		return true
	}
	return p.OAuth != nil && p.OAuth.Connected
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

// shouldBypassPenalty returns true when only one provider is enabled.
// In single-provider setups we should avoid self-isolation penalties
// (cooldown and transient unhealthy status) that can block all routing.
func (r *Router) shouldBypassPenalty() bool {
	return len(r.registry.ListEnabled()) <= 1
}

// RebuildCandidates rebuilds the static candidate snapshot from current provider/model state.
// Call this when providers are added/removed/updated or models change.
// The snapshot is swapped atomically — zero contention on the read path.
func (r *Router) RebuildCandidates() {
	providers := r.registry.ListEnabled()
	snap := &candidateSnapshot{
		byModel: make(map[string][]*RouteCandidate),
		builtAt: timeutil.NowTime(),
	}

	for _, provider := range providers {
		// Skip cloud providers without usable credentials (API key or OAuth)
		if provider.Location == ProviderLocationCloud && !r.hasCredentials(provider) {
			continue
		}

		models, err := r.discovery.GetFilteredModels(provider.ID)
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
			if model.Name != model.ID {
				snap.byModel[model.Name] = append(snap.byModel[model.Name], c)
			}
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

	// If RequireCap filtered out all candidates, retry without the capability
	// filter. Many providers support tools/vision but don't declare it in
	// metadata — better to try and let the upstream decide than to return
	// "no provider available".
	if len(candidates) == 0 && req.RequireCap != nil {
		relaxed := *req
		relaxed.RequireCap = nil
		candidates = r.getCandidatesFromSnapshot(&relaxed)
		if len(candidates) == 0 {
			candidates, _ = r.findCandidates(&relaxed)
		}
		if len(candidates) > 0 {
			slog.Info("[router] no providers matched required capabilities, falling back to all providers",
				"model", req.ModelID, "required_cap", fmt.Sprintf("%+v", *req.RequireCap))
		}
	}

	if len(candidates) == 0 {
		diagReq := *req
		diagReq.Mode = RoutingModeAuto
		diagReq.RequireCap = nil
		diagReq.Exclude = nil
		diagCandidates, _ := r.findCandidates(&diagReq)
		slog.Warn("[router] no route candidates after filtering",
			"model", req.ModelID,
			"mode", req.Mode,
			"strategy", req.Strategy,
			"preferred_provider", req.PreferredProviderID,
			"required_cap", fmt.Sprintf("%+v", req.RequireCap),
			"exclude_count", len(req.Exclude),
			"excluded_providers", req.Exclude,
			"enabled_providers", len(r.registry.ListEnabled()),
			"relaxed_candidates", len(diagCandidates),
		)
		return nil, ErrNoAvailableProvider
	}

	r.sortByStrategy(candidates, req.Strategy, req.ModelID)

	// Sticky routing: if a preferred provider is set (e.g., during tool rounds),
	// move it to the front so it's tried first. Fallbacks remain available.
	if req.PreferredProviderID != "" {
		for i, c := range candidates {
			if c.Provider.ID == req.PreferredProviderID {
				if i > 0 {
					// Move preferred to front, shift others down
					preferred := candidates[i]
					copy(candidates[1:i+1], candidates[:i])
					candidates[0] = preferred
				}
				break
			}
		}
	}

	result := &RouteResult{
		Provider: candidates[0].Provider,
		Model:    candidates[0].Model,
	}

	if apiKey, err := r.registry.GetAPIKey(candidates[0].Provider.ID); err == nil {
		result.APIKey = apiKey
	}

	// Also check for OAuth config if no API key
	if result.APIKey == nil {
		if oauthCfg, err := r.registry.GetOAuthConfig(candidates[0].Provider.ID); err == nil {
			result.OAuth = oauthCfg
		}
	}

	if len(candidates) > 1 {
		result.Fallbacks = candidates[1:]
	}

	return result, nil
}

// HasModel returns true if any provider in the snapshot can serve the given model.
// This is a fast, lock-free check used by routing rules to verify a target model
// is actually available before committing to a model swap.
func (r *Router) HasModel(modelID string) bool {
	snapVal := r.candidates.Load()
	if snapVal == nil {
		return false
	}
	snap := snapVal.(*candidateSnapshot)
	return len(snap.byModel[modelID]) > 0
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

	// Build exclusion set — skip map alloc when no excludes (common case)
	var excludeSet map[string]bool
	if len(req.Exclude) > 0 {
		excludeSet = make(map[string]bool, len(req.Exclude))
		for _, id := range req.Exclude {
			excludeSet[id] = true
		}
	}

	// Copy + filter (runtime state: cooldown, exclude, routing mode, status)
	result := make([]*RouteCandidate, 0, len(source))
	needModeFilter := req.Mode != "" && req.Mode != RoutingModeAuto
	bypassPenalty := r.shouldBypassPenalty()
	for _, c := range source {
		if len(excludeSet) > 0 && excludeSet[c.Provider.ID] {
			continue
		}
		if c.Provider.Status == ProviderStatusError && !bypassPenalty {
			continue
		}
		if !bypassPenalty && r.IsInCooldown(c.Provider.ID) {
			continue
		}
		if needModeFilter {
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
	if req.ModelID == "" && len(result) > 1 {
		result = collapseCandidatesByProvider(result)
	}
	return result
}

func collapseCandidatesByProvider(candidates []*RouteCandidate) []*RouteCandidate {
	if len(candidates) <= 1 {
		return candidates
	}

	seen := make(map[string]struct{}, len(candidates))
	collapsed := make([]*RouteCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Provider == nil {
			continue
		}
		if _, exists := seen[candidate.Provider.ID]; exists {
			continue
		}
		seen[candidate.Provider.ID] = struct{}{}
		collapsed = append(collapsed, candidate)
	}
	return collapsed
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
	bypassPenalty := r.shouldBypassPenalty()

	for _, provider := range providers {
		// Skip excluded providers
		if excludeSet[provider.ID] {
			continue
		}

		// Skip unhealthy providers
		if provider.Status == ProviderStatusError && !bypassPenalty {
			continue
		}

		// Skip providers in cooldown
		if !bypassPenalty && r.IsInCooldown(provider.ID) {
			continue
		}

		// Skip cloud providers without usable credentials (API key or OAuth)
		if provider.Location == ProviderLocationCloud && !r.hasCredentials(provider) {
			continue
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

		// Get models for this provider (respects AllowedModels filter)
		models, err := r.discovery.GetFilteredModels(provider.ID)
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
func (r *Router) sortByStrategy(candidates []*RouteCandidate, strategy RoutingStrategy, modelID string) {
	switch strategy {
	case RoutingStrategyPriority:
		r.sortByPriority(candidates, modelID)
	case RoutingStrategyCost:
		r.sortByCost(candidates)
	case RoutingStrategyLatency:
		r.sortByLatency(candidates, modelID)
	case RoutingStrategyRoundRobin:
		r.sortByRoundRobin(candidates)
	default:
		r.sortByPriority(candidates, modelID)
	}
}

// sortByPriority sorts by provider priority (higher first)
func (r *Router) sortByPriority(candidates []*RouteCandidate, modelID string) {
	sort.SliceStable(candidates, func(i, j int) bool {
		affinityI := providerFormatAffinityScore(modelID, candidates[i].Provider)
		affinityJ := providerFormatAffinityScore(modelID, candidates[j].Provider)
		if affinityI != affinityJ {
			return affinityI > affinityJ
		}
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
func (r *Router) sortByLatency(candidates []*RouteCandidate, modelID string) {
	r.latencyMu.RLock()
	defer r.latencyMu.RUnlock()

	sort.SliceStable(candidates, func(i, j int) bool {
		affinityI := providerFormatAffinityScore(modelID, candidates[i].Provider)
		affinityJ := providerFormatAffinityScore(modelID, candidates[j].Provider)
		if affinityI != affinityJ {
			return affinityI > affinityJ
		}

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
	return timeutil.NowNano() < entry.CooldownUntil.UnixNano()
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
	// Single-provider mode: never apply cooldown penalties that would
	// self-isolate the only available provider.
	if r.shouldBypassPenalty() {
		return
	}

	// Skip recording failures for "no response" errors — these indicate tool-level
	// issues (e.g., tool returned empty/invalid data) rather than provider issues.
	// We don't want to penalize providers for tool processing failures.
	if isNoResponseError(err) {
		return
	}

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
	entry.LastFailure = timeutil.NowTime()
	if err != nil {
		entry.LastError = err.Error()
	}

	// Auth errors (401/403) indicate permanent credential issues — mark provider as error
	// immediately so subsequent requests skip this provider without waiting for cooldown threshold.
	// Set a very long cooldown (1 hour) for auth errors as they typically require user intervention.
	if isAuthError(err) {
		entry.CooldownUntil = timeutil.NowTime().Add(1 * time.Hour)
		entry.LastError = "auth_error: " + err.Error()
		return
	}

	// Transient errors (502/503) get shorter cooldown — these are temporary upstream blips
	transient := isTransientError(err)
	if transient {
		entry.TransientFailureCount++
	}

	// Pick cooldown parameters based on error type
	threshold := r.cooldownCfg.FailureThreshold
	initialCD := r.cooldownCfg.InitialCooldown
	maxCD := r.cooldownCfg.MaxCooldown

	if transient && r.cooldownCfg.TransientFailureThreshold > 0 {
		threshold = r.cooldownCfg.TransientFailureThreshold
		if r.cooldownCfg.TransientInitialCooldown > 0 {
			initialCD = r.cooldownCfg.TransientInitialCooldown
		}
		if r.cooldownCfg.TransientMaxCooldown > 0 {
			maxCD = r.cooldownCfg.TransientMaxCooldown
		}
	}

	// Check if we should enter cooldown
	failCount := entry.FailureCount
	if transient {
		failCount = entry.TransientFailureCount
	}
	if failCount >= threshold {
		// Calculate cooldown duration with exponential backoff
		cooldownDuration := initialCD
		multiplier := 1.0
		for i := threshold; i < failCount; i++ {
			multiplier *= r.cooldownCfg.CooldownMultiplier
		}
		cooldownDuration = time.Duration(float64(cooldownDuration) * multiplier)
		if cooldownDuration > maxCD {
			cooldownDuration = maxCD
		}
		entry.CooldownUntil = timeutil.NowTime().Add(cooldownDuration)
	}
}

// isTransientError returns true for temporary upstream errors (502/503) that
// typically resolve quickly and should not trigger aggressive cooldown.
func isTransientError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "upstream 502") ||
		strings.Contains(s, "upstream 503") ||
		strings.Contains(s, "upstream 504") ||
		strings.Contains(s, "proxy returned 502") ||
		strings.Contains(s, "proxy returned 503") ||
		strings.Contains(s, "proxy returned 504") ||
		strings.Contains(s, "throttled (429)") ||
		strings.Contains(s, "overloaded (529)") {
		return true
	}

	hasTransientRateLimitLikeMarker := strings.Contains(s, "overloaded_error") ||
		strings.Contains(s, `"type":"overloaded"`) ||
		strings.Contains(s, "overloaded") ||
		strings.Contains(s, "rate_limit") ||
		strings.Contains(s, "rate limit") ||
		strings.Contains(s, "too many requests") ||
		strings.Contains(s, "throttled") ||
		strings.Contains(s, "capacity")
	if !hasTransientRateLimitLikeMarker {
		return false
	}
	return strings.Contains(s, "upstream 500") ||
		strings.Contains(s, "provider returned 500") ||
		strings.Contains(s, "proxy returned 502")
}

// isNoResponseError returns true for errors indicating the provider returned
// no response content. These are typically tool-level issues (empty tool output,
// invalid tool response) rather than provider issues, so we don't want to
// trigger provider cooldown.
func isNoResponseError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "returned no response") ||
		strings.Contains(s, "returned empty streaming response")
}

// isAuthError returns true for authentication/authorization errors (401/403) that
// indicate permanent credential issues (expired token, invalid API key, etc.).
// Also includes 404 errors that indicate the API endpoint doesn't exist (misconfigured provider).
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	// Match both "upstream 401", "upstream 403", "upstream 404", and proxy-level errors
	// 404 on API endpoint usually means the provider is misconfigured or the token doesn't have access
	return strings.Contains(s, "upstream 401") ||
		strings.Contains(s, "upstream 403") ||
		strings.Contains(s, "upstream 404") ||
		strings.Contains(s, "provider returned 401") ||
		strings.Contains(s, "provider returned 403") ||
		strings.Contains(s, "provider returned 404") ||
		strings.Contains(s, "unauthorized") ||
		strings.Contains(s, "authentication") ||
		strings.Contains(s, "page not found")
}

// RecordSuccess records a successful request and potentially resets cooldown
func (r *Router) RecordSuccess(providerID string) {
	r.cooldownMu.Lock()
	defer r.cooldownMu.Unlock()

	entry, exists := r.cooldowns[providerID]
	if !exists {
		return
	}

	// Reset failure counts on success
	entry.FailureCount = 0
	entry.TransientFailureCount = 0
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
	now := timeutil.NowTime()
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
	if strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		containsHTTPStatusCode(errStr, 429) {
		return FailoverReasonRateLimit
	}
	if strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "invalid api key") ||
		strings.Contains(errStr, "authentication") ||
		containsHTTPStatusCode(errStr, 401) ||
		containsHTTPStatusCode(errStr, 403) {
		return FailoverReasonAuthError
	}
	if strings.Contains(errStr, "model not found") ||
		strings.Contains(errStr, "does not exist") ||
		containsHTTPStatusCode(errStr, 404) {
		return FailoverReasonModelNotFound
	}
	return FailoverReasonAPIError
}

var statusCodeContextMarkers = [...]string{
	"status ",
	"upstream ",
	"provider returned ",
	"returned ",
	"http ",
	"code ",
	"code:",
	"code=",
}

// containsHTTPStatusCode matches real status codes in error text while avoiding
// accidental matches inside provider IDs like "prov_...4012...".
func containsHTTPStatusCode(errStr string, code int) bool {
	if errStr == "" {
		return false
	}

	codeStr := strconv.Itoa(code)

	if strings.HasPrefix(errStr, codeStr+" ") ||
		strings.HasPrefix(errStr, codeStr+":") ||
		strings.HasPrefix(errStr, codeStr+")") ||
		strings.HasPrefix(errStr, codeStr+"]") ||
		strings.Contains(errStr, "("+codeStr+")") ||
		strings.Contains(errStr, "["+codeStr+"]") {
		return true
	}

	for _, marker := range statusCodeContextMarkers {
		needle := marker + codeStr
		idx := strings.Index(errStr, needle)
		for idx >= 0 {
			end := idx + len(needle)
			if end == len(errStr) || !isASCIIDigit(errStr[end]) {
				return true
			}
			nextStart := idx + 1
			if nextStart >= len(errStr) {
				break
			}
			nextIdx := strings.Index(errStr[nextStart:], needle)
			if nextIdx < 0 {
				break
			}
			idx = nextStart + nextIdx
		}
	}

	return false
}

func isASCIIDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// shouldRetryWithNextAPIKey returns true when retrying with a different API key
// could realistically help (auth failures, per-key rate limits, exhausted quota).
func shouldRetryWithNextAPIKey(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "all auth strategies exhausted") ||
		strings.Contains(errStr, "auth error") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "invalid api key") ||
		strings.Contains(errStr, "invalid key") ||
		strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "throttled (429)") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "insufficient_quota") ||
		strings.Contains(errStr, "insufficient quota") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "billing hard limit") ||
		containsHTTPStatusCode(errStr, 401) ||
		containsHTTPStatusCode(errStr, 402) ||
		containsHTTPStatusCode(errStr, 403) ||
		containsHTTPStatusCode(errStr, 429)
}

func shouldRetrySameProvider(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Empty-stream EOFs can be transient on some relays. Retry same provider once
	// before giving up or failing over.
	if strings.Contains(errStr, "returned empty streaming response") {
		return true
	}
	if isNoResponseError(err) {
		return false
	}
	if shouldRetryWithNextAPIKey(err) {
		return false
	}
	if isAuthError(err) || isTransientError(err) {
		return !isAuthError(err)
	}
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "temporary network") ||
		strings.Contains(errStr, "unexpected eof")
}

func shouldAttemptBlindFallback(err error) bool {
	if err == nil {
		return true
	}
	if isNoResponseError(err) {
		return false
	}
	if isTransientError(err) || shouldRetryWithNextAPIKey(err) {
		return true
	}
	if classifyError(err) == FailoverReasonModelNotFound || classifyError(err) == FailoverReasonTimeout {
		return true
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "upstream") ||
		strings.Contains(errStr, "internal server error") ||
		strings.Contains(errStr, "server error") ||
		strings.Contains(errStr, "service unavailable") {
		return true
	}
	if strings.Contains(errStr, "invalid_request") ||
		strings.Contains(errStr, "invalid request") ||
		strings.Contains(errStr, "bad request") ||
		strings.Contains(errStr, "malformed") ||
		strings.Contains(errStr, "content filter") ||
		strings.Contains(errStr, "policy violation") {
		return false
	}
	return false
}

func executeWithNaturalRetry(ctx context.Context, attemptResult *RouteResult, execute func(*RouteResult) error) (time.Duration, error) {
	start := timeutil.NowTime()
	err := execute(attemptResult)
	if err != nil && ctx.Err() == nil && shouldRetrySameProvider(err) {
		err = execute(attemptResult)
	}
	return timeutil.SinceTime(start), err
}

// buildAPIKeyAttempts builds an ordered API key list to try for a provider.
// Order: preferred key first, then other enabled keys, then first key fallback.
func buildAPIKeyAttempts(provider *Provider, preferred *APIKey) []*APIKey {
	attempts := make([]*APIKey, 0, 2)
	seen := make(map[string]struct{})

	keyFingerprint := func(k *APIKey) string {
		return apiKeyFingerprint(k)
	}

	add := func(k *APIKey) {
		if k == nil || strings.TrimSpace(k.Key) == "" {
			return
		}
		fp := keyFingerprint(k)
		if fp != "" {
			if _, exists := seen[fp]; exists {
				return
			}
			seen[fp] = struct{}{}
		}
		keyCopy := *k
		attempts = append(attempts, &keyCopy)
	}

	add(preferred)

	if provider != nil {
		for i := range provider.APIKeys {
			key := &provider.APIKeys[i]
			if !key.Enabled {
				continue
			}
			add(key)
		}
		// Backward-compat: if no enabled keys exist, try the first key once.
		if len(attempts) == 0 && len(provider.APIKeys) > 0 {
			add(&provider.APIKeys[0])
		}
	}

	return attempts
}

// RouteWithFallback attempts to route with automatic fallback on failure.
// When the exact model is not found or all same-model providers fail,
// it falls back to any healthy provider matching the routing mode (auto/cloud/local),
// forwarding the original model name and letting the upstream decide.
func (r *Router) RouteWithFallback(ctx context.Context, req *RouteRequest, execute func(*RouteResult) error) error {
	// Only allocate failover tracking when a callback is registered
	var failoverResult *FailoverResult
	if r.failoverCallback != nil {
		failoverResult = &FailoverResult{
			RequestID: uuid.New().String(),
			StartTime: timeutil.NowTime(),
		}
		defer func() {
			failoverResult.EndTime = timeutil.NowTime()
			r.failoverCallback(failoverResult)
		}()
	}

	// Track which providers we've already tried (to avoid retrying in blind fallback).
	// Seed with explicit exclusions too — callers use Exclude for hard bans during
	// a retry window (for example, dropping an overloaded pinned provider), and
	// blind fallback must not reintroduce them.
	triedProviders := make(map[string]bool, len(req.Exclude))
	for _, providerID := range req.Exclude {
		if strings.TrimSpace(providerID) == "" {
			continue
		}
		triedProviders[providerID] = true
	}

	allowBlindFallback := false
	var lastErr error

	result, err := r.Route(req)
	if err != nil {
		lastErr = err
		// Empty model means "auto pick any available model". Blind fallback is not
		// useful in this case because it forwards req.ModelID as-is, which would be
		// empty and lead to provider-side model selection errors/misleading logs.
		if strings.TrimSpace(req.ModelID) == "" {
			if failoverResult != nil {
				failoverResult.FinalError = ErrNoAvailableProvider.Error()
			}
			return ErrNoAvailableProvider
		}
		// Model not in snapshot — skip to blind provider fallback
		slog.Info("[router] model not in snapshot, trying blind provider fallback",
			"model", req.ModelID, "mode", req.Mode)
		allowBlindFallback = true
		goto blindFallback
	}

	{
		primaryKeyAttempts := buildAPIKeyAttempts(result.Provider, result.APIKey)
		if len(primaryKeyAttempts) == 0 {
			primaryKeyAttempts = []*APIKey{nil}
		}

		// Try primary provider (with API key failover inside the same provider)
		for keyIdx, apiKey := range primaryKeyAttempts {
			if ctx.Err() != nil {
				if failoverResult != nil {
					failoverResult.FinalError = ctx.Err().Error()
				}
				return ctx.Err()
			}

			attemptResult := &RouteResult{
				Provider: result.Provider,
				Model:    result.Model,
				APIKey:   apiKey,
				OAuth:    result.OAuth,
			}

			if failoverResult != nil {
				failoverResult.TotalAttempts++
			}
			start := timeutil.NowTime()
			latency, attemptErr := executeWithNaturalRetry(ctx, attemptResult, execute)
			triedProviders[result.Provider.ID] = true

			if attemptErr == nil {
				r.UpdateLatency(result.Provider.ID, latency)
				r.RecordSuccess(result.Provider.ID)
				if failoverResult != nil {
					failoverResult.SuccessProvider = result.Provider.ID
					failoverResult.SuccessModel = result.Model.ID
				}
				return nil
			}
			lastErr = attemptErr

			r.UpdateLatency(result.Provider.ID, latency*2)
			r.RecordFailure(result.Provider.ID, attemptErr)

			if failoverResult != nil {
				failoverResult.FailedAttempts = append(failoverResult.FailedAttempts, &FailoverRecord{
					Timestamp:    start,
					ProviderID:   result.Provider.ID,
					ProviderName: result.Provider.Name,
					ModelID:      result.Model.ID,
					Reason:       classifyError(attemptErr),
					Error:        attemptErr.Error(),
					Latency:      latency,
				})
			}

			// Auth-like failures may be caused by a single bad key — try next key on same provider.
			if keyIdx+1 < len(primaryKeyAttempts) && shouldRetryWithNextAPIKey(attemptErr) {
				continue
			}
			break
		}

		// Try same-model fallbacks
		for i, fallback := range result.Fallbacks {
			if ctx.Err() != nil {
				if failoverResult != nil {
					failoverResult.FinalError = ctx.Err().Error()
				}
				return ctx.Err()
			}

			if failoverResult != nil && len(failoverResult.FailedAttempts) > 0 {
				failoverResult.FailedAttempts[len(failoverResult.FailedAttempts)-1].NextProviderID = fallback.Provider.ID
			}

			fallbackResult := &RouteResult{
				Provider: fallback.Provider,
				Model:    fallback.Model,
			}
			if apiKey, keyErr := r.registry.GetAPIKey(fallback.Provider.ID); keyErr == nil {
				fallbackResult.APIKey = apiKey
			}
			if fallbackResult.APIKey == nil {
				if oauthCfg, oauthErr := r.registry.GetOAuthConfig(fallback.Provider.ID); oauthErr == nil {
					fallbackResult.OAuth = oauthCfg
				}
			}

			fallbackKeyAttempts := buildAPIKeyAttempts(fallback.Provider, fallbackResult.APIKey)
			if len(fallbackKeyAttempts) == 0 {
				fallbackKeyAttempts = []*APIKey{nil}
			}

			for keyIdx, apiKey := range fallbackKeyAttempts {
				if ctx.Err() != nil {
					if failoverResult != nil {
						failoverResult.FinalError = ctx.Err().Error()
					}
					return ctx.Err()
				}

				attemptResult := &RouteResult{
					Provider: fallback.Provider,
					Model:    fallback.Model,
					APIKey:   apiKey,
					OAuth:    fallbackResult.OAuth,
				}

				if failoverResult != nil {
					failoverResult.TotalAttempts++
				}
				start := timeutil.NowTime()
				latency, attemptErr := executeWithNaturalRetry(ctx, attemptResult, execute)
				triedProviders[fallback.Provider.ID] = true

				if attemptErr == nil {
					r.UpdateLatency(fallback.Provider.ID, latency)
					r.RecordSuccess(fallback.Provider.ID)
					if failoverResult != nil {
						failoverResult.SuccessProvider = fallback.Provider.ID
						failoverResult.SuccessModel = fallback.Model.ID
					}
					return nil
				}
				lastErr = attemptErr

				r.UpdateLatency(fallback.Provider.ID, latency*2)
				r.RecordFailure(fallback.Provider.ID, attemptErr)

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
						Reason:         classifyError(attemptErr),
						Error:          attemptErr.Error(),
						Latency:        latency,
						NextProviderID: nextProviderID,
					})
				}

				if keyIdx+1 < len(fallbackKeyAttempts) && shouldRetryWithNextAPIKey(attemptErr) {
					continue
				}
				break
			}
		}
	}

	allowBlindFallback = shouldAttemptBlindFallback(lastErr)

blindFallback:
	if !allowBlindFallback {
		if failoverResult != nil {
			if lastErr != nil {
				failoverResult.FinalError = lastErr.Error()
			} else {
				failoverResult.FinalError = ErrNoAvailableProvider.Error()
			}
		}
		if lastErr != nil {
			return lastErr
		}
		return ErrNoAvailableProvider
	}

	// Blind fallback is only meaningful for explicit model IDs not present in the
	// snapshot. For empty model requests, providers were already exhausted above.
	if strings.TrimSpace(req.ModelID) == "" {
		if failoverResult != nil {
			if lastErr != nil {
				failoverResult.FinalError = lastErr.Error()
			} else {
				failoverResult.FinalError = ErrNoAvailableProvider.Error()
			}
		}
		if lastErr != nil {
			return lastErr
		}
		return ErrNoAvailableProvider
	}

	// Blind provider fallback: try any healthy provider matching the routing mode.
	// The original model name is forwarded as-is — the upstream decides if it supports it.
	// This handles cases where our local model list is incomplete or the model is new.
	blindCandidates := r.findBlindFallbackProviders(req.ModelID, req.Mode, triedProviders)
	if len(blindCandidates) == 0 {
		if failoverResult != nil {
			if lastErr != nil {
				failoverResult.FinalError = lastErr.Error()
			} else {
				failoverResult.FinalError = ErrNoAvailableProvider.Error()
			}
		}
		if lastErr != nil {
			return lastErr
		}
		return ErrNoAvailableProvider
	}

	slog.Info("[router] trying blind provider fallback",
		"model", req.ModelID, "candidates", len(blindCandidates))

	// Use a passthrough Model with the original model ID so tryOnProvider sends it as-is
	passthroughModel := &Model{
		ID:      req.ModelID,
		Name:    req.ModelID,
		Enabled: true,
	}

	for _, provider := range blindCandidates {
		if ctx.Err() != nil {
			if failoverResult != nil {
				failoverResult.FinalError = ctx.Err().Error()
			}
			return ctx.Err()
		}

		blindResult := &RouteResult{
			Provider: provider,
			Model:    passthroughModel,
		}
		if apiKey, keyErr := r.registry.GetAPIKey(provider.ID); keyErr == nil {
			blindResult.APIKey = apiKey
		}
		if blindResult.APIKey == nil {
			if oauthCfg, oauthErr := r.registry.GetOAuthConfig(provider.ID); oauthErr == nil {
				blindResult.OAuth = oauthCfg
			}
		}

		blindKeyAttempts := buildAPIKeyAttempts(provider, blindResult.APIKey)
		if len(blindKeyAttempts) == 0 {
			blindKeyAttempts = []*APIKey{nil}
		}

		for keyIdx, apiKey := range blindKeyAttempts {
			if ctx.Err() != nil {
				if failoverResult != nil {
					failoverResult.FinalError = ctx.Err().Error()
				}
				return ctx.Err()
			}

			attemptResult := &RouteResult{
				Provider: provider,
				Model:    passthroughModel,
				APIKey:   apiKey,
				OAuth:    blindResult.OAuth,
			}

			if failoverResult != nil {
				failoverResult.TotalAttempts++
			}
			start := timeutil.NowTime()
			latency, attemptErr := executeWithNaturalRetry(ctx, attemptResult, execute)

			if attemptErr == nil {
				r.UpdateLatency(provider.ID, latency)
				r.RecordSuccess(provider.ID)
				if failoverResult != nil {
					failoverResult.SuccessProvider = provider.ID
					failoverResult.SuccessModel = req.ModelID
				}
				slog.Info("[router] blind fallback succeeded", "provider", provider.ID, "model", req.ModelID)
				return nil
			}
			lastErr = attemptErr

			r.UpdateLatency(provider.ID, latency*2)
			r.RecordFailure(provider.ID, attemptErr)

			if failoverResult != nil {
				failoverResult.FailedAttempts = append(failoverResult.FailedAttempts, &FailoverRecord{
					Timestamp:    start,
					ProviderID:   provider.ID,
					ProviderName: provider.Name,
					ModelID:      req.ModelID,
					Reason:       classifyError(attemptErr),
					Error:        attemptErr.Error(),
					Latency:      latency,
				})
			}

			if keyIdx+1 < len(blindKeyAttempts) && shouldRetryWithNextAPIKey(attemptErr) {
				continue
			}
			break
		}
	}

	if failoverResult != nil {
		if lastErr != nil {
			failoverResult.FinalError = lastErr.Error()
		} else {
			failoverResult.FinalError = ErrNoAvailableProvider.Error()
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return ErrNoAvailableProvider
}

// findBlindFallbackProviders returns healthy providers matching the routing mode,
// excluding already-tried providers. Providers with a curated/fixed model list
// (e.g. Copilot, Anthropic) are skipped if the requested model isn't in their
// known model list — blind fallback is only useful for open-ended providers.
// Sorted by priority (highest first).
func (r *Router) findBlindFallbackProviders(modelID string, mode RoutingMode, exclude map[string]bool) []*Provider {
	providers := r.registry.ListEnabled()
	var candidates []*Provider
	bypassPenalty := r.shouldBypassPenalty()

	for _, p := range providers {
		if exclude[p.ID] {
			continue
		}
		if p.Status == ProviderStatusError && !bypassPenalty {
			continue
		}
		if !bypassPenalty && r.IsInCooldown(p.ID) {
			continue
		}
		// Cloud providers need usable credentials (API key or OAuth)
		if p.Location == ProviderLocationCloud && !r.hasCredentials(p) {
			continue
		}
		// Filter by routing mode
		if mode != "" && mode != RoutingModeAuto {
			if mode == RoutingModeCloud && p.Location != ProviderLocationCloud {
				continue
			}
			if mode == RoutingModeLocal && p.Location != ProviderLocationLocal {
				continue
			}
		}
		// Respect provider model allowlist during blind fallback too.
		if !providerAllowsModel(p, modelID) {
			continue
		}
		// Skip providers with curated model lists that don't include this model.
		// Copilot/Anthropic have fixed model sets — sending unknown models just
		// wastes a round-trip and pollutes logs with 404s.
		if modelID != "" && isCuratedProvider(p) {
			// For Copilot, skip entirely in blind fallback since we know the exact model set
			// and GetModel may have issues with caching/timing. This prevents 404 spam.
			if p.APIFormat == APIFormatCopilot {
				slog.Debug("[router] skipping Copilot in blind fallback for unknown model",
					"model", modelID)
				continue
			}
			if _, err := r.discovery.GetModel(p.ID, modelID); err != nil {
				continue
			}
		}
		candidates = append(candidates, p)
	}

	// Sort by model/provider affinity first, then priority.
	sort.SliceStable(candidates, func(i, j int) bool {
		affinityI := providerFormatAffinityScore(modelID, candidates[i])
		affinityJ := providerFormatAffinityScore(modelID, candidates[j])
		if affinityI != affinityJ {
			return affinityI > affinityJ
		}
		return candidates[i].Priority > candidates[j].Priority
	})

	return candidates
}

// providerAllowsModel checks whether a provider's allowlist allows modelID.
// For modelID=="", allowlist filtering is bypassed (router will pick provider default).
func providerAllowsModel(p *Provider, modelID string) bool {
	if modelID == "" {
		return true
	}

	// Backward compatibility: pre-flag data with non-nil AllowedModels still means configured.
	allowlistConfigured := p.AllowlistConfigured || p.AllowedModels != nil
	if !allowlistConfigured {
		return true
	}
	if len(p.AllowedModels) == 0 {
		return false
	}
	for _, allowed := range p.AllowedModels {
		if allowed == modelID {
			return true
		}
	}
	return false
}

// isCuratedProvider returns true for providers with a fixed/curated model list.
// These providers only support specific models — sending unknown model IDs
// results in 404s. Blind fallback should skip them unless the model is known.
func isCuratedProvider(p *Provider) bool {
	switch p.APIFormat {
	case APIFormatCopilot, APIFormatAnthropic:
		return true
	}
	return false
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
