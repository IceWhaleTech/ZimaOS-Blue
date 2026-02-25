package permission

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

// Handler handles HTTP requests for permission management
type Handler struct {
	service  *Service
	userRepo user.Repository
}

// NewHandler creates a new permission handler
func NewHandler(service *Service, userRepo user.Repository) *Handler {
	return &Handler{
		service:  service,
		userRepo: userRepo,
	}
}

// RegisterRoutes registers the permission routes
func (h *Handler) RegisterRoutes(g *echo.Group) {
	// Get own permissions (any authenticated user)
	g.GET("/users/me/permissions", h.GetMyPermissions)

	// Get available permissions (admin only)
	g.GET("/permissions/available", h.GetAvailablePermissions)

	// Get/set user permissions (admin only)
	g.GET("/users/:id/permissions", h.GetUserPermissions)
	g.PUT("/users/:id/permissions", h.SetUserPermissions)
}

// GetMyPermissions returns the current user's permissions
func (h *Handler) GetMyPermissions(c echo.Context) error {
	claims := auth.GetUserFromContext(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	// Handle preview user
	if claims.UserID == "preview-user" {
		return c.JSON(http.StatusOK, &PermissionsResponse{
			UserID:      uuid.Nil,
			Role:        claims.Role,
			Permissions: AllPagePermissions(),
		})
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	perms, err := h.service.GetEffectivePermissions(c.Request().Context(), userID)
	if err != nil {
		// If user not found, return role-based defaults instead of 500
		if errors.Is(err, user.ErrUserNotFound) {
			return c.JSON(http.StatusOK, &PermissionsResponse{
				UserID:      userID,
				Role:        claims.Role,
				Permissions: h.service.GetDefaultPermissionsForRole(claims.Role),
			})
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get permissions")
	}

	if perms == nil {
		perms = []string{}
	}

	return c.JSON(http.StatusOK, &PermissionsResponse{
		UserID:      userID,
		Role:        claims.Role,
		Permissions: perms,
	})
}

// GetUserPermissions returns a specific user's permissions (admin only)
func (h *Handler) GetUserPermissions(c echo.Context) error {
	// Check if requester is admin
	claims := auth.GetUserFromContext(c)
	if claims == nil || claims.Role != string(user.RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	// Get user to include role
	if h.userRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "user service not available")
	}
	u, err := h.userRepo.GetByID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	perms, err := h.service.GetEffectivePermissions(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get permissions")
	}

	return c.JSON(http.StatusOK, &PermissionsResponse{
		UserID:      userID,
		Role:        string(u.Role),
		Permissions: perms,
	})
}

// SetUserPermissions sets a user's permissions (admin only)
func (h *Handler) SetUserPermissions(c echo.Context) error {
	// Check if requester is admin
	claims := auth.GetUserFromContext(c)
	if claims == nil || claims.Role != string(user.RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user ID")
	}

	// Check if target user exists
	if h.userRepo == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "user service not available")
	}
	u, err := h.userRepo.GetByID(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "user not found")
	}

	// Cannot modify admin permissions
	if u.Role == user.RoleAdmin {
		return echo.NewHTTPError(http.StatusBadRequest, "cannot modify admin permissions")
	}

	var req SetPermissionsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	grantedBy := claims.UserID
	if err := h.service.SetUserPermissions(c.Request().Context(), userID, req.Permissions, &grantedBy); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to set permissions")
	}

	// Return updated permissions
	perms, err := h.service.GetEffectivePermissions(c.Request().Context(), userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to get permissions")
	}

	return c.JSON(http.StatusOK, &PermissionsResponse{
		UserID:      userID,
		Role:        string(u.Role),
		Permissions: perms,
	})
}

// GetAvailablePermissions returns all available permissions
func (h *Handler) GetAvailablePermissions(c echo.Context) error {
	// Check if requester is admin
	claims := auth.GetUserFromContext(c)
	if claims == nil || claims.Role != string(user.RoleAdmin) {
		return echo.NewHTTPError(http.StatusForbidden, "admin access required")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"permissions": h.service.GetAvailablePermissions(),
	})
}
