package bootstrap

func newRouteRuntimeOperationalActivationOptions(state *routeRegistrationState) routeRuntimeContractActivationOptions {
	return routeRuntimeContractActivationOptions{
		e:                     state.e,
		v1:                    state.v1,
		protected:             state.protected,
		deps:                  state.deps,
		logger:                state.logger,
		runtimeLLM:            state.runtimeLLM,
		oauthManager:          state.providerRuntime.oauthManager,
		requirePagePermission: state.authSurface.requirePagePermission,
		authPageV1Group:       state.authSurface.authPageV1Group,
	}
}
