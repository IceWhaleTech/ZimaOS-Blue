package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"

func (binding *runtimeContractBinding) NewRuntimeLLMRef() *runtimeLLMProviderRef {
	if binding == nil {
		return nil
	}
	return newRuntimeLLMProviderRef()
}

func (binding *runtimeContractBinding) HarnessRuntime() *HarnessRuntimeBundle {
	if binding == nil {
		return nil
	}
	return binding.runtime.HarnessRuntime()
}

func (binding *runtimeContractBinding) ResearchService() *deepresearch.Service {
	if binding == nil {
		return nil
	}
	return binding.runtime.ResearchService()
}

func (binding *runtimeContractBinding) RegisterTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration {
	if binding == nil {
		return runtimeTaskSurfaceRegistration{}
	}
	return binding.runtime.RegisterTaskSurface(options)
}
