package bootstrap

import "github.com/labstack/echo/v4"

func registerTaskHarnessSurface(
	bundle *HarnessRuntimeBundle,
	options runtimeTaskSurfaceOptions,
) runtimeTaskHarnessSurfaceRegistration {
	detailProvider, ok := registerHarnessRuntimeWithDetail(
		bundle,
		options.execApprovals,
		options.questionMgr,
		runtimeTaskSurfaceProjectionGroups(options, "/harness", options.securityPermission),
		runtimeTaskSurfaceProjectionGroups(options, "", options.chatPermission),
	)
	if !ok {
		return runtimeTaskHarnessSurfaceRegistration{}
	}
	return runtimeTaskHarnessSurfaceRegistration{
		detailProvider:   detailProvider,
		routesRegistered: true,
	}
}

func runtimeTaskSurfaceProjectionGroups(
	options runtimeTaskSurfaceOptions,
	prefix string,
	middleware echo.MiddlewareFunc,
) []*echo.Group {
	return []*echo.Group{
		runtimeTaskSurfaceGroup(options.protected, prefix, middleware),
		runtimeTaskSurfaceGroup(options.apiProtected, prefix, middleware),
	}
}
