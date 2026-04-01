package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractAnalyzeOptions struct {
	registry          *tools.Registry
	skillRegistry     runtimeSkillRegistrySource
	mediaDir          string
	browserBackend    tools.BrowserBackend
	lazyBrowser       func() *browser.RodService
	analyzeSkill      runtimeAnalyzeSkillTarget
	webQuerySkill     runtimeWebSearchSkillTarget
	webSearchSkill    runtimeWebSearchSkillTarget
	webSearchConfig   tools.WebSearchConfig
	deepResearchSkill runtimeDeepResearchSkillTarget
}
