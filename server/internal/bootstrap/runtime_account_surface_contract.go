package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type permissionRouteRegistrar interface {
	RegisterCurrentUserRoutes(*echo.Group)
	RegisterAdminRoutes(*echo.Group)
}

type routeRuntimeContractAccountSurfaceOptions struct {
	v1                    *echo.Group
	protected             *echo.Group
	authPageV1Group       func(string) *echo.Group
	authMiddleware        *auth.AuthMiddleware
	requirePagePermission func(string) echo.MiddlewareFunc
	userHandler           *user.Handler
	permissionHandler     permissionRouteRegistrar
	apiKeyHandler         routeRegistrar
	autoreplyHandler      routeRegistrar
	logger                *zap.Logger
}

func (binding *runtimeContractBinding) BindAccountSurfaceRuntime(options routeRuntimeContractAccountSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeAccountSurfaces(options)
}

func bindRouteRuntimeAccountSurfaces(options routeRuntimeContractAccountSurfaceOptions) {
	registerRouteRuntimeCurrentUserSurface(options.v1, options.authMiddleware, options.userHandler, options.permissionHandler)
	registerRouteRuntimeAdminUserSurface(options)
	registerAutoreplyRoutes(routeRuntimeAuthGroup(options.authPageV1Group, permission.PageChannels), options.autoreplyHandler)
}

func registerCurrentUserRoutes(v1 *echo.Group, authMiddleware *auth.AuthMiddleware, userHandler *user.Handler) {
	if v1 == nil || userHandler == nil {
		return
	}

	if authMiddleware == nil {
		v1.GET("/users/me", userHandler.GetCurrentUser)
		v1.PUT("/users/me", userHandler.UpdateCurrentUser)
		return
	}

	// Preserve preview-mode access for GET while still hydrating request context
	// from Bearer tokens when present.
	v1.GET("/users/me", userHandler.GetCurrentUser, authMiddleware.OptionalAuthenticate())
	v1.PUT("/users/me", userHandler.UpdateCurrentUser, authMiddleware.Authenticate())
}

func registerRouteRuntimeCurrentUserSurface(
	v1 *echo.Group,
	authMiddleware *auth.AuthMiddleware,
	userHandler *user.Handler,
	permissionHandler permissionRouteRegistrar,
) {
	registerCurrentUserRoutes(v1, authMiddleware, userHandler)
	if v1 == nil || permissionHandler == nil {
		return
	}
	if authMiddleware == nil {
		permissionHandler.RegisterCurrentUserRoutes(v1)
		return
	}
	permissionHandler.RegisterCurrentUserRoutes(v1.Group("", authMiddleware.Authenticate()))
}

func registerRouteRuntimeAdminUserSurface(options routeRuntimeContractAccountSurfaceOptions) {
	if options.protected == nil {
		return
	}

	if options.userHandler != nil {
		usersGroup := options.protected.Group(
			"/users",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageUsers))...,
		)
		usersGroup.GET("", options.userHandler.ListUsers)
		usersGroup.POST("", options.userHandler.CreateUser)
		usersGroup.GET("/:id", options.userHandler.GetUser)
		usersGroup.PUT("/:id", options.userHandler.UpdateUser)
		usersGroup.DELETE("/:id", options.userHandler.DeleteUser)
		usersGroup.POST("/:id/lock", options.userHandler.LockUser)
		usersGroup.POST("/:id/unlock", options.userHandler.UnlockUser)
		usersGroup.POST("/:id/reset-password", options.userHandler.ResetPassword)

		options.protected.POST(
			"/auth/password",
			options.userHandler.ChangePassword,
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageProfile))...,
		)
	}

	if options.permissionHandler != nil {
		options.permissionHandler.RegisterAdminRoutes(options.protected)
		if options.logger != nil {
			options.logger.Info("Permission routes registered")
		}
	}

	if options.apiKeyHandler != nil {
		options.apiKeyHandler.RegisterRoutes(options.protected.Group(
			"/apikeys",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageProfile))...,
		))
	}
}

func registerAutoreplyRoutes(group *echo.Group, handler routeRegistrar) {
	if group == nil || handler == nil {
		return
	}
	handler.RegisterRoutes(group)
}
