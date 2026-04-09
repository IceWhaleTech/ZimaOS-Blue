package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
)

type runtimeProxySurfaceOptions struct {
	protected        *echo.Group
	failoverGuard    echo.MiddlewareFunc
	routingConfig    *proxy.RouteConfig
	connectionConfig *proxy.ConnectionConfig
	dataMasker       *proxy.DataMasker
	sttService       stt.Service
}

type runtimeProxySurfaceBundle struct {
	handler            *proxy.ProxyHandler
	connPool           *proxy.ConnectionPool
	smartFailover      *proxy.SmartFailoverHandler
	failoverAPIHandler *proxy.FailoverAPIHandler
}

func newRuntimeProxySurfaceBundle(options runtimeProxySurfaceOptions) *runtimeProxySurfaceBundle {
	if options.routingConfig == nil {
		return nil
	}

	proxyRouter := proxy.NewRouter(options.routingConfig)
	connPool := proxy.NewConnectionPool(options.connectionConfig)
	failover := proxy.NewFailoverHandler(&options.routingConfig.Failover, proxyRouter)
	handler := proxy.NewProxyHandler(proxyRouter, connPool, failover)
	handler.SetSTTService(options.sttService)
	handler.SetProviderRaceConfig(options.routingConfig.Failover.ProviderRace)
	handler.SetPromptCacheEnabled(true) // default ON for new installs
	handler.SetDataMasker(options.dataMasker)

	smartFailover := proxy.NewSmartFailoverHandler(&options.routingConfig.Failover, proxyRouter)
	failoverAPIHandler := proxy.NewFailoverAPIHandler(smartFailover, &options.routingConfig.Failover)
	failoverAPIHandler.SetProviderRaceStatsProvider(handler.GetProviderRaceStats)
	if options.protected != nil {
		failoverAPIHandler.RegisterRoutes(options.protected.Group("/proxy/failover", filterRouteMiddlewares(options.failoverGuard)...))
	}

	return &runtimeProxySurfaceBundle{
		handler:            handler,
		connPool:           connPool,
		smartFailover:      smartFailover,
		failoverAPIHandler: failoverAPIHandler,
	}
}

func registerRuntimeProxyGatewayRoutes(e *echo.Echo, handler *proxy.ProxyHandler) bool {
	if e == nil || handler == nil {
		return false
	}

	v1ProxyGroup := e.Group("/v1")
	v1ProxyGroup.Any("/chat/completions", echo.WrapHandler(handler))
	v1ProxyGroup.Any("/completions", echo.WrapHandler(handler))
	v1ProxyGroup.Any("/embeddings", echo.WrapHandler(handler))
	v1ProxyGroup.Any("/models", echo.WrapHandler(handler))
	v1ProxyGroup.Any("/messages", echo.WrapHandler(handler))
	return true
}

func registerRuntimeProxyRestrictionRoutes(group *echo.Group, handler *proxy.ProxyHandler) bool {
	if group == nil || handler == nil {
		return false
	}
	restrictionsHandler := proxy.NewRestrictionsHandler(handler.GetProviderMemory())
	restrictionsHandler.RegisterRoutes(group)
	return true
}
