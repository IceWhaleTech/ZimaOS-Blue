package bootstrap

func newRouteRuntimeOperationalOptions(state *routeRegistrationState) routeRuntimeContractOperationalOptions {
	return routeRuntimeContractOperationalOptions{
		taskSurface: newRouteRuntimeOperationalTaskSurfaceOptions(state),
		activation:  newRouteRuntimeOperationalActivationOptions(state),
		support:     newRouteRuntimeOperationalSupportOptions(state),
		deferred:    newRouteRuntimeOperationalDeferredOptions(state),
	}
}
