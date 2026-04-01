package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
)

type routeRegistrar interface {
	RegisterRoutes(*echo.Group)
}

type securityRouteRegistrar interface {
	routeRegistrar
	SetScannerConfig(*security.ScannerConfig)
	SetKVStore(kvstore.Store)
	SetDataDir(string)
}

type workflowRouteRegistrar interface {
	RegisterRoutes(*echo.Echo)
	SetServiceInitHook(func(*workflow.WorkflowService))
	SetRouteMiddlewares(...echo.MiddlewareFunc)
}

type metricsRecorderTarget interface {
	SetMetricsRecorder(server.MetricsRecorder)
}

type memoryRouteRegistrar interface {
	routeRegistrar
	layeredMemoryReadyTarget
}

type channelConfigRouteRegistrar interface {
	routeRegistrar
	SetManager(*channel.Manager)
	SetFactory(*server.ChannelFactory)
}

type externalAuthRouteRegistrar interface {
	RegisterRoutes(*echo.Group)
	RegisterProtectedRoutes(*echo.Group)
}
