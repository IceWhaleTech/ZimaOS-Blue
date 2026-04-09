package bootstrap

import (
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type runtimeTaskSurfaceOptions struct {
	protected, apiProtected            *echo.Group
	chatPermission, securityPermission echo.MiddlewareFunc
	chatHandler                        *serverpkg.ChatHandler
	browserHandler                     *browser.Handler
	runtimeTaskSurfaceKnowledgeDeps
	workspaceDir  string
	execApprovals *tools.ApprovalManager
	questionMgr   *tools.QuestionManager
	logger        *zap.Logger
}

type runtimeTaskHarnessSurfaceRegistration struct {
	detailProvider   *harnessDetailProvider
	routesRegistered bool
}

type runtimeTaskSurfaceRegistration struct {
	research                                                                                                                                                 runtimeTaskResearchSurfaceRegistration
	knowledge                                                                                                                                                runtimeTaskKnowledgeSurfaceRegistration
	harness                                                                                                                                                  runtimeTaskHarnessSurfaceRegistration
	detailProvider                                                                                                                                           *harnessDetailProvider
	deepResearchRegistered, harnessResearchRegistered, knowledgeRoutesRegistered, knowledgeCronRegistered, harnessRoutesRegistered, selfReflectRoutesApplied bool
}
