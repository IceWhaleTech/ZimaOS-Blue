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
	bindRuntimeAnalyzeSkillDependencies(registry, analyzeTool, browserBackend, lazyBrowser, analyzeSkill)
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
	advisorTool := tools.RegisterAdvisorTool(registry)
	if registry != nil && advisorTool != nil {
		advisorTool.SetExecutor(tools.NewExecutor(registry))
	}
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
