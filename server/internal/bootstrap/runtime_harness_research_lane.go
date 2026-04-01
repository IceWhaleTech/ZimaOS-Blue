package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func registerDeepResearchRuntimeRoutes(
	bundle *HarnessRuntimeBundle,
	handler deepResearchRuntimeRouteTarget,
	service *deepresearch.Service,
	workspaceDir string,
	groups []*echo.Group,
) bool {
	if handler == nil {
		return false
	}
	_ = bindHarnessRuntimeToDeepResearchHandler(bundle, handler, service, workspaceDir)
	return registerDeepResearchRouteGroups(handler, groups)
}

func newHarnessRuntimeAutoHarnessTurnHook(bundle *HarnessRuntimeBundle, handler *serverpkg.ChatHandler) serverpkg.TurnHook {
	if handler == nil {
		return nil
	}
	submitter := newHarnessAutoHarnessSubmitter(harnessRuntimeController(bundle))
	if submitter == nil {
		return nil
	}
	return serverpkg.NewAutoHarnessTurnHook(handler, submitter)
}
