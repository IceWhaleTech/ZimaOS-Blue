package bootstrap

import (
	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/connection"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (state *routeRegistrationState) setHTTPBootstrap(connManager *connection.Manager, v1, api *echo.Group) {
	if state == nil {
		return
	}
	state.connManager = connManager
	state.v1 = v1
	state.api = api
	state.runtimeSnapshot.entry.connManager = connManager
	state.runtimeSnapshot.entry.v1 = v1
	state.runtimeSnapshot.entry.api = api
}

func (state *routeRegistrationState) setMediaDir(dir string) {
	if state == nil {
		return
	}
	state.mediaDir = dir
	state.runtimeSnapshot.entry.mediaDir = dir
}

func (state *routeRegistrationState) setSkillAutoReranker(reranker *agentcore.AutoSkillReranker) {
	if state == nil {
		return
	}
	state.skillAutoReranker = reranker
	state.runtimeSnapshot.utility.skillAutoReranker = reranker
}

func (state *routeRegistrationState) setQuestionManager(mgr *tools.QuestionManager) {
	if state == nil {
		return
	}
	state.questionMgr = mgr
	state.runtimeSnapshot.utility.questionMgr = mgr
}

func (state *routeRegistrationState) setExecApprovals(approvals *tools.ApprovalManager) {
	if state == nil {
		return
	}
	state.execApprovals = approvals
	state.runtimeSnapshot.utility.execApprovals = approvals
}

func (state *routeRegistrationState) setMgmtTool(tool *tools.MgmtTool) {
	if state == nil {
		return
	}
	state.mgmtTool = tool
	if state.services != nil {
		state.services.MgmtTool = tool
	}
	state.runtimeSnapshot.utility.mgmtTool = tool
}
