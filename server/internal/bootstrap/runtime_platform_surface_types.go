package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractPlatformSurfaceOptions struct {
	v1                    *echo.Group
	protected             *echo.Group
	authPageV1Group       func(string) *echo.Group
	requirePagePermission func(string) echo.MiddlewareFunc
	authMiddleware        echo.MiddlewareFunc
	serverConfig          *ServerConfig
	logger                *zap.Logger
	billingPool           *providerpool.Pool
	metricsCollector      *metrics.Collector
	metricsWriter         *metrics.MetricsWriter
	metricsTarget         metricsRecorderTarget
	connectionManager     *connection.Manager
	pluginRegistry        *plugin.Registry
	pluginStore           *plugin.Store
	toolRegistry          *tools.Registry
}
