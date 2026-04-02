package bootstrap

type routeRuntimeOperationalBinding interface {
	RegisterTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration
	ActivateRouteRuntime(options routeRuntimeContractActivationOptions) runtimeActivationResult
	RegisterActivationSupportRoutes(
		activation runtimeActivationResult,
		options routeRuntimeContractSupportOptions,
	)
	BindDeferredSupport(
		activation runtimeActivationResult,
		options routeRuntimeContractDeferredSupportOptions,
	)
}

var _ routeRuntimeOperationalBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractOperationalOptions struct {
	taskSurface runtimeTaskSurfaceOptions
	activation  routeRuntimeContractActivationOptions
	support     routeRuntimeContractSupportOptions
	deferred    routeRuntimeContractDeferredSupportOptions
}

type routeRuntimeContractOperationalSupportResult struct {
	activationSupportApplied bool
	deferredSupportApplied   bool
}

type routeRuntimeContractOperationalResult struct {
	taskSurface runtimeTaskSurfaceRegistration
	activation  runtimeActivationResult
	support     routeRuntimeContractOperationalSupportResult
}
