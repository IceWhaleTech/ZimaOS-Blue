package oidc

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrProviderNotInitialized is returned when the provider is not initialized.
	ErrProviderNotInitialized = errors.New("OIDC provider not initialized")
)

// Provider is the main OIDC provider.
type Provider struct {
	config       *Config
	keyManager   *KeyManager
	clientStore  *ClientStore
	codeStore    *AuthorizationCodeStore
	sessionStore *AuthorizationSessionStore
	tokenService *TokenService
	userProvider UserInfoProvider
}

// NewProvider creates a new OIDC provider.
func NewProvider(config *Config, userProvider UserInfoProvider) (*Provider, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Initialize key manager
	keyManager, err := NewKeyManager(config.SigningKeyPath, config.SigningKeyRotationDays)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize key manager: %w", err)
	}

	// Initialize stores
	clientStore := NewClientStoreFromConfig(config.Clients)
	codeStore := NewAuthorizationCodeStore(config.AuthorizationCodeTTL)
	sessionStore := NewAuthorizationSessionStore(config.AuthorizationCodeTTL)

	// Initialize token service
	tokenService := NewTokenService(
		keyManager,
		config.Issuer,
		config.AccessTokenTTL,
		config.RefreshTokenTTL,
		config.IDTokenTTL,
	)

	return &Provider{
		config:       config,
		keyManager:   keyManager,
		clientStore:  clientStore,
		codeStore:    codeStore,
		sessionStore: sessionStore,
		tokenService: tokenService,
		userProvider: userProvider,
	}, nil
}

// GetConfig returns the provider configuration.
func (p *Provider) GetConfig() *Config {
	return p.config
}

// GetKeyManager returns the key manager.
func (p *Provider) GetKeyManager() *KeyManager {
	return p.keyManager
}

// GetClientStore returns the client store.
func (p *Provider) GetClientStore() *ClientStore {
	return p.clientStore
}

// GetTokenService returns the token service.
func (p *Provider) GetTokenService() *TokenService {
	return p.tokenService
}

// CreateAuthorizationSession creates a new authorization session.
func (p *Provider) CreateAuthorizationSession(req *AuthorizationRequest) (*AuthorizationSession, error) {
	// Validate client
	_, err := p.clientStore.Get(req.ClientID)
	if err != nil {
		return nil, err
	}

	// Validate redirect URI
	if err := p.clientStore.ValidateRedirectURI(req.ClientID, req.RedirectURI); err != nil {
		return nil, err
	}

	// Validate scopes
	if err := p.clientStore.ValidateScopes(req.ClientID, req.Scope); err != nil {
		return nil, err
	}

	return p.sessionStore.Create(req), nil
}

// GetAuthorizationSession retrieves an authorization session.
func (p *Provider) GetAuthorizationSession(sessionID string) (*AuthorizationSession, error) {
	return p.sessionStore.Get(sessionID)
}

// CompleteAuthorization completes the authorization and generates a code.
func (p *Provider) CompleteAuthorization(sessionID, userID, username string) (*AuthorizationCode, error) {
	session, err := p.sessionStore.Get(sessionID)
	if err != nil {
		return nil, err
	}

	code, err := p.codeStore.Generate(session.Request, userID, username)
	if err != nil {
		return nil, err
	}

	// Delete session
	p.sessionStore.Delete(sessionID)

	return code, nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (p *Provider) ExchangeCode(code, clientID, clientSecret, redirectURI, codeVerifier string) (*TokenResponse, error) {
	// Authenticate client
	_, err := p.clientStore.Authenticate(clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	// Validate code
	authCode, err := p.codeStore.Validate(code, clientID, redirectURI, codeVerifier)
	if err != nil {
		return nil, err
	}

	// Generate tokens
	return p.tokenService.GenerateTokens(authCode)
}

// RefreshTokens refreshes tokens using a refresh token.
func (p *Provider) RefreshTokens(refreshToken, clientID, clientSecret string) (*TokenResponse, error) {
	// Authenticate client
	_, err := p.clientStore.Authenticate(clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	return p.tokenService.RefreshTokens(refreshToken, clientID)
}

// RevokeToken revokes a token.
func (p *Provider) RevokeToken(token, clientID, clientSecret string) error {
	// Authenticate client
	_, err := p.clientStore.Authenticate(clientID, clientSecret)
	if err != nil {
		return err
	}

	return p.tokenService.RevokeToken(token)
}

// IntrospectToken introspects a token.
func (p *Provider) IntrospectToken(token, clientID, clientSecret string) (*IntrospectionResponse, error) {
	// Authenticate client
	_, err := p.clientStore.Authenticate(clientID, clientSecret)
	if err != nil {
		return nil, err
	}

	return p.tokenService.IntrospectToken(token), nil
}

// ValidateAccessToken validates an access token.
func (p *Provider) ValidateAccessToken(token string) (*TokenClaims, error) {
	return p.tokenService.ValidateAccessToken(token)
}

// GetUserInfo returns user information for a valid access token.
func (p *Provider) GetUserInfo(ctx context.Context, token string) (*UserInfoClaims, error) {
	claims, err := p.tokenService.ValidateAccessToken(token)
	if err != nil {
		return nil, err
	}

	return p.userProvider.GetUserInfo(claims.Subject)
}

// DiscoveryHandler returns the discovery handler.
func (p *Provider) DiscoveryHandler() *DiscoveryHandler {
	return NewDiscoveryHandler(p.config.Issuer)
}

// JWKSHandler returns the JWKS handler.
func (p *Provider) JWKSHandler() *JWKSHandler {
	return NewJWKSHandler(p.keyManager)
}

// UserInfoHandler returns the userinfo handler.
func (p *Provider) UserInfoHandler() *UserInfoHandler {
	return NewUserInfoHandler(p.tokenService, p.userProvider)
}

// RotateKeys rotates the signing keys if needed.
func (p *Provider) RotateKeys() error {
	return p.keyManager.RotateKey()
}

// RegisterClient registers a new OIDC client.
func (p *Provider) RegisterClient(client *Client) {
	p.clientStore.Register(client)
}

// DeleteClient removes an OIDC client.
func (p *Provider) DeleteClient(clientID string) error {
	return p.clientStore.Delete(clientID)
}

// ListClients returns all registered clients.
func (p *Provider) ListClients() []*Client {
	return p.clientStore.List()
}
