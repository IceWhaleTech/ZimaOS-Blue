package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func bindRouteRuntimeResearch(
	harnessRuntime *HarnessRuntimeBundle,
	researchService *deepresearch.Service,
	options routeRuntimeContractResearchOptions,
) routeRuntimeContractResearchResult {
	return bindRouteRuntimeResearchSurface(newRouteRuntimeResearchSurface(harnessRuntime, researchService), options)
}
