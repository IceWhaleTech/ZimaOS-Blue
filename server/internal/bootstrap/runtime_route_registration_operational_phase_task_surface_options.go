package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"

func newRouteRuntimeOperationalTaskSurfaceOptions(state *routeRegistrationState) runtimeTaskSurfaceOptions {
	return withRuntimeTaskSurfaceKnowledgeDeps(state, runtimeTaskSurfaceOptions{
		protected: state.protected, apiProtected: state.apiProtected,
		chatPermission:     state.authSurface.requirePagePermission(permission.PageChat),
		securityPermission: state.authSurface.requirePagePermission(permission.PageSecurity),
		chatHandler:        state.deps.ChatHandler, browserHandler: state.deps.BrowserHandler,
		workspaceDir:  state.workspaceDir,
		execApprovals: state.execApprovals,
		questionMgr:   state.questionMgr, logger: state.logger,
	})
}
