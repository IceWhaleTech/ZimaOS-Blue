package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/deepresearch"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func bindRuntimeKnowledgeSkills(
	registry *tools.Registry,
	analyzeTool runtimeAnalyzeTarget,
	browserBackend tools.BrowserBackend,
	lazyBrowser func() *browser.RodService,
	analyzeSkill runtimeAnalyzeSkillTarget,
	webQuerySkill runtimeWebSearchSkillTarget,
	webSearchSkill runtimeWebSearchSkillTarget,
	webSearchConfig tools.WebSearchConfig,
	deepResearchSkill runtimeDeepResearchSkillTarget,
	deepResearchService *deepresearch.Service,
) {
	if analyzeTool != nil {
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
	if webQuerySkill != nil && registry != nil {
		if tool := registry.Get("web_query"); tool != nil {
			webQuerySkill.SetSearcher(tool)
		}
	}
	if webSearchSkill != nil {
		webSearchSkill.SetSearcher(tools.NewWebSearchTool(webSearchConfig))
	}
	if deepResearchSkill != nil {
		deepResearchSkill.SetExecutor(deepresearch.NewSkillExecutor(deepResearchService))
	}
}

func bindRuntimeAnalyzeTool(
	registry *tools.Registry,
	mediaDir string,
	browserBackend tools.BrowserBackend,
	lazyBrowser func() *browser.RodService,
	analyzeSkill runtimeAnalyzeSkillTarget,
	webQuerySkill runtimeWebSearchSkillTarget,
	webSearchSkill runtimeWebSearchSkillTarget,
	webSearchConfig tools.WebSearchConfig,
	deepResearchSkill runtimeDeepResearchSkillTarget,
	deepResearchService *deepresearch.Service,
) *tools.AnalyzeTool {
	analyzeTool := tools.RegisterAnalyzeTool(registry, mediaDir)
	bindRuntimeKnowledgeSkills(
		registry,
		analyzeTool,
		browserBackend,
		lazyBrowser,
		analyzeSkill,
		webQuerySkill,
		webSearchSkill,
		webSearchConfig,
		deepResearchSkill,
		deepResearchService,
	)
	return analyzeTool
}
