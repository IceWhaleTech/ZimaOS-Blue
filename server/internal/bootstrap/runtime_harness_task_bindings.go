package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type harnessRuntimeTaskRouteRegistration struct {
	detailProvider            *harnessDetailProvider
	deepResearchRegistered    bool
	harnessResearchRegistered bool
	harnessRoutesRegistered   bool
}

func registerHarnessRuntimeTaskRoutes(
	bundle *HarnessRuntimeBundle,
	deepResearchHandler deepResearchRuntimeRouteTarget,
	deepResearchService *deepresearch.Service,
	workspaceDir string,
	execApprovals *tools.ApprovalManager,
	questionMgr *tools.QuestionManager,
	deepResearchGroups []*echo.Group,
	harnessResearchGroups []*echo.Group,
	harnessGroups []*echo.Group,
	projectionGroups []*echo.Group,
) harnessRuntimeTaskRouteRegistration {
	registration := harnessRuntimeTaskRouteRegistration{}
	researchRegistration := registerHarnessRuntimeResearchTaskRoutes(
		bundle,
		deepResearchHandler,
		deepResearchService,
		workspaceDir,
		deepResearchGroups,
		harnessResearchGroups,
	)
	registration.deepResearchRegistered = researchRegistration.deepResearchRegistered
	registration.harnessResearchRegistered = researchRegistration.harnessResearchRegistered
	if detailProvider, ok := registerHarnessRuntimeWithDetail(bundle, execApprovals, questionMgr, harnessGroups, projectionGroups); ok {
		registration.detailProvider = detailProvider
		registration.harnessRoutesRegistered = true
	}
	return registration
}

func newHarnessRuntimeDetailProvider(execApprovals *tools.ApprovalManager, questionMgr *tools.QuestionManager) *harnessDetailProvider {
	return &harnessDetailProvider{
		execApprovals: execApprovals,
		questionMgr:   questionMgr,
	}
}

func registerHarnessRuntimeWithDetail(
	bundle *HarnessRuntimeBundle,
	execApprovals *tools.ApprovalManager,
	questionMgr *tools.QuestionManager,
	harnessGroups []*echo.Group,
	projectionGroups []*echo.Group,
) (*harnessDetailProvider, bool) {
	if harnessRuntimeController(bundle) == nil {
		return nil, false
	}
	detailProvider := newHarnessRuntimeDetailProvider(execApprovals, questionMgr)
	if !registerHarnessRuntimeRoutes(bundle, detailProvider, harnessGroups, projectionGroups) {
		return nil, false
	}
	return detailProvider, true
}

func registerHarnessRuntimeRoutes(
	bundle *HarnessRuntimeBundle,
	detailProvider *harnessDetailProvider,
	harnessGroups []*echo.Group,
	projectionGroups []*echo.Group,
) bool {
	controller := harnessRuntimeController(bundle)
	if controller == nil {
		return false
	}

	harnessHandler := harness.NewHandler(controller)
	harnessHandler.SetDetailProvider(detailProvider)
	for _, group := range harnessGroups {
		if group != nil {
			harnessHandler.RegisterRoutes(group)
		}
	}

	projectionHandler := harness.NewUserTaskProjectionHandler(controller, detailProvider)
	for _, group := range projectionGroups {
		if group != nil {
			projectionHandler.RegisterRoutes(group)
		}
	}
	return true
}
