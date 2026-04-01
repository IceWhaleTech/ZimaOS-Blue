package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerRouteRuntimePluginSurface(options routeRuntimeContractPlatformSurfaceOptions) {
	registerPluginTools(options.toolRegistry, options.pluginRegistry)

	if options.authPageV1Group == nil {
		return
	}

	pluginHandler := serverpkg.NewPluginHandler(options.pluginRegistry)
	pluginHandler.RegisterRoutes(options.authPageV1Group(permission.PagePlugins))

	pluginStoreHandler := serverpkg.NewPluginStoreHandler(options.pluginStore)
	pluginStoreHandler.RegisterRoutes(options.authPageV1Group(permission.PagePlugins))

	toolStoreHandler := serverpkg.NewToolStoreHandler(options.toolRegistry)
	toolStoreHandler.RegisterRoutes(options.authPageV1Group(permission.PageTools))
}
