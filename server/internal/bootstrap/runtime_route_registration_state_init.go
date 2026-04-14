package bootstrap

import (
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/labstack/echo/v4"
)

func newRouteRegistrationStateBase(e *echo.Echo, deps *RoutesDeps) *routeRegistrationState {
	state := &routeRegistrationState{
		e:             e,
		deps:          deps,
		services:      deps.Services,
		cfg:           deps.ServerConfig,
		logger:        deps.Logger,
		registerStart: time.Now(),
	}
	state.trace = NewStartupTrace("bootstrap.register_routes", state.logger)
	state.trace.Mark("enter")
	return state
}

func bindRouteRegistrationStateRuntime(state *routeRegistrationState) {
	if state == nil {
		return
	}
	state.dataDir = state.cfg.DataDir
	state.workspaceDir = ResolveWorkspaceDir(state.cfg.DataDir, state.deps.Config)
	state.workspaceAllowedPaths = ResolveBuiltinToolAllowedPaths(state.deps.Config, state.cfg.DataDir)
	state.webSearchConfig = buildWebSearchConfig(state.deps.Config)
	bindRouteRegistrationStateDB(state)
	state.runtimeContract = newRouteRuntimeContract(state.runtimeWriteDB, state.runtimeReadDB, state.deps.Config, state.logger, state.deps.WorkspaceHandler, state.webSearchConfig)
	state.runtimeLLM = state.runtimeContract.NewRuntimeLLMRef()
	bindRouteRegistrationStateFlags(state)
}

func bindRouteRegistrationStateDB(state *routeRegistrationState) {
	if state == nil {
		return
	}
	if conn := state.services.RuntimeDBConn; conn != nil {
		state.runtimeReadDB = conn.Reader
		state.runtimeWriteDB = conn.Writer
		return
	}
	if conn := state.services.DBConn; conn != nil {
		state.runtimeReadDB = conn.Reader
		state.runtimeWriteDB = conn.Writer
		return
	}
	state.runtimeWriteDB = state.deps.DB
}

func bindRouteRegistrationStateFlags(state *routeRegistrationState) {
	if state == nil {
		return
	}
	state.flagEvaluator = state.deps.FlagEvaluator
	if state.flagEvaluator == nil && state.deps.Config != nil {
		state.flagEvaluator = config.NewFlagEvaluator(&state.deps.Config.Grayscale)
		state.deps.FlagEvaluator = state.flagEvaluator
	}
}
