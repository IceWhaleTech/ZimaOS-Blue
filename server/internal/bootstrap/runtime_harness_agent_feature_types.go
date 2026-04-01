package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	harnessdrivers "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness/drivers"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type agentRuntimeTarget interface {
	SetEventObserver(observer agent.TaskEventObserver)
	SetToolEventObserver(observer tools.RuntimeEventObserver)
	SetSubagentExecutor(executor tools.SubagentExecutor)
	SetWritePathGuard(guard tools.WritePathGuard)
	SetExecPathGuard(guard tools.ExecPathGuard)
}

type agentRouteRegistrar interface {
	RegisterRoutes(group *echo.Group)
}

type agentRuntimeBinding struct {
	taskObserver     agent.TaskEventObserver
	toolObserver     tools.RuntimeEventObserver
	subagentExecutor tools.SubagentExecutor
	writeGuard       tools.WritePathGuard
	execGuard        tools.ExecPathGuard
	agentDriver      *harnessdrivers.AgentDriver
	subagentDriver   *harnessdrivers.AgentDriver
	useCompatHandler bool
}
