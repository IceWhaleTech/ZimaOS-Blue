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
// Each dimension uses an independent ecache LRU with its own TTL.
type ProviderMemory struct {
	format   *ecache.Cache // providerID:host → ProviderType
	models   *ecache.Cache // providerID:host:model → actual model name
	toolCap  *ecache.Cache // providerID:host → ToolCapLevel
	throttle *ecache.Cache // providerID:host → backoff-until (unix nano)
}

// NewProviderMemory creates a new provider memory with sensible TTLs.
func NewProviderMemory() *ProviderMemory {
	return &ProviderMemory{
		format:   ecache.NewLRUCache(4, 64, 24*time.Hour),
		models:   ecache.NewLRUCache(4, 128, 2*time.Hour),
		toolCap:  ecache.NewLRUCache(4, 64, 1*time.Hour),
		throttle: ecache.NewLRUCache(4, 64, 5*time.Minute),
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
	if v, ok := pm.format.Get(memKey(providerID, baseURL)); ok {
		if pt, ok := v.(string); ok {
			return pt, true
		}
	}
	return "", false
}

// RememberFormat caches the API format that worked for a provider.
func (pm *ProviderMemory) RememberFormat(providerID, baseURL string, format string) {
	pm.format.Put(memKey(providerID, baseURL), format)
	slog.Debug("[provider-memory] remembered format", "provider", providerID, "format", format)
}

// ForgetFormat evicts the cached format.
func (pm *ProviderMemory) ForgetFormat(providerID, baseURL string) {
	pm.format.Del(memKey(providerID, baseURL))
}

// --- Model alias memory ---

// RecallModelAlias returns the actual model name that worked for a requested model.
func (pm *ProviderMemory) RecallModelAlias(providerID, baseURL, requestedModel string) (string, bool) {
	key := memKey(providerID, baseURL) + ":" + requestedModel
	if v, ok := pm.models.Get(key); ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}

// RememberModelAlias caches a model name mapping that worked.
func (pm *ProviderMemory) RememberModelAlias(providerID, baseURL, requestedModel, actualModel string) {
	key := memKey(providerID, baseURL) + ":" + requestedModel
	pm.models.Put(key, actualModel)
	slog.Debug("[provider-memory] remembered model alias", "provider", providerID, "requested", requestedModel, "actual", actualModel)
}

// ForgetModelAlias evicts a cached model alias.
func (pm *ProviderMemory) ForgetModelAlias(providerID, baseURL, requestedModel string) {
	key := memKey(providerID, baseURL) + ":" + requestedModel
	pm.models.Del(key)
}

// --- Tool capability memory ---

// RecallToolCap returns the remembered tool capability level.
func (pm *ProviderMemory) RecallToolCap(providerID, baseURL string) (ToolCapLevel, bool) {
	if v, ok := pm.toolCap.Get(memKey(providerID, baseURL)); ok {
		if level, ok := v.(ToolCapLevel); ok {
			return level, true
		}
	}
	return ToolCapUnknown, false
}

// RememberToolCap caches the tool capability level that worked.
func (pm *ProviderMemory) RememberToolCap(providerID, baseURL string, level ToolCapLevel) {
	pm.toolCap.Put(memKey(providerID, baseURL), level)
	slog.Debug("[provider-memory] remembered tool cap", "provider", providerID, "level", level)
}

// ForgetToolCap evicts the cached tool capability.
func (pm *ProviderMemory) ForgetToolCap(providerID, baseURL string) {
	pm.toolCap.Del(memKey(providerID, baseURL))
}

// --- Throttle memory ---

// IsThrottled returns true if the provider is currently in a backoff period.
func (pm *ProviderMemory) IsThrottled(providerID, baseURL string) bool {
	if v, ok := pm.throttle.Get(memKey(providerID, baseURL)); ok {
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
	pm.throttle.Put(memKey(providerID, baseURL), until)
	slog.Info("[provider-memory] throttled provider", "provider", providerID, "retry_after", retryAfter)
}

// ForgetThrottle clears the throttle for a provider.
func (pm *ProviderMemory) ForgetThrottle(providerID, baseURL string) {
	pm.throttle.Del(memKey(providerID, baseURL))
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
