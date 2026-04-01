package bootstrap

import "github.com/labstack/echo/v4"

func routeRuntimeAuthGroup(resolver func(string) *echo.Group, page string) *echo.Group {
	if resolver == nil {
		return nil
	}
	return resolver(page)
}

func registerRouteRuntimeCompanionRoutes(
	v1, companionV1Group, companionAPIGroup *echo.Group,
	companionHandler, companionWSHandler companionGroupRouteRegistrar,
) companionRouteRegistration {
	registration := companionRouteRegistration{}

	if companionHandler != nil {
		if companionV1Group != nil {
			companionHandler.RegisterGroupRoutes(companionV1Group)
		}
		if companionAPIGroup != nil {
			companionHandler.RegisterCompatGroupRoutes(companionAPIGroup)
		}
		registration.handlerRegistered = true
	}
	if companionWSHandler != nil {
		if companionV1Group != nil {
			companionWSHandler.RegisterGroupRoutes(companionV1Group)
		}
		if companionAPIGroup != nil {
			companionWSHandler.RegisterCompatGroupRoutes(companionAPIGroup)
		}
		registration.wsRegistered = true
	}
	if companionHandler == nil && v1 != nil {
		stub := featureDisabled("companion")
		v1.GET("/companion/stream", stub)
	}

	return registration
}
