package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (adapter runtimeCapabilityAdapter) bindChatResearch(
	target chatResearchRuntimeTarget,
	broker *sse.Broker,
	registry *tools.Registry,
	workspaceDir string,
) {
	bindRuntimeCapabilityChatResearch(adapter, target, broker, registry, workspaceDir)
}
