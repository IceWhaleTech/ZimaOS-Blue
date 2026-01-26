package user

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for user management.
type Handler struct {
	service *Service
}

// NewHandler creates a new user handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers the user routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Public routes
	g.POST("/auth/login", h.Login)
	g.POST("/auth/logout", h.Logout)

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
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	User         *User  `json:"user"`
	MFARequired  bool   `json:"mfa_required,omitempty"`
	MFAToken     string `json:"mfa_token,omitempty"`
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
			return echo.NewHTTPError(http.StatusInternalServerError, "authentication failed")
		}
	}

	// Generate tokens
	accessToken := generateToken(32)
	refreshToken := generateToken(64)

	// Create session
	userAgent := c.Request().UserAgent()
	ipAddress := c.RealIP()
	session, err := h.service.CreateSession(c.Request().Context(), user.ID, refreshToken, userAgent, ipAddress)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	return c.JSON(http.StatusOK, &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(session.ExpiresAt.Sub(session.CreatedAt).Seconds()),
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

// GetCurrentUser handles getting the current user.
func (h *Handler) GetCurrentUser(c echo.Context) error {
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

// Helper functions

func generateToken(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func getUserIDFromContext(c echo.Context) uuid.UUID {
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
