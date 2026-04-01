package bootstrap

type routeRuntimeInfrastructureBinding interface {
	BindTLSRuntime(options routeRuntimeContractTLSOptions)
	BindMediaRuntime(options routeRuntimeContractMediaOptions) routeRuntimeContractMediaResult
	BindGatewayRuntime(options routeRuntimeContractGatewayOptions) routeRuntimeContractGatewayResult
	BindChatSurfaceRuntime(options routeRuntimeContractChatSurfaceOptions)
}

var _ routeRuntimeInfrastructureBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractInfrastructureOptions struct {
	tls         routeRuntimeContractTLSOptions
	media       routeRuntimeContractMediaOptions
	gateway     routeRuntimeContractGatewayOptions
	chatSurface routeRuntimeContractChatSurfaceOptions
}

type routeRuntimeContractInfrastructureResult struct {
	tlsConfigured    bool
	media            routeRuntimeContractMediaResult
	gateway          routeRuntimeContractGatewayResult
	chatSurfaceBound bool
}
