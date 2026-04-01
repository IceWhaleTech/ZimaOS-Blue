package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"

func (binding *runtimeContractBinding) BindSchedulerServices(options routeRuntimeContractSchedulerOptions) {
	if binding == nil {
		return
	}
	if options.scheduler == nil {
		options.scheduler = runtimeSkillAsSchedulerTarget(options.skillRegistry, "scheduler")
	}
	bindRuntimeSchedulerServices(
		options.registry,
		options.workflowResolver,
		options.workflowHandler,
		binding.HarnessRuntime(),
		options.workspaceDir,
		options.cronHandler,
		options.scheduler,
		options.calendar,
		options.memoryStore,
		options.broker,
		binding.runtime.ResearchService(),
		options.logger,
	)
}

func (binding *runtimeContractBinding) BindAnalyzeTool(options routeRuntimeContractAnalyzeOptions) *tools.AnalyzeTool {
	if binding == nil {
		return nil
	}
	if options.analyzeSkill == nil {
		options.analyzeSkill = runtimeSkillAsAnalyzeTarget(options.skillRegistry, "analyze")
	}
	if options.webQuerySkill == nil {
		options.webQuerySkill = runtimeSkillAsWebSearchTarget(options.skillRegistry, "web_query")
	}
	if options.webSearchSkill == nil {
		options.webSearchSkill = runtimeSkillAsWebSearchTarget(options.skillRegistry, "web_search")
	}
	if options.deepResearchSkill == nil {
		options.deepResearchSkill = runtimeSkillAsDeepResearchTarget(options.skillRegistry, "deep_research")
	}
	return bindRuntimeAnalyzeTool(
		options.registry,
		options.mediaDir,
		options.browserBackend,
		options.lazyBrowser,
		options.analyzeSkill,
		options.webQuerySkill,
		options.webSearchSkill,
		options.webSearchConfig,
		options.deepResearchSkill,
		binding.runtime.ResearchService(),
	)
}
