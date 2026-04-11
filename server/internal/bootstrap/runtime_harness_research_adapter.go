package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func newHarnessRuntimeResearchToolAdapter(
	bundle *HarnessRuntimeBundle,
	service *deepresearch.Service,
	workspaceDir string,
) *deepResearchToolAdapter {
	adapter := newDeepResearchToolAdapter(service, harnessRuntimeController(bundle), workspaceDir)
	if adapter != nil {
		adapter.jobStore = newHarnessResearchRuntimeService(harnessRuntimeController(bundle), service)
	}
	return adapter
}
