package bootstrap

import (
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeCapabilityChatResearch(
	contract runtimeCapabilityResearchSurface,
	target chatResearchRuntimeTarget,
	broker *sse.Broker,
	registry *tools.Registry,
	workspaceDir string,
) {
	harnessRuntime := contract.HarnessRuntime()
	researchService := contract.ResearchService()
	if handler, ok := any(target).(*serverpkg.ChatHandler); ok {
		bindHarnessRuntimeChatResearch(harnessRuntime, handler, researchService, broker, registry, workspaceDir)
		return
	}
	binding := newChatResearchRuntimeBinding(harnessRuntime, researchService, &serverpkg.ChatHandler{}, broker, workspaceDir)
	_ = applyChatResearchRuntimeBinding(binding, chatResearchRuntimeBindingTargets{
		chatTarget:     target,
		researchTarget: researchService,
		controller:     harnessRuntimeController(harnessRuntime),
		registry:       registry,
	})
}
