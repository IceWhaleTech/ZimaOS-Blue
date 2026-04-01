package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func registerRuntimeProxyRoutingRoutes(options runtimeProxyControlRoutes, saveToggle func(echo.Context)) {
	routingGroup := options.v1.Group("/proxy/routing", filterRouteMiddlewares(options.authMiddleware, options.pageMiddleware)...)
	routingGroup.GET("/config", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"enabled": options.handler.IsRoutingEnabled(),
		})
	})
	routingGroup.PUT("/config", func(c echo.Context) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		if req.Enabled != nil {
			options.handler.SetRoutingEnabled(*req.Enabled)
			saveToggle(c)
		}
		return c.JSON(200, map[string]interface{}{
			"success": true,
			"enabled": options.handler.IsRoutingEnabled(),
		})
	})
	routingGroup.GET("/rules", func(c echo.Context) error {
		rules := options.handler.GetRoutingRules()
		if rules == nil {
			rules = []proxy.RoutingRule{}
		}
		return c.JSON(200, map[string]interface{}{
			"rules": rules,
		})
	})
	routingGroup.PUT("/rules/:name", func(c echo.Context) error {
		name := c.Param("name")
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		if req.Enabled == nil {
			return c.JSON(400, map[string]string{"error": "enabled field required"})
		}
		if !options.handler.SetRoutingRuleEnabled(name, *req.Enabled) {
			return c.JSON(404, map[string]string{"error": "rule not found"})
		}
		saveToggle(c)
		return c.JSON(200, map[string]interface{}{
			"success": true,
			"name":    name,
			"enabled": *req.Enabled,
		})
	})
	routingGroup.GET("/stats", func(c echo.Context) error {
		return c.JSON(200, options.handler.GetRoutingStats())
	})

	options.v1.Group("", filterRouteMiddlewares(options.authMiddleware, options.pageMiddleware)...).GET("/proxy/pipeline/stats", func(c echo.Context) error {
		if options.pipelineStats != nil {
			return c.JSON(200, options.pipelineStats.Snapshot())
		}
		return c.JSON(200, map[string]string{"status": "not configured"})
	})
}
