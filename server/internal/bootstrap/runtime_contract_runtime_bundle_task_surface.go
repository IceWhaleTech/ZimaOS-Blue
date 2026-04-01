package bootstrap

func (bundle *runtimeContractRuntimeBundle) RegisterTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration {
	if surface := bundle.capabilitySurface(); surface != nil {
		bundle.taskSurface = surface.registerTaskSurface(options)
		return bundle.taskSurface
	}
	return runtimeTaskSurfaceRegistration{}
}

func (bundle *runtimeContractRuntimeBundle) ApprovalDetailTarget() approvalRuntimeDetailTarget {
	if bundle == nil || bundle.taskSurface.detailProvider == nil {
		return nil
	}
	return bundle.taskSurface.detailProvider
}
