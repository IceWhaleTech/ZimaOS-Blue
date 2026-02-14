package mfa

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDefaultWebAuthnConfig(t *testing.T) {
	config := DefaultWebAuthnConfig()

	if config.RPDisplayName != "ZimaOS-Blue" {
		t.Errorf("RPDisplayName = %v, want ZimaOS-Blue", config.RPDisplayName)
	}
	if config.RPID != "localhost" {
		t.Errorf("RPID = %v, want localhost", config.RPID)
	}
	if config.Timeout != 60000 {
		t.Errorf("Timeout = %v, want 60000", config.Timeout)
	}
}

func TestNewWebAuthn(t *testing.T) {
	// Test with nil config
	wa, err := NewWebAuthn(nil)
	if err != nil {
		t.Fatalf("NewWebAuthn(nil) error = %v", err)
	}
	if wa == nil {
		t.Fatal("NewWebAuthn(nil) returned nil")
	}

	// Test with custom config
	customConfig := &WebAuthnConfig{
		RPDisplayName: "Test App",
		RPID:          "example.com",
		RPOrigins:     []string{"https://example.com"},
		Timeout:       30000,
	}
	wa, err = NewWebAuthn(customConfig)
	if err != nil {
		t.Fatalf("NewWebAuthn() error = %v", err)
	}
	if wa.config.RPDisplayName != "Test App" {
		t.Errorf("RPDisplayName = %v, want Test App", wa.config.RPDisplayName)
	}
}

func TestWebAuthnUser(t *testing.T) {
	user := &WebAuthnUser{
		ID:          uuid.New(),
		Name:        "testuser",
		DisplayName: "Test User",
		Credentials: []WebAuthnCredential{},
	}

	// Test WebAuthnID
	if len(user.WebAuthnID()) != 16 {
		t.Errorf("WebAuthnID() length = %d, want 16", len(user.WebAuthnID()))
	}

	// Test WebAuthnName
	if user.WebAuthnName() != "testuser" {
		t.Errorf("WebAuthnName() = %v, want testuser", user.WebAuthnName())
	}

	// Test WebAuthnDisplayName
	if user.WebAuthnDisplayName() != "Test User" {
		t.Errorf("WebAuthnDisplayName() = %v, want Test User", user.WebAuthnDisplayName())
	}

	// Test WebAuthnDisplayName fallback
	user.DisplayName = ""
	if user.WebAuthnDisplayName() != "testuser" {
		t.Errorf("WebAuthnDisplayName() fallback = %v, want testuser", user.WebAuthnDisplayName())
	}

	// Test WebAuthnCredentials
	creds := user.WebAuthnCredentials()
	if len(creds) != 0 {
		t.Errorf("WebAuthnCredentials() length = %d, want 0", len(creds))
	}
}

func TestWebAuthnCredential(t *testing.T) {
	cred := &WebAuthnCredential{
		ID:              []byte("test-credential-id"),
		PublicKey:       []byte("test-public-key"),
		AttestationType: "none",
		AAGUID:          []byte("test-aaguid"),
		SignCount:       1,
		CloneWarning:    false,
		Name:            "My Security Key",
		CreatedAt:       time.Now(),
		LastUsedAt:      time.Now(),
	}

	// Test ToWebAuthnCredential
	waCred := cred.ToWebAuthnCredential()
	if string(waCred.ID) != "test-credential-id" {
		t.Errorf("ToWebAuthnCredential() ID = %v, want test-credential-id", string(waCred.ID))
	}
	if string(waCred.PublicKey) != "test-public-key" {
		t.Errorf("ToWebAuthnCredential() PublicKey = %v, want test-public-key", string(waCred.PublicKey))
	}
	if waCred.AttestationType != "none" {
		t.Errorf("ToWebAuthnCredential() AttestationType = %v, want none", waCred.AttestationType)
	}
	if waCred.Authenticator.SignCount != 1 {
		t.Errorf("ToWebAuthnCredential() SignCount = %v, want 1", waCred.Authenticator.SignCount)
	}
}

func TestWebAuthn_BeginRegistration(t *testing.T) {
	wa, _ := NewWebAuthn(nil)

	user := &WebAuthnUser{
		ID:          uuid.New(),
		Name:        "testuser",
		DisplayName: "Test User",
		Credentials: []WebAuthnCredential{},
	}

	options, session, err := wa.BeginRegistration(user)
	if err != nil {
		t.Fatalf("BeginRegistration() error = %v", err)
	}

	if options == nil {
		t.Fatal("BeginRegistration() options is nil")
	}

	if session == nil {
		t.Fatal("BeginRegistration() session is nil")
	}

	if session.UserID != user.ID {
		t.Errorf("BeginRegistration() session.UserID = %v, want %v", session.UserID, user.ID)
	}

	if session.Challenge == "" {
		t.Error("BeginRegistration() session.Challenge is empty")
	}

	if session.ExpiresAt.Before(time.Now()) {
		t.Error("BeginRegistration() session.ExpiresAt is in the past")
	}

	if len(session.SessionData) == 0 {
		t.Error("BeginRegistration() session.SessionData is empty")
	}
}

func TestWebAuthn_BeginLogin(t *testing.T) {
	wa, _ := NewWebAuthn(nil)

	// Create a user with a credential
	user := &WebAuthnUser{
		ID:          uuid.New(),
		Name:        "testuser",
		DisplayName: "Test User",
		Credentials: []WebAuthnCredential{
			{
				ID:              []byte("test-credential-id"),
				PublicKey:       []byte("test-public-key"),
				AttestationType: "none",
				AAGUID:          []byte("test-aaguid"),
				SignCount:       1,
			},
		},
	}

	options, session, err := wa.BeginLogin(user)
	if err != nil {
		t.Fatalf("BeginLogin() error = %v", err)
	}

	if options == nil {
		t.Fatal("BeginLogin() options is nil")
	}

	if session == nil {
		t.Fatal("BeginLogin() session is nil")
	}

	if session.UserID != user.ID {
		t.Errorf("BeginLogin() session.UserID = %v, want %v", session.UserID, user.ID)
	}

	if session.Challenge == "" {
		t.Error("BeginLogin() session.Challenge is empty")
	}
}

func TestWebAuthn_BeginDiscoverableLogin(t *testing.T) {
	wa, _ := NewWebAuthn(nil)

	options, session, err := wa.BeginDiscoverableLogin()
	if err != nil {
		t.Fatalf("BeginDiscoverableLogin() error = %v", err)
	}

	if options == nil {
		t.Fatal("BeginDiscoverableLogin() options is nil")
	}

	if session == nil {
		t.Fatal("BeginDiscoverableLogin() session is nil")
	}

	if session.Challenge == "" {
		t.Error("BeginDiscoverableLogin() session.Challenge is empty")
	}
}

func TestWebAuthn_GetConfig(t *testing.T) {
	config := &WebAuthnConfig{
		RPDisplayName: "Test App",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost"},
		Timeout:       60000,
	}
	wa, err := NewWebAuthn(config)
	if err != nil {
		t.Fatalf("NewWebAuthn() error = %v", err)
	}

	if wa.GetConfig().RPDisplayName != "Test App" {
		t.Error("GetConfig() should return the config")
	}
}

func TestRegistrationSession(t *testing.T) {
	session := &RegistrationSession{
		UserID:      uuid.New(),
		Challenge:   "test-challenge",
		ExpiresAt:   time.Now().Add(time.Minute),
		SessionData: []byte("test-session-data"),
	}

	if session.UserID == uuid.Nil {
		t.Error("RegistrationSession.UserID should not be nil")
	}
	if session.Challenge != "test-challenge" {
		t.Errorf("RegistrationSession.Challenge = %v, want test-challenge", session.Challenge)
	}
}

func TestLoginSession(t *testing.T) {
	session := &LoginSession{
		UserID:      uuid.New(),
		Challenge:   "test-challenge",
		ExpiresAt:   time.Now().Add(time.Minute),
		SessionData: []byte("test-session-data"),
	}

	if session.UserID == uuid.Nil {
		t.Error("LoginSession.UserID should not be nil")
	}
	if session.Challenge != "test-challenge" {
		t.Errorf("LoginSession.Challenge = %v, want test-challenge", session.Challenge)
	}
}
