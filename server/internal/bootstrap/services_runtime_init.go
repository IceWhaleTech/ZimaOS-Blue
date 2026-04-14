package bootstrap

import (
	"context"

	"go.uber.org/zap"
)

func initServicesRuntimeDatabase(s *Services, cfg *ServerConfig, trace *StartupTrace) error {
	conn, err := openRuntimeDatabase(cfg, s.Logger)
	if err != nil {
		logRuntimeDBWarning(s.Logger, "Runtime database unavailable; continuing with primary database", err)
		trace.Mark("runtime_db_disabled")
		return nil
	}
	if _, err := PrepareRuntimeDatabase(context.Background(), cfg.DataDir, s.DBConn, conn, s.Logger); err != nil {
		logRuntimeDBWarning(s.Logger, "Runtime database preparation failed; continuing with primary database", err)
		_ = conn.Close()
		trace.Mark("runtime_db_disabled")
		return nil
	}
	s.RuntimeDBConn = conn
	trace.Mark("runtime_db_opened")
	return nil
}

func logRuntimeDBWarning(logger *zap.Logger, msg string, err error) {
	if logger != nil {
		logger.Warn(msg, zap.Error(err))
	}
}
