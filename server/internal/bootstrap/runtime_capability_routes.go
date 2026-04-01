package bootstrap

import (
	"github.com/labstack/echo/v4"
)

func runtimeTaskSurfaceGroup(parent *echo.Group, prefix string, middleware echo.MiddlewareFunc) *echo.Group {
	if parent == nil {
		return nil
	}
	if middleware != nil {
		return parent.Group(prefix, middleware)
	}
	return parent.Group(prefix)
}
