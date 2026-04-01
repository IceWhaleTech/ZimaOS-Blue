package bootstrap

import (
	"log/slog"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func newRuntimeProxyRoutingSetup(options runtimeProxyRoutingOptions) *runtimeProxyRoutingSetup {
	tierResolver := proxy.NewTierResolver()
	if options.modelCatalog != nil {
		var tierResolverLogOnce sync.Once
		resolveTiersAsync := func(reason string) {
			go func() {
				if tierResolver.Resolve(options.modelCatalog.ListAvailableModels()) {
					tierResolverLogOnce.Do(func() {
						slog.Info("Tier resolver initialized", "reason", reason, "stats", tierResolver.Stats())
					})
				}
			}()
		}
		resolveTiersAsync("startup")
		if options.providerChanges != nil {
			options.providerChanges.AddProviderChangeListener(func(_ *providerpool.Provider, _ string) {
				resolveTiersAsync("provider_change")
			})
		}
	}

	modelRouterCfg := options.modelRouterConfig
	if modelRouterCfg == nil {
		modelRouterCfg = proxy.DefaultModelRouterConfig()
	}
	modelRouter, err := proxy.NewModelRouter(modelRouterCfg)
	if err != nil {
		slog.Warn("Failed to create model router", "error", err)
	} else {
		slog.Info("Model router loaded", "families", len(modelRouterCfg.Families), "rules", len(modelRouterCfg.RegexCustomRules), "enabled", modelRouterCfg.Enabled)
	}

	ruleRoutingCfg := options.ruleRoutingConfig
	if ruleRoutingCfg == nil {
		ruleRoutingCfg = proxy.DefaultRoutingConfig()
	}
	var ruleEngine *proxy.RuleEngine
	if len(ruleRoutingCfg.Rules) > 0 {
		ruleEngine = ruleRoutingCfg.ToRuleEngine(tierResolver)
		slog.Info("Rule engine loaded", "rules", len(ruleRoutingCfg.Rules), "enabled", ruleRoutingCfg.Enabled)
	}

	return &runtimeProxyRoutingSetup{
		tierResolver:   tierResolver,
		modelRouter:    modelRouter,
		ruleEngine:     ruleEngine,
		routingEnabled: ruleRoutingCfg.Enabled,
	}
}

func (s *runtimeProxyRoutingSetup) apply(handler *proxy.ProxyHandler) bool {
	if s == nil || handler == nil {
		return false
	}
	if s.modelRouter != nil {
		handler.SetModelRouter(s.modelRouter)
	}
	if s.ruleEngine != nil {
		handler.SetRuleEngine(s.ruleEngine)
	}
	handler.SetTierResolver(s.tierResolver)
	handler.SetRoutingEnabled(s.routingEnabled)
	return true
}
