package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func bindRuntimeProxyPersistenceHooks(
	failoverAPIHandler *proxy.FailoverAPIHandler,
	prunerHandler *pruner.APIHandler,
	proxyHandler *proxy.ProxyHandler,
	persistence runtimeProxyTogglePersistence,
) func() {
	saveToggle := func() {
		_ = persistence.SaveWithTimeout(context.Background())
	}

	if prunerHandler != nil {
		if persistence.enabled() {
			prunerHandler.SetOnToggle(func(bool) { saveToggle() })
			prunerHandler.SetOnBackendChange(func(string) { saveToggle() })
		}
		prunerHandler.SetOnMiddlewareCreated(func(mw *pruner.Middleware) {
			if proxyHandler != nil {
				proxyHandler.SetPruner(mw)
			}
		})
	}

	if failoverAPIHandler != nil {
		failoverAPIHandler.SetOnProviderRaceChange(func(cfg proxy.ProviderRaceConfig) {
			if proxyHandler != nil {
				proxyHandler.SetProviderRaceConfig(cfg)
			}
		})
		failoverAPIHandler.SetOnConfigSave(func(_ *proxy.FailoverConfig) error {
			return persistence.SaveWithTimeout(context.Background())
		})
	}

	return saveToggle
}
