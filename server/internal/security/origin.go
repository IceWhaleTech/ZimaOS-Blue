// Package security provides security utilities for the application.
package security

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

// OriginConfig contains configuration for origin validation.
type OriginConfig struct {
	// AllowedOrigins is a list of allowed origins.
	// Use "*" to allow all origins (not recommended for production).
	AllowedOrigins []string

	// AllowLocalhost allows localhost origins in development.
	AllowLocalhost bool
}

// DefaultOriginConfig returns the default origin configuration.
// In production, you should configure specific allowed origins.
func DefaultOriginConfig() OriginConfig {
	return OriginConfig{
		AllowedOrigins: []string{
			// Dynamic origins will be added at runtime based on actual server port
		},
		AllowLocalhost: true,
	}
}

// ProductionOriginConfig returns a production-ready origin configuration.
// You should customize this with your actual production domains.
func ProductionOriginConfig(allowedDomains []string) OriginConfig {
	return OriginConfig{
		AllowedOrigins: allowedDomains,
		AllowLocalhost: false,
	}
}

// OriginChecker validates request origins.
type OriginChecker struct {
	config         OriginConfig
	mu             sync.RWMutex
	dynamicOrigins []string // Dynamically added origins (tunnel URLs, TLS domains, etc.)
}

// NewOriginChecker creates a new origin checker.
func NewOriginChecker(config OriginConfig) *OriginChecker {
	return &OriginChecker{
		config:         config,
		dynamicOrigins: make([]string, 0),
	}
}

// AddDynamicOrigin adds a dynamic origin (e.g., tunnel URL, TLS domain).
// This is thread-safe and can be called at runtime.
func (c *OriginChecker) AddDynamicOrigin(origin string) {
	if origin == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	for _, o := range c.dynamicOrigins {
		if o == origin {
			return
		}
	}
	c.dynamicOrigins = append(c.dynamicOrigins, origin)
}

// RemoveDynamicOrigin removes a dynamic origin.
func (c *OriginChecker) RemoveDynamicOrigin(origin string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, o := range c.dynamicOrigins {
		if o == origin {
			c.dynamicOrigins = append(c.dynamicOrigins[:i], c.dynamicOrigins[i+1:]...)
			return
		}
	}
}

// GetDynamicOrigins returns a copy of the dynamic origins list.
func (c *OriginChecker) GetDynamicOrigins() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.dynamicOrigins))
	copy(result, c.dynamicOrigins)
	return result
}

// CheckOrigin validates the origin of an HTTP request.
// Returns true if the origin is allowed, false otherwise.
func (c *OriginChecker) CheckOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// If no origin header, allow the request (same-origin request)
	if origin == "" {
		return true
	}

	return c.IsAllowedOrigin(origin)
}

// IsAllowedOrigin checks if the given origin is allowed.
func (c *OriginChecker) IsAllowedOrigin(origin string) bool {
	// Check for wildcard (not recommended for production)
	for _, allowed := range c.config.AllowedOrigins {
		if allowed == "*" {
			return true
		}
	}

	// Parse the origin URL
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Check localhost if allowed - must also match the server port
	if c.config.AllowLocalhost {
		host := parsedOrigin.Hostname()
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			// Verify port matches the server port
			port := parsedOrigin.Port()
			serverPortStr := fmt.Sprintf("%d", GetServerPort())
			if port == serverPortStr || port == "" {
				return true
			}
		}
	}

	// Check against allowed origins
	for _, allowed := range c.config.AllowedOrigins {
		if matchOrigin(origin, allowed) {
			return true
		}
	}

	// Check against dynamic origins (tunnel URLs, TLS domains)
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, allowed := range c.dynamicOrigins {
		if matchOrigin(origin, allowed) {
			return true
		}
	}

	return false
}

// matchOrigin checks if the origin matches the allowed pattern.
func matchOrigin(origin, allowed string) bool {
	// Exact match
	if origin == allowed {
		return true
	}

	// Parse both URLs for comparison
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}

	parsedAllowed, err := url.Parse(allowed)
	if err != nil {
		return false
	}

	// Compare scheme and host
	if parsedOrigin.Scheme != parsedAllowed.Scheme {
		return false
	}

	// Check for wildcard subdomain matching (e.g., *.example.com)
	allowedHost := parsedAllowed.Host
	originHost := parsedOrigin.Host

	if strings.HasPrefix(allowedHost, "*.") {
		// Wildcard subdomain matching
		baseDomain := allowedHost[2:] // Remove "*."
		return strings.HasSuffix(originHost, baseDomain) ||
			originHost == baseDomain[1:] // Also match the base domain without leading dot
	}

	return originHost == allowedHost
}

// GetAllowedOrigins returns the list of allowed origins for CORS configuration.
func (c *OriginChecker) GetAllowedOrigins() []string {
	origins := make([]string, 0, len(c.config.AllowedOrigins))

	for _, origin := range c.config.AllowedOrigins {
		if origin != "*" {
			origins = append(origins, origin)
		}
	}

	// Add localhost origins with dynamic port if allowed
	if c.config.AllowLocalhost {
		port := GetServerPort()
		localhostOrigins := []string{
			fmt.Sprintf("http://localhost:%d", port),
			fmt.Sprintf("http://127.0.0.1:%d", port),
		}
		for _, lo := range localhostOrigins {
			found := false
			for _, o := range origins {
				if o == lo {
					found = true
					break
				}
			}
			if !found {
				origins = append(origins, lo)
			}
		}
	}

	// Add dynamic origins (tunnel URLs, TLS domains)
	c.mu.RLock()
	for _, do := range c.dynamicOrigins {
		found := false
		for _, o := range origins {
			if o == do {
				found = true
				break
			}
		}
		if !found {
			origins = append(origins, do)
		}
	}
	c.mu.RUnlock()

	return origins
}

// CreateWebSocketCheckOrigin creates a CheckOrigin function for WebSocket upgraders.
func (c *OriginChecker) CreateWebSocketCheckOrigin() func(r *http.Request) bool {
	return c.CheckOrigin
}

// Global default origin checker for convenience
var defaultOriginChecker = NewOriginChecker(DefaultOriginConfig())

// CheckOriginDefault checks the origin using the default configuration.
func CheckOriginDefault(r *http.Request) bool {
	return defaultOriginChecker.CheckOrigin(r)
}

// GetDefaultAllowedOrigins returns the default allowed origins.
func GetDefaultAllowedOrigins() []string {
	return defaultOriginChecker.GetAllowedOrigins()
}

// AddDynamicOriginDefault adds a dynamic origin to the default checker.
// Use this to add tunnel URLs or TLS domain origins at runtime.
func AddDynamicOriginDefault(origin string) {
	defaultOriginChecker.AddDynamicOrigin(origin)
}

// RemoveDynamicOriginDefault removes a dynamic origin from the default checker.
func RemoveDynamicOriginDefault(origin string) {
	defaultOriginChecker.RemoveDynamicOrigin(origin)
}

// GetDynamicOriginsDefault returns the dynamic origins from the default checker.
func GetDynamicOriginsDefault() []string {
	return defaultOriginChecker.GetDynamicOrigins()
}

// serverPort stores the actual server port for dynamic CORS origins.
// Default to 23456, but should be updated via SetServerPort when server starts.
var serverPort int = 23456
var serverPortMu sync.RWMutex

// SetServerPort sets the server port for dynamic CORS origin generation.
// This should be called after the server starts and the actual port is known.
func SetServerPort(port int) {
	serverPortMu.Lock()
	defer serverPortMu.Unlock()
	serverPort = port
}

// GetServerPort returns the current server port for CORS origins.
func GetServerPort() int {
	serverPortMu.RLock()
	defer serverPortMu.RUnlock()
	return serverPort
}
