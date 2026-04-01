package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

func (bundle *runtimeContractRuntimeBundle) capabilitySurface() runtimeCapabilitySurface {
	if bundle == nil {
		return nil
	}
	return bundle.capabilities
}

func (bundle *runtimeContractRuntimeBundle) HarnessRuntime() *HarnessRuntimeBundle {
	if surface := bundle.capabilitySurface(); surface != nil {
		return surface.HarnessRuntime()
	}
	return nil
}

func (bundle *runtimeContractRuntimeBundle) ResearchService() *deepresearch.Service {
	if surface := bundle.capabilitySurface(); surface != nil {
		return surface.ResearchService()
	}
	return nil
}

func (bundle *runtimeContractRuntimeBundle) ReflectService() *selfreflect.Service {
	if surface := bundle.capabilitySurface(); surface != nil {
		return surface.ReflectService()
	}
	return nil
}
