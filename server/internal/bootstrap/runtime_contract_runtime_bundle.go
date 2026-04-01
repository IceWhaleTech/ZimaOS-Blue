package bootstrap

type runtimeContractRuntimeBundle struct {
	capabilities runtimeCapabilitySurface
	taskSurface  runtimeTaskSurfaceRegistration
}

var _ routeRuntimeResearchSurface = (*runtimeContractRuntimeBundle)(nil)

func newRuntimeContractRuntimeBundle(capabilities runtimeCapabilitySurface) runtimeContractRuntimeBundle {
	return runtimeContractRuntimeBundle{capabilities: capabilities}
}
