package oidc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

var (
	// ErrInvalidAuthorizationCode is returned when the authorization code is invalid.
	ErrInvalidAuthorizationCode = errors.New("invalid authorization code")
	// ErrAuthorizationCodeExpired is returned when the authorization code has expired.
	ErrAuthorizationCodeExpired = errors.New("authorization code expired")
	// ErrAuthorizationCodeUsed is returned when the authorization code has already been used.
	ErrAuthorizationCodeUsed = errors.New("authorization code already used")
	// ErrInvalidPKCE is returned when PKCE verification fails.
	ErrInvalidPKCE = errors.New("invalid PKCE code verifier")
	// ErrMissingPKCE is returned when PKCE is required but not provided.
	ErrMissingPKCE = errors.New("PKCE code challenge required")
	// ErrInvalidState is returned when the state parameter is invalid.
	ErrInvalidState = errors.New("invalid state parameter")
)

// AuthorizationRequest represents an authorization request.
type AuthorizationRequest struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               []string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

// AuthorizationCode represents an authorization code.
type AuthorizationCode struct {
	Code                string
	ClientID            string
	UserID              string
	Username            string
	RedirectURI         string
	Scope               []string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
	ExpiresAt           time.Time
	Used                bool
}

// AuthorizationCodeStore manages authorization codes.
type AuthorizationCodeStore struct {
	mu    sync.RWMutex
	codes map[string]*AuthorizationCode
	ttl   time.Duration
}

// NewAuthorizationCodeStore creates a new authorization code store.
func NewAuthorizationCodeStore(ttl time.Duration) *AuthorizationCodeStore {
	store := &AuthorizationCodeStore{
		codes: make(map[string]*AuthorizationCode),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go store.cleanup()

	return store
}

// Generate generates a new authorization code.
func (s *AuthorizationCodeStore) Generate(req *AuthorizationRequest, userID, username string) (*AuthorizationCode, error) {
	// Generate random code
	codeBytes := make([]byte, 32)
	if _, err := rand.Read(codeBytes); err != nil {
		return nil, err
	}
	code := base64.RawURLEncoding.EncodeToString(codeBytes)

	authCode := &AuthorizationCode{
		Code:                code,
		ClientID:            req.ClientID,
		UserID:              userID,
		Username:            username,
		RedirectURI:         req.RedirectURI,
		Scope:               req.Scope,
		Nonce:               req.Nonce,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		ExpiresAt:           timeutil.NowTime().Add(s.ttl),
		Used:                false,
	}

	s.mu.Lock()
	s.codes[code] = authCode
	s.mu.Unlock()

	return authCode, nil
}

// Validate validates and consumes an authorization code.
func (s *AuthorizationCodeStore) Validate(code, clientID, redirectURI, codeVerifier string) (*AuthorizationCode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	authCode, ok := s.codes[code]
	if !ok {
		return nil, ErrInvalidAuthorizationCode
	}

	// Check if expired
	if timeutil.NowNano() > authCode.ExpiresAt.UnixNano() {
		delete(s.codes, code)
		return nil, ErrAuthorizationCodeExpired
	}

	// Check if already used
	if authCode.Used {
		// Security: delete all tokens for this code (replay attack)
		delete(s.codes, code)
		return nil, ErrAuthorizationCodeUsed
	}

	// Validate client ID
	if authCode.ClientID != clientID {
		return nil, ErrInvalidAuthorizationCode
	}

	// Validate redirect URI
	if authCode.RedirectURI != redirectURI {
		return nil, ErrInvalidAuthorizationCode
	}

	// Validate PKCE
	if authCode.CodeChallenge != "" {
		if codeVerifier == "" {
			return nil, ErrMissingPKCE
		}
		if !verifyPKCE(authCode.CodeChallenge, authCode.CodeChallengeMethod, codeVerifier) {
			return nil, ErrInvalidPKCE
		}
	}

	// Mark as used
	authCode.Used = true

	return authCode, nil
}

// verifyPKCE verifies the PKCE code verifier.
func verifyPKCE(challenge, method, verifier string) bool {
	switch method {
	case "S256":
		hash := sha256.Sum256([]byte(verifier))
		computed := base64.RawURLEncoding.EncodeToString(hash[:])
		return computed == challenge
	case "plain":
		return verifier == challenge
	default:
		return false
	}
}

// cleanup periodically removes expired codes.
func (s *AuthorizationCodeStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := timeutil.NowTime()
		for code, authCode := range s.codes {
			if now.After(authCode.ExpiresAt) {
				delete(s.codes, code)
			}
		}
		s.mu.Unlock()
	}
}

// AuthorizationSession represents a pending authorization session.
type AuthorizationSession struct {
	ID        string
	Request   *AuthorizationRequest
	CreatedAt time.Time
	ExpiresAt time.Time
}

// AuthorizationSessionStore manages authorization sessions.
type AuthorizationSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*AuthorizationSession
	ttl      time.Duration
}

// NewAuthorizationSessionStore creates a new authorization session store.
func NewAuthorizationSessionStore(ttl time.Duration) *AuthorizationSessionStore {
	store := &AuthorizationSessionStore{
		sessions: make(map[string]*AuthorizationSession),
		ttl:      ttl,
	}

	go store.cleanup()

	return store
}

// Create creates a new authorization session.
func (s *AuthorizationSessionStore) Create(req *AuthorizationRequest) *AuthorizationSession {
	now := timeutil.NowTime()
	session := &AuthorizationSession{
		ID:        uuid.New().String(),
		Request:   req,
		CreatedAt: now,
		ExpiresAt: now.Add(s.ttl),
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	return session
}

// Get retrieves an authorization session.
func (s *AuthorizationSessionStore) Get(id string) (*AuthorizationSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("session not found")
	}

	if timeutil.NowNano() > session.ExpiresAt.UnixNano() {
		return nil, errors.New("session expired")
	}

	return session, nil
}

// Delete deletes an authorization session.
func (s *AuthorizationSessionStore) Delete(id string) {
	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
}

// cleanup periodically removes expired sessions.
func (s *AuthorizationSessionStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := timeutil.NowTime()
		for id, session := range s.sessions {
			if now.After(session.ExpiresAt) {
				delete(s.sessions, id)
			}
		}
		s.mu.Unlock()
	}
}
