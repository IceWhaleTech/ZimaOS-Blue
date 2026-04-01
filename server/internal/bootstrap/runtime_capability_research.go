package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindHarnessRuntimeChatResearch(
	bundle *HarnessRuntimeBundle,
	handler *serverpkg.ChatHandler,
	service *deepresearch.Service,
	broker *sse.Broker,
	registry *tools.Registry,
	workspaceDir string,
) {
	binding := newChatResearchRuntimeBinding(bundle, service, handler, broker, workspaceDir)
	bindChatResearchRuntime(handler, service, binding)
	registerChatResearchRuntime(harnessRuntimeController(bundle), registry, binding)
}

func bindHarnessRuntimeToResearchService(bundle *HarnessRuntimeBundle, service *deepresearch.Service, broker *sse.Broker) {
	binding := newResearchRuntimeBinding(bundle, service, broker)
	bindResearchRuntimeService(service, binding)
	registerResearchRuntime(harnessRuntimeController(bundle), binding)
}
