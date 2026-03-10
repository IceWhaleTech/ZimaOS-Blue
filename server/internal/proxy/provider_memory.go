package proxy

import (
	"log/slog"
	"strings"
	"time"

	ecache2 "github.com/orca-zhang/ecache2"
)

// ToolCapLevel represents the tool capability level of a provider endpoint.
type ToolCapLevel int

const (
	ToolCapNative  ToolCapLevel = iota // Native tool_use supported
	ToolCapPrompt                      // Tools via system prompt injection
	ToolCapNone                        // No tools at all
	ToolCapUnknown ToolCapLevel = -1   // Not yet probed
)

// ProviderMemory remembers provider capabilities discovered at runtime.
// Uses a single ecache2 with uint64 keys (FNV-1a hashes) for zero-alloc lookups.
type ProviderMemory struct {
	cache *ecache2.Cache[uint64]
}

// NewProviderMemory creates a new provider memory with sensible TTLs.
func NewProviderMemory() *ProviderMemory {
	return &ProviderMemory{
		cache: ecache2.NewLRUCache[uint64](4, 512, 24*time.Hour),
	}
}

// memKey builds a cache key as a raw FNV-1a uint64 hash.
// Zero allocation — used directly as ecache2 uint64 key.
func memKey(prefix, providerID, baseURL string) uint64 {
	host := extractHost(baseURL)
	hash := fnvOffset64
	for i := 0; i < len(prefix); i++ {
		hash ^= uint64(prefix[i])
		hash *= fnvPrime64
	}
	for i := 0; i < len(providerID); i++ {
		hash ^= uint64(providerID[i])
		hash *= fnvPrime64
	}
	if host != "" {
		hash ^= uint64(':')
		hash *= fnvPrime64
		for i := 0; i < len(host); i++ {
			hash ^= uint64(host[i])
			hash *= fnvPrime64
		}
	}
	return hash
}

// extractHost extracts the host from a URL string without url.Parse allocation.
func extractHost(rawURL string) string {
	// Skip scheme: find "://"
	idx := strings.Index(rawURL, "://")
	if idx < 0 {
		return ""
	}
	rest := rawURL[idx+3:]
	// Host ends at '/', '?', '#', or end of string
	for i := 0; i < len(rest); i++ {
		if rest[i] == '/' || rest[i] == '?' || rest[i] == '#' {
			return rest[:i]
		}
	}
	return rest
}

// memKeyWithSuffix builds a cache key with an extra suffix (e.g. model name) as raw FNV-1a uint64.
func memKeyWithSuffix(prefix, providerID, baseURL, suffix string) uint64 {
	host := extractHost(baseURL)
	hash := fnvOffset64
	for i := 0; i < len(prefix); i++ {
		hash ^= uint64(prefix[i])
		hash *= fnvPrime64
	}
	for i := 0; i < len(providerID); i++ {
		hash ^= uint64(providerID[i])
		hash *= fnvPrime64
	}
	if host != "" {
		hash ^= uint64(':')
		hash *= fnvPrime64
		for i := 0; i < len(host); i++ {
			hash ^= uint64(host[i])
			hash *= fnvPrime64
		}
	}
	hash ^= uint64(':')
	hash *= fnvPrime64
	for i := 0; i < len(suffix); i++ {
		hash ^= uint64(suffix[i])
		hash *= fnvPrime64
	}
	return hash
}

func normalizeMemoryBaseURL(rawURL string) string {
	return strings.TrimSuffix(strings.TrimSpace(rawURL), "/")
}

func memKeyWithFullBaseURL(prefix, providerID, baseURL, suffix string) uint64 {
	baseURL = normalizeMemoryBaseURL(baseURL)
	suffix = strings.TrimSpace(suffix)
	hash := fnvOffset64
	for i := 0; i < len(prefix); i++ {
		hash ^= uint64(prefix[i])
		hash *= fnvPrime64
	}
	for i := 0; i < len(providerID); i++ {
		hash ^= uint64(providerID[i])
		hash *= fnvPrime64
	}
	if baseURL != "" {
		hash ^= uint64(':')
		hash *= fnvPrime64
		for i := 0; i < len(baseURL); i++ {
			hash ^= uint64(baseURL[i])
			hash *= fnvPrime64
		}
	}
	if suffix != "" {
		hash ^= uint64(':')
		hash *= fnvPrime64
		for i := 0; i < len(suffix); i++ {
			hash ^= uint64(suffix[i])
			hash *= fnvPrime64
		}
	}
	return hash
}

// --- Format memory ---

// RecallFormat returns the remembered API format for a provider (as a raw string).
func (pm *ProviderMemory) RecallFormat(providerID, baseURL string) (string, bool) {
	if v, ok := pm.cache.Get(memKey("fmt:", providerID, baseURL)); ok {
		if pt, ok := v.(string); ok {
			return pt, true
		}
	}
	return "", false
}

// RememberFormat caches the API format that worked for a provider.
func (pm *ProviderMemory) RememberFormat(providerID, baseURL string, format string) {
	pm.cache.Put(memKey("fmt:", providerID, baseURL), format)
	slog.Debug("[provider-memory] remembered format", "provider", providerID, "format", format)
}

// ForgetFormat evicts the cached format.
func (pm *ProviderMemory) ForgetFormat(providerID, baseURL string) {
	pm.cache.Del(memKey("fmt:", providerID, baseURL))
}

// RecallModelFormat returns the remembered API format for a specific provider+baseURL+requested model.
func (pm *ProviderMemory) RecallModelFormat(providerID, baseURL, requestedModel string) (string, bool) {
	if v, ok := pm.cache.Get(memKeyWithFullBaseURL("model-fmt:", providerID, baseURL, requestedModel)); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// RememberModelFormat caches the API format that worked for a specific requested model.
func (pm *ProviderMemory) RememberModelFormat(providerID, baseURL, requestedModel, format string) {
	pm.cache.Put(memKeyWithFullBaseURL("model-fmt:", providerID, baseURL, requestedModel), format)
	slog.Debug("[provider-memory] remembered model format", "provider", providerID, "base_url", normalizeMemoryBaseURL(baseURL), "requested", strings.TrimSpace(requestedModel), "format", format)
}

// ForgetModelFormat evicts the cached model-scoped format.
func (pm *ProviderMemory) ForgetModelFormat(providerID, baseURL, requestedModel string) {
	pm.cache.Del(memKeyWithFullBaseURL("model-fmt:", providerID, baseURL, requestedModel))
}

// --- Model alias memory ---

// RecallModelAlias returns the actual model name that worked for a requested model.
func (pm *ProviderMemory) RecallModelAlias(providerID, baseURL, requestedModel string) (string, bool) {
	if v, ok := pm.cache.Get(memKeyWithSuffix("model:", providerID, baseURL, requestedModel)); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// RememberModelAlias caches a model name mapping that worked.
func (pm *ProviderMemory) RememberModelAlias(providerID, baseURL, requestedModel, actualModel string) {
	pm.cache.Put(memKeyWithSuffix("model:", providerID, baseURL, requestedModel), actualModel)
	slog.Debug("[provider-memory] remembered model alias", "provider", providerID, "requested", requestedModel, "actual", actualModel)
}

// ForgetModelAlias evicts a cached model alias.
func (pm *ProviderMemory) ForgetModelAlias(providerID, baseURL, requestedModel string) {
	pm.cache.Del(memKeyWithSuffix("model:", providerID, baseURL, requestedModel))
}

// --- Tool capability memory ---

// RecallToolCap returns the remembered tool capability level.
func (pm *ProviderMemory) RecallToolCap(providerID, baseURL string) (ToolCapLevel, bool) {
	if v, ok := pm.cache.Get(memKey("tool:", providerID, baseURL)); ok {
		if level, ok := v.(ToolCapLevel); ok {
			return level, true
		}
	}
	return ToolCapUnknown, false
}

// RememberToolCap caches the tool capability level that worked.
func (pm *ProviderMemory) RememberToolCap(providerID, baseURL string, level ToolCapLevel) {
	pm.cache.Put(memKey("tool:", providerID, baseURL), level)
	slog.Debug("[provider-memory] remembered tool cap", "provider", providerID, "level", level)
}

// ForgetToolCap evicts the cached tool capability.
func (pm *ProviderMemory) ForgetToolCap(providerID, baseURL string) {
	pm.cache.Del(memKey("tool:", providerID, baseURL))
}

// --- Throttle memory ---

// IsThrottled returns true if the provider is currently in a backoff period.
func (pm *ProviderMemory) IsThrottled(providerID, baseURL string) bool {
	if v, ok := pm.cache.Get(memKey("throttle:", providerID, baseURL)); ok {
		if until, ok := v.(int64); ok {
			return time.Now().UnixNano() < until
		}
	}
	return false
}

// RememberThrottle records a 429 backoff for a provider.
func (pm *ProviderMemory) RememberThrottle(providerID, baseURL string, retryAfter time.Duration) {
	if retryAfter <= 0 {
		retryAfter = 30 * time.Second
	}
	until := time.Now().Add(retryAfter).UnixNano()
	pm.cache.Put(memKey("throttle:", providerID, baseURL), until)
	slog.Info("[provider-memory] throttled provider", "provider", providerID, "retry_after", retryAfter)
}

// ForgetThrottle clears the throttle for a provider.
func (pm *ProviderMemory) ForgetThrottle(providerID, baseURL string) {
	pm.cache.Del(memKey("throttle:", providerID, baseURL))
}

// --- Model blacklist memory ---

// IsModelBlacklisted returns true if a model is known to be unavailable on this provider.
func (pm *ProviderMemory) IsModelBlacklisted(providerID, baseURL, model string) bool {
	_, ok := pm.cache.Get(memKeyWithSuffix("bl:", providerID, baseURL, model))
	return ok
}

// BlacklistModel marks a model as unavailable on this provider (skip it next time).
func (pm *ProviderMemory) BlacklistModel(providerID, baseURL, model string) {
	pm.cache.Put(memKeyWithSuffix("bl:", providerID, baseURL, model), true)
	slog.Debug("[provider-memory] blacklisted model", "provider", providerID, "model", model)
}

// ClearModelBlacklist removes a model from the blacklist (user can retry after quota recharge, etc).
func (pm *ProviderMemory) ClearModelBlacklist(providerID, baseURL, model string) {
	pm.cache.Del(memKeyWithSuffix("bl:", providerID, baseURL, model))
	slog.Info("[provider-memory] cleared model blacklist", "provider", providerID, "model", model)
}

// ClearThrottle removes throttle restriction (user can retry after service recovery).
func (pm *ProviderMemory) ClearThrottle(providerID, baseURL string) {
	pm.cache.Del(memKey("throttle:", providerID, baseURL))
	slog.Info("[provider-memory] cleared throttle", "provider", providerID)
}

// GetRestrictions returns all current restrictions (throttles + blacklists) for a provider.
// Used to show users what's temporarily restricted.
func (pm *ProviderMemory) GetRestrictions(providerID, baseURL string) map[string]interface{} {
	throttleKey := memKey("throttle:", providerID, baseURL)
	restrictions := map[string]interface{}{
		"throttled":          false,
		"blacklisted_models": []string{},
	}

	// Check throttle
	if v, ok := pm.cache.Get(throttleKey); ok {
		if until, ok := v.(int64); ok && time.Now().UnixNano() < until {
			restrictions["throttled"] = true
			restrictions["throttle_until"] = time.Unix(0, until).Format(time.RFC3339)
		}
	}

	// Note: ecache doesn't provide iteration, so we can't enumerate all blacklisted models

	return restrictions
}

// ModelAliases maps model names to common alternatives for relay compatibility.
var ModelAliases = map[string][]string{
	"claude-3-5-haiku-20241022":  {"claude-haiku-4-5", "claude-3.5-haiku"},
	"claude-haiku-4-5":           {"claude-3-5-haiku-20241022", "claude-3.5-haiku"},
	"claude-3-5-sonnet-20241022": {"claude-sonnet-4-5", "claude-3.5-sonnet"},
	"claude-sonnet-4-5":          {"claude-3-5-sonnet-20241022", "claude-3.5-sonnet"},
	"claude-3-opus-20240229":     {"claude-opus-4-5", "claude-3-opus"},
	"claude-opus-4-5":            {"claude-3-opus-20240229", "claude-3-opus"},
	"claude-opus-4-6":            {"claude-opus-4-5-20251101"},
	"claude-sonnet-4-20250514":   {"claude-sonnet-4"},
}
