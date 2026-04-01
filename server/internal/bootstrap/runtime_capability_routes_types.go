package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type runtimeTaskSurfaceOptions struct {
	protected          *echo.Group
	apiProtected       *echo.Group
	chatPermission     echo.MiddlewareFunc
	securityPermission echo.MiddlewareFunc
	workspaceDir       string
	execApprovals      *tools.ApprovalManager
	questionMgr        *tools.QuestionManager
	logger             *zap.Logger
}

type runtimeTaskHarnessSurfaceRegistration struct {
	detailProvider   *harnessDetailProvider
	routesRegistered bool
}

type runtimeTaskSurfaceRegistration struct {
	research                  runtimeTaskResearchSurfaceRegistration
	harness                   runtimeTaskHarnessSurfaceRegistration
	detailProvider            *harnessDetailProvider
	deepResearchRegistered    bool
	harnessResearchRegistered bool
	harnessRoutesRegistered   bool
	selfReflectRoutesApplied  bool
}
