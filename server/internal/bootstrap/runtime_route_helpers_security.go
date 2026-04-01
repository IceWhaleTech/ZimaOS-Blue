package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func registerSandboxRoutes(
	protected *echo.Group,
	requirePagePermission func(string) echo.MiddlewareFunc,
	deps *RoutesDeps,
) {
	if protected == nil || deps == nil {
		return
	}

	groupMiddlewares := make([]echo.MiddlewareFunc, 0, 1)
	if requirePagePermission != nil {
		groupMiddlewares = append(groupMiddlewares, requirePagePermission(permission.PageTools))
	}
	sandboxGroup := protected.Group("/sandbox", groupMiddlewares...)

	if deps.SandboxHandler != nil && deps.SandboxManager != nil && deps.SandboxManager.IsSupported() {
		if deps.ConfigStore != nil {
			deps.SandboxHandler.SetConfigStore(deps.ConfigStore)
		}
		deps.SandboxHandler.SetNetworkConfigHook(func(networkEnabled bool) {
			if deps.Config != nil {
				deps.Config.Security.Sandbox.NetworkEnabled = networkEnabled
			}
			if deps.SecurityHandler != nil && deps.Config != nil {
				deps.SecurityHandler.SetScannerConfig(buildSecurityScannerConfig(
					deps.Config,
					deps.SandboxManager != nil && deps.SandboxManager.IsSupported(),
				))
			}
		})
		deps.SandboxHandler.RegisterRoutes(sandboxGroup)
		return
	}

	stub := featureDisabled("sandbox")
	sandboxGroup.PATCH("/config", stub)
	sandboxGroup.GET("/info", stub)
	sandboxGroup.Any("/*", stub)
}

func registerBackupRoutes(v1 *echo.Group, backupHandler routeRegistrar, authMiddleware, pageMiddleware echo.MiddlewareFunc) {
	if v1 == nil {
		return
	}
	if routeRuntimeHasValue(backupHandler) {
		backupHandler.RegisterRoutes(v1.Group("", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		return
	}

	stub := featureDisabled("backup")
	backupGroup := v1.Group("/backup", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	backupGroup.GET("", stub)
	backupGroup.GET("/progress", stub)
	backupGroup.Any("/*", stub)
}

func registerSecurityRoutes(
	protected *echo.Group,
	requirePagePermission func(string) echo.MiddlewareFunc,
	securityHandler securityRouteRegistrar,
	cfg *config.Config,
	configKV kvstore.Store,
	dataDir string,
	sandboxSupported bool,
) {
	if protected == nil {
		return
	}

	var pageMiddleware echo.MiddlewareFunc
	if requirePagePermission != nil {
		pageMiddleware = requirePagePermission(permission.PageSecurity)
	}
	securityGroup := protected.Group("/security", filterRouteMiddlewares(pageMiddleware)...)

	if routeRuntimeHasValue(securityHandler) {
		if cfg != nil {
			securityHandler.SetScannerConfig(buildSecurityScannerConfig(cfg, sandboxSupported))
		}
		if configKV != nil {
			securityHandler.SetKVStore(configKV)
		}
		securityHandler.SetDataDir(dataDir)
		securityHandler.RegisterRoutes(securityGroup)
		return
	}

	stub := featureDisabled("security")
	securityGroup.GET("/sessions", stub)
	securityGroup.GET("/settings", stub)
	securityGroup.GET("/events", stub)
	securityGroup.GET("/stats", stub)
	securityGroup.GET("/blocked-ips", stub)
	securityGroup.GET("/threats/stats", stub)
	securityGroup.GET("/threats", stub)
	securityGroup.GET("/scan", stub)
	securityGroup.GET("/cors", stub)
	securityGroup.GET("/tls", stub)
	securityGroup.Any("/*", stub)
}
