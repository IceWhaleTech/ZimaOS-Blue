package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type runtimeCapabilitySurface interface {
	runtimeCapabilityBoundary()
	HarnessRuntime() *HarnessRuntimeBundle
	ResearchService() *deepresearch.Service
	ReflectService() *selfreflect.Service
	registerTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration
}

type runtimeCapabilityResearchSurface interface {
	HarnessRuntime() *HarnessRuntimeBundle
	ResearchService() *deepresearch.Service
}

type runtimeCapabilityTaskSurface interface {
	runtimeCapabilityResearchSurface
	ReflectService() *selfreflect.Service
}

var _ runtimeCapabilitySurface = runtimeCapabilityAdapter{}
var _ runtimeCapabilityResearchSurface = runtimeCapabilityAdapter{}
var _ runtimeCapabilityTaskSurface = runtimeCapabilityAdapter{}

func (runtimeCapabilityAdapter) runtimeCapabilityBoundary() {}
