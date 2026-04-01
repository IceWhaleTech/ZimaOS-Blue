package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

type routeRuntimeResearchSurface interface {
	HarnessRuntime() *HarnessRuntimeBundle
	ResearchService() *deepresearch.Service
}

type routeRuntimeResearchAdapter struct {
	harnessRuntime  *HarnessRuntimeBundle
	researchService *deepresearch.Service
}

func newRouteRuntimeResearchSurface(
	harnessRuntime *HarnessRuntimeBundle,
	researchService *deepresearch.Service,
) routeRuntimeResearchSurface {
	return routeRuntimeResearchAdapter{harnessRuntime: harnessRuntime, researchService: researchService}
}

func (adapter routeRuntimeResearchAdapter) HarnessRuntime() *HarnessRuntimeBundle {
	return adapter.harnessRuntime
}

func (adapter routeRuntimeResearchAdapter) ResearchService() *deepresearch.Service {
	return adapter.researchService
}
