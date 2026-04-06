package bootstrap

func registerRuntimeTaskSurface(
	contract runtimeCapabilityTaskSurface,
	options runtimeTaskSurfaceOptions,
) runtimeTaskSurfaceRegistration {
	registration := finalizeRuntimeTaskSurfaceRegistration(runtimeTaskSurfaceRegistration{
		research:                 registerRuntimeTaskResearchSurface(newRuntimeTaskResearchSurfaceOptions(contract, options)),
		knowledge:                registerRuntimeTaskKnowledgeSurface(options),
		harness:                  registerTaskHarnessSurface(contract.HarnessRuntime(), options),
		selfReflectRoutesApplied: registerReflectTaskSurface(contract.ReflectService(), runtimeTaskSurfaceProjectionGroups(options, "", options.chatPermission)),
	})
	bindTaskSurfaceProjectionService(contract, options, registration.detailProvider)
	logTaskSurfaceRegistration(options, registration)
	return registration
}
