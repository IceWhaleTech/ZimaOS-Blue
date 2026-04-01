package bootstrap

func registerRuntimeTaskSurface(
	contract runtimeCapabilityTaskSurface,
	options runtimeTaskSurfaceOptions,
) runtimeTaskSurfaceRegistration {
	registration := runtimeTaskSurfaceRegistration{}

	registration.research = registerRuntimeTaskResearchSurface(newRuntimeTaskResearchSurfaceOptions(contract, options))
	registration.deepResearchRegistered = registration.research.deepResearchRegistered
	registration.harnessResearchRegistered = registration.research.harnessResearchRegistered
	registration.harness = registerTaskHarnessSurface(contract.HarnessRuntime(), options)
	registration.detailProvider = registration.harness.detailProvider
	registration.harnessRoutesRegistered = registration.harness.routesRegistered
	registration.selfReflectRoutesApplied = registerReflectTaskSurface(
		contract.ReflectService(),
		runtimeTaskSurfaceProjectionGroups(options, "", options.chatPermission),
	)
	logTaskSurfaceRegistration(options, registration)
	return registration
}
