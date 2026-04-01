package bootstrap

import "github.com/labstack/echo/v4"

func registerMemoryRoutes(
	v1 *echo.Group,
	memoryHandler memoryRouteRegistrar,
	authMiddleware, pageMiddleware echo.MiddlewareFunc,
	chat layeredMemoryChatTarget,
	runner layeredMemoryAgentTarget,
	reflector layeredMemoryReflectionTarget,
) {
	if v1 == nil {
		return
	}
	if memoryHandler != nil {
		memoryHandler.RegisterRoutes(v1.Group("", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
		bindRuntimeLayeredMemory(memoryHandler, chat, runner, reflector)
		return
	}

	stub := featureDisabled("memory")
	memGroup := v1.Group("/memory", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	memGroup.GET("/stats", stub)
	memGroup.GET("/backend", stub)
	memGroup.POST("/search", stub)
	memGroup.Any("/*", stub)
}
