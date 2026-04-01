package bootstrap

import "github.com/labstack/echo/v4"

func registerRuntimeProxyPromptCacheRoutes(options runtimeProxyControlRoutes, saveToggle func(echo.Context)) {
	promptCacheGroup := options.v1.Group("/proxy/prompt-cache", filterRouteMiddlewares(options.authMiddleware, options.pageMiddleware)...)
	promptCacheGroup.GET("/config", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"enabled": options.handler.IsPromptCacheEnabled(),
		})
	})
	promptCacheGroup.GET("/stats", func(c echo.Context) error {
		return c.JSON(200, options.handler.GetPromptCacheStats())
	})
	promptCacheGroup.PUT("/config", func(c echo.Context) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		if req.Enabled != nil {
			options.handler.SetPromptCacheEnabled(*req.Enabled)
			saveToggle(c)
		}
		return c.JSON(200, map[string]interface{}{
			"success": true,
			"enabled": options.handler.IsPromptCacheEnabled(),
		})
	})
}
