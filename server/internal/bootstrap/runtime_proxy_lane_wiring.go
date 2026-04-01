package bootstrap

func newRuntimeProxyLaneProviderBindingsOptions(
	options runtimeProxyLaneOptions,
	bundle *runtimeProxyLaneBundle,
	surface *runtimeProxySurfaceBundle,
) runtimeProxyProviderBindingsOptions {
	var router runtimeProxyFailoverCallbackTarget
	var registry runtimeProxyProviderRegistryTarget
	if options.providerPool != nil {
		router = options.providerPool.Router
		registry = options.providerPool.Registry
	}
	return runtimeProxyProviderBindingsOptions{
		handler:       bundle.handler,
		providerPool:  options.providerPool,
		router:        router,
		registry:      registry,
		oauthManager:  options.oauthManager,
		apiKeys:       options.apiKeyService,
		smartFailover: bundle.smartFailover,
		pipelineStats: bundle.pipelineStats,
		broker:        options.sseBroker,
		connPool:      surface.connPool,
	}
}

func newRuntimeProxyLaneRoutingOptions(options runtimeProxyLaneOptions) runtimeProxyRoutingOptions {
	var modelCatalog runtimeProxyAvailableModelsSource
	var providerChanges runtimeProxyProviderChangeListener
	if options.providerPool != nil {
		modelCatalog = options.providerPool.Router
		providerChanges = options.providerPool.Registry
	}
	return runtimeProxyRoutingOptions{
		modelRouterConfig: options.modelRouterConfig,
		ruleRoutingConfig: options.ruleRoutingConfig,
		modelCatalog:      modelCatalog,
		providerChanges:   providerChanges,
	}
}

func bindRuntimeProxyLanePersistence(
	options runtimeProxyLaneOptions,
	bundle *runtimeProxyLaneBundle,
	surface *runtimeProxySurfaceBundle,
) (runtimeProxyTogglePersistence, func()) {
	persistence := newRuntimeProxyTogglePersistence(
		options.kv,
		bundle.handler,
		options.dataMasker,
		bundle.prunerRuntime.config,
		bundle.prunerRuntime.current,
		func() { bundle.prunerRuntime.ensure() },
		&options.routingConfig.Failover,
	)
	saveToggle := bindRuntimeProxyPersistenceHooks(
		surface.failoverAPIHandler,
		bundle.prunerRuntime.handler,
		bundle.handler,
		persistence,
	)
	return persistence, saveToggle
}
