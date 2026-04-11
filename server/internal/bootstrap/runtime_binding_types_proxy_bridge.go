package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
)

type runtimeProxyBridgeChatTarget interface {
	SetRuntimeProvider(provider llm.Provider)
	SetProxyBridge(bridge *proxybridge.Bridge)
	SetIMModel(model string)
}

type runtimeProxyBridgeVoiceTarget interface {
	SetChatFunc(fn voice.ChatFunc)
}

type runtimeProxyBridgeVLMTarget interface {
	SetVLMBridge(bridge tools.VLMBridge)
}

type runtimeProxyBridgeMediaTarget interface {
	SetFallbackVisionBridge(bridge tools.VLMBridge)
}

type runtimeProxyBridgePDFTarget interface {
	SetVisionService(vision pdfextract.VisionService)
}

type runtimeProxyBridgeImageTarget interface {
	SetVisionBridge(bridge tools.VLMBridge)
}

type runtimeProxyBridgeSkillTarget interface {
	SetBridge(bridge *proxybridge.Bridge)
}

type runtimeProxyBridgeAnalyzeTarget interface {
	SetLLMBridge(bridge tools.LLMBridge)
}

type runtimeProxyBridgeAdvisorTarget interface {
	SetBridge(bridge tools.AdvisorBridge)
}
