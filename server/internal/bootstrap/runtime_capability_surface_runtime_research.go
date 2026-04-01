package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func registerRuntimeTaskResearchSurface(options runtimeTaskResearchSurfaceOptions) runtimeTaskResearchSurfaceRegistration {
	if options.research == nil {
		return runtimeTaskResearchSurfaceRegistration{}
	}
	handler := deepresearch.NewHandler(options.research)
	registration := registerHarnessRuntimeResearchTaskRoutes(
		options.harness,
		handler,
		options.research,
		options.workspaceDir,
		options.deepResearchGroups,
		options.harnessResearchGroups,
	)
	return runtimeTaskResearchSurfaceRegistration{
		creatorBound:              registration.creatorBound,
		deepResearchRegistered:    registration.deepResearchRegistered,
		harnessResearchRegistered: registration.harnessResearchRegistered,
	}
}
