package oidc

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestTokenService(t *testing.T) (*TokenService, func()) {
	tmpDir, err := os.MkdirTemp("", "oidc-token-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	ts := NewTokenService(km, "https://example.com", time.Hour, 30*24*time.Hour, time.Hour)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return ts, cleanup
}

func TestNewTokenService(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	if ts == nil {
		t.Fatal("NewTokenService() returned nil")
	}
}

func TestTokenService_GenerateTokens(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeProfile, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		Nonce:       "test-nonce",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, err := ts.GenerateTokens(authCode)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	if response.AccessToken == "" {
		t.Error("GenerateTokens() AccessToken is empty")
	}

	if response.TokenType != "Bearer" {
		t.Errorf("GenerateTokens() TokenType = %v, want Bearer", response.TokenType)
	}

	if response.ExpiresIn <= 0 {
		t.Error("GenerateTokens() ExpiresIn should be positive")
	}

	// Should have refresh token because offline_access scope was requested
	if response.RefreshToken == "" {
		t.Error("GenerateTokens() RefreshToken is empty (offline_access was requested)")
	}

	// Should have ID token because openid scope was requested
	if response.IDToken == "" {
		t.Error("GenerateTokens() IDToken is empty (openid was requested)")
	}
}

func TestTokenService_GenerateTokens_NoOfflineAccess(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeProfile}, // No offline_access
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, err := ts.GenerateTokens(authCode)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	// Should NOT have refresh token
	if response.RefreshToken != "" {
		t.Error("GenerateTokens() should not have RefreshToken without offline_access")
	}
}

func TestTokenService_GenerateTokens_NoOpenID(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeProfile}, // No openid
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, err := ts.GenerateTokens(authCode)
	if err != nil {
		t.Fatalf("GenerateTokens() error = %v", err)
	}

	// Should NOT have ID token
	if response.IDToken != "" {
		t.Error("GenerateTokens() should not have IDToken without openid scope")
	}
}

func TestTokenService_ValidateAccessToken(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	claims, err := ts.ValidateAccessToken(response.AccessToken)
	if err != nil {
		t.Fatalf("ValidateAccessToken() error = %v", err)
	}

	if claims.Subject != "user-123" {
		t.Errorf("ValidateAccessToken() Subject = %v, want user-123", claims.Subject)
	}

	if claims.ClientID != "test-client" {
		t.Errorf("ValidateAccessToken() ClientID = %v, want test-client", claims.ClientID)
	}

	if claims.Issuer != "https://example.com" {
		t.Errorf("ValidateAccessToken() Issuer = %v, want https://example.com", claims.Issuer)
	}
}

func TestTokenService_ValidateAccessToken_Invalid(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	_, err := ts.ValidateAccessToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateAccessToken(invalid) error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestTokenService_RefreshTokens(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	// Refresh tokens
	newResponse, err := ts.RefreshTokens(response.RefreshToken, "test-client")
	if err != nil {
		t.Fatalf("RefreshTokens() error = %v", err)
	}

	if newResponse.AccessToken == "" {
		t.Error("RefreshTokens() AccessToken is empty")
	}

	if newResponse.RefreshToken == "" {
		t.Error("RefreshTokens() RefreshToken is empty")
	}

	// New refresh token should be different (rotation)
	if newResponse.RefreshToken == response.RefreshToken {
		t.Error("RefreshTokens() should rotate refresh token")
	}

	// Old refresh token should be revoked
	_, err = ts.RefreshTokens(response.RefreshToken, "test-client")
	if err != ErrTokenRevoked {
		t.Errorf("RefreshTokens(old) error = %v, want %v", err, ErrTokenRevoked)
	}
}

func TestTokenService_RefreshTokens_InvalidClient(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	// Try to refresh with wrong client
	_, err := ts.RefreshTokens(response.RefreshToken, "wrong-client")
	if err != ErrInvalidRefreshToken {
		t.Errorf("RefreshTokens(wrong client) error = %v, want %v", err, ErrInvalidRefreshToken)
	}
}

func TestTokenService_RevokeToken(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	// Revoke refresh token
	err := ts.RevokeToken(response.RefreshToken)
	if err != nil {
		t.Fatalf("RevokeToken() error = %v", err)
	}

	// Try to use revoked token
	_, err = ts.RefreshTokens(response.RefreshToken, "test-client")
	if err != ErrTokenRevoked {
		t.Errorf("RefreshTokens(revoked) error = %v, want %v", err, ErrTokenRevoked)
	}
}

func TestTokenService_RevokeUserTokens(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	// Create multiple tokens for same user
	authCode1 := &AuthorizationCode{
		Code:        "test-code-1",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	authCode2 := &AuthorizationCode{
		Code:        "test-code-2",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeOffline},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response1, _ := ts.GenerateTokens(authCode1)
	response2, _ := ts.GenerateTokens(authCode2)

	// Revoke all tokens for user
	ts.RevokeUserTokens("user-123")

	// Both tokens should be revoked
	_, err := ts.RefreshTokens(response1.RefreshToken, "test-client")
	if err != ErrTokenRevoked {
		t.Errorf("RefreshTokens(token1) error = %v, want %v", err, ErrTokenRevoked)
	}

	_, err = ts.RefreshTokens(response2.RefreshToken, "test-client")
	if err != ErrTokenRevoked {
		t.Errorf("RefreshTokens(token2) error = %v, want %v", err, ErrTokenRevoked)
	}
}

func TestTokenService_IntrospectToken(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeProfile},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	introspection := ts.IntrospectToken(response.AccessToken)

	if !introspection.Active {
		t.Error("IntrospectToken() Active = false, want true")
	}

	if introspection.Sub != "user-123" {
		t.Errorf("IntrospectToken() Sub = %v, want user-123", introspection.Sub)
	}

	if introspection.ClientID != "test-client" {
		t.Errorf("IntrospectToken() ClientID = %v, want test-client", introspection.ClientID)
	}

	if introspection.TokenType != "Bearer" {
		t.Errorf("IntrospectToken() TokenType = %v, want Bearer", introspection.TokenType)
	}
}

func TestTokenService_IntrospectToken_Invalid(t *testing.T) {
	ts, cleanup := setupTestTokenService(t)
	defer cleanup()

	introspection := ts.IntrospectToken("invalid-token")

	if introspection.Active {
		t.Error("IntrospectToken(invalid) Active = true, want false")
	}
}

func TestRefreshTokenStore_Generate(t *testing.T) {
	store := NewRefreshTokenStore()

	rt, err := store.Generate("user-123", "testuser", "client-1", []string{ScopeOpenID}, time.Hour)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if rt.Token == "" {
		t.Error("Generate() Token is empty")
	}

	if rt.UserID != "user-123" {
		t.Errorf("Generate() UserID = %v, want user-123", rt.UserID)
	}

	if rt.ClientID != "client-1" {
		t.Errorf("Generate() ClientID = %v, want client-1", rt.ClientID)
	}

	if rt.Revoked {
		t.Error("Generate() Revoked = true, want false")
	}
}

func TestRefreshTokenStore_Validate(t *testing.T) {
	store := NewRefreshTokenStore()

	rt, _ := store.Generate("user-123", "testuser", "client-1", []string{ScopeOpenID}, time.Hour)

	validated, err := store.Validate(rt.Token, "client-1")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	if validated.UserID != rt.UserID {
		t.Errorf("Validate() UserID = %v, want %v", validated.UserID, rt.UserID)
	}
}

func TestRefreshTokenStore_Validate_WrongClient(t *testing.T) {
	store := NewRefreshTokenStore()

	rt, _ := store.Generate("user-123", "testuser", "client-1", []string{ScopeOpenID}, time.Hour)

	_, err := store.Validate(rt.Token, "wrong-client")
	if err != ErrInvalidRefreshToken {
		t.Errorf("Validate(wrong client) error = %v, want %v", err, ErrInvalidRefreshToken)
	}
}

func TestRefreshTokenStore_Revoke(t *testing.T) {
	store := NewRefreshTokenStore()

	rt, _ := store.Generate("user-123", "testuser", "client-1", []string{ScopeOpenID}, time.Hour)

	err := store.Revoke(rt.Token)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}

	_, err = store.Validate(rt.Token, "client-1")
	if err != ErrTokenRevoked {
		t.Errorf("Validate(revoked) error = %v, want %v", err, ErrTokenRevoked)
	}
}

func TestScopesToString(t *testing.T) {
	tests := []struct {
		name     string
		scopes   []string
		expected string
	}{
		{
			name:     "single scope",
			scopes:   []string{"openid"},
			expected: "openid",
		},
		{
			name:     "multiple scopes",
			scopes:   []string{"openid", "profile", "email"},
			expected: "openid profile email",
		},
		{
			name:     "empty scopes",
			scopes:   []string{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scopesToString(tt.scopes)
			if result != tt.expected {
				t.Errorf("scopesToString() = %v, want %v", result, tt.expected)
			}
		})
	}
}
