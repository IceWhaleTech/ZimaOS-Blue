package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestAuthMiddleware_JWT(t *testing.T) {
	jwtCfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-blue",
	}
	jwtSvc := NewJWTService(jwtCfg)

	middleware := NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	e.Use(middleware.Authenticate())
	e.GET("/protected", func(c echo.Context) error {
		claims := GetUserFromContext(c)
		if claims == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "no user"})
		}
		return c.JSON(http.StatusOK, map[string]string{"user_id": claims.UserID})
	})

	t.Run("valid JWT token", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}
		token, _ := jwtSvc.GenerateAccessToken(claims)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})

	t.Run("expired token", func(t *testing.T) {
		expiredCfg := &JWTConfig{
			Secret:            "test-secret-key-at-least-32-chars",
			Expiration:        -time.Hour,
			RefreshExpiration: 24 * time.Hour,
			Issuer:            "zimaos-blue",
		}
		expiredSvc := NewJWTService(expiredCfg)
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}
		token, _ := expiredSvc.GenerateAccessToken(claims)

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestAuthMiddleware_APIKey(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	apiKeySvc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create api key service: %v", err)
	}
	defer func() {
		// Wait for async operations to complete before closing
		time.Sleep(200 * time.Millisecond)
		apiKeySvc.Close()
	}()

	middleware := NewAuthMiddleware(nil, apiKeySvc)

	e := echo.New()
	e.Use(middleware.Authenticate())
	e.GET("/protected", func(c echo.Context) error {
		claims := GetUserFromContext(c)
		if claims == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "no user"})
		}
		return c.JSON(http.StatusOK, map[string]string{"user_id": claims.UserID})
	})

	t.Run("valid API key in header", func(t *testing.T) {
		key, err := apiKeySvc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-123",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("X-API-Key", key.Key)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		// Wait for async last_used update
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("valid API key in query param", func(t *testing.T) {
		key, err := apiKeySvc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "user-456",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected?api_key="+key.Key, nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("invalid API key", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("X-API-Key", "invalid-key")
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rec.Code)
		}
	})
}

func TestAuthMiddleware_Combined(t *testing.T) {
	jwtCfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-blue",
	}
	jwtSvc := NewJWTService(jwtCfg)

	dbPath := filepath.Join(t.TempDir(), "apikeys_test.db")
	apiKeySvc, err := NewAPIKeyService(dbPath)
	if err != nil {
		t.Fatalf("failed to create api key service: %v", err)
	}
	defer func() {
		// Wait for async operations to complete before closing
		time.Sleep(200 * time.Millisecond)
		apiKeySvc.Close()
	}()

	middleware := NewAuthMiddleware(jwtSvc, apiKeySvc)

	e := echo.New()
	e.Use(middleware.Authenticate())
	e.GET("/protected", func(c echo.Context) error {
		claims := GetUserFromContext(c)
		if claims == nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "no user"})
		}
		return c.JSON(http.StatusOK, map[string]string{"user_id": claims.UserID})
	})

	t.Run("JWT takes precedence over API key", func(t *testing.T) {
		jwtClaims := &UserClaims{
			UserID:   "jwt-user",
			Username: "jwtuser",
			Role:     "admin",
		}
		token, _ := jwtSvc.GenerateAccessToken(jwtClaims)

		apiKey, err := apiKeySvc.CreateKey(context.Background(), &CreateKeyRequest{
			UserID: "apikey-user",
			Name:   "test-key",
			Scopes: []string{"read"},
		})
		if err != nil {
			t.Fatalf("failed to create key: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-API-Key", apiKey.Key)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
		// JWT user should be used
		if rec.Body.String() != `{"user_id":"jwt-user"}`+"\n" {
			t.Errorf("expected jwt-user, got %s", rec.Body.String())
		}
	})
}

func TestAuthMiddleware_Optional(t *testing.T) {
	jwtCfg := &JWTConfig{
		Secret:            "test-secret-key-at-least-32-chars",
		Expiration:        time.Hour,
		RefreshExpiration: 24 * time.Hour,
		Issuer:            "zimaos-blue",
	}
	jwtSvc := NewJWTService(jwtCfg)

	middleware := NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	e.Use(middleware.OptionalAuthenticate())
	e.GET("/public", func(c echo.Context) error {
		claims := GetUserFromContext(c)
		if claims == nil {
			return c.JSON(http.StatusOK, map[string]string{"user": "anonymous"})
		}
		return c.JSON(http.StatusOK, map[string]string{"user_id": claims.UserID})
	})

	t.Run("no token - anonymous access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})

	t.Run("valid token - authenticated access", func(t *testing.T) {
		claims := &UserClaims{
			UserID:   "user-123",
			Username: "testuser",
			Role:     "user",
		}
		token, _ := jwtSvc.GenerateAccessToken(claims)

		req := httptest.NewRequest(http.MethodGet, "/public", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rec.Code)
		}
	})
}
