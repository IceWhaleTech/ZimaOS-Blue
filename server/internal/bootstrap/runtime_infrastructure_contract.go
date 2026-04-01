package bootstrap

func (binding *runtimeContractBinding) BindInfrastructureRuntime(options routeRuntimeContractInfrastructureOptions) routeRuntimeContractInfrastructureResult {
	if binding == nil {
		return routeRuntimeContractInfrastructureResult{}
	}
	return bindRouteRuntimeInfrastructure(binding, options)
}

func bindRouteRuntimeInfrastructure(binding routeRuntimeInfrastructureBinding, options routeRuntimeContractInfrastructureOptions) routeRuntimeContractInfrastructureResult {
	binding.BindTLSRuntime(options.tls)
	result := routeRuntimeContractInfrastructureResult{
		tlsConfigured: options.tls.e != nil && options.tls.config != nil,
		media:         binding.BindMediaRuntime(options.media),
		gateway:       binding.BindGatewayRuntime(options.gateway),
	}
	binding.BindChatSurfaceRuntime(options.chatSurface)
	result.chatSurfaceBound = options.chatSurface.chatHandler != nil && options.chatSurface.v1 != nil
	return result
}
