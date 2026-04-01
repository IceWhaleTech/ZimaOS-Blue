package bootstrap

import (
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
)

func newRuntimeProxyPrunerRuntime(options runtimeProxyPrunerOptions) *runtimeProxyPrunerRuntime {
	cfg := pruner.DefaultConfig()
	if options.initialConfig != nil {
		cfg = *options.initialConfig
	}
	cfg.Backend = pruner.NormalizeBackendName(cfg.Backend)
	cfg.ModelDir = filepath.Join(options.dataDir, "models")

	runtime := &runtimeProxyPrunerRuntime{
		config: &cfg,
	}

	var current *pruner.Middleware
	var handler *pruner.APIHandler
	var mu sync.Mutex
	ensure := func() *pruner.Middleware {
		mu.Lock()
		defer mu.Unlock()
		if current != nil {
			return current
		}
		backend, err := pruner.NewBackend(cfg)
		if err != nil {
			slog.Warn("Failed to create pruner backend", "error", err)
			return nil
		}
		current = pruner.NewMiddleware(backend, cfg, pruner.NewStats())
		if handler != nil {
			handler.SetMiddleware(current)
		}
		slog.Info("Context pruner initialized (lazy)", "backend", cfg.Backend, "threshold", cfg.Threshold)
		return current
	}

	prunerModelMgr := pruner.NewPrunerModelManager(cfg.ModelDir)
	handler = pruner.NewAPIHandler(current, &cfg, prunerModelMgr)
	runtime.handler = handler
	runtime.current = func() *pruner.Middleware { return current }
	runtime.ensure = ensure

	if options.handler != nil && cfg.Enabled {
		options.handler.SetPrunerFactory(ensure)
	}
	if options.v1 != nil {
		prunerGroup := options.v1.Group("/proxy/pruner", filterRouteMiddlewares(options.authMiddleware, options.pageMiddleware)...)
		handler.RegisterRoutes(prunerGroup)
	}

	return runtime
}
