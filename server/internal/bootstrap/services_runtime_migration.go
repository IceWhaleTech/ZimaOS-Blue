package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentsessions"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

const runtimeDBSplitMarker = ".runtime_db_split.done"

var runtimeDBSplitTables = []string{
	"harness_runs",
	"harness_run_events",
	"harness_artifacts",
	"harness_run_groups",
	"harness_run_group_items",
	"harness_scorecards",
	"harness_datasets",
	"harness_dataset_versions",
	"harness_eval_specs",
	"harness_eval_runs",
	"harness_baselines",
	"harness_comparison_reports",
	"harness_skill_revisions",
	"harness_skill_evolution_cases",
	"agent_profiles",
	"agent_sessions",
	"agent_runs",
	"agent_run_events",
	"agent_tasks",
	"agent_runtime_events",
}

// RuntimeDBMigrationResult summarizes the one-time split from blue.db to runtime.db.
type RuntimeDBMigrationResult struct {
	TablesMoved []string
	RowsMoved   int
	Vacuumed    bool
}

// PrepareRuntimeDatabase ensures runtime.db has the required schemas and, on first run,
// moves harness/agent tables out of blue.db so future startup checks touch less data.
func PrepareRuntimeDatabase(
	ctx context.Context,
	dataDir string,
	primaryConn *dbutil.SQLiteConn,
	runtimeConn *dbutil.SQLiteConn,
	logger *zap.Logger,
) (*RuntimeDBMigrationResult, error) {
	if runtimeConn == nil || runtimeConn.Writer == nil || strings.TrimSpace(dataDir) == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := harness.NewSQLiteStore(runtimeConn.Writer); err != nil {
		return nil, fmt.Errorf("prepare runtime harness schema: %w", err)
	}
	if _, err := agent.NewStore(runtimeConn.Writer); err != nil {
		return nil, fmt.Errorf("prepare runtime agent schema: %w", err)
	}
	if _, err := agentsessions.NewSQLiteStore(runtimeConn.Writer); err != nil {
		return nil, fmt.Errorf("prepare runtime agent sessions schema: %w", err)
	}
	if _, err := harness.MigrateLegacyStore(ctx, runtimeConn.Writer, dataDir); err != nil {
		return nil, fmt.Errorf("migrate legacy harness store to runtime.db: %w", err)
	}
	if primaryConn == nil || primaryConn.Writer == nil {
		return nil, nil
	}
	markerPath := filepath.Join(dataDir, runtimeDBSplitMarker)
	if _, err := os.Stat(markerPath); err == nil {
		return nil, nil
	}
	hasRuntimeRows, err := runtimeDBHasRows(ctx, runtimeConn.Writer)
	if err != nil {
		return nil, err
	}
	if hasRuntimeRows {
		if logger != nil {
			logger.Info("Skipping blue.db runtime table split because runtime.db already contains runtime rows")
		}
		return nil, nil
	}
	result, err := migrateBlueRuntimeTables(ctx, dataDir, runtimeConn.Writer)
	if err != nil || result == nil || len(result.TablesMoved) == 0 {
		return result, err
	}
	if err := dbutil.CheckpointWAL(ctx, primaryConn.Writer, dbutil.CheckpointTruncate); err != nil && logger != nil {
		logger.Warn("Primary database checkpoint after runtime split failed", zap.Error(err))
	}
	if _, err := primaryConn.Writer.ExecContext(ctx, "VACUUM"); err != nil {
		if logger != nil {
			logger.Warn("Primary database vacuum after runtime split failed", zap.Error(err))
		}
	} else {
		result.Vacuumed = true
	}
	if err := os.WriteFile(markerPath, []byte(fmt.Sprintf("%d tables %d rows", len(result.TablesMoved), result.RowsMoved)), 0o644); err != nil && logger != nil {
		logger.Warn("Failed to persist runtime db split marker", zap.String("path", markerPath), zap.Error(err))
	}
	return result, nil
}

func runtimeDBHasRows(ctx context.Context, runtimeDB *sql.DB) (bool, error) {
	for _, table := range runtimeDBSplitTables {
		var count int
		if err := runtimeDB.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			return false, fmt.Errorf("count runtime table %s: %w", table, err)
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}

func migrateBlueRuntimeTables(ctx context.Context, dataDir string, runtimeDB *sql.DB) (*RuntimeDBMigrationResult, error) {
	conn, err := runtimeDB.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("open runtime.db connection: %w", err)
	}
	defer conn.Close()

	sourcePath := filepath.Join(dataDir, "blue.db")
	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS blue_source`, sourcePath); err != nil {
		return nil, fmt.Errorf("attach blue.db to runtime.db: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `DETACH DATABASE blue_source`)
	}()

	result := &RuntimeDBMigrationResult{}
	for _, table := range runtimeDBSplitTables {
		exists, err := attachedTableExists(ctx, conn, "blue_source", table)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		sourceRows, err := attachedTableCount(ctx, conn, "blue_source", table)
		if err != nil {
			return nil, err
		}
		if sourceRows > 0 {
			if _, err := conn.ExecContext(ctx, fmt.Sprintf(`INSERT OR IGNORE INTO main.%s SELECT * FROM blue_source.%s`, table, table)); err != nil {
				return nil, fmt.Errorf("copy %s into runtime.db: %w", table, err)
			}
			targetRows, err := attachedTableCount(ctx, conn, "main", table)
			if err != nil {
				return nil, err
			}
			if targetRows != sourceRows {
				return nil, fmt.Errorf("verify %s rows after copy: runtime=%d blue=%d", table, targetRows, sourceRows)
			}
			result.RowsMoved += sourceRows
		}
		if _, err := conn.ExecContext(ctx, fmt.Sprintf(`DROP TABLE blue_source.%s`, table)); err != nil {
			return nil, fmt.Errorf("drop %s from blue.db: %w", table, err)
		}
		result.TablesMoved = append(result.TablesMoved, table)
	}
	if len(result.TablesMoved) == 0 {
		return nil, nil
	}
	return result, nil
}

func attachedTableExists(ctx context.Context, conn *sql.Conn, schema, table string) (bool, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s.sqlite_master WHERE type='table' AND name=?`, schema)
	var count int
	if err := conn.QueryRowContext(ctx, query, table).Scan(&count); err != nil {
		return false, fmt.Errorf("check table %s in %s: %w", table, schema, err)
	}
	return count > 0, nil
}

func attachedTableCount(ctx context.Context, conn *sql.Conn, schema, table string) (int, error) {
	var count int
	if err := conn.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s`, schema, table)).Scan(&count); err != nil {
		return 0, fmt.Errorf("count rows for %s.%s: %w", schema, table, err)
	}
	return count, nil
}
