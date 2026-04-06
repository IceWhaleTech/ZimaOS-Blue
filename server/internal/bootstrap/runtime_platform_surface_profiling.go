package bootstrap

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/profiling"
)

func registerRouteRuntimeProfilingSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	if options.e == nil || options.appConfig == nil {
		return
	}

	cfg := options.appConfig.Performance.Profiling
	if !cfg.PprofEnabled {
		return
	}

	path := strings.TrimSpace(cfg.PprofPath)
	if path == "" {
		path = "/debug/pprof"
	}

	profiler := profiling.New(&profiling.Config{
		Enabled:        true,
		EndpointPrefix: path,
		AuthRequired:   options.authMiddleware != nil,
	})
	if options.authMiddleware != nil {
		profiler.RegisterRoutes(options.e, options.authMiddleware)
		return
	}
	profiler.RegisterRoutes(options.e)
}
