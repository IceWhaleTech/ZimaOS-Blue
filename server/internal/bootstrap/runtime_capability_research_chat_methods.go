package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (binding chatResearchRuntimeBinding) applyChat(target chatResearchRuntimeTarget) {
	bindChatResearchRuntime(target, nil, binding)
}

func (binding chatResearchRuntimeBinding) applyResearchService(target researchEventPublisherTarget) {
	bindResearchRuntimeService(target, binding.research)
}

func (binding chatResearchRuntimeBinding) register(controller *harness.Controller, registry *tools.Registry) {
	registerChatResearchRuntime(controller, registry, binding)
}
