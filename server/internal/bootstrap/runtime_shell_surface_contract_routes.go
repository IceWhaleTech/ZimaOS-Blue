package bootstrap

import "github.com/labstack/echo/v4"

func registerFormfillerRoutes(v1 *echo.Group, formfillerHandler routeRegistrar, authMiddleware, pageMiddleware echo.MiddlewareFunc) {
	if v1 == nil {
		return
	}
	if formfillerHandler != nil {
		formfillerHandler.RegisterRoutes(v1.Group("/formfiller", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		return
	}

	stub := featureDisabled("formfiller")
	formfillerGroup := v1.Group("/formfiller", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	formfillerGroup.GET("/templates", stub)
	formfillerGroup.GET("/config", stub)
	formfillerGroup.Any("/*", stub)
}

func registerExternalAuthRoutes(v1, protected *echo.Group, handler externalAuthRouteRegistrar) {
	if handler == nil {
		return
	}
	if v1 != nil {
		handler.RegisterRoutes(v1.Group("/auth"))
	}
	if protected != nil {
		handler.RegisterProtectedRoutes(protected.Group("/auth"))
	}
}
