package bootstrap

import (
	"context"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mcp"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func (runtime routeToolRuntimeBinding) registerMCPSurface(
	protected *echo.Group,
	cfg *ServerConfig,
	deps *RoutesDeps,
	logger *zap.Logger,
	agentLLMCaller agent.LLMCaller,
	mcpPermission ...echo.MiddlewareFunc,
) bool {
	return registerRuntimeMCPRoutes(
		protected,
		runtime.registry,
		runtime.executor,
		cfg,
		deps,
		logger,
		agentLLMCaller,
		mcpPermission...,
	)
}

func registerRuntimeMCPRoutes(
	protected *echo.Group,
	registry *tools.Registry,
	executor *tools.Executor,
	cfg *ServerConfig,
	deps *RoutesDeps,
	logger *zap.Logger,
	agentLLMCaller agent.LLMCaller,
	mcpPermission ...echo.MiddlewareFunc,
) bool {
	if protected == nil || registry == nil || executor == nil || cfg == nil || deps == nil || logger == nil {
		return false
	}

	mcpServer := newRuntimeMCPServer(registry, executor, cfg, deps, agentLLMCaller)
	mcpHandler := mcp.NewHandler(mcpServer)
	mcpGroup := protected.Group("/mcp")
	if len(mcpPermission) > 0 && mcpPermission[0] != nil {
		mcpGroup = protected.Group("/mcp", mcpPermission[0])
	}
	mcpHandler.RegisterRoutes(mcpGroup)
	logger.Info("MCP server routes registered")
	return true
}

func newRuntimeMCPServer(
	registry *tools.Registry,
	executor *tools.Executor,
	cfg *ServerConfig,
	deps *RoutesDeps,
	agentLLMCaller agent.LLMCaller,
) *mcp.Server {
	server := mcp.NewServer(registry, executor)
	if cfg != nil {
		var appCfg *config.Config
		if deps != nil {
			appCfg = deps.Config
		}
		server.SetWorkspaceRoot(resolveMCPWorkspaceRoot(cfg.DataDir, appCfg))
	}
	if runner := newRuntimeMCPGenerativeRunner(agentLLMCaller); runner != nil {
		server.SetGenerativeRunner(runner)
	}
	if deps != nil && deps.WorkspaceHandler != nil {
		if mgr := deps.WorkspaceHandler.Manager(); mgr != nil {
			server.SetWorkspace(mgr)
		}
	}
	return server
}

func newRuntimeMCPGenerativeRunner(agentLLMCaller agent.LLMCaller) func(context.Context, string, int) (string, error) {
	if agentLLMCaller == nil {
		return nil
	}
	return func(ctx context.Context, prompt string, maxTokens int) (string, error) {
		req := llm.ChatRequest{
			Model:       "auto",
			Messages:    []llm.Message{{Role: llm.RoleUser, Content: prompt}},
			Temperature: 0.2,
			MaxTokens:   maxTokens,
		}
		resp, err := agentLLMCaller.Chat(ctx, req)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(resp.Message.Content), nil
	}
}
