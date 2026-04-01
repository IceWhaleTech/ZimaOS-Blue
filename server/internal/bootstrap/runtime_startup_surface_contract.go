package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/preview"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type routeRuntimeContractStartupSurfaceOptions struct {
	e              *echo.Echo
	v1             *echo.Group
	dataDir        string
	services       *Services
	authMiddleware *auth.AuthMiddleware
	userHandler    *user.Handler
	logger         *zap.Logger
}

func (binding *runtimeContractBinding) BindStartupSurfaceRuntime(options routeRuntimeContractStartupSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeStartupSurface(options)
}

func bindRouteRuntimeStartupSurface(options routeRuntimeContractStartupSurfaceOptions) {
	if options.services != nil && options.userHandler != nil && options.e != nil {
		previewModeService := preview.NewModeService(options.services.UserService)
		var previewUpgradeService *preview.UpgradeService
		if options.services.DBConn != nil {
			previewUpgradeService = preview.NewUpgradeServiceWithReadDB(
				options.services.UserService,
				options.services.DBConn.Writer,
				options.services.DBConn.Reader,
			)
		} else {
			previewUpgradeService = preview.NewUpgradeService(options.services.UserService, options.services.DB)
		}
		previewHandler := preview.NewHandler(
			previewModeService,
			previewUpgradeService,
			options.services.JWTService,
			options.services.UserService,
		)
		previewHandler.SetDataDir(options.dataDir)
		previewHandler.RegisterRoutes(options.e)

		if options.authMiddleware != nil {
			options.authMiddleware.SetPreviewModeChecker(previewModeService)
		}
		options.userHandler.SetModeService(previewModeService)
	}

	registerPublicAuthRoutes(options.v1, options.userHandler)
	if options.logger != nil {
		options.logger.Info("Preview mode routes registered")
	}
}

func registerPublicAuthRoutes(v1 *echo.Group, userHandler *user.Handler) {
	if v1 == nil || userHandler == nil {
		return
	}

	v1.POST("/auth/login", userHandler.Login)
	v1.POST("/auth/logout", userHandler.Logout)
	v1.POST("/auth/refresh", userHandler.RefreshToken)
	v1.GET("/auth/password-policy", userHandler.GetPasswordPolicy)
}
