package bootstrap

import (
	"net/http"
	"reflect"

	"github.com/labstack/echo/v4"
)

// featureDisabled returns an echo handler that responds with a standard
// "feature not enabled" JSON payload. This is used as a catch-all for
// optional features whose handler was not initialised at startup so that
// the frontend never sees a raw 404.
func featureDisabled(feature string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": false,
			"feature": feature,
			"status":  "not_initialized",
			"message": feature + " is not enabled",
		})
	}
}

func filterRouteMiddlewares(middlewares ...echo.MiddlewareFunc) []echo.MiddlewareFunc {
	if len(middlewares) == 0 {
		return nil
	}
	filtered := make([]echo.MiddlewareFunc, 0, len(middlewares))
	for _, middleware := range middlewares {
		if middleware != nil {
			filtered = append(filtered, middleware)
		}
	}
	return filtered
}

func routeRuntimeHasValue(value any) bool {
	if value == nil {
		return false
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return !v.IsNil()
	default:
		return true
	}
}
