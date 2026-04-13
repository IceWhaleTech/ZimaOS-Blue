package bootstrap

import (
	"path/filepath"

	"go.uber.org/zap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

// openRuntimeDatabase opens the runtime database (harness + agent tables)
// This database is separate from the primary blue.db to reduce startup time.
func openRuntimeDatabase(cfg *ServerConfig, logger *zap.Logger) (*database.SQLiteConn, error) {
	dbPath := filepath.Join(cfg.DataDir, "runtime.db")

	open := func() (*database.SQLiteConn, error) {
		// Use SkipIntegrityCheckOnOpen for faster startup
		opts := &database.SQLiteOpenOpts{
			MaxReaders:               4,
			BusyTimeout:              5000,
			CacheSize:              -2000,
			ForeignKeys:            true,
			SkipIntegrityCheckOnOpen: true,
		}
		return database.OpenSQLite(dbPath, opts)
	}

	dbConn, err := open()
	if err == nil {
		return dbConn, nil
	}

	// If we can't open, try with recovery (similar to primary DB pattern)
	if logger != nil {
		logger.Warn("Runtime database open failed, attempting recovery", zap.String("db_path", dbPath), zap.Error(err))
	}

	// For now, just return the error - recovery can be added later if needed
	return nil, err
}
