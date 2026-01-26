package oidc

import (
	"testing"
	"time"
)

func TestAuthorizationCodeStore(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost:3000/callback",
		ResponseType:        "code",
		Scope:               []string{ScopeOpenID, ScopeProfile},
		State:               "test-state",
		Nonce:               "test-nonce",
		CodeChallenge:       "",
		CodeChallengeMethod: "",
	}

	// Generate code
	code, err := store.Generate(req, "user-123", "testuser")
	if err != nil {
		t.Fatalf("failed to generate code: %v", err)
	}

	if code.Code == "" {
		t.Error("expected non-empty code")
	}

	if code.ClientID != req.ClientID {
		t.Errorf("expected client ID %s, got %s", req.ClientID, code.ClientID)
	}

	if code.UserID != "user-123" {
		t.Errorf("expected user ID user-123, got %s", code.UserID)
	}

	// Validate code
	validated, err := store.Validate(code.Code, "test-client", "http://localhost:3000/callback", "")
	if err != nil {
		t.Fatalf("failed to validate code: %v", err)
	}

	if validated.UserID != "user-123" {
		t.Errorf("expected user ID user-123, got %s", validated.UserID)
	}

	// Code should be marked as used
	_, err = store.Validate(code.Code, "test-client", "http://localhost:3000/callback", "")
	if err != ErrAuthorizationCodeUsed {
		t.Errorf("expected ErrAuthorizationCodeUsed, got %v", err)
	}
}

func TestAuthorizationCodeStoreInvalidClient(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:3000/callback",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Try to validate with wrong client ID
	_, err := store.Validate(code.Code, "wrong-client", "http://localhost:3000/callback", "")
	if err != ErrInvalidAuthorizationCode {
		t.Errorf("expected ErrInvalidAuthorizationCode, got %v", err)
	}
}

func TestAuthorizationCodeStoreInvalidRedirectURI(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:3000/callback",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Try to validate with wrong redirect URI
	_, err := store.Validate(code.Code, "test-client", "http://evil.com/callback", "")
	if err != ErrInvalidAuthorizationCode {
		t.Errorf("expected ErrInvalidAuthorizationCode, got %v", err)
	}
}

func TestAuthorizationCodeStorePKCE(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	// Test S256 PKCE
	// code_verifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	// code_challenge = base64url(sha256(code_verifier)) = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	codeChallenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	req := &AuthorizationRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost:3000/callback",
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: "S256",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Validate with correct verifier
	_, err := store.Validate(code.Code, "test-client", "http://localhost:3000/callback", codeVerifier)
	if err != nil {
		t.Errorf("expected successful PKCE validation, got error: %v", err)
	}
}

func TestAuthorizationCodeStorePKCEMissing(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost:3000/callback",
		CodeChallenge:       "some-challenge",
		CodeChallengeMethod: "S256",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Try to validate without verifier
	_, err := store.Validate(code.Code, "test-client", "http://localhost:3000/callback", "")
	if err != ErrMissingPKCE {
		t.Errorf("expected ErrMissingPKCE, got %v", err)
	}
}

func TestAuthorizationCodeStorePKCEInvalid(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost:3000/callback",
		CodeChallenge:       "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		CodeChallengeMethod: "S256",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Try to validate with wrong verifier
	_, err := store.Validate(code.Code, "test-client", "http://localhost:3000/callback", "wrong-verifier")
	if err != ErrInvalidPKCE {
		t.Errorf("expected ErrInvalidPKCE, got %v", err)
	}
}

func TestAuthorizationCodeStorePlainPKCE(t *testing.T) {
	store := NewAuthorizationCodeStore(10 * time.Minute)

	verifier := "plain-verifier-value"

	req := &AuthorizationRequest{
		ClientID:            "test-client",
		RedirectURI:         "http://localhost:3000/callback",
		CodeChallenge:       verifier, // For plain, challenge = verifier
		CodeChallengeMethod: "plain",
	}

	code, _ := store.Generate(req, "user-123", "testuser")

	// Validate with correct verifier
	_, err := store.Validate(code.Code, "test-client", "http://localhost:3000/callback", verifier)
	if err != nil {
		t.Errorf("expected successful plain PKCE validation, got error: %v", err)
	}
}

func TestAuthorizationSessionStore(t *testing.T) {
	store := NewAuthorizationSessionStore(10 * time.Minute)

	req := &AuthorizationRequest{
		ClientID:    "test-client",
		RedirectURI: "http://localhost:3000/callback",
		Scope:       []string{ScopeOpenID},
		State:       "test-state",
	}

	// Create session
	session := store.Create(req)
	if session.ID == "" {
		t.Error("expected non-empty session ID")
	}

	// Get session
	retrieved, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if retrieved.Request.ClientID != req.ClientID {
		t.Errorf("expected client ID %s, got %s", req.ClientID, retrieved.Request.ClientID)
	}

	// Delete session
	store.Delete(session.ID)

	// Session should not exist
	_, err = store.Get(session.ID)
	if err == nil {
		t.Error("expected error getting deleted session")
	}
}

func TestVerifyPKCE(t *testing.T) {
	tests := []struct {
		name      string
		challenge string
		method    string
		verifier  string
		expected  bool
	}{
		{
			name:      "S256 valid",
			challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			method:    "S256",
			verifier:  "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk",
			expected:  true,
		},
		{
			name:      "S256 invalid",
			challenge: "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
			method:    "S256",
			verifier:  "wrong-verifier",
			expected:  false,
		},
		{
			name:      "plain valid",
			challenge: "plain-verifier",
			method:    "plain",
			verifier:  "plain-verifier",
			expected:  true,
		},
		{
			name:      "plain invalid",
			challenge: "plain-verifier",
			method:    "plain",
			verifier:  "wrong-verifier",
			expected:  false,
		},
		{
			name:      "unknown method",
			challenge: "challenge",
			method:    "unknown",
			verifier:  "verifier",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifyPKCE(tt.challenge, tt.method, tt.verifier)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
