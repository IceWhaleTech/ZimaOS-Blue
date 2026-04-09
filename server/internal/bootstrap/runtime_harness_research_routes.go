package bootstrap

import (
	"github.com/labstack/echo/v4"
)

func registerDeepResearchRouteGroups(handler deepResearchRuntimeRouteTarget, groups []*echo.Group) bool {
	if handler == nil {
		return false
	}
	registered := false
	for _, group := range groups {
		if group == nil {
			continue
		}
		handler.RegisterGroup(group)
		registered = true
	}
	return registered
}

func registerHarnessResearchCapabilityRoutes(handler deepResearchRuntimeRouteTarget, groups []*echo.Group) bool {
	return registerDeepResearchRouteGroups(handler, groups)
}
