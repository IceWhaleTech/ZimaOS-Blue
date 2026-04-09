package bootstrap

import "github.com/labstack/echo/v4"

func runtimeTaskSurfaceProjectionGroups(
	options runtimeTaskSurfaceOptions,
	prefix string,
	middleware echo.MiddlewareFunc,
) []*echo.Group {
	return []*echo.Group{
		runtimeTaskSurfaceGroup(options.protected, prefix, middleware),
		runtimeTaskSurfaceGroup(options.apiProtected, prefix, middleware),
	}
}
