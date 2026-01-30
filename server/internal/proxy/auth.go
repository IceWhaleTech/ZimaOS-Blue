package proxy

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	Enabled      bool     `json:"enabled"`
	Type         string   `json:"type"` // "none", "api_key", "bearer"
	APIKeys      []string `json:"api_keys"`
	AllowedIPs   []string `json:"allowed_ips"`
	SkipPaths    []string `json:"skip_paths"`
	HeaderName   string   `json:"header_name"`
}

// DefaultAuthConfig returns default auth configuration
func DefaultAuthConfig() *AuthConfig {
	return &AuthConfig{
		Enabled:    false,
		Type:       "api_key",
		HeaderName: "X-API-Key",
		SkipPaths:  []string{"/health", "/api/v1/proxy/health"},
	}
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Enabled         bool          `json:"enabled"`
	RequestsPerMin  int           `json:"requests_per_min"`
	BurstSize       int           `json:"burst_size"`
	PerIP           bool          `json:"per_ip"`
	PerAPIKey       bool          `json:"per_api_key"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
}

// DefaultRateLimitConfig returns default rate limit configuration
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		Enabled:         false,
		RequestsPerMin:  60,
		BurstSize:       10,
		PerIP:           true,
		PerAPIKey:       false,
		CleanupInterval: 5 * time.Minute,
	}
}

// rateLimitEntry tracks rate limit state for a client
type rateLimitEntry struct {
	tokens     float64
	lastUpdate time.Time
}

// Authenticator handles authentication and rate limiting
type Authenticator struct {
	authConfig      *AuthConfig
	rateLimitConfig *RateLimitConfig
	apiKeySet       map[string]bool
	allowedIPSet    map[string]bool
	rateLimits      map[string]*rateLimitEntry
	mu              sync.RWMutex

	// Stats
	authFailures   int64
	rateLimitHits  int64
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(authConfig *AuthConfig, rateLimitConfig *RateLimitConfig) *Authenticator {
	if authConfig == nil {
		authConfig = DefaultAuthConfig()
	}
	if rateLimitConfig == nil {
		rateLimitConfig = DefaultRateLimitConfig()
	}

	a := &Authenticator{
		authConfig:      authConfig,
		rateLimitConfig: rateLimitConfig,
		apiKeySet:       make(map[string]bool),
		allowedIPSet:    make(map[string]bool),
		rateLimits:      make(map[string]*rateLimitEntry),
	}

	// Build API key set
	for _, key := range authConfig.APIKeys {
		a.apiKeySet[key] = true
	}

	// Build allowed IP set
	for _, ip := range authConfig.AllowedIPs {
		a.allowedIPSet[ip] = true
	}

	return a
}

// Authenticate checks if a request is authenticated
func (a *Authenticator) Authenticate(r *http.Request) (bool, string) {
	if !a.authConfig.Enabled {
		return true, ""
	}

	// Check skip paths
	for _, path := range a.authConfig.SkipPaths {
		if strings.HasPrefix(r.URL.Path, path) {
			return true, ""
		}
	}

	// Check allowed IPs
	clientIP := a.getClientIP(r)
	if len(a.allowedIPSet) > 0 {
		if a.allowedIPSet[clientIP] {
			return true, ""
		}
	}

	// Check authentication based on type
	switch a.authConfig.Type {
	case "none":
		return true, ""

	case "api_key":
		return a.checkAPIKey(r)

	case "bearer":
		return a.checkBearerToken(r)

	default:
		return a.checkAPIKey(r)
	}
}

// checkAPIKey validates API key authentication
func (a *Authenticator) checkAPIKey(r *http.Request) (bool, string) {
	headerName := a.authConfig.HeaderName
	if headerName == "" {
		headerName = "X-API-Key"
	}

	apiKey := r.Header.Get(headerName)
	if apiKey == "" {
		a.incrementAuthFailure()
		return false, "missing API key"
	}

	// Constant-time comparison
	for key := range a.apiKeySet {
		if subtle.ConstantTimeCompare([]byte(apiKey), []byte(key)) == 1 {
			return true, ""
		}
	}

	a.incrementAuthFailure()
	return false, "invalid API key"
}

// checkBearerToken validates Bearer token authentication
func (a *Authenticator) checkBearerToken(r *http.Request) (bool, string) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		a.incrementAuthFailure()
		return false, "missing Authorization header"
	}

	if !strings.HasPrefix(auth, "Bearer ") {
		a.incrementAuthFailure()
		return false, "invalid Authorization format"
	}

	token := strings.TrimPrefix(auth, "Bearer ")

	// Check against API keys
	for key := range a.apiKeySet {
		if subtle.ConstantTimeCompare([]byte(token), []byte(key)) == 1 {
			return true, ""
		}
	}

	a.incrementAuthFailure()
	return false, "invalid token"
}

// CheckRateLimit checks if a request is within rate limits
func (a *Authenticator) CheckRateLimit(r *http.Request) (bool, string) {
	if !a.rateLimitConfig.Enabled {
		return true, ""
	}

	// Determine rate limit key
	key := a.getRateLimitKey(r)

	a.mu.Lock()
	defer a.mu.Unlock()

	entry, ok := a.rateLimits[key]
	now := time.Now()

	if !ok {
		// New client
		entry = &rateLimitEntry{
			tokens:     float64(a.rateLimitConfig.BurstSize),
			lastUpdate: now,
		}
		a.rateLimits[key] = entry
	}

	// Token bucket algorithm
	elapsed := now.Sub(entry.lastUpdate).Seconds()
	refillRate := float64(a.rateLimitConfig.RequestsPerMin) / 60.0
	entry.tokens += elapsed * refillRate

	// Cap at burst size
	if entry.tokens > float64(a.rateLimitConfig.BurstSize) {
		entry.tokens = float64(a.rateLimitConfig.BurstSize)
	}

	entry.lastUpdate = now

	// Check if we have tokens
	if entry.tokens < 1 {
		a.rateLimitHits++
		return false, "rate limit exceeded"
	}

	// Consume a token
	entry.tokens--
	return true, ""
}

// getRateLimitKey determines the rate limit key for a request
func (a *Authenticator) getRateLimitKey(r *http.Request) string {
	if a.rateLimitConfig.PerAPIKey {
		apiKey := r.Header.Get(a.authConfig.HeaderName)
		if apiKey != "" {
			return "key:" + apiKey
		}
	}

	if a.rateLimitConfig.PerIP {
		return "ip:" + a.getClientIP(r)
	}

	return "global"
}

// getClientIP extracts client IP from request
func (a *Authenticator) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Use RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// incrementAuthFailure increments auth failure counter
func (a *Authenticator) incrementAuthFailure() {
	a.mu.Lock()
	a.authFailures++
	a.mu.Unlock()
}

// CleanupRateLimits removes stale rate limit entries
func (a *Authenticator) CleanupRateLimits() {
	a.mu.Lock()
	defer a.mu.Unlock()

	cutoff := time.Now().Add(-a.rateLimitConfig.CleanupInterval)
	for key, entry := range a.rateLimits {
		if entry.lastUpdate.Before(cutoff) {
			delete(a.rateLimits, key)
		}
	}
}

// AddAPIKey adds an API key
func (a *Authenticator) AddAPIKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.apiKeySet[key] = true
	a.authConfig.APIKeys = append(a.authConfig.APIKeys, key)
}

// RemoveAPIKey removes an API key
func (a *Authenticator) RemoveAPIKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.apiKeySet, key)

	// Remove from config
	newKeys := make([]string, 0)
	for _, k := range a.authConfig.APIKeys {
		if k != key {
			newKeys = append(newKeys, k)
		}
	}
	a.authConfig.APIKeys = newKeys
}

// AddAllowedIP adds an allowed IP
func (a *Authenticator) AddAllowedIP(ip string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.allowedIPSet[ip] = true
	a.authConfig.AllowedIPs = append(a.authConfig.AllowedIPs, ip)
}

// Stats returns authenticator statistics
func (a *Authenticator) Stats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return map[string]interface{}{
		"auth_enabled":       a.authConfig.Enabled,
		"auth_type":          a.authConfig.Type,
		"api_key_count":      len(a.apiKeySet),
		"allowed_ip_count":   len(a.allowedIPSet),
		"auth_failures":      a.authFailures,
		"rate_limit_enabled": a.rateLimitConfig.Enabled,
		"requests_per_min":   a.rateLimitConfig.RequestsPerMin,
		"burst_size":         a.rateLimitConfig.BurstSize,
		"rate_limit_hits":    a.rateLimitHits,
		"active_limiters":    len(a.rateLimits),
	}
}
