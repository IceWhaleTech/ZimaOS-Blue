// Package oidc provides OpenID Connect authentication support.
package oidc

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrProviderNotFound is returned when a provider is not found.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrInvalidState is returned when the state parameter is invalid.
	ErrInvalidState = errors.New("invalid state parameter")
	// ErrInvalidCode is returned when the authorization code is invalid.
	ErrInvalidCode = errors.New("invalid authorization code")
	// ErrTokenExpired is returned when the token has expired.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidToken is returned when the token is invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidNonce is returned when the nonce is invalid.
	ErrInvalidNonce = errors.New("invalid nonce")
	// ErrProviderDisabled is returned when the provider is disabled.
	ErrProviderDisabled = errors.New("provider disabled")
	// ErrUserNotFound is returned when the user is not found.
	ErrUserNotFound = errors.New("user not found")
	// ErrAccountAlreadyLinked is returned when the account is already linked.
	ErrAccountAlreadyLinked = errors.New("account already linked to another user")
)

// ProviderType represents the type of OIDC provider.
type ProviderType string

const (
	// ProviderGeneric is a generic OIDC provider.
	ProviderGeneric ProviderType = "generic"
	// ProviderGoogle is Google's OIDC provider.
	ProviderGoogle ProviderType = "google"
	// ProviderGitHub is GitHub's OAuth provider.
	ProviderGitHub ProviderType = "github"
	// ProviderMicrosoft is Microsoft/Azure AD's OIDC provider.
	ProviderMicrosoft ProviderType = "microsoft"
	// ProviderKeycloak is Keycloak's OIDC provider.
	ProviderKeycloak ProviderType = "keycloak"
	// ProviderAuthentik is Authentik's OIDC provider.
	ProviderAuthentik ProviderType = "authentik"
	// ProviderAuth0 is Auth0's OIDC provider.
	ProviderAuth0 ProviderType = "auth0"
)

// ProviderConfig holds the configuration for an OIDC provider.
type ProviderConfig struct {
	// ID is the unique identifier for this provider configuration.
	ID string `json:"id" yaml:"id"`
	// Name is the display name for this provider.
	Name string `json:"name" yaml:"name"`
	// Type is the provider type.
	Type ProviderType `json:"type" yaml:"type"`
	// Enabled indicates if this provider is enabled.
	Enabled bool `json:"enabled" yaml:"enabled"`
	// ClientID is the OAuth client ID.
	ClientID string `json:"client_id" yaml:"client_id"`
	// ClientSecret is the OAuth client secret.
	ClientSecret string `json:"client_secret" yaml:"client_secret"`
	// IssuerURL is the OIDC issuer URL.
	IssuerURL string `json:"issuer_url" yaml:"issuer_url"`
	// AuthURL is the authorization endpoint URL (optional, auto-discovered).
	AuthURL string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	// TokenURL is the token endpoint URL (optional, auto-discovered).
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	// UserInfoURL is the userinfo endpoint URL (optional, auto-discovered).
	UserInfoURL string `json:"userinfo_url,omitempty" yaml:"userinfo_url,omitempty"`
	// JWKSURL is the JWKS endpoint URL (optional, auto-discovered).
	JWKSURL string `json:"jwks_url,omitempty" yaml:"jwks_url,omitempty"`
	// Scopes is the list of scopes to request.
	Scopes []string `json:"scopes" yaml:"scopes"`
	// RedirectURL is the callback URL for this provider.
	RedirectURL string `json:"redirect_url" yaml:"redirect_url"`
	// ClaimMapping maps OIDC claims to user attributes.
	ClaimMapping ClaimMapping `json:"claim_mapping,omitempty" yaml:"claim_mapping,omitempty"`
	// RoleMapping maps OIDC claims to user roles.
	RoleMapping []RoleMapping `json:"role_mapping,omitempty" yaml:"role_mapping,omitempty"`
	// AutoCreateUser indicates if users should be auto-created on first login.
	AutoCreateUser bool `json:"auto_create_user" yaml:"auto_create_user"`
	// DefaultRole is the default role for auto-created users.
	DefaultRole string `json:"default_role" yaml:"default_role"`
	// AllowedDomains restricts login to specific email domains.
	AllowedDomains []string `json:"allowed_domains,omitempty" yaml:"allowed_domains,omitempty"`
}

// ClaimMapping maps OIDC claims to user attributes.
type ClaimMapping struct {
	// Subject is the claim for the user's unique identifier.
	Subject string `json:"subject" yaml:"subject"`
	// Email is the claim for the user's email.
	Email string `json:"email" yaml:"email"`
	// Name is the claim for the user's display name.
	Name string `json:"name" yaml:"name"`
	// Picture is the claim for the user's avatar URL.
	Picture string `json:"picture,omitempty" yaml:"picture,omitempty"`
	// Groups is the claim for the user's groups.
	Groups string `json:"groups,omitempty" yaml:"groups,omitempty"`
	// Roles is the claim for the user's roles.
	Roles string `json:"roles,omitempty" yaml:"roles,omitempty"`
}

// RoleMapping maps OIDC claims to user roles.
type RoleMapping struct {
	// Claim is the claim name to check.
	Claim string `json:"claim" yaml:"claim"`
	// Value is the claim value to match.
	Value string `json:"value" yaml:"value"`
	// Role is the role to assign when matched.
	Role string `json:"role" yaml:"role"`
}

// DefaultClaimMapping returns the default claim mapping.
func DefaultClaimMapping() ClaimMapping {
	return ClaimMapping{
		Subject: "sub",
		Email:   "email",
		Name:    "name",
		Picture: "picture",
		Groups:  "groups",
		Roles:   "roles",
	}
}

// DiscoveryDocument represents the OIDC discovery document.
type DiscoveryDocument struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint,omitempty"`
	JWKSURI                          string   `json:"jwks_uri"`
	RegistrationEndpoint             string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                  []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	ResponseModesSupported           []string `json:"response_modes_supported,omitempty"`
	GrantTypesSupported              []string `json:"grant_types_supported,omitempty"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
	ClaimsSupported                  []string `json:"claims_supported,omitempty"`
	CodeChallengeMethodsSupported    []string `json:"code_challenge_methods_supported,omitempty"`
}

// TokenResponse represents the token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// IDTokenClaims represents the claims in an ID token.
type IDTokenClaims struct {
	Issuer          string   `json:"iss"`
	Subject         string   `json:"sub"`
	Audience        []string `json:"aud"`
	ExpiresAt       int64    `json:"exp"`
	IssuedAt        int64    `json:"iat"`
	AuthTime        int64    `json:"auth_time,omitempty"`
	Nonce           string   `json:"nonce,omitempty"`
	ACR             string   `json:"acr,omitempty"`
	AMR             []string `json:"amr,omitempty"`
	AZP             string   `json:"azp,omitempty"`
	Email           string   `json:"email,omitempty"`
	EmailVerified   bool     `json:"email_verified,omitempty"`
	Name            string   `json:"name,omitempty"`
	GivenName       string   `json:"given_name,omitempty"`
	FamilyName      string   `json:"family_name,omitempty"`
	PreferredName   string   `json:"preferred_username,omitempty"`
	Picture         string   `json:"picture,omitempty"`
	Locale          string   `json:"locale,omitempty"`
	Groups          []string `json:"groups,omitempty"`
	Roles           []string `json:"roles,omitempty"`
	AdditionalClaims map[string]interface{} `json:"-"`
}

// UserInfo represents the userinfo endpoint response.
type UserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email,omitempty"`
	EmailVerified bool   `json:"email_verified,omitempty"`
	Name          string `json:"name,omitempty"`
	GivenName     string `json:"given_name,omitempty"`
	FamilyName    string `json:"family_name,omitempty"`
	Picture       string `json:"picture,omitempty"`
	Locale        string `json:"locale,omitempty"`
	Profile       string `json:"profile,omitempty"`
}

// AuthState represents the state stored during the auth flow.
type AuthState struct {
	// State is the random state parameter.
	State string `json:"state"`
	// Nonce is the random nonce for ID token validation.
	Nonce string `json:"nonce"`
	// ProviderID is the provider being used.
	ProviderID string `json:"provider_id"`
	// RedirectURI is where to redirect after auth.
	RedirectURI string `json:"redirect_uri"`
	// CodeVerifier is the PKCE code verifier.
	CodeVerifier string `json:"code_verifier,omitempty"`
	// CreatedAt is when the state was created.
	CreatedAt time.Time `json:"created_at"`
	// ExpiresAt is when the state expires.
	ExpiresAt time.Time `json:"expires_at"`
	// UserID is set if linking to an existing user.
	UserID string `json:"user_id,omitempty"`
}

// LinkedAccount represents an OIDC account linked to a user.
type LinkedAccount struct {
	// ID is the unique identifier for this linked account.
	ID string `json:"id"`
	// UserID is the user this account is linked to.
	UserID string `json:"user_id"`
	// ProviderID is the OIDC provider ID.
	ProviderID string `json:"provider_id"`
	// ProviderUserID is the user's ID at the provider.
	ProviderUserID string `json:"provider_user_id"`
	// Email is the email from the provider.
	Email string `json:"email,omitempty"`
	// Name is the name from the provider.
	Name string `json:"name,omitempty"`
	// Picture is the avatar URL from the provider.
	Picture string `json:"picture,omitempty"`
	// AccessToken is the current access token.
	AccessToken string `json:"-"`
	// RefreshToken is the refresh token.
	RefreshToken string `json:"-"`
	// TokenExpiry is when the access token expires.
	TokenExpiry time.Time `json:"token_expiry,omitempty"`
	// CreatedAt is when the account was linked.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the account was last updated.
	UpdatedAt time.Time `json:"updated_at"`
}

// AuthorizeRequest represents a request to start the auth flow.
type AuthorizeRequest struct {
	// ProviderID is the provider to use.
	ProviderID string `json:"provider_id"`
	// RedirectURI is where to redirect after auth.
	RedirectURI string `json:"redirect_uri,omitempty"`
	// State is an optional state to pass through.
	State string `json:"state,omitempty"`
	// UserID is set if linking to an existing user.
	UserID string `json:"user_id,omitempty"`
}

// AuthorizeResponse represents the response to start the auth flow.
type AuthorizeResponse struct {
	// AuthURL is the URL to redirect the user to.
	AuthURL string `json:"auth_url"`
	// State is the state parameter to verify on callback.
	State string `json:"state"`
}

// CallbackRequest represents the callback from the provider.
type CallbackRequest struct {
	// ProviderID is the provider that called back.
	ProviderID string `json:"provider_id"`
	// Code is the authorization code.
	Code string `json:"code"`
	// State is the state parameter.
	State string `json:"state"`
	// Error is the error code if auth failed.
	Error string `json:"error,omitempty"`
	// ErrorDescription is the error description.
	ErrorDescription string `json:"error_description,omitempty"`
}

// CallbackResponse represents the response after processing the callback.
type CallbackResponse struct {
	// User is the authenticated user.
	User *AuthenticatedUser `json:"user"`
	// AccessToken is the JWT access token for the user.
	AccessToken string `json:"access_token"`
	// RefreshToken is the refresh token.
	RefreshToken string `json:"refresh_token,omitempty"`
	// ExpiresIn is the token expiry in seconds.
	ExpiresIn int `json:"expires_in"`
	// IsNewUser indicates if this is a newly created user.
	IsNewUser bool `json:"is_new_user"`
}

// AuthenticatedUser represents an authenticated user.
type AuthenticatedUser struct {
	// ID is the user's ID.
	ID string `json:"id"`
	// Email is the user's email.
	Email string `json:"email"`
	// Name is the user's display name.
	Name string `json:"name"`
	// Picture is the user's avatar URL.
	Picture string `json:"picture,omitempty"`
	// Roles is the user's roles.
	Roles []string `json:"roles"`
	// ProviderID is the provider used for authentication.
	ProviderID string `json:"provider_id"`
}

// Service defines the OIDC service interface.
type Service interface {
	// ListProviders returns all configured providers.
	ListProviders(ctx context.Context) ([]*ProviderConfig, error)

	// GetProvider returns a provider by ID.
	GetProvider(ctx context.Context, id string) (*ProviderConfig, error)

	// Authorize starts the authorization flow.
	Authorize(ctx context.Context, req *AuthorizeRequest) (*AuthorizeResponse, error)

	// Callback handles the authorization callback.
	Callback(ctx context.Context, req *CallbackRequest) (*CallbackResponse, error)

	// RefreshToken refreshes an access token.
	RefreshToken(ctx context.Context, providerID, refreshToken string) (*TokenResponse, error)

	// GetUserInfo gets user info from the provider.
	GetUserInfo(ctx context.Context, providerID, accessToken string) (*UserInfo, error)

	// LinkAccount links an OIDC account to an existing user.
	LinkAccount(ctx context.Context, userID string, req *AuthorizeRequest) (*AuthorizeResponse, error)

	// UnlinkAccount unlinks an OIDC account from a user.
	UnlinkAccount(ctx context.Context, userID, providerID string) error

	// GetLinkedAccounts returns all linked accounts for a user.
	GetLinkedAccounts(ctx context.Context, userID string) ([]*LinkedAccount, error)
}

// StateStore defines the interface for storing auth state.
type StateStore interface {
	// Save saves an auth state.
	Save(ctx context.Context, state *AuthState) error

	// Get retrieves an auth state by state parameter.
	Get(ctx context.Context, state string) (*AuthState, error)

	// Delete deletes an auth state.
	Delete(ctx context.Context, state string) error

	// Cleanup removes expired states.
	Cleanup(ctx context.Context) error
}

// AccountStore defines the interface for storing linked accounts.
type AccountStore interface {
	// Save saves a linked account.
	Save(ctx context.Context, account *LinkedAccount) error

	// GetByProviderUser gets a linked account by provider and provider user ID.
	GetByProviderUser(ctx context.Context, providerID, providerUserID string) (*LinkedAccount, error)

	// GetByUser gets all linked accounts for a user.
	GetByUser(ctx context.Context, userID string) ([]*LinkedAccount, error)

	// Delete deletes a linked account.
	Delete(ctx context.Context, userID, providerID string) error
}
