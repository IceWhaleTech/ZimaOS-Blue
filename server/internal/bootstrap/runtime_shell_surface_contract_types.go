package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
)

type runtimeWorkerStatsSource interface {
	Stats() worker.Stats
}

type routeRuntimeContractShellSurfaceOptions struct {
	e                     *echo.Echo
	v1                    *echo.Group
	protected             *echo.Group
	authPageV1Group       func(string) *echo.Group
	authMiddleware        echo.MiddlewareFunc
	requirePagePermission func(string) echo.MiddlewareFunc
	serverConfig          *ServerConfig
	logger                *zap.Logger
	mediaDir              string
	workerPool            runtimeWorkerStatsSource
	hotReloader           *config.HotReloader
	configStore           *config.ConfigStore
	formfillerHandler     routeRegistrar
	extauthHandler        externalAuthRouteRegistrar
	toolRegistry          *tools.Registry
	a2uiManager           *a2ui.Manager
	pdfService            tools.PDFService
}
