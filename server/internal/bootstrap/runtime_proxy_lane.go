package bootstrap

func activateRuntimeProxyLane(options runtimeProxyLaneOptions) *runtimeProxyLaneBundle {
	if options.routingConfig == nil {
		return nil
	}

	surface := newRuntimeProxySurfaceBundle(runtimeProxySurfaceOptions{
		protected:        options.protected,
		failoverGuard:    options.failoverGuard,
		routingConfig:    options.routingConfig,
		connectionConfig: options.connectionConfig,
		dataMasker:       options.dataMasker,
		sttService:       options.sttService,
	})
	if surface == nil {
		return nil
	}

	bundle := &runtimeProxyLaneBundle{
		handler:       surface.handler,
		smartFailover: surface.smartFailover,
		surface:       surface,
	}

	bundle.pipelineStats = newRuntimeProxyPipelineStatsCollector(options.metricsWriter, options.fallbackWriteDB, options.fallbackReadDB, bundle.handler, bundle.smartFailover, options.closers)
	bindRuntimeProxyProviderBindings(newRuntimeProxyLaneProviderBindingsOptions(options, bundle, surface))

	bundle.prunerRuntime = newRuntimeProxyPrunerRuntime(runtimeProxyPrunerOptions{
		v1:             options.v1,
		authMiddleware: options.authMiddleware,
		pageMiddleware: options.pageMiddleware,
		dataDir:        options.dataDir,
		initialConfig:  options.prunerConfig,
		handler:        bundle.handler,
	})

	newRuntimeProxyRoutingSetup(newRuntimeProxyLaneRoutingOptions(options)).apply(bundle.handler)
	bundle.persistence, bundle.saveToggle = bindRuntimeProxyLanePersistence(options, bundle, surface)

	registerRuntimeProxyGatewayRoutes(options.e, bundle.handler)
	registerRuntimeProxyControlRoutes(runtimeProxyControlRoutes{
		v1:             options.v1,
		authMiddleware: options.authMiddleware,
		pageMiddleware: options.pageMiddleware,
		handler:        bundle.handler,
		pipelineStats:  bundle.pipelineStats,
		persistence:    bundle.persistence,
	})
	registerRuntimeProxyRestrictionRoutes(options.restrictionGroup, bundle.handler)

	return bundle
}
