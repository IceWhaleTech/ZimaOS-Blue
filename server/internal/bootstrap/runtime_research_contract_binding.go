package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type chatResearchRuntimeBindingTargets struct {
	chatTarget     chatResearchRuntimeTarget
	researchTarget researchEventPublisherTarget
	controller     *harness.Controller
	registry       *tools.Registry
}

func applyChatResearchRuntimeBinding(
	binding chatResearchRuntimeBinding,
	targets chatResearchRuntimeBindingTargets,
) routeRuntimeContractResearchResult {
	if targets.chatTarget != nil {
		binding.applyChat(targets.chatTarget)
	}
	if targets.researchTarget != nil {
		binding.applyResearchService(targets.researchTarget)
	}
	binding.register(targets.controller, targets.registry)
	return routeRuntimeContractResearchResult{
		serviceBound:        targets.chatTarget != nil && binding.service != nil,
		turnHookBound:       targets.chatTarget != nil && binding.turnHook != nil,
		eventPublisherBound: targets.researchTarget != nil && binding.research.eventPublisher != nil,
		driverRegistered:    targets.controller != nil && binding.research.driver != nil,
		toolRegistered:      targets.registry != nil && binding.toolAdapter != nil,
	}
}
