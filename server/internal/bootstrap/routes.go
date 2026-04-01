// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// RegisterAllRoutes registers all API routes on the Echo instance.
// Returns the authenticated API group for late-binding route registration.
func RegisterAllRoutes(e *echo.Echo, deps *RoutesDeps) *echo.Group {
	state := newRouteRegistrationState(e, deps)
	bindRouteRuntimeRegistration(state)
	state.logger.Info("All routes registered", zap.Duration("elapsed", time.Since(state.registerStart)))
	return state.apiProtected
}
