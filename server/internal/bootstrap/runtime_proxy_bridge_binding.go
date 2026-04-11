package bootstrap

import (
	"net/http"

	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeProxyBridge(
	handler http.Handler,
	providerPool *providerpool.Pool,
	metrics runtimeCounterRecorder,
	runtimeProvider *runtimeLLMProviderRef,
	auxiliary *auxiliaryLLMCaller,
	chat runtimeProxyBridgeChatTarget,
	voice runtimeProxyBridgeVoiceTarget,
	uiTool runtimeProxyBridgeVLMTarget,
	media runtimeProxyBridgeMediaTarget,
	pdf runtimeProxyBridgePDFTarget,
	image runtimeProxyBridgeImageTarget,
	uiSkill runtimeProxyBridgeSkillTarget,
	analyze runtimeProxyBridgeAnalyzeTarget,
) *proxybridge.Bridge {
	if handler == nil {
		return nil
	}

	bridge := proxybridge.NewBridge(handler)
	if metrics != nil {
		bridge.SetMetricsRecorder(metrics)
	}

	proxyProvider := newProxyBridgeProvider(bridge, providerPool)
	if runtimeProvider != nil {
		runtimeProvider.SetProvider(proxyProvider)
	}
	if auxiliary != nil && runtimeProvider != nil {
		auxiliary.SetFallback(runtimeProvider)
	}
	if chat != nil && runtimeProvider != nil {
		chat.SetRuntimeProvider(runtimeProvider)
		chat.SetProxyBridge(bridge)
		// Keep IM model empty so ChatHandler can continue resolving defaults dynamically.
		chat.SetIMModel("")
	}
	if voice != nil && runtimeProvider != nil {
		voice.SetChatFunc(runtimeProvider.Chat)
	}

	vlmBridge := tools.NewProxyBridgeVLMAdapter(bridge)
	if uiTool != nil {
		uiTool.SetVLMBridge(vlmBridge)
	}
	if media != nil {
		media.SetFallbackVisionBridge(vlmBridge)
	}
	if pdf != nil {
		pdf.SetVisionService(pdfextract.NewProxyBridgeVisionAdapter(bridge))
	}
	if image != nil {
		image.SetVisionBridge(vlmBridge)
	}
	if uiSkill != nil {
		uiSkill.SetBridge(bridge)
	}
	if analyze != nil {
		analyze.SetLLMBridge(tools.NewProxyBridgeLLMAdapter(bridge))
	}
	return bridge
}
