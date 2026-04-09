package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func bindHarnessRuntimeToDeepResearchHandler(
	bundle *HarnessRuntimeBundle,
	handler deepResearchJobCreatorTarget,
	service *deepresearch.Service,
	workspaceDir string,
) bool {
	if handler == nil {
		return false
	}
	bound := false
	if runtimeService := newHarnessResearchRuntimeService(harnessRuntimeController(bundle), service); runtimeService != nil {
		handler.SetJobService(runtimeService)
		bound = true
	}
	if creator := newHarnessResearchCreator(harnessRuntimeController(bundle), service, workspaceDir); creator != nil {
		handler.SetJobCreator(creator)
		bound = true
	}
	return bound
}
