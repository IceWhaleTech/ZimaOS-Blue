package auth

import (
	"errors"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken     = errors.New("invalid token")
	ErrExpiredToken     = errors.New("token has expired")
	ErrRevokedToken     = errors.New("token has been revoked")
	ErrInvalidTokenType = errors.New("invalid token type")
)

// TokenType represents the type of JWT token
type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret            string        `mapstructure:"secret"`
	Expiration        time.Duration `mapstructure:"expiration"`
	RefreshExpiration time.Duration `mapstructure:"refresh_expiration"`
	Issuer            string        `mapstructure:"issuer"`
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

// NewJWTService creates a new JWT service
func NewJWTService(cfg *JWTConfig) *JWTService {
	return &JWTService{
		config:    cfg,
		blacklist: make(map[string]time.Time),
	}
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
	now := time.Now()
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
	now := time.Now()
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
