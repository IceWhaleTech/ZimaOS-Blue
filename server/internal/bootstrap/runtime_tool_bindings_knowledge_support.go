package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeAnalyzeSkillDependencies(
	registry *tools.Registry,
	analyzeTool runtimeAnalyzeTarget,
	browserBackend tools.BrowserBackend,
	lazyBrowser func() *browser.RodService,
	analyzeSkill runtimeAnalyzeSkillTarget,
) {
	if analyzeTool == nil {
		return
	}
	if browserBackend != nil {
		analyzeTool.SetBrowser(browserBackend)
	} else if lazyBrowser != nil {
		analyzeTool.SetBrowser(tools.NewLazyRodBrowserBackend(lazyBrowser))
	}
	if registry != nil {
		analyzeTool.SetExecutor(tools.NewExecutor(registry))
	}
	if analyzeSkill != nil {
		analyzeSkill.SetExecutor(analyzeTool)
	}
}
