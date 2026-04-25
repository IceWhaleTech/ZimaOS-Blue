package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeProxyBridgeSurface struct {
	providerPool *providerpool.Pool
	metrics      *metrics.MetricsWriter
	chat         runtimeProxyBridgeChatTarget
	voice        runtimeProxyBridgeVoiceTarget
	uiTool       runtimeProxyBridgeVLMTarget
	media        runtimeProxyBridgeMediaTarget
	pdf          runtimeProxyBridgePDFTarget
	image        runtimeProxyBridgeImageTarget
	uiSkill      runtimeProxyBridgeSkillTarget
	analyze      runtimeProxyBridgeAnalyzeTarget
	a11y         runtimeProxyBridgeA11yTarget
	advisor      runtimeProxyBridgeAdvisorTarget
}

func newRuntimeProxyBridgeSurface(services *Services, deps *RoutesDeps) runtimeProxyBridgeSurface {
	surface := runtimeProxyBridgeSurface{}
	if deps != nil {
		surface.providerPool = deps.ProviderPool
		surface.metrics = deps.MetricsWriter
		if deps.ChatHandler != nil {
			surface.chat = deps.ChatHandler
		}
		if deps.UIReviewerTool != nil {
			surface.uiTool = deps.UIReviewerTool
		}
		if deps.MediaManager != nil {
			surface.media = deps.MediaManager
		}
		if deps.AnalyzeTool != nil {
			surface.analyze = deps.AnalyzeTool
		}
		if deps.VoiceHandler != nil {
			surface.voice = deps.VoiceHandler.Service()
		}
	}
	if services != nil {
		if services.PDFService != nil {
			surface.pdf = services.PDFService
		}
		if services.ToolRegistry != nil {
			if tool := services.ToolRegistry.Get("computer_use"); tool != nil {
				surface.a11y, _ = tool.(*tools.A11yTool)
			}
			if tool := services.ToolRegistry.Get("image"); tool != nil {
				surface.image, _ = tool.(*tools.ImageTool)
			}
			if tool := services.ToolRegistry.Get("advisor"); tool != nil {
				surface.advisor, _ = tool.(*tools.AdvisorTool)
			}
		}
		if services.SkillRegistry != nil {
			if sk := services.SkillRegistry.Get("ui_reviewer"); sk != nil {
				surface.uiSkill, _ = sk.(*builtin.UIReviewer)
			}
		}
	}
	return surface
}

func (surface runtimeProxyBridgeSurface) bind(
	proxyHandler *proxy.ProxyHandler,
	runtimeProvider *runtimeLLMProviderRef,
	auxiliary *auxiliaryLLMCaller,
) {
	bridge := bindRuntimeProxyBridge(
		proxyHandler,
		surface.providerPool,
		surface.metrics,
		runtimeProvider,
		auxiliary,
		surface.chat,
		surface.voice,
		surface.uiTool,
		surface.media,
		surface.pdf,
		surface.image,
		surface.uiSkill,
		surface.analyze,
		surface.a11y,
	)
	if bridge != nil && surface.advisor != nil {
		surface.advisor.SetBridge(tools.NewProxyBridgeAdvisorAdapter(bridge, surface.providerPool))
	}
}
