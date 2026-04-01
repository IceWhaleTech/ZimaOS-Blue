package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
)

type harnessRuntimeResearchRouteRegistration struct {
	creatorBound              bool
	deepResearchRegistered    bool
	harnessResearchRegistered bool
}

func registerHarnessRuntimeResearchTaskRoutes(
	bundle *HarnessRuntimeBundle,
	deepResearchHandler deepResearchRuntimeRouteTarget,
	deepResearchService *deepresearch.Service,
	workspaceDir string,
	deepResearchGroups []*echo.Group,
	harnessResearchGroups []*echo.Group,
) harnessRuntimeResearchRouteRegistration {
	registration := harnessRuntimeResearchRouteRegistration{}
	if deepResearchHandler == nil {
		return registration
	}
	registration.creatorBound = bindHarnessRuntimeToDeepResearchHandler(bundle, deepResearchHandler, deepResearchService, workspaceDir)
	registration.deepResearchRegistered = registerDeepResearchRouteGroups(deepResearchHandler, deepResearchGroups)
	registration.harnessResearchRegistered = registerHarnessResearchCapabilityRoutes(deepResearchHandler, harnessResearchGroups)
	return registration
}
