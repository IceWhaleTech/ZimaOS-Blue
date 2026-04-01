package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"

func (binding *runtimeContractBinding) BindShellSurfaceRuntime(options routeRuntimeContractShellSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeShellSurfaces(options)
}

func bindRouteRuntimeShellSurfaces(options routeRuntimeContractShellSurfaceOptions) {
	registerRouteRuntimeHealthSurface(options.v1, options.serverConfig, options.workerPool, options.logger)
	registerRouteRuntimeStaticSurface(options.e, options.v1, options.mediaDir)
	registerRouteRuntimeConfigSurface(options.authPageV1Group, options.v1, options.hotReloader, options.configStore)
	registerFormfillerRoutes(
		options.v1,
		options.formfillerHandler,
		options.authMiddleware,
		routeRuntimePageMiddleware(options.requirePagePermission, permission.PageTools),
	)
	registerExternalAuthRoutes(options.v1, options.protected, options.extauthHandler)
	registerRouteRuntimeCanvasSurface(options.protected, options.toolRegistry, options.a2uiManager, options.pdfService)
	registerRouteRuntimeMFASurface(options.protected)
}
