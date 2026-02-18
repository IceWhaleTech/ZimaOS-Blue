package proxy

import (
	"log/slog"
	"net/url"
	"time"

	"github.com/orca-zhang/ecache"
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
// Uses a single ecache with different prefixes for different data types.
type ProviderMemory struct {
	cache *ecache.Cache // All data with type prefixes: "fmt:", "model:", "tool:", "throttle:", "blacklist:"
}

// NewProviderMemory creates a new provider memory with sensible TTLs.
func NewProviderMemory() *ProviderMemory {
	return &ProviderMemory{
		cache: ecache.NewLRUCache(4, 512, 24*time.Hour), // Single cache for all data
	}
}

// memKey builds a cache key scoped to provider + upstream host.
func memKey(providerID, baseURL string) string {
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		return providerID + ":" + u.Host
	}
	return providerID
}

// --- Format memory ---

// RecallFormat returns the remembered API format for a provider (as a raw string).
func (pm *ProviderMemory) RecallFormat(providerID, baseURL string) (string, bool) {
	if v, ok := pm.cache.Get("fmt:" + memKey(providerID, baseURL)); ok {
		if pt, ok := v.(string); ok {
			return pt, true
		}
	}
	return "", false
}

// RememberFormat caches the API format that worked for a provider.
func (pm *ProviderMemory) RememberFormat(providerID, baseURL string, format string) {
	pm.cache.Put("fmt:"+memKey(providerID, baseURL), format)
	slog.Debug("[provider-memory] remembered format", "provider", providerID, "format", format)
}

// ForgetFormat evicts the cached format.
func (pm *ProviderMemory) ForgetFormat(providerID, baseURL string) {
	pm.cache.Del("fmt:" + memKey(providerID, baseURL))
}

// --- Model alias memory ---

// RecallModelAlias returns the actual model name that worked for a requested model.
func (pm *ProviderMemory) RecallModelAlias(providerID, baseURL, requestedModel string) (string, bool) {
	key := "model:" + memKey(providerID, baseURL) + ":" + requestedModel
	if v, ok := pm.cache.Get(key); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// RememberModelAlias caches a model name mapping that worked.
func (pm *ProviderMemory) RememberModelAlias(providerID, baseURL, requestedModel, actualModel string) {
	key := "model:" + memKey(providerID, baseURL) + ":" + requestedModel
	pm.cache.Put(key, actualModel)
	slog.Debug("[provider-memory] remembered model alias", "provider", providerID, "requested", requestedModel, "actual", actualModel)
}

// ForgetModelAlias evicts a cached model alias.
func (pm *ProviderMemory) ForgetModelAlias(providerID, baseURL, requestedModel string) {
	key := "model:" + memKey(providerID, baseURL) + ":" + requestedModel
	pm.cache.Del(key)
}

// --- Tool capability memory ---

// RecallToolCap returns the remembered tool capability level.
func (pm *ProviderMemory) RecallToolCap(providerID, baseURL string) (ToolCapLevel, bool) {
	if v, ok := pm.cache.Get("tool:" + memKey(providerID, baseURL)); ok {
		if level, ok := v.(ToolCapLevel); ok {
			return level, true
		}
	}
	return ToolCapUnknown, false
}

// RememberToolCap caches the tool capability level that worked.
func (pm *ProviderMemory) RememberToolCap(providerID, baseURL string, level ToolCapLevel) {
	pm.cache.Put("tool:"+memKey(providerID, baseURL), level)
	slog.Debug("[provider-memory] remembered tool cap", "provider", providerID, "level", level)
}

// ForgetToolCap evicts the cached tool capability.
func (pm *ProviderMemory) ForgetToolCap(providerID, baseURL string) {
	pm.cache.Del("tool:" + memKey(providerID, baseURL))
}

// --- Throttle memory ---

// IsThrottled returns true if the provider is currently in a backoff period.
func (pm *ProviderMemory) IsThrottled(providerID, baseURL string) bool {
	if v, ok := pm.cache.Get("throttle:" + memKey(providerID, baseURL)); ok {
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
	pm.cache.Put("throttle:"+memKey(providerID, baseURL), until)
	slog.Info("[provider-memory] throttled provider", "provider", providerID, "retry_after", retryAfter)
}

// ForgetThrottle clears the throttle for a provider.
func (pm *ProviderMemory) ForgetThrottle(providerID, baseURL string) {
	pm.cache.Del("throttle:" + memKey(providerID, baseURL))
}

// --- Model blacklist memory ---

// IsModelBlacklisted returns true if a model is known to be unavailable on this provider.
func (pm *ProviderMemory) IsModelBlacklisted(providerID, baseURL, model string) bool {
	key := "blacklist:" + memKey(providerID, baseURL) + ":" + model
	_, ok := pm.cache.Get(key)
	return ok
}

// BlacklistModel marks a model as unavailable on this provider (skip it next time).
func (pm *ProviderMemory) BlacklistModel(providerID, baseURL, model string) {
	key := "blacklist:" + memKey(providerID, baseURL) + ":" + model
	pm.cache.Put(key, true)
	slog.Debug("[provider-memory] blacklisted model", "provider", providerID, "model", model)
}

// ClearModelBlacklist removes a model from the blacklist (user can retry after quota recharge, etc).
func (pm *ProviderMemory) ClearModelBlacklist(providerID, baseURL, model string) {
	key := "blacklist:" + memKey(providerID, baseURL) + ":" + model
	pm.cache.Del(key)
	slog.Info("[provider-memory] cleared model blacklist", "provider", providerID, "model", model)
}

// ClearThrottle removes throttle restriction (user can retry after service recovery).
func (pm *ProviderMemory) ClearThrottle(providerID, baseURL string) {
	pm.cache.Del("throttle:" + memKey(providerID, baseURL))
	slog.Info("[provider-memory] cleared throttle", "provider", providerID)
}

// GetRestrictions returns all current restrictions (throttles + blacklists) for a provider.
// Used to show users what's temporarily restricted.
func (pm *ProviderMemory) GetRestrictions(providerID, baseURL string) map[string]interface{} {
	key := memKey(providerID, baseURL)
	restrictions := map[string]interface{}{
		"throttled": false,
		"blacklisted_models": []string{},
	}

	// Check throttle
	if v, ok := pm.cache.Get("throttle:" + key); ok {
		if until, ok := v.(int64); ok && time.Now().UnixNano() < until {
			restrictions["throttled"] = true
			restrictions["throttle_until"] = time.Unix(0, until).Format(time.RFC3339)
		}
	}

	// Check blacklisted models (scan cache for blacklist: prefix)
	// Note: ecache doesn't provide iteration, so we can't enumerate all blacklisted models
	// This is a limitation - consider adding to database for full visibility

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
