package oidc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestProvider(t *testing.T) (*Provider, func()) {
	tmpDir, err := os.MkdirTemp("", "oidc-provider-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	config := &Config{
		Issuer:                 "https://example.com",
		SigningKeyPath:         filepath.Join(tmpDir, "test.key"),
		SigningKeyRotationDays: 90,
		AccessTokenTTL:         time.Hour,
		RefreshTokenTTL:        30 * 24 * time.Hour,
		AuthorizationCodeTTL:   10 * time.Minute,
		IDTokenTTL:             time.Hour,
		Clients: []ClientConfig{
			{
				ClientID:          "test-client",
				ClientSecret:      "test-secret",
				RedirectURIs:      []string{"http://localhost:3000/callback"},
				AllowedScopes:     []string{ScopeOpenID, ScopeProfile, ScopeEmail, ScopeOffline},
				AllowedGrantTypes: []string{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
				Public:            false,
			},
			{
				ClientID:          "public-client",
				ClientSecret:      "",
				RedirectURIs:      []string{"http://localhost:3000/callback"},
				AllowedScopes:     []string{ScopeOpenID, ScopeProfile},
				AllowedGrantTypes: []string{GrantTypeAuthorizationCode},
				Public:            true,
			},
		},
	}

	provider, err := NewProvider(config, newMockUserInfoProvider())
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return provider, cleanup
}

func TestNewProvider(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	if provider == nil {
		t.Fatal("NewProvider() returned nil")
	}
}

func TestNewProvider_DefaultConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "oidc-provider-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Override default config's key path
	defaultConfig := DefaultConfig()
	defaultConfig.SigningKeyPath = filepath.Join(tmpDir, "test.key")

	provider, err := NewProvider(defaultConfig, newMockUserInfoProvider())
	if err != nil {
		t.Fatalf("NewProvider(default) error = %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider(default) returned nil")
	}
}

func TestProvider_GetConfig(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	config := provider.GetConfig()
	if config == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if config.Issuer != "https://example.com" {
		t.Errorf("GetConfig() Issuer = %v, want https://example.com", config.Issuer)
	}
}

func TestProvider_GetKeyManager(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	km := provider.GetKeyManager()
	if km == nil {
		t.Fatal("GetKeyManager() returned nil")
	}
}

func TestProvider_GetClientStore(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	cs := provider.GetClientStore()
	if cs == nil {
		t.Fatal("GetClientStore() returned nil")
	}
}

func TestProvider_GetTokenService(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	ts := provider.GetTokenService()
	if ts == nil {
		t.Fatal("GetTokenService() returned nil")
	}
}

func TestProvider_CreateAuthorizationSession(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID, ScopeProfile},
		State:        "test-state",
	}

	session, err := provider.CreateAuthorizationSession(req)
	if err != nil {
		t.Fatalf("CreateAuthorizationSession() error = %v", err)
	}

	if session == nil {
		t.Fatal("CreateAuthorizationSession() returned nil")
	}

	if session.ID == "" {
		t.Error("CreateAuthorizationSession() session ID is empty")
	}
}

func TestProvider_CreateAuthorizationSession_InvalidClient(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "invalid-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	_, err := provider.CreateAuthorizationSession(req)
	if err == nil {
		t.Error("CreateAuthorizationSession(invalid client) expected error")
	}
}

func TestProvider_CreateAuthorizationSession_InvalidRedirectURI(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://evil.com/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	_, err := provider.CreateAuthorizationSession(req)
	if err == nil {
		t.Error("CreateAuthorizationSession(invalid redirect) expected error")
	}
}

func TestProvider_GetAuthorizationSession(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	session, _ := provider.CreateAuthorizationSession(req)

	retrieved, err := provider.GetAuthorizationSession(session.ID)
	if err != nil {
		t.Fatalf("GetAuthorizationSession() error = %v", err)
	}

	if retrieved.ID != session.ID {
		t.Errorf("GetAuthorizationSession() ID = %v, want %v", retrieved.ID, session.ID)
	}
}

func TestProvider_GetAuthorizationSession_NotFound(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	_, err := provider.GetAuthorizationSession("invalid-session-id")
	if err == nil {
		t.Error("GetAuthorizationSession(invalid) expected error")
	}
}

func TestProvider_CompleteAuthorization(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
		State:        "test-state",
	}

	session, _ := provider.CreateAuthorizationSession(req)

	code, err := provider.CompleteAuthorization(session.ID, "user-123", "testuser")
	if err != nil {
		t.Fatalf("CompleteAuthorization() error = %v", err)
	}

	if code == nil {
		t.Fatal("CompleteAuthorization() returned nil")
	}

	if code.Code == "" {
		t.Error("CompleteAuthorization() code is empty")
	}

	if code.UserID != "user-123" {
		t.Errorf("CompleteAuthorization() UserID = %v, want user-123", code.UserID)
	}

	// Session should be deleted
	_, err = provider.GetAuthorizationSession(session.ID)
	if err == nil {
		t.Error("CompleteAuthorization() should delete session")
	}
}

func TestProvider_ExchangeCode(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	// Create session and complete authorization
	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID, ScopeOffline},
		State:        "test-state",
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")

	// Exchange code for tokens
	response, err := provider.ExchangeCode(code.Code, "test-client", "test-secret", "http://localhost:3000/callback", "")
	if err != nil {
		t.Fatalf("ExchangeCode() error = %v", err)
	}

	if response.AccessToken == "" {
		t.Error("ExchangeCode() AccessToken is empty")
	}

	if response.RefreshToken == "" {
		t.Error("ExchangeCode() RefreshToken is empty (offline_access was requested)")
	}
}

func TestProvider_ExchangeCode_InvalidClient(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")

	_, err := provider.ExchangeCode(code.Code, "test-client", "wrong-secret", "http://localhost:3000/callback", "")
	if err == nil {
		t.Error("ExchangeCode(wrong secret) expected error")
	}
}

func TestProvider_RefreshTokens(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID, ScopeOffline},
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")
	tokens, _ := provider.ExchangeCode(code.Code, "test-client", "test-secret", "http://localhost:3000/callback", "")

	// Refresh tokens
	newTokens, err := provider.RefreshTokens(tokens.RefreshToken, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("RefreshTokens() error = %v", err)
	}

	if newTokens.AccessToken == "" {
		t.Error("RefreshTokens() AccessToken is empty")
	}
}

func TestProvider_RevokeToken(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID, ScopeOffline},
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")
	tokens, _ := provider.ExchangeCode(code.Code, "test-client", "test-secret", "http://localhost:3000/callback", "")

	err := provider.RevokeToken(tokens.RefreshToken, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}

	// Token should be revoked
	_, err = provider.RefreshTokens(tokens.RefreshToken, "test-client", "test-secret")
	if err == nil {
		t.Error("RefreshTokens(revoked) expected error")
	}
}

func TestProvider_IntrospectToken(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")
	tokens, _ := provider.ExchangeCode(code.Code, "test-client", "test-secret", "http://localhost:3000/callback", "")

	introspection, err := provider.IntrospectToken(tokens.AccessToken, "test-client", "test-secret")
	if err != nil {
		t.Fatalf("IntrospectToken() error = %v", err)
	}

	if !introspection.Active {
		t.Error("IntrospectToken() Active = false, want true")
	}
}

func TestProvider_ValidateAccessToken(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	req := &AuthorizationRequest{
		ClientID:     "test-client",
		RedirectURI:  "http://localhost:3000/callback",
		ResponseType: "code",
		Scope:        []string{ScopeOpenID},
	}

	session, _ := provider.CreateAuthorizationSession(req)
	code, _ := provider.CompleteAuthorization(session.ID, "user-123", "testuser")
	tokens, _ := provider.ExchangeCode(code.Code, "test-client", "test-secret", "http://localhost:3000/callback", "")

	claims, err := provider.ValidateAccessToken(tokens.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("ValidateAccessToken() Subject = %v, want user-123", claims.Subject)
	}
}

func TestProvider_DiscoveryHandler(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	handler := provider.DiscoveryHandler()
	if handler == nil {
		t.Fatal("DiscoveryHandler() returned nil")
	}
}

func TestProvider_JWKSHandler(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	handler := provider.JWKSHandler()
	if handler == nil {
		t.Fatal("JWKSHandler() returned nil")
	}
}

func TestProvider_UserInfoHandler(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	handler := provider.UserInfoHandler()
	if handler == nil {
		t.Fatal("UserInfoHandler() returned nil")
	}
}

func TestProvider_RotateKeys(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	err := provider.RotateKeys()
	if err != nil {
		t.Fatalf("RotateKeys() error = %v", err)
	}
}

func TestProvider_RegisterClient(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	client := &Client{
		ID:            "new-client",
		Secret:        "new-secret",
		RedirectURIs:  []string{"http://localhost:4000/callback"},
		AllowedScopes: []string{ScopeOpenID},
		Public:        false,
	}

	provider.RegisterClient(client)

	// Verify client was registered
	cs := provider.GetClientStore()
	retrieved, err := cs.Get("new-client")
	if err != nil {
		t.Fatalf("RegisterClient() client not found: %v", err)
	}

	if retrieved.ID != "new-client" {
		t.Errorf("RegisterClient() ID = %v, want new-client", retrieved.ID)
	}
}

func TestProvider_DeleteClient(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	err := provider.DeleteClient("test-client")
	if err != nil {
		t.Fatalf("DeleteClient() error = %v", err)
	}

	// Verify client was deleted
	cs := provider.GetClientStore()
	_, err = cs.Get("test-client")
	if err == nil {
		t.Error("DeleteClient() client should be deleted")
	}
}

func TestProvider_ListClients(t *testing.T) {
	provider, cleanup := setupTestProvider(t)
	defer cleanup()

	clients := provider.ListClients()
	if len(clients) != 2 {
		t.Errorf("ListClients() returned %d clients, want 2", len(clients))
	}
}
