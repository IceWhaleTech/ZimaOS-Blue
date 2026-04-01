package bootstrap

func bindRouteRuntimeOperationalSupport(
	binding routeRuntimeOperationalBinding,
	activation runtimeActivationResult,
	options routeRuntimeContractOperationalOptions,
) routeRuntimeContractOperationalSupportResult {
	if binding == nil {
		return routeRuntimeContractOperationalSupportResult{}
	}
	return routeRuntimeContractOperationalSupportResult{
		activationSupportApplied: applyRouteRuntimeOperationalSupport(binding, activation, options.support),
		deferredSupportApplied:   applyRouteRuntimeOperationalDeferredSupport(binding, activation, options.deferred),
	}
}
