package bootstrap

import (
	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/heartbeat"
)

type routeRuntimeOperationalBinding interface {
	RegisterTaskSurface(options runtimeTaskSurfaceOptions) runtimeTaskSurfaceRegistration
	ActivateRouteRuntime(options routeRuntimeContractActivationOptions) runtimeActivationResult
	RegisterActivationSupportRoutes(
		activation runtimeActivationResult,
		options routeRuntimeContractSupportOptions,
	)
	BindDeferredSupport(
		activation runtimeActivationResult,
		options routeRuntimeContractDeferredSupportOptions,
	)
}

var _ routeRuntimeOperationalBinding = (*runtimeContractBinding)(nil)

type routeRuntimeContractHeartbeatOptions struct {
	apiProtected *echo.Group
	config       *heartbeat.Config
	dataDir      string
	runtimeLLM   *runtimeLLMProviderRef
	ctx          context.Context
	logger       *zap.Logger
}

type routeRuntimeContractOperationalOptions struct {
	taskSurface runtimeTaskSurfaceOptions
	activation  routeRuntimeContractActivationOptions
	support     routeRuntimeContractSupportOptions
	deferred    routeRuntimeContractDeferredSupportOptions
}

type routeRuntimeContractOperationalSupportResult struct {
	activationSupportApplied bool
	deferredSupportApplied   bool
}

type routeRuntimeContractOperationalResult struct {
	taskSurface runtimeTaskSurfaceRegistration
	activation  runtimeActivationResult
	support     routeRuntimeContractOperationalSupportResult
}
