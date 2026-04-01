package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/billing"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerRouteRuntimeBillingSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	var billingHandler routeRegistrar
	if options.billingPool != nil && options.billingPool.Storage != nil {
		billingService := billing.NewService(options.billingPool.Storage)
		billingHandler = billing.NewHandler(billingService)
	}
	registerBillingRoutes(options.protected, billingHandler)
}

func registerRouteRuntimeMetricsSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	var metricsSummaryHandler routeRegistrar
	if options.metricsCollector != nil {
		metricsSummaryHandler = serverpkg.NewMetricsHandler(options.metricsCollector)
	}

	var detailedMetricsHandler routeRegistrar
	if options.metricsWriter != nil {
		detailedMetricsHandler = metrics.NewHandler(options.metricsWriter)
	}

	registerMetricsRoutes(
		options.v1,
		metricsSummaryHandler,
		detailedMetricsHandler,
		options.authMiddleware,
		routeRuntimePageMiddleware(options.requirePagePermission, permission.PageHome),
		options.metricsWriter,
		options.metricsTarget,
	)
}
