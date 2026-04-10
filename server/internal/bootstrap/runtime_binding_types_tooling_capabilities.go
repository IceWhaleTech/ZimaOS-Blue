package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeBrowserMediaDirTarget interface {
	SetMediaDir(dir string)
}

type runtimeAnalyzeTarget interface {
	builtin.AnalyzeExecutor
	SetBrowser(browser tools.BrowserBackend)
	SetExecutor(executor *tools.Executor)
}

type runtimeAnalyzeSkillTarget interface {
	SetExecutor(e builtin.AnalyzeExecutor)
}

type runtimeEmailSkillTarget interface {
	SetExecutor(e builtin.EmailExecutor)
}

type runtimeCalendarSkillTarget interface {
	SetExecutor(e builtin.CalendarExecutor)
}

type runtimeContactsSkillTarget interface {
	SetExecutor(e builtin.ContactsExecutor)
}

type runtimeWebSearchSkillTarget interface {
	SetSearcher(s builtin.WebSearcher)
}

type runtimeDeepResearchSkillTarget interface {
	SetExecutor(e builtin.DeepResearchExecutor)
}

type runtimeImageToolTarget interface {
	SetOCRService(svc tools.ImageOCRService)
	SetProviderVision(vision tools.ProviderAwareImageVision)
	SetPPTService(svc tools.PPTGenerateService)
}

type runtimeBrowserAccessTarget interface {
	SetBrowser(browser tools.BrowserBackend)
	SetLightpandaShim(service *browser.LightpandaService)
}

type runtimeBrowserSkillTarget interface {
	SetBrowserService(svc builtin.BrowserServiceInterface)
	SetMediaDir(dir string)
}

type runtimeUIReviewerSkillTarget interface {
	SetBrowserService(svc builtin.BrowserServiceInterface)
}
