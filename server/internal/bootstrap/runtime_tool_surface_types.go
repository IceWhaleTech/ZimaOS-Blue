package bootstrap

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

type runtimeToolSurfacesOptions struct {
	protected      *echo.Group
	services       *Services
	cfg            *ServerConfig
	deps           *RoutesDeps
	logger         *zap.Logger
	agentLLMCaller agent.LLMCaller
	harnessRuntime *HarnessRuntimeBundle
	reflectService *selfreflect.Service
	skillRegistry  *skillpkg.Registry
	mcpPermission  []echo.MiddlewareFunc
}

type runtimeToolSurfacesResult struct {
	agentRunner   *agent.Runner
	mcpRegistered bool
}
