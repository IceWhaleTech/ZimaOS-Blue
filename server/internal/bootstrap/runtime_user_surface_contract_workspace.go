package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func registerRouteRuntimeWorkspaceRoutes(
	protected *echo.Group,
	requirePagePermission func(string) echo.MiddlewareFunc,
	workspaceHandler routeRegistrar,
	logger *zap.Logger,
) {
	if protected == nil || workspaceHandler == nil {
		return
	}

	workspaceGroup := protected.Group(
		"/workspace",
		filterRouteMiddlewares(routeRuntimePageMiddleware(requirePagePermission, permission.PageChat))...,
	)
	workspaceHandler.RegisterRoutes(workspaceGroup)
	if logger != nil {
		logger.Info("Workspace routes registered")
	}
}
