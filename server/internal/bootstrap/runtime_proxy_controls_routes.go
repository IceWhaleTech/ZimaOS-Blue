package bootstrap

import (
	"github.com/labstack/echo/v4"
)

func registerRuntimeProxyControlRoutes(options runtimeProxyControlRoutes) bool {
	if options.v1 == nil || options.handler == nil {
		return false
	}

	saveToggle := func(c echo.Context) {
		if c == nil {
			return
		}
		_ = options.persistence.SaveWithTimeout(c.Request().Context())
	}

	registerRuntimeProxyRoutingRoutes(options, saveToggle)
	registerRuntimeProxyPromptCacheRoutes(options, saveToggle)

	return true
}
