package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

func newResearchRuntimeBinding(bundle *HarnessRuntimeBundle, service *deepresearch.Service, broker *sse.Broker) researchRuntimeBinding {
	if service == nil {
		return researchRuntimeBinding{}
	}
	controller := harnessRuntimeController(bundle)
	if controller == nil {
		return researchRuntimeBinding{eventPublisher: broker}
	}
	driver := harnessdrivers.NewResearchDriver(service, controller)
	if broker != nil {
		driver.SetNextPublisher(broker)
	}
	return researchRuntimeBinding{
		eventPublisher: driver,
		driver:         driver,
	}
}

func bindResearchRuntimeService(target researchEventPublisherTarget, binding researchRuntimeBinding) {
	if target == nil || binding.eventPublisher == nil {
		return
	}
	target.SetEventPublisher(binding.eventPublisher)
}

func registerResearchRuntime(controller *harness.Controller, binding researchRuntimeBinding) {
	if controller == nil || binding.driver == nil {
		return
	}
	controller.RegisterDriver(binding.driver)
}

func (binding researchRuntimeBinding) apply(target researchEventPublisherTarget) {
	bindResearchRuntimeService(target, binding)
}

func (binding researchRuntimeBinding) register(controller *harness.Controller) {
	registerResearchRuntime(controller, binding)
}
