package rbac

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// ContextKey is the type for context keys
type ContextKey string

const (
	// RoleContextKey is the key for storing role in context
	RoleContextKey ContextKey = "role"
)

// Middleware provides RBAC middleware for Echo
type Middleware struct {
	rbac *RBAC
}

// NewMiddleware creates a new RBAC middleware
func NewMiddleware(rbac *RBAC) *Middleware {
	return &Middleware{rbac: rbac}
}

// RequirePermission returns a middleware that requires a specific permission
func (m *Middleware) RequirePermission(permission string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetRoleFromContext(c)
			if role == "" {
				return echo.NewHTTPError(http.StatusForbidden, "no role assigned")
			}

			if !m.rbac.HasPermission(role, permission) {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// RequireAnyPermission returns a middleware that requires any of the specified permissions
func (m *Middleware) RequireAnyPermission(permissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetRoleFromContext(c)
			if role == "" {
				return echo.NewHTTPError(http.StatusForbidden, "no role assigned")
			}

			for _, perm := range permissions {
				if m.rbac.HasPermission(role, perm) {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
		}
	}
}

// RequireAllPermissions returns a middleware that requires all of the specified permissions
func (m *Middleware) RequireAllPermissions(permissions ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetRoleFromContext(c)
			if role == "" {
				return echo.NewHTTPError(http.StatusForbidden, "no role assigned")
			}

			for _, perm := range permissions {
				if !m.rbac.HasPermission(role, perm) {
					return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
				}
			}

			return next(c)
		}
	}
}

// RequireRole returns a middleware that requires a specific role
func (m *Middleware) RequireRole(roleName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetRoleFromContext(c)
			if role != roleName {
				return echo.NewHTTPError(http.StatusForbidden, "role not allowed")
			}

			return next(c)
		}
	}
}

// RequireAnyRole returns a middleware that requires any of the specified roles
func (m *Middleware) RequireAnyRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetRoleFromContext(c)
			for _, r := range roles {
				if role == r {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusForbidden, "role not allowed")
		}
	}
}

// SetRoleInContext stores the role in the echo context
func SetRoleInContext(c echo.Context, role string) {
	c.Set(string(RoleContextKey), role)
}

// GetRoleFromContext retrieves the role from the echo context
func GetRoleFromContext(c echo.Context) string {
	role, ok := c.Get(string(RoleContextKey)).(string)
	if !ok {
		return ""
	}
	return role
}
