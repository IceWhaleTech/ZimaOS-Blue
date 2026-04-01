package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func newRuntimeProxyToggleState(
	handler *proxy.ProxyHandler,
	dataMasker *proxy.DataMasker,
	prunerCfg pruner.Config,
	currentPruner *pruner.Middleware,
	failoverConfig *proxy.FailoverConfig,
) *proxy.ToggleState {
	state := &proxy.ToggleState{
		PrunerEnabled:      prunerCfg.Enabled && (currentPruner == nil || currentPruner.Enabled()),
		PrunerBackend:      pruner.NormalizeBackendName(prunerCfg.Backend),
		PromptCacheEnabled: handler != nil && handler.IsPromptCacheEnabled(),
		MaskingEnabled:     dataMasker != nil && dataMasker.IsEnabled(),
		Version:            1,
	}
	if handler != nil {
		state.RoutingEnabled = handler.IsRoutingEnabled()
		if rules := handler.GetRoutingRules(); len(rules) > 0 {
			state.RoutingRules = make(map[string]bool, len(rules))
			for _, rule := range rules {
				state.RoutingRules[rule.Name] = rule.Enabled != nil && *rule.Enabled
			}
		}
	}
	if dataMasker != nil {
		if rules := dataMasker.ListRules(); len(rules) > 0 {
			state.MaskingRules = make(map[string]bool, len(rules))
			for _, rule := range rules {
				if rule == nil || strings.TrimSpace(rule.ID) == "" {
					continue
				}
				state.MaskingRules[rule.ID] = rule.Enabled
			}
		}
	}
	if failoverConfig != nil {
		cfg := *failoverConfig
		state.FailoverConfig = &cfg
	}
	return state
}

func applyRuntimeProxyToggleState(
	saved *proxy.ToggleState,
	handler *proxy.ProxyHandler,
	dataMasker *proxy.DataMasker,
	prunerCfg *pruner.Config,
	currentPruner func() *pruner.Middleware,
	ensurePrunerFactory func(),
	failoverConfig *proxy.FailoverConfig,
) bool {
	if saved == nil {
		return false
	}

	migrated := false
	if saved.Version < 1 {
		saved.RoutingEnabled = true
		saved.MaskingEnabled = false
		saved.PromptCacheEnabled = true
		saved.Version = 1
		migrated = true
	}

	if prunerCfg != nil {
		if saved.PrunerBackend != "" {
			prunerCfg.Backend = pruner.NormalizeBackendName(saved.PrunerBackend)
		}
		prunerCfg.Enabled = saved.PrunerEnabled
		current := (*pruner.Middleware)(nil)
		if currentPruner != nil {
			current = currentPruner()
		}
		switch {
		case current != nil:
			current.SetEnabled(saved.PrunerEnabled)
		case saved.PrunerEnabled && ensurePrunerFactory != nil:
			ensurePrunerFactory()
		}
	}

	if handler != nil {
		handler.SetRoutingEnabled(saved.RoutingEnabled)
		handler.SetPromptCacheEnabled(saved.PromptCacheEnabled)
		for name, enabled := range saved.RoutingRules {
			handler.SetRoutingRuleEnabled(name, enabled)
		}
	}

	if dataMasker != nil {
		dataMasker.SetEnabled(saved.MaskingEnabled)
		for id, enabled := range saved.MaskingRules {
			dataMasker.SetRuleEnabled(id, enabled)
		}
	}

	if saved.FailoverConfig != nil && failoverConfig != nil {
		*failoverConfig = *saved.FailoverConfig
		if handler != nil {
			handler.SetProviderRaceConfig(failoverConfig.ProviderRace)
		}
	}

	return migrated
}
