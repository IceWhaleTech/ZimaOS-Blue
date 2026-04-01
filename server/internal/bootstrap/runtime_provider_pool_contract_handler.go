package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func newRouteRuntimeProviderPoolHandler(options routeRuntimeContractProviderPoolOptions) *providerpool.Handler {
	handler := providerpool.NewHandler(options.providerPool)
	if options.trace != nil {
		options.trace.Mark("provider_pool_handler_created")
	}

	handler.SetMediaPricingLookup(func(modelID string) *providerpool.MediaPricingInfo {
		mp := mediagen.GetMediaModelPricing(modelID)
		if mp == nil {
			return nil
		}
		return &providerpool.MediaPricingInfo{Price: mp.OutputPrice, Unit: string(mp.Unit)}
	})
	if options.trace != nil {
		options.trace.Mark("provider_pool_media_pricing_lookup_wired")
	}

	options.providerPool.SetMediaPricingApplier(func(modelID string, outputPrice float64, unit string) {
		mediagen.SetMediaModelPricing(modelID, &mediagen.MediaModelPricing{
			OutputPrice: outputPrice,
			Unit:        mediagen.MediaPricingUnit(unit),
		})
	})
	if options.trace != nil {
		options.trace.Mark("provider_pool_media_pricing_applier_wired")
	}

	return handler
}

func registerRouteRuntimeProviderPoolRoutes(
	protected *echo.Group,
	requirePagePermission func(string) echo.MiddlewareFunc,
	handler *providerpool.Handler,
	trace *StartupTrace,
) {
	if protected == nil || handler == nil {
		return
	}

	pageMiddleware := routeRuntimePageMiddleware(requirePagePermission, permission.PageProviders)
	handler.RegisterRoutes(protected.Group("/providers", filterRouteMiddlewares(pageMiddleware)...))
	handler.RegisterModelRoutes(protected.Group("/models", filterRouteMiddlewares(pageMiddleware)...))
	handler.RegisterIDERoutes(protected.Group("/ide", filterRouteMiddlewares(pageMiddleware)...))
	handler.RegisterPricingRoutes(protected.Group("/pricing", filterRouteMiddlewares(pageMiddleware)...))
	handler.RegisterConfigRoutes(protected.Group("/config", filterRouteMiddlewares(pageMiddleware)...))
	if trace != nil {
		trace.Mark("provider_pool_routes_registered")
	}
}

func startRouteRuntimeProviderPool(pool *providerpool.Pool) {
	if pool == nil {
		return
	}
	serverpkg.OnServerStart(func(int) {
		go pool.Start(context.Background())
	})
}
