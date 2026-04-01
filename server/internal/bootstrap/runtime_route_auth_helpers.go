package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func routeRuntimeAuthMiddleware(authMiddleware *auth.AuthMiddleware) echo.MiddlewareFunc {
	if authMiddleware == nil {
		return nil
	}
	return authMiddleware.Authenticate()
}

func routeRuntimePageMiddleware(resolver func(string) echo.MiddlewareFunc, page string) echo.MiddlewareFunc {
	if resolver == nil {
		return nil
	}
	return resolver(page)
}
