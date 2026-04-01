package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
)

func registerRuntimeExecSupportRoutes(v1 *echo.Group, authMiddleware *auth.AuthMiddleware, pageMiddleware echo.MiddlewareFunc, bundle runtimeExecSupportBundle) {
	if v1 == nil {
		return
	}

	var authRouteMiddleware echo.MiddlewareFunc
	if authMiddleware != nil {
		authRouteMiddleware = authMiddleware.Authenticate()
	}

	execGroup := v1.Group("/exec", filterRouteMiddlewares(authRouteMiddleware, pageMiddleware)...)
	registerExecApprovalRoutes(execGroup, bundle.Approvals)
	registerExecDirectoryApprovalRoutes(execGroup, bundle.DirStore)
}

func (bundle runtimeExecSupportBundle) registerConvertRoutes(protected *echo.Group, pageMiddleware echo.MiddlewareFunc) bool {
	if protected == nil || bundle.ConvertHandler == nil {
		return false
	}
	bundle.ConvertHandler.RegisterRoutes(protected.Group("/convert", filterRouteMiddlewares(pageMiddleware)...))
	return true
}
