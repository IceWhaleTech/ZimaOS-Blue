package proxy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	ecache2 "github.com/orca-zhang/ecache2"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

// AuthStrategy represents an authentication method to try against an upstream provider.
type AuthStrategy int

const (
	AuthBearer    AuthStrategy = iota // Authorization: Bearer <key>
	AuthXAPIKey                       // x-api-key: <key>
	AuthAnthropic                     // x-api-key + anthropic-version header
	AuthNone                          // No auth header
)

func (s AuthStrategy) String() string {
	switch s {
	case AuthBearer:
		return "bearer"
	case AuthXAPIKey:
		return "x-api-key"
	case AuthAnthropic:
		return "anthropic"
	case AuthNone:
		return "none"
	}
	return "unknown"
}

// AuthProber manages auth strategy probing and remembers what works.
type AuthProber struct {
	cacheOnce sync.Once
	cacheMu   sync.RWMutex
	cache     *ecache2.Cache[uint64] // key: FNV-1a hash of "providerID:baseURL[#format]" → AuthStrategy
}

// NewAuthProber creates a new auth prober with 1-hour TTL memory.
func NewAuthProber() *AuthProber {
	return &AuthProber{}
}

func (ap *AuthProber) cacheOrNil() *ecache2.Cache[uint64] {
	if ap == nil {
		return nil
	}
	ap.cacheMu.RLock()
	defer ap.cacheMu.RUnlock()
	return ap.cache
}

func (ap *AuthProber) ensureCache() *ecache2.Cache[uint64] {
	if ap == nil {
		return nil
	}
	ap.cacheOnce.Do(func() {
		cache := ecache2.NewLRUCache[uint64](4, 64, 1*time.Hour)
		ap.cacheMu.Lock()
		ap.cache = cache
		ap.cacheMu.Unlock()
	})
	return ap.cacheOrNil()
}

func normalizeAuthStrategyMemoryKey(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func authStrategyMemoryKey(baseURL string, effectiveFormat providerpool.APIFormat) string {
	key := normalizeAuthStrategyMemoryKey(baseURL)
	if key == "" {
		return ""
	}
	if format := strings.TrimSpace(string(effectiveFormat)); format != "" {
		return key + "#" + format
	}
	return key
}

// cacheKey builds the memory key as a raw FNV-1a uint64 hash. Unlike provider
// memory, auth probing must keep the full normalized endpoint key so different
// formats on the same host do not poison each other.
func cacheKey(providerID, memoryKey string) uint64 {
	memoryKey = strings.TrimSpace(memoryKey)
	hash := fnvOffset64
	for i := 0; i < len(providerID); i++ {
		hash ^= uint64(providerID[i])
		hash *= fnvPrime64
	}
	if memoryKey != "" {
		hash ^= uint64(':')
		hash *= fnvPrime64
		for i := 0; i < len(memoryKey); i++ {
			hash ^= uint64(memoryKey[i])
			hash *= fnvPrime64
		}
	}
	return hash
}

// Recall returns the cached winning strategy, if any.
func (ap *AuthProber) Recall(providerID, memoryKey string) (AuthStrategy, bool) {
	cache := ap.cacheOrNil()
	if cache == nil {
		return 0, false
	}
	if v, ok := cache.Get(cacheKey(providerID, memoryKey)); ok {
		if s, ok := v.(AuthStrategy); ok {
			return s, true
		}
	}
	return 0, false
}

// Remember caches the winning auth strategy for a provider+endpoint key.
func (ap *AuthProber) Remember(providerID, memoryKey string, strategy AuthStrategy) {
	if cache := ap.ensureCache(); cache != nil {
		cache.Put(cacheKey(providerID, memoryKey), strategy)
	}
}

// Forget evicts the cached strategy (e.g. on provider config change).
func (ap *AuthProber) Forget(providerID, memoryKey string) {
	if cache := ap.cacheOrNil(); cache != nil {
		cache.Del(cacheKey(providerID, memoryKey))
	}
}

// Pre-allocated strategy arrays — avoids slice allocation on every call.
var (
	strategiesNone      = []AuthStrategy{AuthNone}
	strategiesAnthropic = []AuthStrategy{AuthAnthropic, AuthBearer, AuthXAPIKey, AuthNone}
	strategiesOllama    = []AuthStrategy{AuthNone, AuthBearer}
	strategiesDefault   = []AuthStrategy{AuthBearer, AuthXAPIKey, AuthNone}
	// Cached-winner fast paths keep the winning strategy first, but preserve
	// fallback auth modes because /v1/models often accepts looser auth than the
	// actual completion endpoints.
	strategiesOllamaCachedBearer   = []AuthStrategy{AuthBearer, AuthNone}
	strategiesDefaultCachedXAPIKey = []AuthStrategy{AuthXAPIKey, AuthBearer, AuthNone}
	strategiesDefaultCachedNone    = []AuthStrategy{AuthNone, AuthBearer, AuthXAPIKey}
)

// Strategies returns an ordered list of auth strategies to try.
// The cached winner (if any) is placed first for zero-latency happy path.
// effectiveFormat is the API format being used for this request (may differ from provider.APIFormat).
func (ap *AuthProber) Strategies(provider *providerpool.Provider, apiKey *providerpool.APIKey, effectiveFormat providerpool.APIFormat) []AuthStrategy {
	hasKey := apiKey != nil && apiKey.Key != ""

	// No key → only try without auth
	if !hasKey {
		return strategiesNone
	}

	// Build base order by effective API format (not provider.APIFormat)
	// This allows different endpoints to use different auth strategies
	var base []AuthStrategy
	switch effectiveFormat {
	case providerpool.APIFormatAnthropic:
		base = strategiesAnthropic
	case providerpool.APIFormatOllama:
		base = strategiesOllama
	default: // openai, google, custom
		base = strategiesDefault
	}

	// If we have a cached winner, return a pre-built single-element slice
	// with the cached strategy first while preserving fallback auth modes.
	// Cache is scoped by normalized endpoint + effective format so /v1/models
	// probing cannot poison chat-completions or anthropic-native requests.
	memoryKey := authStrategyMemoryKey(provider.EffectiveBaseURL(), effectiveFormat)
	if cached, ok := ap.Recall(provider.ID, memoryKey); ok {
		switch effectiveFormat {
		case providerpool.APIFormatAnthropic:
			// Anthropic-native requests need the anthropic-version header; cached
			// auth from /v1/models must not override the format-native strategy.
			return strategiesAnthropic
		case providerpool.APIFormatOllama:
			switch cached {
			case AuthBearer:
				return strategiesOllamaCachedBearer
			case AuthNone:
				return strategiesOllama
			}
		default:
			switch cached {
			case AuthBearer:
				return strategiesDefault
			case AuthXAPIKey:
				return strategiesDefaultCachedXAPIKey
			case AuthNone:
				return strategiesDefaultCachedNone
			}
		}
	}

	return base
}

// Apply sets the appropriate auth headers on the request for the strategy.
func (ap *AuthProber) Apply(req *http.Request, strategy AuthStrategy, apiKey *providerpool.APIKey, provider *providerpool.Provider) {
	// Clear any existing auth headers first
	req.Header.Del("Authorization")
	req.Header.Del("x-api-key")
	req.Header.Del("anthropic-version")

	key := ""
	if apiKey != nil {
		key = apiKey.Key
	}

	slog.Debug("[proxy] Apply auth", "provider", provider.ID, "strategy", strategy.String(), "hasKey", key != "", "keyLen", len(key))

	switch strategy {
	case AuthBearer:
		req.Header.Set("Authorization", "Bearer "+key)
	case AuthXAPIKey:
		req.Header.Set("x-api-key", key)
	case AuthAnthropic:
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case AuthNone:
		// No auth headers
	}

	// Always apply provider custom headers (may override above)
	for k, v := range provider.Headers {
		req.Header.Set(k, v)
	}
}

var authProbeNonAuth403Markers = [...]string{
	"insufficient_user_quota",
	"insufficient_quota",
	"quota exceeded",
	"quota",
	"billing",
	"credit balance",
	"payment required",
	"rate limit",
	"too many requests",
	"throttled",
	"capacity",
	"overloaded",
	"policy violation",
	"content filter",
	"safety",
	"banned",
}

var authProbeAuthMarkers = [...]string{
	"unauthorized",
	"authentication",
	"invalid api key",
	"invalid key",
	"invalid token",
	"expired token",
	"missing token",
	"missing api key",
	"api key required",
	"credential",
	"forbidden",
	"未提供令牌",
}

func containsMarker(msg string, markers []string) bool {
	for _, marker := range markers {
		if marker != "" && strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}

func readAndRestoreBody(resp *http.Response) string {
	if resp == nil || resp.Body == nil {
		return ""
	}
	body := readErrorBody(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return strings.TrimSpace(string(body))
}

// isAuthError is the coarse status-code gate used by warmup/probing paths that
// do not retain the upstream body. Body-aware auth probing should use
// shouldContinueAuthProbe instead.
func isAuthError(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

// shouldContinueAuthProbe returns true when the response clearly indicates that
// the current auth strategy failed and another strategy is worth trying.
func shouldContinueAuthProbe(resp *http.Response) (bool, string) {
	if resp == nil {
		return false, ""
	}
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return true, readAndRestoreBody(resp)
	case http.StatusForbidden:
		body := readAndRestoreBody(resp)
		bodyLower := strings.ToLower(body)
		if containsMarker(bodyLower, authProbeNonAuth403Markers[:]) {
			return false, body
		}
		if containsMarker(bodyLower, authProbeAuthMarkers[:]) {
			return true, body
		}
		// Preserve existing compatibility for opaque 403 responses where the body
		// does not tell us whether the relay rejected the auth scheme or the key.
		return true, body
	default:
		return false, ""
	}
}

// ProbeAndForward tries auth strategies in order until one succeeds (non-401/403).
// Returns the successful response, or an error if all strategies are exhausted.
// effectiveFormat is the API format being used (openai, anthropic, etc.) to determine auth strategy.
func (ap *AuthProber) ProbeAndForward(
	provider *providerpool.Provider,
	apiKey *providerpool.APIKey,
	effectiveFormat providerpool.APIFormat,
	buildRequest func() (*http.Request, error),
	doRequest func(*http.Request) (*http.Response, error),
) (*http.Response, error) {
	strategies := ap.Strategies(provider, apiKey, effectiveFormat)
	lastStatusCode := 0
	lastBody := ""

	for i, strat := range strategies {
		req, err := buildRequest()
		if err != nil {
			return nil, err
		}
		ap.Apply(req, strat, apiKey, provider)

		// Body is already an io.NopCloser(bytes.NewReader(...)) from buildUpstreamRequestWithFormat,
		// so it's seekable and doesn't need to be re-read. No copy needed.

		resp, err := doRequest(req)
		if err != nil {
			return nil, err
		}

		continueProbe, probeBody := shouldContinueAuthProbe(resp)
		if !continueProbe {
			// Success, or an upstream error unrelated to auth probing — remember the
			// working strategy and let the caller decide how to handle the response.
			// Cache is scoped by endpoint + effective format.
			ap.Remember(provider.ID, authStrategyMemoryKey(provider.EffectiveBaseURL(), effectiveFormat), strat)
			return resp, nil
		}

		// Auth failed — remember the last rejection so callers can surface the
		// real upstream cause instead of falling through to unrelated model errors.
		lastStatusCode = resp.StatusCode
		lastBody = probeBody
		resp.Body.Close()

		slog.Warn("[proxy] auth strategy failed",
			"provider", provider.ID,
			"strategy", strat.String(),
			"status", resp.StatusCode,
			"attempt", i+1,
			"remaining", len(strategies)-i-1,
		)
	}

	// All strategies exhausted — evict stale cache entry for this endpoint/format.
	ap.Forget(provider.ID, authStrategyMemoryKey(provider.EffectiveBaseURL(), effectiveFormat))
	return nil, &AuthExhaustedError{
		ProviderID:     provider.ID,
		LastStatusCode: lastStatusCode,
		LastBody:       lastBody,
	}
}

// AuthExhaustedError indicates all auth strategies failed for a provider.
type AuthExhaustedError struct {
	ProviderID     string
	LastStatusCode int
	LastBody       string
}

func (e *AuthExhaustedError) Error() string {
	if e == nil {
		return "all auth strategies exhausted"
	}
	if e.LastStatusCode >= http.StatusBadRequest {
		if body := strings.TrimSpace(e.LastBody); body != "" {
			return fmt.Sprintf("provider %s auth error (%d): %s", e.ProviderID, e.LastStatusCode, body)
		}
		return fmt.Sprintf("provider %s auth error (%d)", e.ProviderID, e.LastStatusCode)
	}
	return "all auth strategies exhausted for provider " + e.ProviderID
}

func asAuthExhaustedError(err error) (*AuthExhaustedError, bool) {
	var authErr *AuthExhaustedError
	if errors.As(err, &authErr) {
		return authErr, true
	}
	return nil, false
}
