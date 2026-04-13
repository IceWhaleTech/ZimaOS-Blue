package bootstrap

import (
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func configureResearchRuntimeDriver(driver *harnessdrivers.ResearchDriver, registry *tools.Registry) {
	if driver == nil || registry == nil {
		return
	}
	driver.SetAnalyzeExecutor(tools.GetAnalyzeTool(registry))
	driver.SetAdvisorExecutor(tools.GetAdvisorTool(registry))
	driver.SetUIReviewExecutor(tools.GetUIReviewerTool(registry))
	driver.SetRecentExecutor(tools.GetWebQueryTool(registry))
}
