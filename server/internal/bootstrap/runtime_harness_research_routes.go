package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

func bindHarnessRuntimeToDeepResearchHandler(
	bundle *HarnessRuntimeBundle,
	handler deepResearchJobCreatorTarget,
	service *deepresearch.Service,
	workspaceDir string,
) bool {
	if handler == nil {
		return false
	}
	if creator := newHarnessResearchCreator(harnessRuntimeController(bundle), service, workspaceDir); creator != nil {
		handler.SetJobCreator(creator)
		return true
	}
	return false
}

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
