package bootstrap

func newRouteRuntimeCoreSupportOptions(state *routeRegistrationState) routeRuntimeContractCoreSupportOptions {
	return routeRuntimeContractCoreSupportOptions{
		ask:        newRouteRuntimeCoreAskSupportOptions(state),
		exec:       newRouteRuntimeCoreExecSupportOptions(state),
		capability: newRouteRuntimeCoreCapabilitySupportOptions(state),
	}
}
