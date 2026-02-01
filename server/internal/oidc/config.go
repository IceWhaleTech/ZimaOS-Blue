// Package oidc provides an OpenID Connect provider implementation.
package oidc

import (
	"time"
)

// Config holds the OIDC provider configuration.
type Config struct {
	// Issuer is the OIDC issuer URL (e.g., "https://your-domain.com").
	Issuer string
	// SigningKeyPath is the path to the RSA private key for signing tokens.
	SigningKeyPath string
	// SigningKeyRotationDays is how often to rotate signing keys.
	SigningKeyRotationDays int
	// AccessTokenTTL is the access token lifetime.
	AccessTokenTTL time.Duration
	// RefreshTokenTTL is the refresh token lifetime.
	RefreshTokenTTL time.Duration
	// AuthorizationCodeTTL is the authorization code lifetime.
	AuthorizationCodeTTL time.Duration
	// IDTokenTTL is the ID token lifetime.
	IDTokenTTL time.Duration
	// Clients is the list of registered OIDC clients.
	Clients []ClientConfig
}

// ClientConfig holds configuration for an OIDC client.
type ClientConfig struct {
	// ClientID is the unique client identifier.
	ClientID string
	// ClientSecret is the client secret (empty for public clients).
	ClientSecret string
	// RedirectURIs are the allowed redirect URIs.
	RedirectURIs []string
	// AllowedScopes are the scopes this client can request.
	AllowedScopes []string
	// AllowedGrantTypes are the grant types this client can use.
	AllowedGrantTypes []string
	// Public indicates if this is a public client (no secret).
	Public bool
}

// DefaultConfig returns the default OIDC configuration.
func DefaultConfig() *Config {
	return &Config{
		Issuer:                 "http://localhost:23456",
		SigningKeyPath:         "./keys/oidc.key",
		SigningKeyRotationDays: 90,
		AccessTokenTTL:         time.Hour,
		RefreshTokenTTL:        30 * 24 * time.Hour, // 30 days
		AuthorizationCodeTTL:   10 * time.Minute,
		IDTokenTTL:             time.Hour,
		Clients:                []ClientConfig{},
	}
}

// Scope constants for OIDC.
const (
	ScopeOpenID  = "openid"
	ScopeProfile = "profile"
	ScopeEmail   = "email"
	ScopeOffline = "offline_access"
)

// GrantType constants for OIDC.
const (
	GrantTypeAuthorizationCode = "authorization_code"
	GrantTypeRefreshToken      = "refresh_token"
	GrantTypeClientCredentials = "client_credentials"
)

// ResponseType constants for OIDC.
const (
	ResponseTypeCode    = "code"
	ResponseTypeToken   = "token"
	ResponseTypeIDToken = "id_token"
)

// SupportedScopes returns the list of supported scopes.
func SupportedScopes() []string {
	return []string{ScopeOpenID, ScopeProfile, ScopeEmail, ScopeOffline}
}

// SupportedGrantTypes returns the list of supported grant types.
func SupportedGrantTypes() []string {
	return []string{GrantTypeAuthorizationCode, GrantTypeRefreshToken}
}

// SupportedResponseTypes returns the list of supported response types.
func SupportedResponseTypes() []string {
	return []string{ResponseTypeCode}
}
