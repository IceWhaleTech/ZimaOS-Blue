package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

func newRuntimeProxyRuntimeOptions(options runtimeActivationOptions) runtimeProxyRuntimeOptions {
	return runtimeProxyRuntimeOptions{
		e:                     options.e,
		v1:                    options.v1,
		protected:             options.protected,
		restrictionGroup:      options.restrictionGroup,
		failoverGuard:         options.failoverGuard,
		authMiddleware:        options.authMiddleware,
		pageMiddleware:        options.pageMiddleware,
		maskingPageMiddleware: options.maskingPageMiddleware,
		deps:                  options.deps,
		runtimeLLM:            options.runtimeLLM,
		oauthManager:          options.oauthManager,
		logger:                options.logger,
	}
}

func newRuntimeActivationToolSurfacesOptions(options runtimeActivationOptions, agentLLMCaller agent.LLMCaller) runtimeToolSurfacesOptions {
	return runtimeToolSurfacesOptions{
		protected:      options.protected,
		services:       options.services,
		cfg:            options.cfg,
		deps:           options.deps,
		logger:         options.logger,
		agentLLMCaller: agentLLMCaller,
		harnessRuntime: options.harnessRuntime,
		reflectService: options.reflectService,
		skillRegistry:  options.skillRegistry,
		mcpPermission:  options.mcpPermission,
	}
}

func newRuntimeActivationResult(proxyRuntime runtimeProxyRuntimeResult, toolSurfaces runtimeToolSurfacesResult) runtimeActivationResult {
	return runtimeActivationResult{
		lane:           proxyRuntime.lane,
		dataMasker:     proxyRuntime.dataMasker,
		agentLLMCaller: proxyRuntime.agentLLMCaller,
		auxiliaryLLM:   proxyRuntime.auxiliaryLLM,
		agentRunner:    toolSurfaces.agentRunner,
		mcpRegistered:  toolSurfaces.mcpRegistered,
	}
}

func newRuntimeActivationOptionsFromRoute(
	options routeRuntimeActivationOptions,
	authMiddleware echo.MiddlewareFunc,
	pageProviders echo.MiddlewareFunc,
) runtimeActivationOptions {
	var (
		services      *Services
		cfg           *ServerConfig
		skillRegistry *skillpkg.Registry
	)
	if options.deps != nil {
		services = options.deps.Services
		cfg = options.deps.ServerConfig
		if services != nil {
			skillRegistry = services.SkillRegistry
		}
	}

	return runtimeActivationOptions{
		e:                     options.e,
		v1:                    options.v1,
		protected:             options.protected,
		restrictionGroup:      routeRuntimeAuthGroup(options.authPageV1Group, permission.PageProviders),
		failoverGuard:         pageProviders,
		authMiddleware:        authMiddleware,
		pageMiddleware:        pageProviders,
		maskingPageMiddleware: routeRuntimePageMiddleware(options.requirePagePermission, permission.PageSecurity),
		mcpPermission:         []echo.MiddlewareFunc{routeRuntimePageMiddleware(options.requirePagePermission, permission.PageTools)},
		services:              services,
		cfg:                   cfg,
		deps:                  options.deps,
		logger:                options.logger,
		runtimeLLM:            options.runtimeLLM,
		oauthManager:          options.oauthManager,
		harnessRuntime:        options.harnessRuntime,
		reflectService:        options.reflectService,
		skillRegistry:         skillRegistry,
	}
}
