package bootstrap

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
)

func newRuntimeProxyDataMasker() *proxy.DataMasker {
	dataMasker := proxy.NewDataMasker(nil)
	for _, rule := range proxy.GetDefaultRules() {
		_ = dataMasker.AddRule(rule)
	}
	return dataMasker
}

func registerRuntimeProxyMaskingRoutes(options runtimeProxyMaskingRoutesOptions) bool {
	if options.v1 == nil || options.dataMasker == nil {
		return false
	}

	maskingGroup := options.v1.Group("/proxy/masking", filterRouteMiddlewares(options.authMiddleware, options.pageMiddleware)...)
	maskingGroup.GET("/stats", func(c echo.Context) error {
		return c.JSON(200, options.dataMasker.Stats())
	})
	maskingGroup.GET("/rules", func(c echo.Context) error {
		return c.JSON(200, map[string]interface{}{
			"rules":         options.dataMasker.ListRules(),
			"default_rules": proxy.GetDefaultRules(),
		})
	})
	maskingGroup.POST("/rules", func(c echo.Context) error {
		var rule proxy.MaskingRule
		if err := c.Bind(&rule); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid request"})
		}
		if err := options.dataMasker.AddRule(&rule); err != nil {
			return c.JSON(400, map[string]string{"error": err.Error()})
		}
		return c.JSON(201, map[string]interface{}{"message": "rule added", "rule": rule})
	})
	maskingGroup.PUT("/rules/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(400, map[string]string{"error": "id required"})
		}

		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil || req.Enabled == nil {
			return c.JSON(400, map[string]string{"error": "enabled field required"})
		}

		if !options.dataMasker.SetRuleEnabled(id, *req.Enabled) {
			return c.JSON(404, map[string]string{"error": "rule not found"})
		}
		if options.onToggle != nil {
			options.onToggle()
		}
		rule, _ := options.dataMasker.GetRule(id)
		return c.JSON(200, map[string]interface{}{
			"message": "rule updated",
			"rule":    rule,
			"stats":   options.dataMasker.Stats(),
		})
	})
	maskingGroup.DELETE("/rules", func(c echo.Context) error {
		id := c.QueryParam("id")
		if id == "" {
			return c.JSON(400, map[string]string{"error": "id required"})
		}
		if options.dataMasker.RemoveRule(id) {
			return c.JSON(200, map[string]string{"message": "rule removed"})
		}
		return c.JSON(404, map[string]string{"error": "rule not found"})
	})
	maskingGroup.PUT("/toggle", func(c echo.Context) error {
		var req struct {
			Enabled *bool `json:"enabled"`
		}
		if err := c.Bind(&req); err != nil || req.Enabled == nil {
			return c.JSON(400, map[string]string{"error": "enabled field required"})
		}
		options.dataMasker.SetEnabled(*req.Enabled)
		if options.onToggle != nil {
			options.onToggle()
		}
		return c.JSON(200, options.dataMasker.Stats())
	})

	return true
}
