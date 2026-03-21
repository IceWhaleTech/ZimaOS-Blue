package harness

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const LegacyStoreDBFilename = "harness.db"

var legacyHarnessTables = []string{
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
}

var sqliteArtifactSuffixes = []string{"", "-wal", "-shm"}

// LegacyStoreMigrationResult describes a one-time import from the legacy
// standalone harness.db file into the shared blue.db store.
type LegacyStoreMigrationResult struct {
	SourcePath   string
	ArchivedPath string
	RowsImported int
}

// LegacyStoreDBPath returns the legacy standalone harness database path.
func LegacyStoreDBPath(dataDir string) string {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return LegacyStoreDBFilename
	}
	return filepath.Join(dataDir, LegacyStoreDBFilename)
}

// MigrateLegacyStore imports rows from the legacy harness.db file into the
// shared SQLite database and archives the standalone file afterward.
func MigrateLegacyStore(ctx context.Context, db *sql.DB, dataDir string) (*LegacyStoreMigrationResult, error) {
	if db == nil || strings.TrimSpace(dataDir) == "" {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if _, err := NewSQLiteStore(db); err != nil {
		return nil, fmt.Errorf("prepare shared harness schema: %w", err)
	}

	legacyPath := LegacyStoreDBPath(dataDir)
	if _, err := os.Stat(legacyPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat legacy harness db %s: %w", legacyPath, err)
	}

	legacyDB, err := sql.Open("sqlite3", legacyPath)
	if err != nil {
		return nil, fmt.Errorf("open legacy harness db %s: %w", legacyPath, err)
	}

	if _, err := NewSQLiteStore(legacyDB); err != nil {
		_ = legacyDB.Close()
		return nil, fmt.Errorf("prepare legacy harness schema %s: %w", legacyPath, err)
	}
	if err := legacyDB.Close(); err != nil {
		return nil, fmt.Errorf("close legacy harness db %s: %w", legacyPath, err)
	}

	rowsImported, err := importLegacyHarnessTables(ctx, db, legacyPath)
	if err != nil {
		return nil, fmt.Errorf("import legacy harness rows from %s: %w", legacyPath, err)
	}

	result := &LegacyStoreMigrationResult{
		SourcePath:   legacyPath,
		RowsImported: rowsImported,
	}
	if archivedPath, err := archiveLegacyHarnessDB(legacyPath); err == nil {
		result.ArchivedPath = archivedPath
	}
	return result, nil
}

func importLegacyHarnessTables(ctx context.Context, db *sql.DB, legacyPath string) (int, error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS legacy_harness`, legacyPath); err != nil {
		return 0, err
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `DETACH DATABASE legacy_harness`)
	}()

	total := 0
	for _, table := range legacyHarnessTables {
		query := fmt.Sprintf(`INSERT OR IGNORE INTO %s SELECT * FROM legacy_harness.%s`, table, table)
		res, err := conn.ExecContext(ctx, query)
		if err != nil {
			return 0, err
		}
		if n, err := res.RowsAffected(); err == nil {
			total += int(n)
		}
	}
	return total, nil
}

func archiveLegacyHarnessDB(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("legacy harness path is empty")
	}
	archivedPath := path + ".migrated"
	if _, err := os.Stat(archivedPath); err == nil {
		archivedPath = fmt.Sprintf("%s.%s", archivedPath, time.Now().UTC().Format("20060102T150405.000000000"))
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := moveSQLiteArtifacts(path, archivedPath); err != nil {
		return "", err
	}
	return archivedPath, nil
}

func moveSQLiteArtifacts(srcBase, dstBase string) error {
	moved := make([]string, 0, len(sqliteArtifactSuffixes))
	for _, suffix := range sqliteArtifactSuffixes {
		src := srcBase + suffix
		dst := dstBase + suffix
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := os.Rename(src, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = os.Rename(dstBase+moved[i], srcBase+moved[i])
			}
			return err
		}
		moved = append(moved, suffix)
	}
	if len(moved) == 0 {
		return fmt.Errorf("no sqlite files found at %s", srcBase)
	}
	return nil
}
