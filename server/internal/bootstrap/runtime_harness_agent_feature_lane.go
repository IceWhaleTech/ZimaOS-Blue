package bootstrap

import (
	"database/sql"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerHarnessRuntimeAgentFeature(
	bundle *HarnessRuntimeBundle,
	agentGroup *echo.Group,
	db *sql.DB,
	llmCaller agent.LLMCaller,
	registry *tools.Registry,
	executor *tools.Executor,
	broker *sse.Broker,
	workspaceDir string,
	logger *zap.Logger,
	disabled echo.HandlerFunc,
) *agent.Runner {
	return registerHarnessRuntimeAgentFeatureWithReadDB(bundle, agentGroup, db, db, llmCaller, registry, executor, broker, workspaceDir, logger, disabled)
}

func registerHarnessRuntimeAgentFeatureWithReadDB(
	bundle *HarnessRuntimeBundle,
	agentGroup *echo.Group,
	writeDB *sql.DB,
	readDB *sql.DB,
	llmCaller agent.LLMCaller,
	registry *tools.Registry,
	executor *tools.Executor,
	broker *sse.Broker,
	workspaceDir string,
	logger *zap.Logger,
	disabled echo.HandlerFunc,
) *agent.Runner {
	if agentGroup == nil {
		return nil
	}
	if writeDB == nil || broker == nil || llmCaller == nil {
		registerHarnessRuntimeAgentDisabled(agentGroup, disabled)
		return nil
	}

	agentStore, err := newHarnessRuntimeAgentStore(writeDB, readDB, logger)
	if err != nil {
		registerHarnessRuntimeAgentDisabled(agentGroup, disabled)
		return nil
	}
	startHarnessRuntimeAgentRecovery(agentStore, logger)

	agentRunner := agent.NewRunner(agentStore, llmCaller, registry, executor, broker, agent.RunnerConfig{})
	bindHarnessRuntimeToAgentRunner(bundle, agentGroup, agentRunner, agentStore, workspaceDir)
	if logger != nil {
		logger.Info("Agent task routes registered")
	}
	return agentRunner
}
