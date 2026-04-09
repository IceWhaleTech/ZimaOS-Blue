package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"

func registerTaskHarnessSurface(
	bundle *HarnessRuntimeBundle,
	reflectService *selfreflect.Service,
	options runtimeTaskSurfaceOptions,
) runtimeTaskHarnessSurfaceRegistration {
	detailProvider, ok := registerHarnessRuntimeWithDetail(
		bundle,
		newHarnessEvolutionProposalSummaryProvider(reflectService),
		options.execApprovals,
		options.questionMgr,
		runtimeTaskSurfaceProjectionGroups(options, "/harness", options.securityPermission),
		runtimeTaskSurfaceProjectionGroups(options, "", options.chatPermission),
	)
	if !ok {
		return runtimeTaskHarnessSurfaceRegistration{}
	}
	return runtimeTaskHarnessSurfaceRegistration{
		detailProvider:   detailProvider,
		routesRegistered: true,
	}
}
