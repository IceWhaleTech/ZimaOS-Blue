package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"

func newRouteRuntimeOperationalSupportOptions(state *routeRegistrationState) routeRuntimeContractSupportOptions {
	return routeRuntimeContractSupportOptions{
		v1:             state.v1,
		authMiddleware: state.authSurface.authMiddleware,
		pageMiddleware: state.authSurface.requirePagePermission(permission.PageChat),
		memoryHandler:  state.deps.MemoryHandler,
		chat:           state.deps.ChatHandler,
	}
}
