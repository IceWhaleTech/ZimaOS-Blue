package tenant

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ContextKey is the type for context keys.
type ContextKey string

const (
	// TenantIDKey is the context key for tenant ID.
	TenantIDKey ContextKey = "tenant_id"
	// TenantKey is the context key for tenant object.
	TenantKey ContextKey = "tenant"
	// MemberKey is the context key for member object.
	MemberKey ContextKey = "tenant_member"
)

// Middleware provides tenant-related middleware.
type Middleware struct {
	service *Service
}

// NewMiddleware creates a new tenant middleware.
func NewMiddleware(service *Service) *Middleware {
	return &Middleware{service: service}
}

// RequireTenant middleware ensures a tenant is selected and the user is a member.
// It expects the tenant ID to be in the X-Tenant-ID header or tenant_id query parameter.
func (m *Middleware) RequireTenant() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := getUserIDFromContext(c)
			if userID == uuid.Nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
			}

			// Get tenant ID from header or query
			tenantIDStr := c.Request().Header.Get("X-Tenant-ID")
			if tenantIDStr == "" {
				tenantIDStr = c.QueryParam("tenant_id")
			}

			if tenantIDStr == "" {
				return echo.NewHTTPError(http.StatusBadRequest, "tenant ID required")
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid tenant ID")
			}

			// Check if user is a member
			member, err := m.service.GetMember(c.Request().Context(), tenantID, userID)
			if err != nil {
				if err == ErrMemberNotFound {
					return echo.NewHTTPError(http.StatusForbidden, "not a member of this tenant")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to check membership")
			}

			// Get tenant
			tenant, err := m.service.GetByID(c.Request().Context(), tenantID)
			if err != nil {
				if err == ErrTenantNotFound {
					return echo.NewHTTPError(http.StatusNotFound, "tenant not found")
				}
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to get tenant")
			}

			// Check if tenant is active
			if !tenant.IsActive() {
				return echo.NewHTTPError(http.StatusForbidden, "tenant is not active")
			}

			// Set tenant info in context
			c.Set(string(TenantIDKey), tenantID)
			c.Set(string(TenantKey), tenant)
			c.Set(string(MemberKey), member)

			// Also set in request context for downstream use
			ctx := context.WithValue(c.Request().Context(), TenantIDKey, tenantID)
			ctx = context.WithValue(ctx, TenantKey, tenant)
			ctx = context.WithValue(ctx, MemberKey, member)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// RequireRole middleware ensures the user has at least the specified role in the tenant.
// Must be used after RequireTenant middleware.
func (m *Middleware) RequireRole(requiredRole MemberRole) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			member, ok := c.Get(string(MemberKey)).(*TenantMember)
			if !ok || member == nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "tenant context not set")
			}

			if !hasRolePermission(member.Role, requiredRole) {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}

			return next(c)
		}
	}
}

// OptionalTenant middleware sets tenant context if provided, but doesn't require it.
func (m *Middleware) OptionalTenant() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userID := getUserIDFromContext(c)
			if userID == uuid.Nil {
				return next(c)
			}

			// Get tenant ID from header or query
			tenantIDStr := c.Request().Header.Get("X-Tenant-ID")
			if tenantIDStr == "" {
				tenantIDStr = c.QueryParam("tenant_id")
			}

			if tenantIDStr == "" {
				return next(c)
			}

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				return next(c)
			}

			// Check if user is a member
			member, err := m.service.GetMember(c.Request().Context(), tenantID, userID)
			if err != nil {
				return next(c)
			}

			// Get tenant
			tenant, err := m.service.GetByID(c.Request().Context(), tenantID)
			if err != nil || !tenant.IsActive() {
				return next(c)
			}

			// Set tenant info in context
			c.Set(string(TenantIDKey), tenantID)
			c.Set(string(TenantKey), tenant)
			c.Set(string(MemberKey), member)

			ctx := context.WithValue(c.Request().Context(), TenantIDKey, tenantID)
			ctx = context.WithValue(ctx, TenantKey, tenant)
			ctx = context.WithValue(ctx, MemberKey, member)
			c.SetRequest(c.Request().WithContext(ctx))

			return next(c)
		}
	}
}

// GetTenantFromContext retrieves the tenant from context.
func GetTenantFromContext(ctx context.Context) *Tenant {
	if tenant, ok := ctx.Value(TenantKey).(*Tenant); ok {
		return tenant
	}
	return nil
}

// GetTenantIDFromContext retrieves the tenant ID from context.
func GetTenantIDFromContext(ctx context.Context) uuid.UUID {
	if id, ok := ctx.Value(TenantIDKey).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// GetMemberFromContext retrieves the member from context.
func GetMemberFromContext(ctx context.Context) *TenantMember {
	if member, ok := ctx.Value(MemberKey).(*TenantMember); ok {
		return member
	}
	return nil
}

// GetTenantFromEchoContext retrieves the tenant from Echo context.
func GetTenantFromEchoContext(c echo.Context) *Tenant {
	if tenant, ok := c.Get(string(TenantKey)).(*Tenant); ok {
		return tenant
	}
	return nil
}

// GetTenantIDFromEchoContext retrieves the tenant ID from Echo context.
func GetTenantIDFromEchoContext(c echo.Context) uuid.UUID {
	if id, ok := c.Get(string(TenantIDKey)).(uuid.UUID); ok {
		return id
	}
	return uuid.Nil
}

// GetMemberFromEchoContext retrieves the member from Echo context.
func GetMemberFromEchoContext(c echo.Context) *TenantMember {
	if member, ok := c.Get(string(MemberKey)).(*TenantMember); ok {
		return member
	}
	return nil
}
