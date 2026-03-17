package permission

import (
	"context"
	"net/http"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// RequirePagePermission ensures the authenticated user has the requested page permission.
// Preview mode keeps full access so the onboarding/admin flow continues to work.
func RequirePagePermission(service *Service, pagePermission string) echo.MiddlewareFunc {
	required := strings.TrimSpace(pagePermission)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if required == "" {
				return next(c)
			}

			claims := auth.GetUserFromContext(c)
			if claims == nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
			}
			if strings.TrimSpace(claims.UserID) == "preview-user" {
				return next(c)
			}

			userID, err := uuid.Parse(strings.TrimSpace(claims.UserID))
			if err != nil {
				if hasDefaultRolePermission(service, claims.Role, required) {
					return next(c)
				}
				return echo.NewHTTPError(http.StatusForbidden, "invalid user identity")
			}

			allowed, err := hasPagePermission(c.Request().Context(), service, userID, claims.Role, required)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to check permissions")
			}
			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "insufficient permissions")
			}
			return next(c)
		}
	}
}

func hasPagePermission(ctx context.Context, service *Service, userID uuid.UUID, role, pagePermission string) (bool, error) {
	if strings.TrimSpace(pagePermission) == "" {
		return true, nil
	}
	if service == nil || service.userRepo == nil {
		return hasDefaultRolePermission(service, role, pagePermission), nil
	}

	allowed, err := service.HasPermission(ctx, userID, pagePermission)
	if err != nil {
		if hasDefaultRolePermission(service, role, pagePermission) {
			return true, nil
		}
		return false, err
	}
	return allowed, nil
}

func hasDefaultRolePermission(service *Service, role, pagePermission string) bool {
	var defaults []string
	if service != nil {
		defaults = service.GetDefaultPermissionsForRole(role)
	} else {
		defaults = (&Service{}).GetDefaultPermissionsForRole(role)
	}
	for _, permission := range defaults {
		if permission == pagePermission {
			return true
		}
	}
	return false
}
