package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func (binding *runtimeContractBinding) BindProviderPoolRuntime(
	options routeRuntimeContractProviderPoolOptions,
) routeRuntimeContractProviderPoolResult {
	if binding == nil {
		return routeRuntimeContractProviderPoolResult{}
	}
	return bindRouteRuntimeProviderPool(options)
}

func bindRouteRuntimeProviderPool(
	options routeRuntimeContractProviderPoolOptions,
) routeRuntimeContractProviderPoolResult {
	if options.providerPool == nil {
		registerProviderUnavailableRoutes(options.protected, routeRuntimePageMiddleware(options.requirePagePermission, permission.PageProviders))
		return routeRuntimeContractProviderPoolResult{}
	}

	handler := newRouteRuntimeProviderPoolHandler(options)
	oauthManager := bindRouteRuntimeProviderPoolOAuth(options, handler)
	registerRouteRuntimeProviderPoolRoutes(options.protected, options.requirePagePermission, handler, options.trace)
	startRouteRuntimeProviderPool(options.providerPool)
	return routeRuntimeContractProviderPoolResult{oauthManager: oauthManager}
}
