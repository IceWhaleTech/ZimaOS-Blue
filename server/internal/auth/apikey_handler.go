package auth

import (
	"net/http"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/labstack/echo/v4"
)

// APIKeyHandler handles HTTP requests for API key management.
type APIKeyHandler struct {
	service *APIKeyService
}

// NewAPIKeyHandler creates a new API key handler.
func NewAPIKeyHandler(service *APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{service: service}
}

// RegisterRoutes registers the API key routes.
func (h *APIKeyHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.ListKeys)
	g.POST("", h.CreateKey)
	g.DELETE("/:id", h.DeleteKey)
	g.POST("/:id/rotate", h.RotateKey)
}

// CreateKeyRequest represents a request to create an API key.
type CreateKeyRequestDTO struct {
	Name      string   `json:"name" validate:"required"`
	Scopes    []string `json:"scopes" validate:"required"`
	ExpiresIn string   `json:"expires_in,omitempty"` // e.g., "7d", "30d", "90d", "365d", ""
}

// CreateKeyResponse represents the response after creating an API key.
type CreateKeyResponseDTO struct {
	APIKey *APIKeyResponseDTO `json:"api_key"`
	Key    string             `json:"key"` // Full key, only shown once
}

// APIKeyResponseDTO represents an API key in responses.
type APIKeyResponseDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// ListKeys returns all API keys for the current user.
func (h *APIKeyHandler) ListKeys(c echo.Context) error {
	userID := getUserIDFromEchoContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	keys, err := h.service.ListKeys(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list API keys")
	}

	// Convert to response DTOs
	result := make([]*APIKeyResponseDTO, len(keys))
	for i, key := range keys {
		result[i] = &APIKeyResponseDTO{
			ID:         key.ID,
			Name:       key.Name,
			KeyPrefix:  key.Prefix,
			Scopes:     key.Scopes,
			CreatedAt:  key.CreatedAt,
			ExpiresAt:  key.ExpiresAt,
			LastUsedAt: key.LastUsed,
		}
	}

	return c.JSON(http.StatusOK, result)
}

// CreateKey creates a new API key.
func (h *APIKeyHandler) CreateKey(c echo.Context) error {
	userID := getUserIDFromEchoContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	var req CreateKeyRequestDTO
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	if len(req.Scopes) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "at least one scope is required")
	}

	// Parse expiration
	var expiresAt time.Time
	if req.ExpiresIn != "" {
		duration, err := parseDuration(req.ExpiresIn)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid expires_in format")
		}
		expiresAt = timeutil.NowTime().Add(duration)
	}

	key, err := h.service.CreateKey(c.Request().Context(), &CreateKeyRequest{
		UserID:    userID,
		Name:      req.Name,
		Scopes:    req.Scopes,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create API key")
	}

	return c.JSON(http.StatusCreated, &CreateKeyResponseDTO{
		APIKey: &APIKeyResponseDTO{
			ID:        key.ID,
			Name:      key.Name,
			KeyPrefix: key.Prefix,
			Scopes:    key.Scopes,
			CreatedAt: key.CreatedAt,
			ExpiresAt: key.ExpiresAt,
		},
		Key: key.Key,
	})
}

// DeleteKey deletes an API key.
func (h *APIKeyHandler) DeleteKey(c echo.Context) error {
	userID := getUserIDFromEchoContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	keyID := c.Param("id")
	if keyID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "key ID is required")
	}

	err := h.service.RevokeKey(c.Request().Context(), keyID, userID)
	if err != nil {
		if err == ErrUnauthorized {
			return echo.NewHTTPError(http.StatusNotFound, "API key not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete API key")
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// RotateKey rotates an API key.
func (h *APIKeyHandler) RotateKey(c echo.Context) error {
	userID := getUserIDFromEchoContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	keyID := c.Param("id")
	if keyID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "key ID is required")
	}

	result, err := h.service.RotateKey(c.Request().Context(), &RotateKeyRequest{
		KeyID:       keyID,
		UserID:      userID,
		GracePeriod: 24 * time.Hour,
	})
	if err != nil {
		switch err {
		case ErrAPIKeyNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "API key not found")
		case ErrAPIKeyRevoked:
			return echo.NewHTTPError(http.StatusBadRequest, "API key has been revoked")
		case ErrRotationPending:
			return echo.NewHTTPError(http.StatusConflict, "rotation already pending")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to rotate API key")
		}
	}

	return c.JSON(http.StatusOK, &CreateKeyResponseDTO{
		APIKey: &APIKeyResponseDTO{
			ID:        result.NewKey.ID,
			Name:      result.NewKey.Name,
			KeyPrefix: result.NewKey.Prefix,
			Scopes:    result.NewKey.Scopes,
			CreatedAt: result.NewKey.CreatedAt,
			ExpiresAt: result.NewKey.ExpiresAt,
		},
		Key: result.NewKey.Key,
	})
}

// getUserIDFromEchoContext extracts user ID from echo context.
func getUserIDFromEchoContext(c echo.Context) string {
	// Try to get from context value (set by auth middleware)
	if claims := GetUserFromContext(c); claims != nil {
		return claims.UserID
	}

	// Try to get from echo context directly
	if userID, ok := c.Get("user_id").(string); ok {
		return userID
	}

	return ""
}

// parseDuration parses a duration string like "7d", "30d", "90d", "365d".
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, ErrInvalidDuration
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	default:
		return 0, ErrInvalidDuration
	}

	var num int
	for _, c := range value {
		if c < '0' || c > '9' {
			return 0, ErrInvalidDuration
		}
		num = num*10 + int(c-'0')
	}

	return time.Duration(num) * multiplier, nil
}

// ErrInvalidDuration is returned when a duration string is invalid.
var ErrInvalidDuration = echo.NewHTTPError(http.StatusBadRequest, "invalid duration format")
