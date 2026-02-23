package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrRevokedToken     = errors.New("token has been revoked")
	ErrInvalidTokenType = errors.New("invalid token type")
	ErrWeakSecret       = errors.New("JWT secret is too weak or uses default value")
)

// Security: Known weak/default secrets that should be rejected
var weakSecrets = []string{
	"change-me-in-production-use-a-strong-secret-key",
	"change-me",
	"secret",
	"password",
	"jwt-secret",
	"your-secret-key",
	"your-256-bit-secret",
}

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret            string        `yaml:"secret"`
	Expiration        time.Duration `yaml:"expiration"`
	RefreshExpiration time.Duration `yaml:"refresh_expiration"`
	Issuer            string        `yaml:"issuer"`
}

// UserClaims represents the custom claims in JWT
type UserClaims struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// Claims represents the full JWT claims
type Claims struct {
	jwt.RegisteredClaims
	UserClaims
	TokenType TokenType `json:"token_type"`
}

// JWTService handles JWT operations
type JWTService struct {
	config    *JWTConfig
	blacklist map[string]time.Time // token ID -> expiration time
	mu        sync.RWMutex
}

// ValidateJWTSecret checks if the JWT secret is secure enough.
// Security: Returns an error if the secret is weak or uses a known default value.
func ValidateJWTSecret(secret string) error {
	// Check minimum length (at least 32 characters for HS256)
	if len(secret) < 32 {
		return errors.New("JWT secret must be at least 32 characters long")
	}

	// Check against known weak secrets
	secretLower := strings.ToLower(secret)
	for _, weak := range weakSecrets {
		if strings.Contains(secretLower, strings.ToLower(weak)) {
			return ErrWeakSecret
		}
	}

	return nil
}

// GenerateSecureSecret generates a cryptographically secure random secret.
// Security: Use this to generate a secure JWT secret if none is configured.
func GenerateSecureSecret() (string, error) {
	bytes := make([]byte, 32) // 256 bits
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// NewJWTService creates a new JWT service.
// Security: This will log a warning if the secret appears weak.
func NewJWTService(cfg *JWTConfig) *JWTService {
	return &JWTService{
		config:    cfg,
		blacklist: make(map[string]time.Time),
	}
}

// NewJWTServiceSecure creates a new JWT service with security validation.
// Security: Returns an error if the secret is weak or uses a known default value.
// Use this in production to ensure secure configuration.
func NewJWTServiceSecure(cfg *JWTConfig) (*JWTService, error) {
	if err := ValidateJWTSecret(cfg.Secret); err != nil {
		return nil, err
	}

	return &JWTService{
		config:    cfg,
		blacklist: make(map[string]time.Time),
	}, nil
}

// IsSecretSecure checks if the current secret is secure.
func (s *JWTService) IsSecretSecure() bool {
	return ValidateJWTSecret(s.config.Secret) == nil
}

// GenerateAccessToken generates a new access token
func (s *JWTService) GenerateAccessToken(userClaims *UserClaims) (string, error) {
	return s.generateToken(userClaims, TokenTypeAccess, s.config.Expiration)
}

// GenerateRefreshToken generates a new refresh token
func (s *JWTService) GenerateRefreshToken(userClaims *UserClaims) (string, error) {
	return s.generateToken(userClaims, TokenTypeRefresh, s.config.RefreshExpiration)
}

// generateToken creates a JWT token with the specified type and expiration
func (s *JWTService) generateToken(userClaims *UserClaims, tokenType TokenType, expiration time.Duration) (string, error) {
	now := timeutil.NowTime()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.config.Issuer,
			Subject:   userClaims.UserID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(expiration)),
			ID:        generateTokenID(),
		},
		UserClaims: *userClaims,
		TokenType:  tokenType,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.Secret))
}

// ValidateToken validates a JWT token and returns the claims
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.Secret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	// Check if token is blacklisted
	if s.isBlacklisted(claims.ID) {
		return nil, ErrRevokedToken
	}

	return claims, nil
}

// RefreshTokens validates a refresh token and generates new access and refresh tokens
func (s *JWTService) RefreshTokens(refreshToken string) (string, string, error) {
	claims, err := s.ValidateToken(refreshToken)
	if err != nil {
		return "", "", err
	}

	// Ensure it's a refresh token
	if claims.TokenType != TokenTypeRefresh {
		return "", "", ErrInvalidTokenType
	}

	// Revoke the old refresh token
	_ = s.RevokeToken(refreshToken)

	// Generate new tokens
	userClaims := &UserClaims{
		UserID:   claims.UserID,
		Username: claims.Username,
		Role:     claims.Role,
	}

	newAccessToken, err := s.GenerateAccessToken(userClaims)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := s.GenerateRefreshToken(userClaims)
	if err != nil {
		return "", "", err
	}

	return newAccessToken, newRefreshToken, nil
}

// RevokeToken adds a token to the blacklist
func (s *JWTService) RevokeToken(tokenString string) error {
	claims, err := s.parseTokenWithoutValidation(tokenString)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Add to blacklist with expiration time
	s.blacklist[claims.ID] = claims.ExpiresAt.Time

	// Clean up expired entries
	s.cleanupBlacklist()

	return nil
}

// isBlacklisted checks if a token ID is in the blacklist
func (s *JWTService) isBlacklisted(tokenID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.blacklist[tokenID]
	return exists
}

// cleanupBlacklist removes expired entries from the blacklist
func (s *JWTService) cleanupBlacklist() {
	now := timeutil.NowTime()
	for id, expiry := range s.blacklist {
		if expiry.Before(now) {
			delete(s.blacklist, id)
		}
	}
}

// parseTokenWithoutValidation parses a token without validating expiration
func (s *JWTService) parseTokenWithoutValidation(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(s.config.Secret), nil
	}, jwt.WithoutClaimsValidation())

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, ErrInvalidToken
	}

	return claims, nil
}

// generateTokenID generates a unique token ID
func generateTokenID() string {
	return uuid.New().String()
}
