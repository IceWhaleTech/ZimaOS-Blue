package mfa

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
)

var (
	// ErrWebAuthnNotConfigured is returned when WebAuthn is not configured.
	ErrWebAuthnNotConfigured = errors.New("webauthn not configured")
	// ErrCredentialNotFound is returned when a credential is not found.
	ErrCredentialNotFound = errors.New("credential not found")
	// ErrInvalidChallenge is returned when the challenge is invalid.
	ErrInvalidChallenge = errors.New("invalid challenge")
)

// WebAuthnConfig holds WebAuthn configuration.
type WebAuthnConfig struct {
	// RPDisplayName is the display name of the relying party.
	RPDisplayName string
	// RPID is the relying party ID (usually the domain).
	RPID string
	// RPOrigins are the allowed origins.
	RPOrigins []string
	// Timeout is the timeout for ceremonies in milliseconds.
	Timeout int
	// AuthenticatorSelection specifies authenticator requirements.
	AuthenticatorSelection *protocol.AuthenticatorSelection
	// AttestationPreference specifies attestation conveyance preference.
	AttestationPreference protocol.ConveyancePreference
}

// DefaultWebAuthnConfig returns the default WebAuthn configuration.
func DefaultWebAuthnConfig() *WebAuthnConfig {
	return &WebAuthnConfig{
		RPDisplayName: "ZimaOS-Echo",
		RPID:          "localhost",
		RPOrigins:     []string{"http://localhost:3000", "https://localhost:3000"},
		Timeout:       60000, // 60 seconds
		AuthenticatorSelection: &protocol.AuthenticatorSelection{
			AuthenticatorAttachment: protocol.CrossPlatform,
			UserVerification:        protocol.VerificationPreferred,
			ResidentKey:             protocol.ResidentKeyRequirementDiscouraged,
		},
		AttestationPreference: protocol.PreferNoAttestation,
	}
}

// WebAuthn provides WebAuthn registration and authentication.
type WebAuthn struct {
	webauthn *webauthn.WebAuthn
	config   *WebAuthnConfig
}

// NewWebAuthn creates a new WebAuthn instance.
func NewWebAuthn(config *WebAuthnConfig) (*WebAuthn, error) {
	if config == nil {
		config = DefaultWebAuthnConfig()
	}

	wconfig := &webauthn.Config{
		RPDisplayName: config.RPDisplayName,
		RPID:          config.RPID,
		RPOrigins:     config.RPOrigins,
		Timeouts: webauthn.TimeoutsConfig{
			Login: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Duration(config.Timeout) * time.Millisecond,
				TimeoutUVD: time.Duration(config.Timeout) * time.Millisecond,
			},
			Registration: webauthn.TimeoutConfig{
				Enforce:    true,
				Timeout:    time.Duration(config.Timeout) * time.Millisecond,
				TimeoutUVD: time.Duration(config.Timeout) * time.Millisecond,
			},
		},
	}

	w, err := webauthn.New(wconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create webauthn: %w", err)
	}

	return &WebAuthn{
		webauthn: w,
		config:   config,
	}, nil
}

// WebAuthnUser represents a user for WebAuthn operations.
type WebAuthnUser struct {
	ID          uuid.UUID
	Name        string
	DisplayName string
	Credentials []WebAuthnCredential
}

// WebAuthnID returns the user's WebAuthn ID.
func (u *WebAuthnUser) WebAuthnID() []byte {
	return u.ID[:]
}

// WebAuthnName returns the user's name.
func (u *WebAuthnUser) WebAuthnName() string {
	return u.Name
}

// WebAuthnDisplayName returns the user's display name.
func (u *WebAuthnUser) WebAuthnDisplayName() string {
	if u.DisplayName != "" {
		return u.DisplayName
	}
	return u.Name
}

// WebAuthnCredentials returns the user's credentials.
func (u *WebAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	creds := make([]webauthn.Credential, len(u.Credentials))
	for i, c := range u.Credentials {
		creds[i] = c.ToWebAuthnCredential()
	}
	return creds
}

// WebAuthnCredential represents a stored WebAuthn credential.
type WebAuthnCredential struct {
	ID              []byte    `json:"id"`
	PublicKey       []byte    `json:"public_key"`
	AttestationType string    `json:"attestation_type"`
	AAGUID          []byte    `json:"aaguid"`
	SignCount       uint32    `json:"sign_count"`
	CloneWarning    bool      `json:"clone_warning"`
	Name            string    `json:"name"`
	CreatedAt       time.Time `json:"created_at"`
	LastUsedAt      time.Time `json:"last_used_at"`
}

// ToWebAuthnCredential converts to webauthn.Credential.
func (c *WebAuthnCredential) ToWebAuthnCredential() webauthn.Credential {
	return webauthn.Credential{
		ID:              c.ID,
		PublicKey:       c.PublicKey,
		AttestationType: c.AttestationType,
		Authenticator: webauthn.Authenticator{
			AAGUID:       c.AAGUID,
			SignCount:    c.SignCount,
			CloneWarning: c.CloneWarning,
		},
	}
}

// FromWebAuthnCredential creates a WebAuthnCredential from webauthn.Credential.
func FromWebAuthnCredential(cred *webauthn.Credential, name string) *WebAuthnCredential {
	now := time.Now().UTC()
	return &WebAuthnCredential{
		ID:              cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		CloneWarning:    cred.Authenticator.CloneWarning,
		Name:            name,
		CreatedAt:       now,
		LastUsedAt:      now,
	}
}

// RegistrationSession holds the state for a registration ceremony.
type RegistrationSession struct {
	UserID    uuid.UUID `json:"user_id"`
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
	// SessionData is the serialized webauthn.SessionData
	SessionData []byte `json:"session_data"`
}

// LoginSession holds the state for a login ceremony.
type LoginSession struct {
	UserID    uuid.UUID `json:"user_id"`
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
	// SessionData is the serialized webauthn.SessionData
	SessionData []byte `json:"session_data"`
}

// BeginRegistration starts a WebAuthn registration ceremony.
func (w *WebAuthn) BeginRegistration(user *WebAuthnUser) (*protocol.CredentialCreation, *RegistrationSession, error) {
	options, session, err := w.webauthn.BeginRegistration(user,
		webauthn.WithAuthenticatorSelection(*w.config.AuthenticatorSelection),
		webauthn.WithConveyancePreference(w.config.AttestationPreference),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin registration: %w", err)
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	regSession := &RegistrationSession{
		UserID:      user.ID,
		Challenge:   session.Challenge,
		ExpiresAt:   time.Now().Add(time.Duration(w.config.Timeout) * time.Millisecond),
		SessionData: sessionData,
	}

	return options, regSession, nil
}

// FinishRegistration completes a WebAuthn registration ceremony.
func (w *WebAuthn) FinishRegistration(user *WebAuthnUser, session *RegistrationSession, response *protocol.ParsedCredentialCreationData) (*WebAuthnCredential, error) {
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	credential, err := w.webauthn.CreateCredential(user, sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("failed to create credential: %w", err)
	}

	return FromWebAuthnCredential(credential, ""), nil
}

// BeginLogin starts a WebAuthn login ceremony.
func (w *WebAuthn) BeginLogin(user *WebAuthnUser) (*protocol.CredentialAssertion, *LoginSession, error) {
	options, session, err := w.webauthn.BeginLogin(user)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin login: %w", err)
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	loginSession := &LoginSession{
		UserID:      user.ID,
		Challenge:   session.Challenge,
		ExpiresAt:   time.Now().Add(time.Duration(w.config.Timeout) * time.Millisecond),
		SessionData: sessionData,
	}

	return options, loginSession, nil
}

// FinishLogin completes a WebAuthn login ceremony.
func (w *WebAuthn) FinishLogin(user *WebAuthnUser, session *LoginSession, response *protocol.ParsedCredentialAssertionData) (*WebAuthnCredential, error) {
	var sessionData webauthn.SessionData
	if err := json.Unmarshal(session.SessionData, &sessionData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	credential, err := w.webauthn.ValidateLogin(user, sessionData, response)
	if err != nil {
		return nil, fmt.Errorf("failed to validate login: %w", err)
	}

	// Find and update the matching credential
	for i, c := range user.Credentials {
		if string(c.ID) == string(credential.ID) {
			user.Credentials[i].SignCount = credential.Authenticator.SignCount
			user.Credentials[i].LastUsedAt = time.Now().UTC()
			return &user.Credentials[i], nil
		}
	}

	return nil, ErrCredentialNotFound
}

// BeginDiscoverableLogin starts a discoverable (usernameless) login ceremony.
func (w *WebAuthn) BeginDiscoverableLogin() (*protocol.CredentialAssertion, *LoginSession, error) {
	options, session, err := w.webauthn.BeginDiscoverableLogin()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to begin discoverable login: %w", err)
	}

	sessionData, err := json.Marshal(session)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal session: %w", err)
	}

	loginSession := &LoginSession{
		Challenge:   session.Challenge,
		ExpiresAt:   time.Now().Add(time.Duration(w.config.Timeout) * time.Millisecond),
		SessionData: sessionData,
	}

	return options, loginSession, nil
}

// GetConfig returns the current WebAuthn configuration.
func (w *WebAuthn) GetConfig() *WebAuthnConfig {
	return w.config
}
