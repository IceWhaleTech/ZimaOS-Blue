package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerBillingRoutes(protected *echo.Group, billingHandler routeRegistrar) {
	if protected == nil {
		return
	}

	billingGroup := protected.Group("/billing")
	if routeRuntimeHasValue(billingHandler) {
		billingHandler.RegisterRoutes(billingGroup)
		return
	}

	stub := featureDisabled("billing")
	billingGroup.GET("/summary", stub)
	billingGroup.GET("/lines", stub)
	billingGroup.GET("/export", stub)
	billingGroup.Any("/*", stub)
}

func registerMetricsRoutes(
	v1 *echo.Group,
	summaryHandler, detailedHandler routeRegistrar,
	authMiddleware, pageMiddleware echo.MiddlewareFunc,
	metricsWriter *metrics.MetricsWriter,
	metricsTarget metricsRecorderTarget,
) {
	if v1 == nil {
		return
	}

	if routeRuntimeHasValue(summaryHandler) {
		summaryHandler.RegisterRoutes(v1.Group("", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
	}
	if routeRuntimeHasValue(detailedHandler) {
		detailedHandler.RegisterRoutes(v1.Group("/metrics", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		if routeRuntimeHasValue(metricsTarget) && metricsWriter != nil {
			metricsTarget.SetMetricsRecorder(metricsWriter)
		}
	}
	if routeRuntimeHasValue(summaryHandler) || routeRuntimeHasValue(detailedHandler) {
		return
	}

	stub := featureDisabled("metrics")
	metricsGroup := v1.Group("/metrics", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	metricsGroup.GET("/summary", stub)
	metricsGroup.GET("/all", stub)
	metricsGroup.Any("/*", stub)
}

func registerProviderUnavailableRoutes(protected *echo.Group, pageMiddleware echo.MiddlewareFunc) {
	if protected == nil {
		return
	}

	stub := featureDisabled("providers")
	providersGroup := protected.Group("/providers", filterRouteMiddlewares(pageMiddleware)...)
	providersGroup.GET("", stub)
	providersGroup.Any("/*", stub)

	modelsGroup := protected.Group("/models", filterRouteMiddlewares(pageMiddleware)...)
	modelsGroup.GET("", stub)

	ideGroup := protected.Group("/ide", filterRouteMiddlewares(pageMiddleware)...)
	ideGroup.GET("/scan", stub)
	ideGroup.Any("/*", stub)

	pricingGroup := protected.Group("/pricing", filterRouteMiddlewares(pageMiddleware)...)
	pricingGroup.GET("", stub)
	pricingGroup.Any("/*", stub)

	failoverStub := featureDisabled("proxy_failover")
	failoverGroup := protected.Group("/proxy/failover", filterRouteMiddlewares(pageMiddleware)...)
	failoverGroup.GET("/config", failoverStub)
	failoverGroup.GET("/metrics", failoverStub)
	failoverGroup.GET("/breakers", failoverStub)
	failoverGroup.Any("/*", failoverStub)
}

func registerProxyCacheRoutes(v1 *echo.Group, authMiddleware, pageMiddleware echo.MiddlewareFunc) {
	if v1 == nil {
		return
	}

	stub := featureDisabled("proxy_cache")
	cacheGroup := v1.Group("/proxy/cache", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	cacheGroup.GET("/stats", stub)
	cacheGroup.GET("/config", stub)
	cacheGroup.Any("/*", stub)
}

func registerChannelConfigRoutes(
	api *echo.Group,
	channelHandler channelConfigRouteRegistrar,
	authMiddleware, pageMiddleware echo.MiddlewareFunc,
	manager *channel.Manager,
	factory *server.ChannelFactory,
) {
	if api == nil {
		return
	}

	if routeRuntimeHasValue(channelHandler) {
		channelHandler.SetManager(manager)
		channelHandler.SetFactory(factory)
		channelHandler.RegisterRoutes(api.Group("", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		return
	}

	stub := featureDisabled("channels")
	api.Group("", filterRouteMiddlewares(authMiddleware, pageMiddleware)...).GET("/channels", stub)
}
