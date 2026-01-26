// Package extauth provides external authentication provider integration.
// This package implements OIDC/OAuth2 client functionality to authenticate
// users via external identity providers like Google, GitHub, Microsoft, etc.
package extauth

import (
	"context"
	"errors"
	"time"
)

var (
	// ErrProviderNotFound is returned when a provider is not found.
	ErrProviderNotFound = errors.New("provider not found")
	// ErrProviderDisabled is returned when a provider is disabled.
	ErrProviderDisabled = errors.New("provider disabled")
	// ErrInvalidState is returned when the state parameter is invalid.
	ErrInvalidState = errors.New("invalid state parameter")
	// ErrStateExpired is returned when the state has expired.
	ErrStateExpired = errors.New("state expired")
	// ErrInvalidCode is returned when the authorization code is invalid.
	ErrInvalidCode = errors.New("invalid authorization code")
	// ErrTokenExpired is returned when the token has expired.
	ErrTokenExpired = errors.New("token expired")
	// ErrInvalidToken is returned when the token is invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrInvalidNonce is returned when the nonce is invalid.
	ErrInvalidNonce = errors.New("invalid nonce")
	// ErrUserNotFound is returned when the user is not found.
	ErrUserNotFound = errors.New("user not found")
	// ErrAccountAlreadyLinked is returned when the account is already linked.
	ErrAccountAlreadyLinked = errors.New("account already linked to another user")
	// ErrEmailNotAllowed is returned when the email domain is not allowed.
	ErrEmailNotAllowed = errors.New("email domain not allowed")
	// ErrDiscoveryFailed is returned when OIDC discovery fails.
	ErrDiscoveryFailed = errors.New("OIDC discovery failed")
)

// ProviderType represents the type of external auth provider.
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

// ProviderConfig holds the configuration for an external auth provider.
type ProviderConfig struct {
	// ID is the unique identifier for this provider.
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
	// IssuerURL is the OIDC issuer URL (for discovery).
	IssuerURL string `json:"issuer_url,omitempty" yaml:"issuer_url,omitempty"`
	// AuthURL is the authorization endpoint URL.
	AuthURL string `json:"auth_url,omitempty" yaml:"auth_url,omitempty"`
	// TokenURL is the token endpoint URL.
	TokenURL string `json:"token_url,omitempty" yaml:"token_url,omitempty"`
	// UserInfoURL is the userinfo endpoint URL.
	UserInfoURL string `json:"userinfo_url,omitempty" yaml:"userinfo_url,omitempty"`
	// JWKSURL is the JWKS endpoint URL.
	JWKSURL string `json:"jwks_url,omitempty" yaml:"jwks_url,omitempty"`
	// Scopes is the list of scopes to request.
	Scopes []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
	// RedirectURL is the callback URL.
	RedirectURL string `json:"redirect_url" yaml:"redirect_url"`
	// ClaimMapping maps provider claims to user attributes.
	ClaimMapping *ClaimMapping `json:"claim_mapping,omitempty" yaml:"claim_mapping,omitempty"`
	// RoleMappings maps provider claims to user roles.
	RoleMappings []RoleMapping `json:"role_mappings,omitempty" yaml:"role_mappings,omitempty"`
	// AutoCreateUser indicates if users should be auto-created.
	AutoCreateUser bool `json:"auto_create_user" yaml:"auto_create_user"`
	// DefaultRole is the default role for auto-created users.
	DefaultRole string `json:"default_role" yaml:"default_role"`
	// AllowedDomains restricts login to specific email domains.
	AllowedDomains []string `json:"allowed_domains,omitempty" yaml:"allowed_domains,omitempty"`
	// IconURL is the URL for the provider's icon.
	IconURL string `json:"icon_url,omitempty" yaml:"icon_url,omitempty"`
	// Order is the display order for this provider.
	Order int `json:"order" yaml:"order"`
}

// ClaimMapping maps OIDC claims to user attributes.
type ClaimMapping struct {
	Subject  string `json:"subject" yaml:"subject"`
	Email    string `json:"email" yaml:"email"`
	Name     string `json:"name" yaml:"name"`
	Picture  string `json:"picture,omitempty" yaml:"picture,omitempty"`
	Groups   string `json:"groups,omitempty" yaml:"groups,omitempty"`
	Roles    string `json:"roles,omitempty" yaml:"roles,omitempty"`
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
}

// RoleMapping maps a claim value to a role.
type RoleMapping struct {
	Claim string `json:"claim" yaml:"claim"`
	Value string `json:"value" yaml:"value"`
	Role  string `json:"role" yaml:"role"`
}

// DefaultClaimMapping returns the default claim mapping.
func DefaultClaimMapping() *ClaimMapping {
	return &ClaimMapping{
		Subject:  "sub",
		Email:    "email",
		Name:     "name",
		Picture:  "picture",
		Groups:   "groups",
		Roles:    "roles",
		Username: "preferred_username",
	}
}

// DiscoveryDocument represents the OIDC discovery document.
type DiscoveryDocument struct {
	Issuer                           string   `json:"issuer"`
	AuthorizationEndpoint            string   `json:"authorization_endpoint"`
	TokenEndpoint                    string   `json:"token_endpoint"`
	UserInfoEndpoint                 string   `json:"userinfo_endpoint,omitempty"`
	JWKSURI                          string   `json:"jwks_uri,omitempty"`
	ScopesSupported                  []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported           []string `json:"response_types_supported,omitempty"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported,omitempty"`
	ClaimsSupported                  []string `json:"claims_supported,omitempty"`
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

// UserInfo represents user information from the provider.
type UserInfo struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email,omitempty"`
	EmailVerified bool     `json:"email_verified,omitempty"`
	Name          string   `json:"name,omitempty"`
	GivenName     string   `json:"given_name,omitempty"`
	FamilyName    string   `json:"family_name,omitempty"`
	Picture       string   `json:"picture,omitempty"`
	Locale        string   `json:"locale,omitempty"`
	Username      string   `json:"preferred_username,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Roles         []string `json:"roles,omitempty"`
}

// AuthState represents the state stored during the auth flow.
type AuthState struct {
	State        string    `json:"state"`
	Nonce        string    `json:"nonce"`
	ProviderID   string    `json:"provider_id"`
	RedirectURI  string    `json:"redirect_uri"`
	CodeVerifier string    `json:"code_verifier,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	UserID       string    `json:"user_id,omitempty"` // For account linking
}

// LinkedAccount represents an external account linked to a user.
type LinkedAccount struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	ProviderID     string    `json:"provider_id"`
	ProviderUserID string    `json:"provider_user_id"`
	Email          string    `json:"email,omitempty"`
	Name           string    `json:"name,omitempty"`
	Picture        string    `json:"picture,omitempty"`
	AccessToken    string    `json:"-"`
	RefreshToken   string    `json:"-"`
	TokenExpiry    time.Time `json:"token_expiry,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AuthorizeRequest represents a request to start the auth flow.
type AuthorizeRequest struct {
	ProviderID  string `json:"provider_id"`
	RedirectURI string `json:"redirect_uri,omitempty"`
	State       string `json:"state,omitempty"`
	UserID      string `json:"user_id,omitempty"` // For account linking
}

// AuthorizeResponse represents the response to start the auth flow.
type AuthorizeResponse struct {
	AuthURL string `json:"auth_url"`
	State   string `json:"state"`
}

// CallbackRequest represents the callback from the provider.
type CallbackRequest struct {
	ProviderID       string `json:"provider_id"`
	Code             string `json:"code"`
	State            string `json:"state"`
	Error            string `json:"error,omitempty"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// CallbackResponse represents the response after processing the callback.
type CallbackResponse struct {
	User         *AuthenticatedUser `json:"user"`
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token,omitempty"`
	ExpiresIn    int                `json:"expires_in"`
	IsNewUser    bool               `json:"is_new_user"`
}

// AuthenticatedUser represents an authenticated user.
type AuthenticatedUser struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Picture    string   `json:"picture,omitempty"`
	Roles      []string `json:"roles"`
	ProviderID string   `json:"provider_id"`
}

// ProviderInfo represents public information about a provider.
type ProviderInfo struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Type    ProviderType `json:"type"`
	IconURL string       `json:"icon_url,omitempty"`
	Order   int          `json:"order"`
}

// Service defines the external auth service interface.
type Service interface {
	// ListProviders returns all enabled providers.
	ListProviders(ctx context.Context) ([]*ProviderInfo, error)

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

	// LinkAccount links an external account to an existing user.
	LinkAccount(ctx context.Context, userID string, req *AuthorizeRequest) (*AuthorizeResponse, error)

	// UnlinkAccount unlinks an external account from a user.
	UnlinkAccount(ctx context.Context, userID, providerID string) error

	// GetLinkedAccounts returns all linked accounts for a user.
	GetLinkedAccounts(ctx context.Context, userID string) ([]*LinkedAccount, error)
}

// StateStore defines the interface for storing auth state.
type StateStore interface {
	Save(ctx context.Context, state *AuthState) error
	Get(ctx context.Context, state string) (*AuthState, error)
	Delete(ctx context.Context, state string) error
	Cleanup(ctx context.Context) error
}

// AccountStore defines the interface for storing linked accounts.
type AccountStore interface {
	Save(ctx context.Context, account *LinkedAccount) error
	GetByProviderUser(ctx context.Context, providerID, providerUserID string) (*LinkedAccount, error)
	GetByUser(ctx context.Context, userID string) ([]*LinkedAccount, error)
	Delete(ctx context.Context, userID, providerID string) error
}

// UserStore defines the interface for user operations.
type UserStore interface {
	GetByID(ctx context.Context, id string) (*AuthenticatedUser, error)
	GetByEmail(ctx context.Context, email string) (*AuthenticatedUser, error)
	Create(ctx context.Context, user *AuthenticatedUser) error
	Update(ctx context.Context, user *AuthenticatedUser) error
}
