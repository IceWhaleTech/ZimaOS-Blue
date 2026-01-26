package auth

import (
	"testing"
	"time"
)

func TestJWTService_GenerateToken(t *testing.T) {
	cfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-echo",
	}
	svc := NewJWTService(cfg)

	t.Run("generate access token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		token, err := svc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}
		if token == "" {
			t.Fatal("token should not be empty")
		}
	})

	t.Run("generate refresh token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		token, err := svc.GenerateRefreshToken(claims)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}
		if token == "" {
			t.Fatal("refresh token should not be empty")
		}
	})
}

func TestJWTService_ValidateToken(t *testing.T) {
	cfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-echo",
	}
	svc := NewJWTService(cfg)

	t.Run("validate valid token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "admin",
		}

		token, err := svc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		validatedClaims, err := svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("failed to validate token: %v", err)
		}

		if validatedClaims.UserID != claims.UserID {
			t.Errorf("expected user_id %s, got %s", claims.UserID, validatedClaims.UserID)
		}
		if validatedClaims.Username != claims.Username {
			t.Errorf("expected username %s, got %s", claims.Username, validatedClaims.Username)
		}
		if validatedClaims.Role != claims.Role {
			t.Errorf("expected role %s, got %s", claims.Role, validatedClaims.Role)
		}
	})

	t.Run("validate invalid token", func(t *testing.T) {
		_, err := svc.ValidateToken("invalid-token")
		if err == nil {
			t.Fatal("expected error for invalid token")
		}
	})

	t.Run("validate token with wrong secret", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		token, err := svc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Create a new service with different secret
		wrongCfg := &JWTConfig{
			Secret:            "different-secret-key-at-least-32",
			Expiration:        time.Hour,
			RefreshExpiration: 24 * time.Hour,
			Issuer:            "zimaos-echo",
		}
		wrongSvc := NewJWTService(wrongCfg)

		_, err = wrongSvc.ValidateToken(token)
		if err == nil {
			t.Fatal("expected error for token with wrong secret")
		}
	})

	t.Run("validate expired token", func(t *testing.T) {
		expiredCfg := &JWTConfig{
			Secret:            "test-secret-key-at-least-32-chars",
			Expiration:        -time.Hour, // Already expired
			RefreshExpiration: 24 * time.Hour,
			Issuer:            "zimaos-echo",
		}
		expiredSvc := NewJWTService(expiredCfg)

		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		token, err := expiredSvc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		_, err = svc.ValidateToken(token)
		if err == nil {
			t.Fatal("expected error for expired token")
		}
	})
}

func TestJWTService_RefreshToken(t *testing.T) {
	cfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-echo",
	}
	svc := NewJWTService(cfg)

	t.Run("refresh valid token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		refreshToken, err := svc.GenerateRefreshToken(claims)
		if err != nil {
			t.Fatalf("failed to generate refresh token: %v", err)
		}

		newAccessToken, newRefreshToken, err := svc.RefreshTokens(refreshToken)
		if err != nil {
			t.Fatalf("failed to refresh token: %v", err)
		}

		if newAccessToken == "" {
			t.Fatal("new access token should not be empty")
		}
		if newRefreshToken == "" {
			t.Fatal("new refresh token should not be empty")
		}

		// Validate the new access token
		validatedClaims, err := svc.ValidateToken(newAccessToken)
		if err != nil {
			t.Fatalf("failed to validate new access token: %v", err)
		}

		if validatedClaims.UserID != claims.UserID {
			t.Errorf("expected user_id %s, got %s", claims.UserID, validatedClaims.UserID)
		}
	})

	t.Run("refresh with access token should fail", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		accessToken, err := svc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate access token: %v", err)
		}

		_, _, err = svc.RefreshTokens(accessToken)
		if err == nil {
			t.Fatal("expected error when refreshing with access token")
		}
	})
}

func TestJWTService_TokenBlacklist(t *testing.T) {
	cfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-echo",
	}
	svc := NewJWTService(cfg)

	t.Run("blacklist token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}

		token, err := svc.GenerateAccessToken(claims)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		// Token should be valid before blacklisting
		_, err = svc.ValidateToken(token)
		if err != nil {
			t.Fatalf("token should be valid before blacklisting: %v", err)
		}

		// Blacklist the token
		err = svc.RevokeToken(token)
		if err != nil {
			t.Fatalf("failed to revoke token: %v", err)
		}

		// Token should be invalid after blacklisting
		_, err = svc.ValidateToken(token)
		if err == nil {
			t.Fatal("expected error for blacklisted token")
		}
	})
}
