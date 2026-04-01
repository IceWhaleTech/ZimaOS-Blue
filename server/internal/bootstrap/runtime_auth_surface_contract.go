package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func (binding *runtimeContractBinding) BindAuthSurfaceRuntime(
	options routeRuntimeContractAuthSurfaceOptions,
) routeRuntimeContractAuthSurfaceResult {
	if binding == nil {
		return routeRuntimeContractAuthSurfaceResult{}
	}
	return bindRouteRuntimeAuthSurface(options)
}

func bindRouteRuntimeAuthSurface(options routeRuntimeContractAuthSurfaceOptions) routeRuntimeContractAuthSurfaceResult {
	authMiddleware := routeRuntimeAuthMiddleware(options.authMiddleware)

	permRepo, err := routeRuntimePermissionRepository(options.writeDB, options.readDB)
	if err != nil && options.logger != nil {
		options.logger.Error("Failed to initialize permission repository", zap.Error(err))
	}

	permService := permission.NewService(permRepo, options.userRepo)
	requirePagePermission := func(page string) echo.MiddlewareFunc {
		return permission.RequirePagePermission(permService, page)
	}

	return routeRuntimeContractAuthSurfaceResult{
		protected:             routeRuntimeProtectedGroup(options.v1, authMiddleware),
		apiProtected:          routeRuntimeProtectedGroup(options.api, authMiddleware),
		authPageV1Group:       routeRuntimeAuthPageGroupResolver(options.v1, authMiddleware, requirePagePermission),
		authPageAPIGroup:      routeRuntimeAuthPageGroupResolver(options.api, authMiddleware, requirePagePermission),
		requirePagePermission: requirePagePermission,
		authMiddleware:        authMiddleware,
		permissionHandler:     permission.NewHandler(permService, options.userRepo),
	}
}

func routeRuntimePermissionRepository(writeDB, readDB *sql.DB) (*permission.Repository, error) {
	if writeDB == nil {
		return nil, nil
	}
	return permission.NewRepositoryWithReadDB(writeDB, readDB)
}

func routeRuntimeProtectedGroup(parent *echo.Group, authMiddleware echo.MiddlewareFunc) *echo.Group {
	if parent == nil {
		return nil
	}

	group := parent.Group("")
	if authMiddleware != nil {
		group.Use(authMiddleware)
	}
	return group
}

func routeRuntimeAuthPageGroupResolver(
	parent *echo.Group,
	authMiddleware echo.MiddlewareFunc,
	requirePagePermission func(string) echo.MiddlewareFunc,
) func(string) *echo.Group {
	return func(page string) *echo.Group {
		if parent == nil {
			return nil
		}
		return parent.Group(
			"",
			filterRouteMiddlewares(authMiddleware, routeRuntimePageMiddleware(requirePagePermission, page))...,
		)
	}
}
