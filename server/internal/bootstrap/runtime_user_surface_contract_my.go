package bootstrap

import (
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerRouteRuntimeMyRoutes(options routeRuntimeContractUserSurfaceOptions) {
	if options.protected == nil {
		return
	}

	myGroup := options.protected.Group(
		"/my",
		filterRouteMiddlewares(routeRuntimePageMiddleware(options.requirePagePermission, permission.PageProfile))...,
	)

	registerRouteRuntimeUserScopedRoutes(
		myGroup,
		"/providers",
		options.logger,
		"Failed to initialize user provider handler",
		"User provider routes registered",
		func() (routeRegistrar, error) {
			if options.writeDB == nil {
				return nil, fmt.Errorf("user provider database is not configured")
			}
			readDB := options.readDB
			if readDB == nil {
				readDB = options.writeDB
			}
			return serverpkg.NewUserProviderHandlerWithReadDB(options.writeDB, readDB, options.llmRegistry)
		},
	)

	registerRouteRuntimeUserScopedRoutes(
		myGroup,
		"/skills",
		options.logger,
		"Failed to initialize user skill handler",
		"User skill routes registered",
		func() (routeRegistrar, error) {
			if options.writeDB == nil {
				return nil, fmt.Errorf("user skill database is not configured")
			}
			readDB := options.readDB
			if readDB == nil {
				readDB = options.writeDB
			}
			return serverpkg.NewUserSkillHandlerWithReadDB(options.writeDB, readDB, options.skillsDir)
		},
	)

	var usageHandler echo.HandlerFunc
	if options.metricsWriter != nil {
		usageHandler = metrics.NewHandler(options.metricsWriter).GetMyUsage
	}
	registerRouteRuntimeMyUsageRoute(myGroup, usageHandler, options.logger)
}

func registerRouteRuntimeUserScopedRoutes(
	myGroup *echo.Group,
	path string,
	logger *zap.Logger,
	initFailureLog, successLog string,
	factory func() (routeRegistrar, error),
) {
	if myGroup == nil || factory == nil {
		return
	}

	start := time.Now()
	handler, err := factory()
	elapsed := time.Since(start)
	if err != nil {
		if logger != nil {
			logger.Warn(initFailureLog, zap.Error(err), zap.Duration("elapsed", elapsed))
		}
		return
	}
	if handler == nil {
		return
	}

	handler.RegisterRoutes(myGroup.Group(path))
	if logger != nil {
		logger.Info(successLog, zap.Duration("elapsed", elapsed))
	}
}

func registerRouteRuntimeMyUsageRoute(myGroup *echo.Group, handler echo.HandlerFunc, logger *zap.Logger) {
	if myGroup == nil {
		return
	}
	if handler != nil {
		myGroup.GET("/usage", handler)
		if logger != nil {
			logger.Info("User usage route registered")
		}
		return
	}
	myGroup.GET("/usage", featureDisabled("metrics"))
}
