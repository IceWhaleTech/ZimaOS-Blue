package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerRouteRuntimePluginSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	if options.authPageV1Group == nil {
		return
	}

	toolStoreHandler := serverpkg.NewToolStoreHandler(options.toolRegistry)
	toolStoreHandler.RegisterRoutes(options.authPageV1Group(permission.PageTools))
}
