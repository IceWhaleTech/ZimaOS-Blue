package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func newChatResearchRuntimeBinding(
	bundle *HarnessRuntimeBundle,
	service *deepresearch.Service,
	handler *serverpkg.ChatHandler,
	broker *sse.Broker,
	workspaceDir string,
) chatResearchRuntimeBinding {
	binding := chatResearchRuntimeBinding{
		service:  service,
		turnHook: newHarnessRuntimeAutoHarnessTurnHook(bundle, handler),
		research: newResearchRuntimeBinding(bundle, service, broker),
	}
	if service != nil {
		binding.toolAdapter = newHarnessRuntimeResearchToolAdapter(bundle, service, workspaceDir)
	}
	return binding
}

func bindChatResearchRuntime(chatTarget chatResearchRuntimeTarget, researchTarget researchEventPublisherTarget, binding chatResearchRuntimeBinding) {
	if chatTarget != nil {
		chatTarget.SetDeepResearchService(binding.service)
		bindHarnessRuntimeAutoHarnessTurnHook(chatTarget, binding.turnHook)
	}
	bindResearchRuntimeService(researchTarget, binding.research)
}

func registerChatResearchRuntime(controller *harness.Controller, registry *tools.Registry, binding chatResearchRuntimeBinding) {
	registerResearchRuntime(controller, binding.research)
	tools.RegisterResearchTools(registry, binding.toolAdapter)
}

func (binding chatResearchRuntimeBinding) applyChat(target chatResearchRuntimeTarget) {
	bindChatResearchRuntime(target, nil, binding)
}

func (binding chatResearchRuntimeBinding) applyResearchService(target researchEventPublisherTarget) {
	bindResearchRuntimeService(target, binding.research)
}

func (binding chatResearchRuntimeBinding) register(controller *harness.Controller, registry *tools.Registry) {
	registerChatResearchRuntime(controller, registry, binding)
}
