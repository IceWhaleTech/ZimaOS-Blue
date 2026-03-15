package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// UserContextKey is the key for storing user claims in context
	UserContextKey ContextKey = "user"
	// APIKeyContextKey is the key for storing API key info in context
	APIKeyContextKey ContextKey = "apikey"
)

// AuthMiddleware handles authentication for HTTP requests
type AuthMiddleware struct {
	jwtService    *JWTService
	apiKeyService *APIKeyService
	previewMode   interface {
		IsPreviewMode(context.Context) (bool, error)
	}
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(jwtSvc *JWTService, apiKeySvc *APIKeyService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService:    jwtSvc,
		apiKeyService: apiKeySvc,
	}
}

// SetPreviewModeChecker configures preview mode verification for preview JWTs.
// When set, tokens for the synthetic preview user are rejected once preview mode is off.
func (m *AuthMiddleware) SetPreviewModeChecker(checker interface {
	IsPreviewMode(context.Context) (bool, error)
}) {
	m.previewMode = checker
}

// Authenticate returns a middleware that requires authentication
func (m *AuthMiddleware) Authenticate() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try JWT authentication first
			if m.jwtService != nil {
				if claims, ok := m.authenticateJWT(c); ok {
					setUserInContext(c, claims)
					return next(c)
				}
			}

			// Try API key authentication
			if m.apiKeyService != nil {
				if claims, apiKeyInfo, ok := m.authenticateAPIKey(c); ok {
					setUserInContext(c, claims)
					setAPIKeyInContext(c, apiKeyInfo)
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
		}
	}
}

// OptionalAuthenticate returns a middleware that optionally authenticates
func (m *AuthMiddleware) OptionalAuthenticate() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Try JWT authentication first
			if m.jwtService != nil {
				if claims, ok := m.authenticateJWT(c); ok {
					setUserInContext(c, claims)
					return next(c)
				}
			}

			// Try API key authentication
			if m.apiKeyService != nil {
				if claims, apiKeyInfo, ok := m.authenticateAPIKey(c); ok {
					setUserInContext(c, claims)
					setAPIKeyInContext(c, apiKeyInfo)
					return next(c)
				}
			}

			// Continue without authentication
			return next(c)
		}
	}
}

// authenticateJWT attempts to authenticate using JWT
func (m *AuthMiddleware) authenticateJWT(c echo.Context) (*UserClaims, bool) {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader == "" {
		// Fallback: check query param for SSE/EventSource (no custom headers)
		if qToken := c.QueryParam("token"); qToken != "" {
			claims, err := m.jwtService.ValidateToken(qToken)
			if err != nil {
				return nil, false
			}
			return &claims.UserClaims, true
		}
		return nil, false
	}

	// Check for Bearer token
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, false
	}

	token := parts[1]
	claims, err := m.jwtService.ValidateToken(token)
	if err != nil {
		return nil, false
	}
	if claims.UserClaims.UserID == "preview-user" && m.previewMode != nil {
		isPreview, err := m.previewMode.IsPreviewMode(c.Request().Context())
		if err != nil || !isPreview {
			return nil, false
		}
	}

	return &claims.UserClaims, true
}

// authenticateAPIKey attempts to authenticate using API key
func (m *AuthMiddleware) authenticateAPIKey(c echo.Context) (*UserClaims, *APIKeyInfo, bool) {
	// Check header first
	apiKey := c.Request().Header.Get("X-API-Key")
	if apiKey == "" {
		// Check query parameter
		apiKey = c.QueryParam("api_key")
	}

	if apiKey == "" {
		return nil, nil, false
	}

	info, err := m.apiKeyService.ValidateKey(c.Request().Context(), apiKey)
	if err != nil {
		return nil, nil, false
	}

	// Convert API key info to user claims
	claims := &UserClaims{
		UserID:   info.UserID,
		Username: "", // API keys don't have username
		Role:     "", // Role will be determined by RBAC
	}

	return claims, info, true
}

// setUserInContext stores user claims in the echo context
func setUserInContext(c echo.Context, claims *UserClaims) {
	ctx := context.WithValue(c.Request().Context(), UserContextKey, claims)
	c.SetRequest(c.Request().WithContext(ctx))

	// Also set in echo context for handlers that use c.Get()
	c.Set("user_id", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("role", claims.Role)
}

// setAPIKeyInContext stores API key info in the echo context
func setAPIKeyInContext(c echo.Context, info *APIKeyInfo) {
	ctx := context.WithValue(c.Request().Context(), APIKeyContextKey, info)
	c.SetRequest(c.Request().WithContext(ctx))
}

// GetUserFromContext retrieves user claims from the echo context
func GetUserFromContext(c echo.Context) *UserClaims {
	claims, ok := c.Request().Context().Value(UserContextKey).(*UserClaims)
	if !ok {
		return nil
	}
	return claims
}

// GetAPIKeyFromContext retrieves API key info from the echo context
func GetAPIKeyFromContext(c echo.Context) *APIKeyInfo {
	info, ok := c.Request().Context().Value(APIKeyContextKey).(*APIKeyInfo)
	if !ok {
		return nil
	}
	return info
}

// RequireScope returns a middleware that requires a specific scope
func (m *AuthMiddleware) RequireScope(scope string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Check API key scope
			apiKeyInfo := GetAPIKeyFromContext(c)
			if apiKeyInfo != nil {
				if !apiKeyInfo.HasScope(scope) {
					return echo.NewHTTPError(http.StatusForbidden, "insufficient scope")
				}
			}

			return next(c)
		}
	}
}
