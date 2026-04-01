package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeImageTools(
	registry *tools.Registry,
	image runtimeImageToolTarget,
	uiReviewer *tools.UIReviewerTool,
	workspaceAllowedPaths []string,
	mediaManager *mediagen.Manager,
	mediaStorage *mediagen.MediaStorage,
	llmRegistry *llm.ProviderRegistry,
	ocr *ocrruntime.TesseractService,
	providerPool *providerpool.Pool,
) {
	generate := newImageGenerateAdapter(mediaManager, workspaceAllowedPaths)
	lookup := newImageTaskLookupAdapter(mediaManager)
	if registry != nil {
		tools.RegisterImageTool(registry, uiReviewer, generate, lookup)
	}
	pptSvc := newPPTService(mediaManager, mediaStorage, uiReviewer, newProviderRegistryLLMCaller(llmRegistry))
	if registry != nil {
		tools.RegisterPPTTool(registry, pptSvc)
	}
	if image == nil && registry != nil {
		if registered, ok := registry.Get("image").(*tools.ImageTool); ok {
			image = registered
		}
	}
	if image == nil {
		return
	}
	image.SetOCRService(newImageOCRAdapter(ocr))
	image.SetProviderVision(tools.NewMiniMaxImageVision(providerPool))
	image.SetPPTService(pptSvc)
}
