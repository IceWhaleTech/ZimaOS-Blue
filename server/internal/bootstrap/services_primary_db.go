package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

func openPrimaryDatabase(cfg *ServerConfig, logger *zap.Logger) (*database.SQLiteConn, error) {
	dbPath := filepath.Join(cfg.DataDir, "blue.db")

	open := func() (*database.SQLiteConn, error) {
		return database.OpenSQLite(dbPath, nil)
	}

	dbConn, err := open()
	if err == nil {
		return dbConn, nil
	}
	if !database.IsSQLiteCorruptionError(err) {
		return nil, err
	}

	if logger != nil {
		logger.Warn("Primary database open reported corruption; attempting startup auto-recovery",
			zap.String("db_path", dbPath),
			zap.Error(err),
		)
	}

	var recoverErr error
	mgr, mgrErr := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(cfg.DataDir, "backups"),
		SkillsPath:    ResolveWorkspaceSkillsDir(cfg.DataDir, nil),
	}, cfg.DataDir, cfg.DataDir)
	if mgrErr != nil {
		recoverErr = fmt.Errorf("init backup manager: %w", mgrErr)
		if logger != nil {
			logger.Warn("Failed to initialize backup manager for corrupted primary database; recreating a fresh database instead",
				zap.String("db_path", dbPath),
				zap.Error(mgrErr),
			)
		}
	} else {
		result, autoRecoverErr := mgr.CheckAndAutoRecover(context.Background(), []string{dbPath})
		recoverErr = autoRecoverErr
		if autoRecoverErr == nil {
			if logger != nil && result != nil {
				if len(result.RepairedDatabases) > 0 {
					fields := []zap.Field{zap.Strings("repaired_databases", result.RepairedDatabases)}
					if len(result.RepairDetails) > 0 {
						fields = append(fields, zap.Any("repair_details", result.RepairDetails))
					}
					logger.Info("Primary database repaired during startup recovery", fields...)
				}
				if result.Recovered {
					logger.Info("Primary database restored from backup during startup recovery",
						zap.Strings("corrupted_databases", result.CorruptedDatabases),
						zap.String("backup_id", result.BackupID),
					)
				}
			}

			dbConn, retryErr := open()
			if retryErr == nil {
				return dbConn, nil
			}
			if !database.IsSQLiteCorruptionError(retryErr) {
				return nil, fmt.Errorf("open database after startup auto-recovery: %w", retryErr)
			}
			recoverErr = retryErr
			if logger != nil {
				logger.Warn("Primary database still failed after startup auto-recovery; rotating the unrecoverable database aside and recreating fresh",
					zap.String("db_path", dbPath),
					zap.Error(retryErr),
				)
			}
		} else if logger != nil {
			logger.Warn("Startup auto-recovery did not produce a usable primary database; recreating a fresh database",
				zap.String("db_path", dbPath),
				zap.Error(autoRecoverErr),
			)
		}
	}

	backupPath, rotateErr := database.RotateCorruptSQLiteDatabase(dbPath)
	if rotateErr != nil {
		return nil, fmt.Errorf("open database: %w (startup auto-recovery failed: %v; rotate corrupt database failed: %v)", err, recoverErr, rotateErr)
	}
	if logger != nil {
		logger.Warn("Rotated corrupt primary database to .bak backup and recreating a fresh database",
			zap.String("db_path", dbPath),
			zap.String("backup_path", backupPath),
		)
	}

	dbConn, retryErr := open()
	if retryErr != nil {
		return nil, fmt.Errorf("open database after recreating corrupt primary database: %w", retryErr)
	}
	return dbConn, nil
}
