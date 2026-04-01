package bootstrap

import (
	"context"
	"database/sql"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
)

func newHarnessRuntimeAgentStore(writeDB, readDB *sql.DB, logger *zap.Logger) (*agent.Store, error) {
	agentStoreInitStart := time.Now()
	agentStore, err := agent.NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to initialize agent store", zap.Error(err), zap.Duration("elapsed", time.Since(agentStoreInitStart)))
		}
		return nil, err
	}
	if logger != nil {
		logger.Info("Agent store initialized", zap.Duration("elapsed", time.Since(agentStoreInitStart)))
	}
	return agentStore, nil
}

func registerHarnessRuntimeAgentDisabled(agentGroup *echo.Group, disabled echo.HandlerFunc) {
	if agentGroup == nil || disabled == nil {
		return
	}
	agentGroup.Any("/*", disabled)
}

func startHarnessRuntimeAgentRecovery(agentStore *agent.Store, logger *zap.Logger) {
	if agentStore == nil {
		return
	}
	go func() {
		recoveryStart := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		recovered, err := agentStore.RecoverStaleTasks(ctx)
		if err != nil {
			if logger != nil {
				logger.Warn("Failed to recover stale agent tasks", zap.Error(err), zap.Duration("elapsed", time.Since(recoveryStart)))
			}
			return
		}
		if logger != nil {
			logger.Info("Stale agent task recovery completed", zap.Int64("count", recovered), zap.Duration("elapsed", time.Since(recoveryStart)))
		}
	}()
}
