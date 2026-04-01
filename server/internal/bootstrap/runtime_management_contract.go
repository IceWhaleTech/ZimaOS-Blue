package bootstrap

import "context"

func (binding *runtimeContractBinding) BindManagementSupport(
	options routeRuntimeContractManagementSupportOptions,
) routeRuntimeContractManagementSupportResult {
	if binding == nil {
		return routeRuntimeContractManagementSupportResult{}
	}
	return bindRouteRuntimeManagementSupport(options)
}

func bindRouteRuntimeManagementSupport(
	options routeRuntimeContractManagementSupportOptions,
) routeRuntimeContractManagementSupportResult {
	ctx := options.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	bindRouteRuntimeRemoteAccessSupport(options)
	updateHandler, otaChecker := bindRouteRuntimeUpdateSupport(ctx, options)
	providerSettings := bindRouteRuntimeProviderSettings(options)

	return routeRuntimeContractManagementSupportResult{
		updateHandler:    updateHandler,
		otaChecker:       otaChecker,
		providerSettings: providerSettings,
	}
}
