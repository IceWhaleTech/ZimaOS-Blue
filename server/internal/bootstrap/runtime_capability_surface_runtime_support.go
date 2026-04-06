package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"

func finalizeRuntimeTaskSurfaceRegistration(registration runtimeTaskSurfaceRegistration) runtimeTaskSurfaceRegistration {
	registration.detailProvider = registration.harness.detailProvider
	registration.deepResearchRegistered = registration.research.deepResearchRegistered
	registration.harnessResearchRegistered = registration.research.harnessResearchRegistered
	registration.knowledgeRoutesRegistered = registration.knowledge.routesRegistered
	registration.knowledgeCronRegistered = registration.knowledge.cronRegistered
	registration.harnessRoutesRegistered = registration.harness.routesRegistered
	return registration
}

func bindTaskSurfaceProjectionService(contract runtimeCapabilityTaskSurface, options runtimeTaskSurfaceOptions, detailProvider *harnessDetailProvider) {
	if detailProvider == nil {
		return
	}
	if controller := harnessRuntimeController(contract.HarnessRuntime()); controller != nil {
		projectionService := harness.NewUserTaskProjectionService(controller, detailProvider)
		if options.chatHandler != nil {
			options.chatHandler.SetTaskProjectionService(projectionService)
		}
		if options.browserHandler != nil {
			options.browserHandler.SetTaskProjectionService(browserOverviewTaskProjectionAdapter{
				service: projectionService,
			})
		}
	}
}
