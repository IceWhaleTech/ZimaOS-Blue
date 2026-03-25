package proxy

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
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
	cache *ecache2.Cache[uint64] // key: FNV-1a hash of "providerID:host" → AuthStrategy
}

// NewAuthProber creates a new auth prober with 1-hour TTL memory.
func NewAuthProber() *AuthProber {
	return &AuthProber{
		cache: ecache2.NewLRUCache[uint64](4, 64, 1*time.Hour),
	}
}

// cacheKey builds the memory key as a raw FNV-1a uint64 hash. Zero allocation.
func cacheKey(providerID, baseURL string) uint64 {
	host := extractHost(baseURL)
	hash := fnvOffset64
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

// Recall returns the cached winning strategy, if any.
func (ap *AuthProber) Recall(providerID, baseURL string) (AuthStrategy, bool) {
	if v, ok := ap.cache.Get(cacheKey(providerID, baseURL)); ok {
		if s, ok := v.(AuthStrategy); ok {
			return s, true
		}
	}
	return 0, false
}

// Remember caches the winning auth strategy for a provider+host.
func (ap *AuthProber) Remember(providerID, baseURL string, strategy AuthStrategy) {
	ap.cache.Put(cacheKey(providerID, baseURL), strategy)
}

// Forget evicts the cached strategy (e.g. on provider config change).
func (ap *AuthProber) Forget(providerID, baseURL string) {
	ap.cache.Del(cacheKey(providerID, baseURL))
}

// Pre-allocated strategy arrays — avoids slice allocation on every call.
var (
	strategiesNone      = []AuthStrategy{AuthNone}
	strategiesAnthropic = []AuthStrategy{AuthAnthropic, AuthBearer, AuthXAPIKey, AuthNone}
	strategiesOllama    = []AuthStrategy{AuthNone, AuthBearer}
	strategiesDefault   = []AuthStrategy{AuthBearer, AuthXAPIKey, AuthNone}
	// Cached-winner fast paths — single strategy, zero alloc
	strategiesCachedBearer    = []AuthStrategy{AuthBearer}
	strategiesCachedXAPIKey   = []AuthStrategy{AuthXAPIKey}
	strategiesCachedAnthropic = []AuthStrategy{AuthAnthropic}
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
	// Use effective base URL + format so different endpoints/formats get separate caches
	cacheKey := provider.EffectiveBaseURL() + "#" + string(effectiveFormat)
	if cached, ok := ap.Recall(provider.ID, cacheKey); ok {
		switch cached {
		case AuthBearer:
			return strategiesCachedBearer
		case AuthXAPIKey:
			return strategiesCachedXAPIKey
		case AuthAnthropic:
			return strategiesCachedAnthropic
		case AuthNone:
			return strategiesNone
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

// isAuthError returns true if the HTTP status indicates an authentication failure.
func isAuthError(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
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

		if !isAuthError(resp.StatusCode) {
			// Success (or non-auth error like 400/500) — remember and return
			// Use effective base URL so the auth strategy is cached per endpoint
			ap.Remember(provider.ID, provider.EffectiveBaseURL(), strat)
			return resp, nil
		}

		// Auth failed — remember the last rejection so callers can surface the
		// real upstream cause instead of falling through to unrelated model errors.
		lastStatusCode = resp.StatusCode
		lastBody = strings.TrimSpace(string(readErrorBody(resp.Body)))
		resp.Body.Close()

		slog.Warn("[proxy] auth strategy failed",
			"provider", provider.ID,
			"strategy", strat.String(),
			"status", resp.StatusCode,
			"attempt", i+1,
			"remaining", len(strategies)-i-1,
		)
	}

	// All strategies exhausted — evict stale cache entry
	// Use effective base URL to evict the correct endpoint's cache
	ap.Forget(provider.ID, provider.EffectiveBaseURL())
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
