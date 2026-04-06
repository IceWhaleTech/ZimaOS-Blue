package bootstrap

func (binding *runtimeContractBinding) BindPlatformSurfaceRuntime(options routeRuntimeContractPlatformSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimePlatformSurfaces(options)
}

func bindRouteRuntimePlatformSurfaces(options routeRuntimeContractPlatformSurfaceOptions) {
	registerRouteRuntimeProfilingSurface(options)
	registerRouteRuntimeBillingSurface(options)
	registerRouteRuntimeNetworkSurface(options)
	registerRouteRuntimeMetricsSurface(options)
	registerRouteRuntimeSystemSurface(options)
	registerRouteRuntimePluginSurface(options)
}
