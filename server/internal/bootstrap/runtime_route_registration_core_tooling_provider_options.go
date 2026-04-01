package bootstrap

func newRouteRuntimeCoreProviderOptions(state *routeRegistrationState) routeRuntimeContractProviderPoolOptions {
	return routeRuntimeContractProviderPoolOptions{
		e:                     state.e,
		protected:             state.protected,
		providerPool:          state.deps.ProviderPool,
		writeDB:               state.runtimeWriteDB,
		readDB:                state.runtimeReadDB,
		dataDir:               state.dataDir,
		logger:                state.logger,
		trace:                 state.trace,
		requirePagePermission: state.authSurface.requirePagePermission,
	}
}
