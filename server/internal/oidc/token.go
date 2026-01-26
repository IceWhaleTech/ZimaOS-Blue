package oidc

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
)

var (
	// ErrInvalidToken is returned when a token is invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrTokenExpired is returned when a token has expired.
	ErrTokenExpired = errors.New("token expired")
	// ErrTokenRevoked is returned when a token has been revoked.
	ErrTokenRevoked = errors.New("token revoked")
	// ErrInvalidRefreshToken is returned when a refresh token is invalid.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// TokenClaims represents the claims in an access token.
type TokenClaims struct {
	jwt.Claims
	Scope    string `json:"scope,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// IDTokenClaims represents the claims in an ID token.
type IDTokenClaims struct {
	jwt.Claims
	Nonce             string `json:"nonce,omitempty"`
	AuthTime          int64  `json:"auth_time,omitempty"`
	Name              string `json:"name,omitempty"`
	Email             string `json:"email,omitempty"`
	EmailVerified     bool   `json:"email_verified,omitempty"`
	PreferredUsername string `json:"preferred_username,omitempty"`
}

// TokenResponse represents the token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// RefreshToken represents a stored refresh token.
type RefreshToken struct {
	Token     string
	UserID    string
	Username  string
	ClientID  string
	Scope     []string
	ExpiresAt time.Time
	Revoked   bool
}

// RefreshTokenStore manages refresh tokens.
type RefreshTokenStore struct {
	mu     sync.RWMutex
	tokens map[string]*RefreshToken
}

// NewRefreshTokenStore creates a new refresh token store.
func NewRefreshTokenStore() *RefreshTokenStore {
	store := &RefreshTokenStore{
		tokens: make(map[string]*RefreshToken),
	}

	go store.cleanup()

	return store
}

// Generate generates a new refresh token.
func (s *RefreshTokenStore) Generate(userID, username, clientID string, scope []string, ttl time.Duration) (*RefreshToken, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)

	rt := &RefreshToken{
		Token:     token,
		UserID:    userID,
		Username:  username,
		ClientID:  clientID,
		Scope:     scope,
		ExpiresAt: time.Now().Add(ttl),
		Revoked:   false,
	}

	s.mu.Lock()
	s.tokens[token] = rt
	s.mu.Unlock()

	return rt, nil
}

// Validate validates a refresh token.
func (s *RefreshTokenStore) Validate(token, clientID string) (*RefreshToken, error) {
	s.mu.RLock()
	rt, ok := s.tokens[token]
	s.mu.RUnlock()

	if !ok {
		return nil, ErrInvalidRefreshToken
	}

	if rt.Revoked {
		return nil, ErrTokenRevoked
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, ErrTokenExpired
	}

	if rt.ClientID != clientID {
		return nil, ErrInvalidRefreshToken
	}

	return rt, nil
}

// Revoke revokes a refresh token.
func (s *RefreshTokenStore) Revoke(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt, ok := s.tokens[token]
	if !ok {
		return ErrInvalidRefreshToken
	}

	rt.Revoked = true
	return nil
}

// RevokeByUser revokes all refresh tokens for a user.
func (s *RefreshTokenStore) RevokeByUser(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rt := range s.tokens {
		if rt.UserID == userID {
			rt.Revoked = true
		}
	}
}

// cleanup periodically removes expired tokens.
func (s *RefreshTokenStore) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for token, rt := range s.tokens {
			if now.After(rt.ExpiresAt) || rt.Revoked {
				delete(s.tokens, token)
			}
		}
		s.mu.Unlock()
	}
}

// TokenService handles token generation and validation.
type TokenService struct {
	keyManager        *KeyManager
	refreshTokenStore *RefreshTokenStore
	issuer            string
	accessTokenTTL    time.Duration
	refreshTokenTTL   time.Duration
	idTokenTTL        time.Duration
}

// NewTokenService creates a new token service.
func NewTokenService(keyManager *KeyManager, issuer string, accessTTL, refreshTTL, idTTL time.Duration) *TokenService {
	return &TokenService{
		keyManager:        keyManager,
		refreshTokenStore: NewRefreshTokenStore(),
		issuer:            issuer,
		accessTokenTTL:    accessTTL,
		refreshTokenTTL:   refreshTTL,
		idTokenTTL:        idTTL,
	}
}

// GenerateTokens generates access, refresh, and ID tokens.
func (s *TokenService) GenerateTokens(authCode *AuthorizationCode) (*TokenResponse, error) {
	now := time.Now()

	// Generate access token
	accessToken, err := s.generateAccessToken(authCode.UserID, authCode.ClientID, authCode.Scope, now)
	if err != nil {
		return nil, err
	}

	// Generate refresh token if offline_access scope is requested
	var refreshToken string
	for _, scope := range authCode.Scope {
		if scope == ScopeOffline {
			rt, err := s.refreshTokenStore.Generate(
				authCode.UserID,
				authCode.Username,
				authCode.ClientID,
				authCode.Scope,
				s.refreshTokenTTL,
			)
			if err != nil {
				return nil, err
			}
			refreshToken = rt.Token
			break
		}
	}

	// Generate ID token if openid scope is requested
	var idToken string
	for _, scope := range authCode.Scope {
		if scope == ScopeOpenID {
			idToken, err = s.generateIDToken(authCode.UserID, authCode.Username, authCode.ClientID, authCode.Nonce, now)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
		RefreshToken: refreshToken,
		IDToken:      idToken,
		Scope:        scopesToString(authCode.Scope),
	}, nil
}

// RefreshTokens refreshes tokens using a refresh token.
func (s *TokenService) RefreshTokens(refreshToken, clientID string) (*TokenResponse, error) {
	rt, err := s.refreshTokenStore.Validate(refreshToken, clientID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Generate new access token
	accessToken, err := s.generateAccessToken(rt.UserID, rt.ClientID, rt.Scope, now)
	if err != nil {
		return nil, err
	}

	// Optionally rotate refresh token
	newRT, err := s.refreshTokenStore.Generate(rt.UserID, rt.Username, rt.ClientID, rt.Scope, s.refreshTokenTTL)
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	_ = s.refreshTokenStore.Revoke(refreshToken)

	// Generate new ID token if openid scope
	var idToken string
	for _, scope := range rt.Scope {
		if scope == ScopeOpenID {
			idToken, err = s.generateIDToken(rt.UserID, rt.Username, rt.ClientID, "", now)
			if err != nil {
				return nil, err
			}
			break
		}
	}

	return &TokenResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    int64(s.accessTokenTTL.Seconds()),
		RefreshToken: newRT.Token,
		IDToken:      idToken,
		Scope:        scopesToString(rt.Scope),
	}, nil
}

// generateAccessToken generates a JWT access token.
func (s *TokenService) generateAccessToken(userID, clientID string, scope []string, now time.Time) (string, error) {
	signer, err := s.keyManager.Signer()
	if err != nil {
		return "", err
	}

	claims := TokenClaims{
		Claims: jwt.Claims{
			ID:       uuid.New().String(),
			Issuer:   s.issuer,
			Subject:  userID,
			Audience: jwt.Audience{clientID},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(s.accessTokenTTL)),
		},
		Scope:    scopesToString(scope),
		ClientID: clientID,
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return "", err
	}

	return token, nil
}

// generateIDToken generates a JWT ID token.
func (s *TokenService) generateIDToken(userID, username, clientID, nonce string, now time.Time) (string, error) {
	signer, err := s.keyManager.Signer()
	if err != nil {
		return "", err
	}

	claims := IDTokenClaims{
		Claims: jwt.Claims{
			Issuer:   s.issuer,
			Subject:  userID,
			Audience: jwt.Audience{clientID},
			IssuedAt: jwt.NewNumericDate(now),
			Expiry:   jwt.NewNumericDate(now.Add(s.idTokenTTL)),
		},
		Nonce:             nonce,
		AuthTime:          now.Unix(),
		PreferredUsername: username,
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	if err != nil {
		return "", err
	}

	return token, nil
}

// ValidateAccessToken validates an access token.
func (s *TokenService) ValidateAccessToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseSigned(tokenString, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, ErrInvalidToken
	}

	jwks := s.keyManager.GetJWKS()

	var claims TokenClaims
	if err := token.Claims(jwks, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	// Validate expiry
	if claims.Expiry != nil && time.Now().After(claims.Expiry.Time()) {
		return nil, ErrTokenExpired
	}

	// Validate issuer
	if claims.Issuer != s.issuer {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

// RevokeToken revokes a token.
func (s *TokenService) RevokeToken(token string) error {
	// Try to revoke as refresh token
	return s.refreshTokenStore.Revoke(token)
}

// RevokeUserTokens revokes all tokens for a user.
func (s *TokenService) RevokeUserTokens(userID string) {
	s.refreshTokenStore.RevokeByUser(userID)
}

// IntrospectionResponse represents the token introspection response.
type IntrospectionResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	TokenType string `json:"token_type,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
}

// IntrospectToken introspects a token.
func (s *TokenService) IntrospectToken(tokenString string) *IntrospectionResponse {
	claims, err := s.ValidateAccessToken(tokenString)
	if err != nil {
		return &IntrospectionResponse{Active: false}
	}

	var aud string
	if len(claims.Audience) > 0 {
		aud = claims.Audience[0]
	}

	return &IntrospectionResponse{
		Active:    true,
		Scope:     claims.Scope,
		ClientID:  claims.ClientID,
		TokenType: "Bearer",
		Exp:       claims.Expiry.Time().Unix(),
		Iat:       claims.IssuedAt.Time().Unix(),
		Sub:       claims.Subject,
		Aud:       aud,
		Iss:       claims.Issuer,
	}
}

// Helper functions

func scopesToString(scopes []string) string {
	result := ""
	for i, scope := range scopes {
		if i > 0 {
			result += " "
		}
		result += scope
	}
	return result
}

// TokenError represents an OAuth error response.
type TokenError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// MarshalJSON implements json.Marshaler.
func (e *TokenError) MarshalJSON() ([]byte, error) {
	type Alias TokenError
	return json.Marshal((*Alias)(e))
}
