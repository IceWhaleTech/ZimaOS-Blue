package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
)

func activateRuntimeActivation(options runtimeActivationOptions) runtimeActivationResult {
	proxyRuntime := activateRuntimeProxyRuntime(newRuntimeProxyRuntimeOptions(options))
	toolSurfaces := activateRuntimeToolSurfaces(newRuntimeActivationToolSurfacesOptions(options, proxyRuntime.agentLLMCaller))
	return newRuntimeActivationResult(proxyRuntime, toolSurfaces)
}

func activateRouteRuntimeActivation(options routeRuntimeActivationOptions) runtimeActivationResult {
	var authMiddleware echo.MiddlewareFunc
	if options.deps != nil && options.deps.AuthMiddleware != nil {
		authMiddleware = options.deps.AuthMiddleware.Authenticate()
	}

	pageProviders := routeRuntimePageMiddleware(options.requirePagePermission, permission.PageProviders)
	registerProxyCacheRoutes(options.v1, authMiddleware, pageProviders)
	return activateRuntimeActivation(newRuntimeActivationOptionsFromRoute(options, authMiddleware, pageProviders))
}
