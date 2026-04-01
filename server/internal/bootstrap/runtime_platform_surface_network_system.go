package bootstrap

import (
	"time"

	networkapi "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/api"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"go.uber.org/zap"
)

func registerRouteRuntimeNetworkSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	if options.authPageV1Group != nil {
		networkHandler := networkapi.NewNetworkHandler(routeRuntimeServerPort(options.serverConfig))
		networkHandler.RegisterGroupRoutes(options.authPageV1Group(permission.PageSettings))

		linkPreviewHandler := networkapi.NewLinkPreviewHandler()
		linkPreviewHandler.RegisterRoutes(options.authPageV1Group(permission.PageChat))
	}

	serverPort := routeRuntimeServerPort(options.serverConfig)
	logger := options.logger
	serverpkg.OnServerStart(func(port int) {
		go func() {
			start := time.Now()
			handler := networkapi.NewNetworkHandler(port)
			if err := handler.InitializeCORSOrigins(); err != nil {
				if logger != nil {
					logger.Warn("Failed to initialize CORS origins from network addresses", zap.Error(err), zap.Duration("elapsed", time.Since(start)))
				}
				return
			}
			if logger != nil {
				logger.Info("CORS origins initialized from network addresses",
					zap.Int("port", routeRuntimeResolvedPort(port, serverPort)),
					zap.Duration("elapsed", time.Since(start)),
				)
			}
		}()
	})
}

func registerRouteRuntimeSystemSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	systemHandler := serverpkg.NewSystemHandler(
		routeRuntimeServerVersion(options.serverConfig),
		routeRuntimeServerBuildTime(options.serverConfig),
		routeRuntimeServerGitCommit(options.serverConfig),
		routeRuntimeServerDataDir(options.serverConfig),
	)

	if options.authPageV1Group != nil {
		systemHomeGroup := options.authPageV1Group(permission.PageHome)
		systemHomeGroup.GET("/system/info", systemHandler.GetInfo)

		systemSettingsGroup := options.authPageV1Group(permission.PageSettings)
		systemSettingsGroup.GET("/system/config", systemHandler.GetConfig)
		systemSettingsGroup.PUT("/system/config", systemHandler.UpdateConfig)
		systemSettingsGroup.POST("/system/restart", systemHandler.RestartService)

		systemSecurityGroup := options.authPageV1Group(permission.PageSecurity)
		systemSecurityGroup.GET("/system/logs", systemHandler.GetLogs)
		systemSecurityGroup.POST("/system/logs", systemHandler.WriteLog)
		systemSecurityGroup.GET("/system/certificate", systemHandler.GetCertificate)

		serviceHandler := serverpkg.NewServiceHandler()
		serviceHandler.RegisterRoutes(options.authPageV1Group(permission.PageSettings))
	}

	if options.protected != nil {
		systemHandler.RegisterFileBridgeRoutes(options.protected.Group(
			"",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageChat))...,
		))
	}

	if options.protected != nil && options.connectionManager != nil {
		connHandler := connection.NewHandler(options.connectionManager)
		connHandler.RegisterRoutes(options.protected.Group(
			"/connections",
			filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageSecurity))...,
		))
	}
}

func routeRuntimeResolvedPort(primary, fallback int) int {
	if primary != 0 {
		return primary
	}
	return fallback
}
