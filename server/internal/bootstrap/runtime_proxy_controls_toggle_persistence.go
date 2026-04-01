package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func newRuntimeProxyTogglePersistence(
	kv kvstore.Store,
	handler *proxy.ProxyHandler,
	dataMasker *proxy.DataMasker,
	prunerCfg *pruner.Config,
	currentPruner func() *pruner.Middleware,
	ensurePrunerFactory func(),
	failoverConfig *proxy.FailoverConfig,
) runtimeProxyTogglePersistence {
	if kv == nil {
		slog.Warn("Failed to create shared kvstore, toggles will not persist")
		return runtimeProxyTogglePersistence{}
	}

	persistence := runtimeProxyTogglePersistence{
		store: proxy.NewToggleStore(kv),
		snapshot: func() *proxy.ToggleState {
			cfg := pruner.Config{}
			if prunerCfg != nil {
				cfg = *prunerCfg
			}
			current := (*pruner.Middleware)(nil)
			if currentPruner != nil {
				current = currentPruner()
			}
			return newRuntimeProxyToggleState(handler, dataMasker, cfg, current, failoverConfig)
		},
	}

	toggleLoadStart := time.Now()
	saved, loadErr := persistence.store.Load(context.Background())
	slog.Info("Feature toggles load completed", "found", saved != nil, "elapsed", time.Since(toggleLoadStart))
	if loadErr != nil {
		slog.Warn("Failed to load feature toggles", "error", loadErr, "elapsed", time.Since(toggleLoadStart))
		return persistence
	}
	if saved == nil {
		return persistence
	}

	if applyRuntimeProxyToggleState(saved, handler, dataMasker, prunerCfg, currentPruner, ensurePrunerFactory, failoverConfig) {
		slog.Info("Migrated feature toggles to v1 (routing, prompt_cache -> enabled, masking -> disabled)")
		_ = persistence.SaveWithTimeout(context.Background())
	}
	slog.Info("Restored feature toggles",
		"pruner", saved.PrunerEnabled,
		"pruner_backend", saved.PrunerBackend,
		"routing", saved.RoutingEnabled,
		"masking", saved.MaskingEnabled,
		"prompt_cache", saved.PromptCacheEnabled,
	)
	return persistence
}
