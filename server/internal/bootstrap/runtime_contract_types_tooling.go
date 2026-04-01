package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractToolingOptions struct {
	registry              *tools.Registry
	skillRegistry         runtimeSkillRegistrySource
	mediaDir              string
	browserBackend        tools.BrowserBackend
	lightpanda            *browser.LightpandaService
	acquireBrowser        func() (*browser.RodService, func(), error)
	lazyBrowser           func() *browser.RodService
	sttService            stt.Service
	browserSkill          runtimeBrowserSkillTarget
	uiReviewerSkill       runtimeUIReviewerSkillTarget
	pushService           *push.Service
	reminderSkill         runtimeReminderSkillTarget
	workspaceAllowedPaths []string
	mediaManager          *mediagen.Manager
	mediaStorage          *mediagen.MediaStorage
	llmRegistry           *llm.ProviderRegistry
	ocr                   *ocrruntime.TesseractService
	providerPool          *providerpool.Pool
	speechSource          runtimeSpeechServiceSource
	voiceSource           runtimeVoiceServiceSource
}

type routeRuntimeContractToolingResult struct {
	uiReviewerTool *tools.UIReviewerTool
}
