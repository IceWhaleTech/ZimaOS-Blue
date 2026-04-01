package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"

func (binding *runtimeContractBinding) BindUserSurfaceRuntime(options routeRuntimeContractUserSurfaceOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeUserSurface(options)
}

func bindRouteRuntimeUserSurface(options routeRuntimeContractUserSurfaceOptions) {
	registration := registerRouteRuntimeCompanionRoutes(
		options.v1,
		routeRuntimeAuthGroup(options.authPageV1Group, permission.PageSecurity),
		routeRuntimeAuthGroup(options.authPageAPIGroup, permission.PageSecurity),
		options.companionHandler,
		options.companionWSHandler,
	)
	if options.trace != nil {
		if registration.handlerRegistered {
			options.trace.Mark("companion_routes_registered")
		}
		if registration.wsRegistered {
			options.trace.Mark("companion_ws_routes_registered")
		}
	}

	registerRouteRuntimeWorkspaceRoutes(options.protected, options.requirePagePermission, options.workspace, options.logger)
	registerRouteRuntimeMyRoutes(options)
}
