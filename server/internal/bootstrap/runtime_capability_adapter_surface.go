package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

func (adapter runtimeCapabilityAdapter) HarnessRuntime() *HarnessRuntimeBundle {
	return adapter.contract.Harness
}

func (adapter runtimeCapabilityAdapter) ResearchService() *deepresearch.Service {
	return adapter.contract.Research
}

func (adapter runtimeCapabilityAdapter) ReflectService() *selfreflect.Service {
	return adapter.contract.Reflect
}
