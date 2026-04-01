package bootstrap

func newRouteRuntimeCoreInfrastructureOptions(state *routeRegistrationState) routeRuntimeContractInfrastructureOptions {
	return routeRuntimeContractInfrastructureOptions{
		tls:         newRouteRuntimeInfrastructureTLSOptions(state),
		media:       newRouteRuntimeInfrastructureMediaOptions(state),
		gateway:     newRouteRuntimeInfrastructureGatewayOptions(state),
		chatSurface: newRouteRuntimeInfrastructureChatSurfaceOptions(state),
	}
}
