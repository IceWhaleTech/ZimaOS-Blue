package bootstrap

func newRouteRuntimeInfrastructureTLSOptions(state *routeRegistrationState) routeRuntimeContractTLSOptions {
	return routeRuntimeContractTLSOptions{
		e:        state.e,
		config:   state.deps.Config,
		dataDir:  state.dataDir,
		configKV: state.deps.ConfigKV,
		logger:   state.logger,
	}
}
