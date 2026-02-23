package user

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

// PermissionService interface for permission operations
type PermissionService interface {
	GetEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]string, error)
	SetUserPermissions(ctx context.Context, userID uuid.UUID, permissions []string, grantedBy *string) error
	InitializeUserPermissions(ctx context.Context, userID uuid.UUID, role string, grantedBy *string) error
	DeleteUserPermissions(ctx context.Context, userID uuid.UUID) error
}

// Handler handles HTTP requests for user management.
type Handler struct {
	service           *Service
	jwtService        *auth.JWTService
	permissionService PermissionService
	modeService       interface{ IsPreviewMode(context.Context) (bool, error) }
}

// NewHandler creates a new user handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SetJWTService sets the JWT service for token generation.
func (h *Handler) SetJWTService(jwtService *auth.JWTService) {
	h.jwtService = jwtService
}

// SetPermissionService sets the permission service.
func (h *Handler) SetPermissionService(permissionService PermissionService) {
	h.permissionService = permissionService
}

// SetModeService sets the mode service for preview mode detection.
func (h *Handler) SetModeService(modeService interface{ IsPreviewMode(context.Context) (bool, error) }) {
	h.modeService = modeService
}

// RegisterRoutes registers the user routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Public routes
	g.POST("/auth/login", h.Login)
	g.POST("/auth/logout", h.Logout)
	g.POST("/auth/refresh", h.RefreshToken)
	g.GET("/auth/password-policy", h.GetPasswordPolicy)

	// Protected routes (require authentication)
	users := g.Group("/users")
	users.POST("", h.CreateUser)
	users.GET("", h.ListUsers)
	users.GET("/me", h.GetCurrentUser)
	users.PUT("/me", h.UpdateCurrentUser)
	users.GET("/:id", h.GetUser)
	users.PUT("/:id", h.UpdateUser)
	users.DELETE("/:id", h.DeleteUser)
	users.POST("/:id/lock", h.LockUser)
	users.POST("/:id/unlock", h.UnlockUser)
	users.POST("/:id/reset-password", h.ResetPassword)

	// Password routes
	g.POST("/auth/password", h.ChangePassword)
}

// LoginRequest represents a login request.
type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents a login response.
type LoginResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	User         *User  `json:"user"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
	MFAToken     string `json:"mfa_token,omitempty"`
}

// GetPasswordPolicy returns the password policy configuration.
// GET /api/v1/auth/password-policy
func (h *Handler) GetPasswordPolicy(c echo.Context) error {
	cfg := h.service.GetPasswordPolicy()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"min_length":        cfg.MinLength,
		"require_uppercase": cfg.RequireUppercase,
		"require_lowercase": cfg.RequireLowercase,
		"require_letter":    cfg.RequireLetter,
		"require_number":    cfg.RequireNumber,
		"require_special":   cfg.RequireSpecial,
	})
}

// Login handles user login.
func (h *Handler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := h.service.Authenticate(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		switch err {
		case ErrInvalidCredentials:
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
		case ErrAccountLocked:
			return echo.NewHTTPError(http.StatusForbidden, "account is locked")
		case ErrAccountDisabled:
			return echo.NewHTTPError(http.StatusForbidden, "account is disabled")
		case ErrMFARequired:
			// Generate MFA challenge token
			mfaToken := generateToken(32)
			return c.JSON(http.StatusOK, &LoginResponse{
				MFARequired: true,
				MFAToken:    mfaToken,
				User:        user,
			})
		default:
			// Log the actual error for debugging
			c.Logger().Errorf("authentication error for user %s: %v", req.Username, err)
			return echo.NewHTTPError(http.StatusInternalServerError, "authentication service temporarily unavailable")
		}
	}

	var accessToken, refreshToken string

	// Use JWT service if available, otherwise fall back to random tokens
	if h.jwtService != nil {
		userClaims := &auth.UserClaims{
			UserID:   user.ID.String(),
			Username: user.Username,
			Role:     string(user.Role),
		}

		accessToken, err = h.jwtService.GenerateAccessToken(userClaims)
		if err != nil {
			c.Logger().Errorf("failed to generate access token for user %s: %v", user.Username, err)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate access token")
		}

		refreshToken, err = h.jwtService.GenerateRefreshToken(userClaims)
		if err != nil {
			c.Logger().Errorf("failed to generate refresh token for user %s: %v", user.Username, err)
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate refresh token")
		}
	} else {
		// Fallback to random tokens
		accessToken = generateToken(32)
		refreshToken = generateToken(64)
	}

	// Create session
	userAgent := c.Request().UserAgent()
	ipAddress := c.RealIP()
	session, err := h.service.CreateSession(c.Request().Context(), user.ID, refreshToken, userAgent, ipAddress)
	if err != nil {
		c.Logger().Errorf("failed to create session for user %s: %v", user.Username, err)
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	return c.JSON(http.StatusOK, &LoginResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(session.ExpiresAt.Sub(session.CreatedAt).Seconds()),
		ExpiresAt:    session.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User:         user,
	})
}

// RefreshToken handles token refresh.
func (h *Handler) RefreshToken(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.Bind(&req); err != nil || req.RefreshToken == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "refresh_token is required")
	}

	// Validate session
	session, err := h.service.GetSession(c.Request().Context(), req.RefreshToken)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired refresh token")
	}

	if h.jwtService == nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "jwt service not available")
	}

	// Revoke old session
	_ = h.service.RevokeSession(c.Request().Context(), session.ID)

	// Look up user for claims
	user, err := h.service.GetByID(c.Request().Context(), session.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "user not found")
	}

	userClaims := &auth.UserClaims{
		UserID:   user.ID.String(),
		Username: user.Username,
		Role:     string(user.Role),
	}

	accessToken, err := h.jwtService.GenerateAccessToken(userClaims)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate access token")
	}

	newRefreshToken, err := h.jwtService.GenerateRefreshToken(userClaims)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate refresh token")
	}

	// Create new session
	userAgent := c.Request().UserAgent()
	ipAddress := c.RealIP()
	newSession, err := h.service.CreateSession(c.Request().Context(), user.ID, newRefreshToken, userAgent, ipAddress)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	return c.JSON(http.StatusOK, &LoginResponse{
		Token:        accessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(newSession.ExpiresAt.Sub(newSession.CreatedAt).Seconds()),
		ExpiresAt:    newSession.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		User:         user,
	})
}

// Logout handles user logout.
func (h *Handler) Logout(c echo.Context) error {
	// Get refresh token from request
	refreshToken := c.Request().Header.Get("X-Refresh-Token")
	if refreshToken == "" {
		return c.NoContent(http.StatusNoContent)
	}

	// Revoke session
	session, err := h.service.GetSession(c.Request().Context(), refreshToken)
	if err == nil {
		_ = h.service.RevokeSession(c.Request().Context(), session.ID)
	}

	return c.NoContent(http.StatusNoContent)
}

// CreateUser handles user creation.
func (h *Handler) CreateUser(c echo.Context) error {
	var req CreateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := h.service.Create(c.Request().Context(), &req)
	if err != nil {
		switch err {
		case ErrUsernameExists:
			return echo.NewHTTPError(http.StatusConflict, "username already exists")
		case ErrEmailExists:
			return echo.NewHTTPError(http.StatusConflict, "email already exists")
		case ErrInvalidPassword:
			return echo.NewHTTPError(http.StatusBadRequest, "password does not meet requirements")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create user")
		}
	}

	return c.JSON(http.StatusCreated, user)
}

// GetUser handles getting a user by ID.
func (h *Handler) GetUser(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	user, err := h.service.GetByID(c.Request().Context(), id)
	if err != nil {
		if err == ErrUserNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	return c.JSON(http.StatusOK, user)
}

// PreviewUserID is the special user ID for preview mode.
const PreviewUserID = "preview-user"

// GetCurrentUser handles getting the current user.
func (h *Handler) GetCurrentUser(c echo.Context) error {
	// Check if this is a preview user first
	claims := auth.GetUserFromContext(c)
	if claims != nil && claims.UserID == PreviewUserID {
		// Return a synthetic preview user with admin role
		// In preview mode, users should have full admin access
		return c.JSON(http.StatusOK, &User{
			ID:       uuid.Nil,
			Username: claims.Username,
			Role:     RoleAdmin,
			Status:   StatusActive,
		})
	}

	// In preview mode without token, return preview user
	if h.modeService != nil {
		isPreview, err := h.modeService.IsPreviewMode(c.Request().Context())
		if err == nil && isPreview {
			return c.JSON(http.StatusOK, &User{
				ID:       uuid.Nil,
				Username: "preview",
				Role:     RoleAdmin,
				Status:   StatusActive,
			})
		}
	}

	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	user, err := h.service.GetByID(c.Request().Context(), userID)
	if err != nil {
		if err == ErrUserNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get user")
	}

	return c.JSON(http.StatusOK, user)
}

// UpdateUser handles updating a user.
func (h *Handler) UpdateUser(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := h.service.Update(c.Request().Context(), id, &req)
	if err != nil {
		switch err {
		case ErrUserNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		case ErrEmailExists:
			return echo.NewHTTPError(http.StatusConflict, "email already exists")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update user")
		}
	}

	return c.JSON(http.StatusOK, user)
}

// UpdateCurrentUser handles updating the current user.
func (h *Handler) UpdateCurrentUser(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	// Don't allow changing own role or status
	req.Role = nil
	req.Status = nil

	user, err := h.service.Update(c.Request().Context(), userID, &req)
	if err != nil {
		switch err {
		case ErrUserNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		case ErrEmailExists:
			return echo.NewHTTPError(http.StatusConflict, "email already exists")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to update user")
		}
	}

	return c.JSON(http.StatusOK, user)
}

// DeleteUser handles deleting a user.
func (h *Handler) DeleteUser(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		if err == ErrUserNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete user")
	}

	return c.NoContent(http.StatusNoContent)
}

// ListUsers handles listing users.
func (h *Handler) ListUsers(c echo.Context) error {
	query := &ListUsersQuery{
		Page:     1,
		PageSize: 20,
	}

	if page := c.QueryParam("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			query.Page = p
		}
	}

	if pageSize := c.QueryParam("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			query.PageSize = ps
		}
	}

	query.Search = c.QueryParam("search")
	query.SortBy = c.QueryParam("sort_by")
	query.SortDir = c.QueryParam("sort_dir")

	if role := c.QueryParam("role"); role != "" {
		r := Role(role)
		query.Role = &r
	}

	if status := c.QueryParam("status"); status != "" {
		s := Status(status)
		query.Status = &s
	}

	result, err := h.service.List(c.Request().Context(), query)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list users")
	}

	return c.JSON(http.StatusOK, result)
}

// LockUser handles locking a user.
func (h *Handler) LockUser(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.service.Lock(c.Request().Context(), id); err != nil {
		if err == ErrUserNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to lock user")
	}

	return c.NoContent(http.StatusNoContent)
}

// UnlockUser handles unlocking a user.
func (h *Handler) UnlockUser(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	if err := h.service.Unlock(c.Request().Context(), id); err != nil {
		if err == ErrUserNotFound {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to unlock user")
	}

	return c.NoContent(http.StatusNoContent)
}

// ChangePassword handles password change.
func (h *Handler) ChangePassword(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == uuid.Nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req ChangePasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.ChangePassword(c.Request().Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
		switch err {
		case ErrInvalidCredentials:
			return echo.NewHTTPError(http.StatusUnauthorized, "current password is incorrect")
		case ErrInvalidPassword:
			return echo.NewHTTPError(http.StatusBadRequest, "new password does not meet requirements")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to change password")
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// ResetPassword handles admin resetting a user's password.
func (h *Handler) ResetPassword(c echo.Context) error {
	// Check if requester is admin
	claims := auth.GetUserFromContext(c)
	if claims == nil || claims.Role != string(RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	var req ResetPasswordRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if err := h.service.ResetPassword(c.Request().Context(), id, req.NewPassword); err != nil {
		switch err {
		case ErrUserNotFound:
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		case ErrInvalidPassword:
			return echo.NewHTTPError(http.StatusBadRequest, "password does not meet requirements")
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to reset password")
		}
	}

	return c.NoContent(http.StatusNoContent)
}

// Helper functions

func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func getUserIDFromContext(c echo.Context) uuid.UUID {
	// Try to get from auth middleware (stored in request context)
	if claims := auth.GetUserFromContext(c); claims != nil {
		if id, err := uuid.Parse(claims.UserID); err == nil {
			return id
		}
	}

	// Fallback: try to get from echo context directly
	if id, ok := c.Get("user_id").(uuid.UUID); ok {
		return id
	}
	if idStr, ok := c.Get("user_id").(string); ok {
		if id, err := uuid.Parse(idStr); err == nil {
			return id
		}
	}
	return uuid.Nil
}
