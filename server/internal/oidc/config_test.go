package oidc

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.Issuer != "http://localhost:23456" {
		t.Errorf("expected issuer http://localhost:23456, got %s", config.Issuer)
	}

	if config.AccessTokenTTL != time.Hour {
		t.Errorf("expected access token TTL 1h, got %v", config.AccessTokenTTL)
	}

	if config.RefreshTokenTTL != 30*24*time.Hour {
		t.Errorf("expected refresh token TTL 30d, got %v", config.RefreshTokenTTL)
	}

	if config.AuthorizationCodeTTL != 10*time.Minute {
		t.Errorf("expected authorization code TTL 10m, got %v", config.AuthorizationCodeTTL)
	}
}

func TestSupportedScopes(t *testing.T) {
	scopes := SupportedScopes()

	expected := []string{ScopeOpenID, ScopeProfile, ScopeEmail, ScopeOffline}
	if len(scopes) != len(expected) {
		t.Errorf("expected %d scopes, got %d", len(expected), len(scopes))
	}

	scopeMap := make(map[string]bool)
	for _, s := range scopes {
		scopeMap[s] = true
	}

	for _, e := range expected {
		if !scopeMap[e] {
			t.Errorf("expected scope %s not found", e)
		}
	}
}

func TestSupportedGrantTypes(t *testing.T) {
	grantTypes := SupportedGrantTypes()

	expected := []string{GrantTypeAuthorizationCode, GrantTypeRefreshToken}
	if len(grantTypes) != len(expected) {
		t.Errorf("expected %d grant types, got %d", len(expected), len(grantTypes))
	}
}

func TestSupportedResponseTypes(t *testing.T) {
	responseTypes := SupportedResponseTypes()

	if len(responseTypes) != 1 || responseTypes[0] != ResponseTypeCode {
		t.Errorf("expected [code], got %v", responseTypes)
	}
}
