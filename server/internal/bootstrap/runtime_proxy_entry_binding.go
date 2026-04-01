package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

type runtimeProxyEntryMaskingBinding struct {
	result         runtimeProxyEntryResult
	setMaskingHook func(func())
}

func newRuntimeProxyEntryMaskingBinding(options runtimeProxyEntryOptions) runtimeProxyEntryMaskingBinding {
	result := runtimeProxyEntryResult{
		agentLLMCaller: options.defaultAgentCaller,
		dataMasker:     options.dataMasker,
	}
	if result.dataMasker == nil {
		result.dataMasker = newRuntimeProxyDataMasker()
	}

	var maskingOnToggle func()
	registerRuntimeProxyMaskingRoutes(runtimeProxyMaskingRoutesOptions{
		v1:             options.v1,
		authMiddleware: options.maskingAuth,
		pageMiddleware: options.maskingPage,
		dataMasker:     result.dataMasker,
		onToggle: func() {
			if maskingOnToggle != nil {
				maskingOnToggle()
			}
		},
	})

	return runtimeProxyEntryMaskingBinding{
		result: result,
		setMaskingHook: func(toggle func()) {
			maskingOnToggle = toggle
		},
	}
}

func resolveRuntimeProxyRouteConfig(appConfig *config.Config) *proxy.RouteConfig {
	if appConfig == nil || appConfig.Proxy == nil || !appConfig.Proxy.Enabled {
		return nil
	}
	if appConfig.Proxy.Route != nil {
		return appConfig.Proxy.Route
	}
	return &appConfig.Proxy.Routing
}

func newRuntimeProxyLaneOptions(options runtimeProxyEntryOptions, dataMasker *proxy.DataMasker) *runtimeProxyLaneOptions {
	routingConfig := resolveRuntimeProxyRouteConfig(options.appConfig)
	if routingConfig == nil {
		return nil
	}
	return &runtimeProxyLaneOptions{
		e:                 options.e,
		v1:                options.v1,
		protected:         options.protected,
		restrictionGroup:  options.restrictionGroup,
		failoverGuard:     options.failoverGuard,
		authMiddleware:    options.authMiddleware,
		pageMiddleware:    options.pageMiddleware,
		routingConfig:     routingConfig,
		connectionConfig:  &options.appConfig.Proxy.Connection,
		dataMasker:        dataMasker,
		sttService:        options.sttService,
		kv:                options.kv,
		dataDir:           options.dataDir,
		prunerConfig:      options.prunerConfig,
		modelRouterConfig: options.appConfig.Proxy.ModelRouter,
		ruleRoutingConfig: options.appConfig.Proxy.RuleRouting,
		metricsWriter:     options.metricsWriter,
		fallbackWriteDB:   options.fallbackWriteDB,
		fallbackReadDB:    options.fallbackReadDB,
		closers:           options.closers,
		providerPool:      options.providerPool,
		apiKeyService:     options.apiKeyService,
		sseBroker:         options.sseBroker,
		oauthManager:      options.oauthManager,
	}
}
