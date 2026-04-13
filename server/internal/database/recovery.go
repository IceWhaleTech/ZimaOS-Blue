package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var sqliteDatabaseFileExtensions = map[string]struct{}{
	".db":      {},
	".sqlite":  {},
	".sqlite3": {},
}

const sqliteRecoverTimeout = 2 * time.Minute

var sqliteAuxiliarySuffixes = []string{"", "-wal", "-shm"}

const defaultSQLiteQuickCheckTimeout = 2 * time.Minute

var sqliteQuickCheckTimeout = func() time.Duration {
	raw := strings.TrimSpace(os.Getenv("BLUE_SQLITE_QUICK_CHECK_TIMEOUT"))
	if raw == "" {
		return defaultSQLiteQuickCheckTimeout
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return defaultSQLiteQuickCheckTimeout
	}
	return parsed
}()

func sqliteLogf(format string, args ...interface{}) {
	log.Printf("[sqlite] "+format, args...)
}

var runIntegrityCheckOpenDatabase = integrityCheckOpenDatabase

// IsSQLiteCorruptionError reports whether err looks like SQLite file corruption.
func IsSQLiteCorruptionError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "database is malformed") ||
		strings.Contains(msg, "database corruption detected") ||
		strings.Contains(msg, "fts5: corruption") ||
		strings.Contains(msg, "malformed inverted index for fts5 table") ||
		strings.Contains(msg, "sqlite_corrupt") ||
		strings.Contains(msg, "file is not a database")
}

// WrapSQLiteOpenError annotates SQLite corruption errors with the database path.
func WrapSQLiteOpenError(dbPath string, err error) error {
	if err == nil {
		return nil
	}
	if IsSQLiteCorruptionError(err) {
		return fmt.Errorf("sqlite database %s appears corrupted: %w", dbPath, err)
	}
	return err
}

// SQLiteRepairResult describes an in-place repair of a SQLite database file.
type SQLiteRepairResult struct {
	// BackupPath is set only when a rotated corrupt copy is intentionally retained.
	BackupPath string `json:"backup_path,omitempty"`
	Repaired   bool   `json:"repaired"`
	// PartialImport indicates sqlite3 .recover reported problems but enough
	// readable data was batch-imported to produce a healthy replacement DB.
	PartialImport bool `json:"partial_import,omitempty"`
	// RecoverWarning captures sqlite3 .recover/import warnings when the repair
	// still succeeded overall.
	RecoverWarning string `json:"recover_warning,omitempty"`
}

// OpenSQLiteWithRecovery opens a SQLite database and retries after best-effort
// recovery steps when SQLite reports corruption.
func OpenSQLiteWithRecovery(dsn, dbPath string, configure func(*sql.DB) error) (*sql.DB, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = dsn
	}

	var checkpointErr error
	var ftsRepairErr error
	var repairErr error
	triedCheckpoint := false
	triedFTSRepair := false
	triedRepair := false

	for {
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, WrapSQLiteOpenError(dbPath, err)
		}

		if configure != nil {
			if err := configure(db); err != nil {
				_ = db.Close()
				if IsSQLiteCorruptionError(err) {
					if !triedCheckpoint {
						triedCheckpoint = true
						checkpointErr = CheckpointWALForDatabase(dbPath, CheckpointTruncate)
						if checkpointErr == nil {
							continue
						}
					}
					if !triedFTSRepair {
						triedFTSRepair = true
						var repaired []string
						repaired, ftsRepairErr = RepairKnownFTSIndexes(dbPath)
						if ftsRepairErr == nil && len(repaired) > 0 {
							continue
						}
					}
					if !triedRepair {
						triedRepair = true
						_, repairErr = RepairSQLiteDatabase(dbPath)
						if repairErr == nil {
							continue
						}
					}

					baseErr := WrapSQLiteOpenError(dbPath, err)
					switch {
					case checkpointErr != nil && ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, checkpointErr, ftsRepairErr, repairErr)
					case checkpointErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to repair database: %v)", baseErr, checkpointErr, repairErr)
					case ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, ftsRepairErr, repairErr)
					case ftsRepairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v)", baseErr, ftsRepairErr)
					case checkpointErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v)", baseErr, checkpointErr)
					case repairErr != nil:
						return nil, fmt.Errorf("%w (failed to repair database: %v)", baseErr, repairErr)
					default:
						return nil, baseErr
					}
				}
				return nil, WrapSQLiteOpenError(dbPath, err)
			}
		}
		if triedCheckpoint || triedFTSRepair || triedRepair {
			integrityCheckStartedAt := time.Now()
			sqliteLogf("startup integrity_check started db_path=%s timeout=%s", dbPath, 30*time.Second)
			if err := runIntegrityCheckOpenDatabase(db); err != nil {
				sqliteLogf("startup integrity_check failed db_path=%s duration=%s error=%v", dbPath, time.Since(integrityCheckStartedAt), err)
				_ = db.Close()
				if IsSQLiteCorruptionError(err) {
					baseErr := WrapSQLiteOpenError(dbPath, err)
					switch {
					case checkpointErr != nil && ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, checkpointErr, ftsRepairErr, repairErr)
					case checkpointErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to repair database: %v)", baseErr, checkpointErr, repairErr)
					case ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, ftsRepairErr, repairErr)
					case ftsRepairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v)", baseErr, ftsRepairErr)
					case checkpointErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v)", baseErr, checkpointErr)
					case repairErr != nil:
						return nil, fmt.Errorf("%w (failed to repair database: %v)", baseErr, repairErr)
					default:
						return nil, baseErr
					}
				}
				return nil, WrapSQLiteOpenError(dbPath, err)
			}
			sqliteLogf("startup integrity_check completed db_path=%s duration=%s", dbPath, time.Since(integrityCheckStartedAt))
		} else if shouldQuickCheckSQLitePath(dbPath) {
			quickCheckStartedAt := time.Now()
			sqliteLogf("startup quick_check started db_path=%s timeout=%s", dbPath, sqliteQuickCheckTimeout)
			if err := quickCheckOpenDatabase(db); err != nil {
				sqliteLogf("startup quick_check failed db_path=%s duration=%s error=%v", dbPath, time.Since(quickCheckStartedAt), err)
				_ = db.Close()
				if IsSQLiteCorruptionError(err) {
					if !triedCheckpoint {
						triedCheckpoint = true
						checkpointErr = CheckpointWALForDatabase(dbPath, CheckpointTruncate)
						if checkpointErr == nil {
							continue
						}
					}
					if !triedFTSRepair {
						triedFTSRepair = true
						var repaired []string
						repaired, ftsRepairErr = RepairKnownFTSIndexes(dbPath)
						if ftsRepairErr == nil && len(repaired) > 0 {
							continue
						}
					}
					if !triedRepair {
						triedRepair = true
						_, repairErr = RepairSQLiteDatabase(dbPath)
						if repairErr == nil {
							continue
						}
					}

					baseErr := WrapSQLiteOpenError(dbPath, err)
					switch {
					case checkpointErr != nil && ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, checkpointErr, ftsRepairErr, repairErr)
					case checkpointErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to repair database: %v)", baseErr, checkpointErr, repairErr)
					case ftsRepairErr != nil && repairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v; failed to repair database: %v)", baseErr, ftsRepairErr, repairErr)
					case ftsRepairErr != nil:
						return nil, fmt.Errorf("%w (failed to rebuild known FTS indexes: %v)", baseErr, ftsRepairErr)
					case checkpointErr != nil:
						return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v)", baseErr, checkpointErr)
					case repairErr != nil:
						return nil, fmt.Errorf("%w (failed to repair database: %v)", baseErr, repairErr)
					default:
						return nil, baseErr
					}
				}
				return nil, WrapSQLiteOpenError(dbPath, err)
			}
			sqliteLogf("startup quick_check completed db_path=%s duration=%s", dbPath, time.Since(quickCheckStartedAt))
		}

		return db, nil
	}
}

// OpenSQLiteWithRecoveryAndRecreate opens a SQLite database, attempts the normal
// recovery flow, and if the database still appears corrupted, rotates the old
// file set to .bak.<timestamp> before creating a fresh database in its place.
func OpenSQLiteWithRecoveryAndRecreate(dsn, dbPath string, configure func(*sql.DB) error) (*sql.DB, error) {
	db, err := OpenSQLiteWithRecovery(dsn, dbPath, configure)
	if err == nil || !IsSQLiteCorruptionError(err) {
		return db, err
	}

	if strings.TrimSpace(dbPath) == "" {
		dbPath = dsn
	}

	backupPath, rotateErr := RotateCorruptSQLiteDatabase(dbPath)
	if rotateErr != nil {
		return nil, fmt.Errorf("sqlite database %s remains corrupted: %w (rotate to .bak failed: %v)", dbPath, err, rotateErr)
	}

	db, retryErr := OpenSQLiteWithRecovery(dsn, dbPath, configure)
	if retryErr != nil {
		return nil, fmt.Errorf("recreate sqlite database %s after rotating corrupt copy to %s: %w", dbPath, backupPath, retryErr)
	}
	return db, nil
}

func shouldQuickCheckSQLitePath(dbPath string) bool {
	// Keep proactive startup integrity scans focused on the primary conversation
	// store. Auxiliary SQLite files are either rebuildable or already validated
	// on-demand, so scanning all of them after a dirty shutdown multiplies the
	// startup I/O cost without improving first-boot availability enough.
	return consumeStartupQuickCheckPath(dbPath)
}

func quickCheckOpenDatabase(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), sqliteQuickCheckTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return fmt.Errorf("quick check failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("failed to scan quick check result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("quick check error: %w", err)
	}
	if len(results) == 1 && results[0] == "ok" {
		return nil
	}
	return fmt.Errorf("database corruption detected: %v", results)
}

func integrityCheckOpenDatabase(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, "PRAGMA integrity_check")
	if err != nil {
		return fmt.Errorf("integrity check failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("failed to scan integrity check result: %w", err)
		}
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("integrity check error: %w", err)
	}
	if len(results) == 1 && results[0] == "ok" {
		return nil
	}
	return fmt.Errorf("database corruption detected: %v", results)
}

// RepairSQLiteDatabase tries to salvage a corrupted SQLite database using the
// sqlite3 CLI's .recover command, then atomically replaces the original file.
// The corrupt copy is rotated aside only during installation and removed once
// the recovered database has been verified and installed successfully.
func RepairSQLiteDatabase(dbPath string) (result *SQLiteRepairResult, err error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return nil, fmt.Errorf("database path is empty")
	}
	if dbPath == ":memory:" {
		return nil, fmt.Errorf("sqlite repair requires a filesystem path")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("failed to stat database %s: %w", dbPath, err)
	}
	if _, err := exec.LookPath("sqlite3"); err != nil {
		return nil, fmt.Errorf("sqlite3 CLI is unavailable: %w", err)
	}
	startedAt := time.Now()
	sqliteLogf("repair started db_path=%s timeout=%s", dbPath, sqliteRecoverTimeout)
	defer func() {
		if err != nil {
			sqliteLogf("repair failed db_path=%s duration=%s error=%v", dbPath, time.Since(startedAt), err)
			return
		}
		if result != nil {
			sqliteLogf("repair completed db_path=%s duration=%s partial_import=%t warning=%q", dbPath, time.Since(startedAt), result.PartialImport, result.RecoverWarning)
		}
	}()

	suffix := time.Now().UTC().Format("20060102T150405.000000000")
	snapshotPath := dbPath + ".repair-src." + suffix
	recoveredPath := dbPath + ".repair-out." + suffix
	backupPath := corruptSQLiteBackupPath(dbPath, suffix)
	replacedOriginal := false

	defer removeSQLiteArtifacts(snapshotPath)
	defer func() {
		if !replacedOriginal {
			removeSQLiteArtifacts(recoveredPath)
		}
	}()
	defer func() {
		if replacedOriginal {
			_ = restoreSQLiteArtifacts(backupPath, dbPath)
		}
	}()

	if err := copySQLiteArtifacts(dbPath, snapshotPath); err != nil {
		return nil, fmt.Errorf("failed to snapshot database before repair: %w", err)
	}
	sourceSchemaCount, sourceSchemaErr := sqliteUserSchemaObjectCount(snapshotPath)
	recoverErr := runSQLiteRecover(snapshotPath, recoveredPath)
	if recoverErr == nil {
		if err := retryPlainSQLiteRecoverIfRecoveredOutputUnusable(snapshotPath, recoveredPath, sourceSchemaCount, sourceSchemaErr); err != nil {
			return nil, err
		}
	}
	if info, err := os.Stat(recoveredPath); err != nil {
		if recoverErr != nil {
			return nil, fmt.Errorf("failed to recover sqlite database %s: %w", dbPath, recoverErr)
		}
		return nil, fmt.Errorf("sqlite recovery did not produce %s: %w", recoveredPath, err)
	} else if info.Size() == 0 {
		if recoverErr != nil {
			return nil, fmt.Errorf("sqlite recovery produced an empty database file: %w", recoverErr)
		}
		return nil, fmt.Errorf("sqlite recovery produced an empty database file")
	}
	if err := CheckDatabaseIntegrity(recoveredPath); err != nil {
		if recoverErr != nil {
			return nil, fmt.Errorf("recovered sqlite database failed integrity check after partial import: %w (recover warnings: %v)", err, recoverErr)
		}
		return nil, fmt.Errorf("recovered sqlite database failed integrity check: %w", err)
	}
	if err := ensureRecoveredSQLitePreservedUserSchema(recoveredPath, sourceSchemaCount, sourceSchemaErr); err != nil {
		if recoverErr != nil {
			return nil, fmt.Errorf("%w (recover warnings: %v)", err, recoverErr)
		}
		return nil, err
	}
	if err := rotateSQLiteArtifacts(dbPath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to rotate corrupted database out of the way: %w", err)
	}
	replacedOriginal = true
	if err := os.Rename(recoveredPath, dbPath); err != nil {
		return nil, fmt.Errorf("failed to install repaired database %s: %w", dbPath, err)
	}
	replacedOriginal = false
	removeSQLiteArtifacts(backupPath)

	result = &SQLiteRepairResult{
		Repaired:       true,
		PartialImport:  recoverErr != nil,
		RecoverWarning: repairWarningString(recoverErr),
	}
	return result, nil
}

func repairWarningString(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

// RotateCorruptSQLiteDatabase moves a corrupted SQLite database and its
// auxiliary files out of the way so callers can recreate a fresh database.
func RotateCorruptSQLiteDatabase(dbPath string) (string, error) {
	dbPath = strings.TrimSpace(dbPath)
	if dbPath == "" {
		return "", fmt.Errorf("database path is empty")
	}
	if dbPath == ":memory:" {
		return "", fmt.Errorf("sqlite rotation requires a filesystem path")
	}

	suffix := time.Now().UTC().Format("20060102T150405.000000000")
	backupPath := corruptSQLiteBackupPath(dbPath, suffix)
	if err := rotateSQLiteArtifacts(dbPath, backupPath); err != nil {
		return "", err
	}
	return backupPath, nil
}

func corruptSQLiteBackupPath(dbPath, suffix string) string {
	return dbPath + ".bak." + suffix
}

func copySQLiteArtifacts(srcBase, dstBase string) error {
	for _, suffix := range sqliteAuxiliarySuffixes {
		src := srcBase + suffix
		dst := dstBase + suffix
		if err := copySQLiteArtifact(src, dst); err != nil {
			return err
		}
	}
	return nil
}

func copySQLiteArtifact(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode().Perm())
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return fmt.Errorf("copy %s to %s: %w", src, dst, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dst, err)
	}
	if err := os.Chtimes(dst, info.ModTime(), info.ModTime()); err != nil {
		return fmt.Errorf("set times for %s: %w", dst, err)
	}
	return nil
}

func rotateSQLiteArtifacts(srcBase, dstBase string) error {
	moved := make([]string, 0, len(sqliteAuxiliarySuffixes))
	for _, suffix := range sqliteAuxiliarySuffixes {
		src := srcBase + suffix
		dst := dstBase + suffix
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat %s: %w", src, err)
		}
		if err := os.Rename(src, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				restoreSrc := dstBase + moved[i]
				restoreDst := srcBase + moved[i]
				_ = os.Rename(restoreSrc, restoreDst)
			}
			return fmt.Errorf("rename %s to %s: %w", src, dst, err)
		}
		moved = append(moved, suffix)
	}
	if len(moved) == 0 {
		return fmt.Errorf("no sqlite files found at %s", srcBase)
	}
	return nil
}

func restoreSQLiteArtifacts(srcBase, dstBase string) error {
	var errs []error
	for _, suffix := range sqliteAuxiliarySuffixes {
		src := srcBase + suffix
		dst := dstBase + suffix
		if _, err := os.Stat(src); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			errs = append(errs, fmt.Errorf("stat %s: %w", src, err))
			continue
		}
		if err := os.Rename(src, dst); err != nil {
			errs = append(errs, fmt.Errorf("rename %s to %s: %w", src, dst, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to restore sqlite files: %v", errs)
	}
	return nil
}

func removeSQLiteArtifacts(base string) {
	for _, suffix := range sqliteAuxiliarySuffixes {
		_ = os.Remove(base + suffix)
	}
}

func runSQLiteRecover(srcPath, dstPath string) error {
	err := runSQLiteRecoverAttempt(srcPath, dstPath, ".recover --ignore-freelist")
	if err == nil {
		return nil
	}
	if !shouldRetrySQLiteRecoverWithoutIgnoreFreelist(err) {
		return err
	}

	sqliteLogf("retrying sqlite recover without --ignore-freelist src_path=%s error=%v", srcPath, err)
	fallbackErr := runSQLiteRecoverAttempt(srcPath, dstPath, ".recover")
	if fallbackErr == nil {
		return nil
	}
	return fmt.Errorf("%v; fallback plain recover failed: %w", err, fallbackErr)
}

func runSQLiteRecoverAttempt(srcPath, dstPath string, recoverCommand string) error {
	_ = os.Remove(dstPath)

	ctx, cancel := context.WithTimeout(context.Background(), sqliteRecoverTimeout)
	defer cancel()

	recoverCmd := exec.CommandContext(ctx, "sqlite3", "-batch", srcPath, recoverCommand)
	importCmd := exec.CommandContext(
		ctx,
		"sqlite3",
		"-batch",
		"-cmd", "PRAGMA journal_mode=OFF",
		"-cmd", "PRAGMA synchronous=OFF",
		"-cmd", "PRAGMA temp_store=MEMORY",
		dstPath,
	)

	recoverOut, err := recoverCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create recover stdout pipe: %w", err)
	}

	var recoverStderr bytes.Buffer
	var importStdout bytes.Buffer
	var importStderr bytes.Buffer
	recoverCmd.Stderr = &recoverStderr
	importCmd.Stdin = recoverOut
	importCmd.Stdout = &importStdout
	importCmd.Stderr = &importStderr

	if err := importCmd.Start(); err != nil {
		return fmt.Errorf("start sqlite import: %w", err)
	}
	if err := recoverCmd.Start(); err != nil {
		_ = importCmd.Process.Kill()
		_, _ = importCmd.Process.Wait()
		return fmt.Errorf("start sqlite recover: %w", err)
	}

	recoverErr := recoverCmd.Wait()
	importErr := importCmd.Wait()
	if ctx.Err() != nil {
		return fmt.Errorf("sqlite recover timed out after %s", sqliteRecoverTimeout)
	}
	if recoverErr != nil || importErr != nil {
		var details []string
		if recoverErr != nil {
			details = append(details, fmt.Sprintf("recover command failed: %v", recoverErr))
		}
		if msg := strings.TrimSpace(recoverStderr.String()); msg != "" {
			details = append(details, "recover stderr: "+msg)
		}
		if importErr != nil {
			details = append(details, fmt.Sprintf("import command failed: %v", importErr))
		}
		if msg := strings.TrimSpace(importStderr.String()); msg != "" {
			details = append(details, "import stderr: "+msg)
		}
		if msg := strings.TrimSpace(importStdout.String()); msg != "" {
			details = append(details, "import stdout: "+msg)
		}
		return fmt.Errorf("%s", strings.Join(details, "; "))
	}

	return nil
}

func shouldRetrySQLiteRecoverWithoutIgnoreFreelist(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlite_dbpage") ||
		strings.Contains(msg, "ignore-freelist")
}

func retryPlainSQLiteRecoverIfRecoveredOutputUnusable(srcPath, dstPath string, sourceSchemaCount int, sourceSchemaErr error) error {
	reason, err := recoveredSQLitePlainRecoverFallbackReason(dstPath, sourceSchemaCount, sourceSchemaErr)
	if err != nil {
		return err
	}
	if reason == "" {
		return nil
	}

	sqliteLogf("retrying sqlite recover without --ignore-freelist src_path=%s reason=%s", srcPath, reason)
	if err := runSQLiteRecoverAttempt(srcPath, dstPath, ".recover"); err != nil {
		return fmt.Errorf("%s: plain .recover retry failed: %w", reason, err)
	}
	if postReason, err := recoveredSQLitePlainRecoverFallbackReason(dstPath, sourceSchemaCount, sourceSchemaErr); err != nil {
		return err
	} else if postReason != "" {
		return fmt.Errorf("%s after plain .recover retry", postReason)
	}
	return nil
}

func recoveredSQLitePlainRecoverFallbackReason(dstPath string, sourceSchemaCount int, sourceSchemaErr error) (string, error) {
	info, err := os.Stat(dstPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "sqlite recovery did not produce a database file", nil
		}
		return "", fmt.Errorf("stat recovered sqlite database %s: %w", dstPath, err)
	}
	if info.Size() == 0 {
		return "sqlite recovery produced an empty database file", nil
	}
	if err := CheckDatabaseIntegrity(dstPath); err != nil {
		return fmt.Sprintf("recovered sqlite database failed integrity check: %v", err), nil
	}
	if err := ensureRecoveredSQLitePreservedUserSchema(dstPath, sourceSchemaCount, sourceSchemaErr); err != nil {
		return err.Error(), nil
	}
	return "", nil
}

func ensureRecoveredSQLitePreservedUserSchema(recoveredPath string, sourceSchemaCount int, sourceSchemaErr error) error {
	if sourceSchemaErr != nil || sourceSchemaCount <= 0 {
		return nil
	}
	recoveredSchemaCount, err := sqliteUserSchemaObjectCount(recoveredPath)
	if err != nil {
		return fmt.Errorf("failed to inspect recovered sqlite schema: %w", err)
	}
	if recoveredSchemaCount == 0 {
		return fmt.Errorf("recovered sqlite database lost all %d user schema objects", sourceSchemaCount)
	}
	return nil
}

func sqliteUserSchemaObjectCount(dbPath string) (int, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	var count int
	err = db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type IN ('table', 'view', 'trigger', 'index')
		  AND name NOT LIKE 'sqlite_%'
	`).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CheckpointWAL runs PRAGMA wal_checkpoint(mode) on an already opened DB.
func CheckpointWAL(ctx context.Context, db *sql.DB, mode CheckpointMode) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	mode = normalizeCheckpointMode(mode)

	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		return fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	var busy, logFrames, checkpointed int
	query := fmt.Sprintf("PRAGMA wal_checkpoint(%s)", mode)
	if err := db.QueryRowContext(ctx, query).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return fmt.Errorf("wal checkpoint query failed: %w", err)
	}
	if busy > 0 {
		return fmt.Errorf("wal checkpoint %s busy=%d log_frames=%d checkpointed=%d", mode, busy, logFrames, checkpointed)
	}
	return nil
}

// CheckpointWALForDatabase opens a SQLite database by path and checkpoints WAL.
func CheckpointWALForDatabase(dbPath string, mode CheckpointMode) (err error) {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("database path is empty")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat database %s: %w", dbPath, err)
	}
	startedAt := time.Now()
	sqliteLogf("wal checkpoint started db_path=%s mode=%s", dbPath, mode)
	defer func() {
		if err != nil {
			sqliteLogf("wal checkpoint failed db_path=%s mode=%s duration=%s error=%v", dbPath, mode, time.Since(startedAt), err)
			return
		}
		sqliteLogf("wal checkpoint completed db_path=%s mode=%s duration=%s", dbPath, mode, time.Since(startedAt))
	}()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database for WAL checkpoint: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := CheckpointWAL(ctx, db, mode); err != nil {
		return fmt.Errorf("failed to checkpoint WAL for %s: %w", dbPath, err)
	}

	return nil
}

// CheckpointAllDatabasesInDir checkpoints WAL for top-level SQLite DB files in dir.
func CheckpointAllDatabasesInDir(dir string, mode CheckpointMode) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var errs []error
	for _, entry := range entries {
		if entry.IsDir() || !isSQLiteDatabaseFilename(entry.Name()) {
			continue
		}
		dbPath := filepath.Join(dir, entry.Name())
		if err := CheckpointWALForDatabase(dbPath, mode); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to checkpoint WAL for some databases: %v", errs)
	}
	return nil
}

// StartPeriodicWALCheckpoint runs wal_checkpoint(mode) on a fixed interval until ctx is done.
func StartPeriodicWALCheckpoint(ctx context.Context, db *sql.DB, interval time.Duration, mode CheckpointMode, onError func(error)) {
	if db == nil || interval <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if onError == nil {
		onError = func(error) {}
	}

	mode = normalizeCheckpointMode(mode)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				startedAt := time.Now()
				sqliteLogf("scheduled wal checkpoint started interval=%s mode=%s", interval, mode)
				checkpointCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				err := CheckpointWAL(checkpointCtx, db, mode)
				cancel()
				if err != nil {
					sqliteLogf("scheduled wal checkpoint failed interval=%s mode=%s duration=%s error=%v", interval, mode, time.Since(startedAt), err)
					onError(err)
					continue
				}
				sqliteLogf("scheduled wal checkpoint completed interval=%s mode=%s duration=%s", interval, mode, time.Since(startedAt))
			}
		}
	}()
}

func normalizeCheckpointMode(mode CheckpointMode) CheckpointMode {
	switch mode {
	case CheckpointPassive, CheckpointFull, CheckpointRestart, CheckpointTruncate:
		return mode
	default:
		return CheckpointTruncate
	}
}

func isSQLiteDatabaseFilename(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	_, ok := sqliteDatabaseFileExtensions[ext]
	return ok
}

// CleanWALFiles removes SQLite WAL auxiliary files (.shm and .wal).
//
// Deprecated: prefer CheckpointWALForDatabase(..., CheckpointTruncate) so
// committed WAL frames are merged into the main database instead of dropped.
func CleanWALFiles(dbPath string) error {
	shmPath := dbPath + "-shm"
	walPath := dbPath + "-wal"

	var errs []error

	// Remove .shm file if exists
	if _, err := os.Stat(shmPath); err == nil {
		if err := os.Remove(shmPath); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", shmPath, err))
		}
	}

	// Remove .wal file if exists
	if _, err := os.Stat(walPath); err == nil {
		if err := os.Remove(walPath); err != nil {
			errs = append(errs, fmt.Errorf("failed to remove %s: %w", walPath, err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to clean WAL files: %v", errs)
	}

	return nil
}

// CleanAllWALFilesInDir removes all SQLite WAL auxiliary files in a directory.
//
// Deprecated: prefer CheckpointAllDatabasesInDir(..., CheckpointTruncate).
func CleanAllWALFilesInDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var errs []error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) == ".db" {
			dbPath := filepath.Join(dir, entry.Name())
			if err := CleanWALFiles(dbPath); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("failed to clean some WAL files: %v", errs)
	}

	return nil
}

// CheckDatabaseIntegrity checks if a SQLite database is corrupted.
// Returns nil if the database is healthy, or an error describing the corruption.
// Uses a timeout to prevent hanging on severely corrupted databases.
func CheckDatabaseIntegrity(dbPath string) error {
	// First check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil // No database file, nothing to check
	}

	// Try to open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	return runIntegrityCheckOpenDatabase(db)
}

// QuickCheckDatabase performs a quick integrity check on the database.
// This is faster than full integrity check but may miss some issues.
// Uses a timeout to prevent hanging on severely corrupted databases.
func QuickCheckDatabase(dbPath string) (err error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}
	startedAt := time.Now()
	sqliteLogf("quick check started db_path=%s timeout=%s", dbPath, sqliteQuickCheckTimeout)
	defer func() {
		if err != nil {
			sqliteLogf("quick check failed db_path=%s duration=%s error=%v", dbPath, time.Since(startedAt), err)
			return
		}
		sqliteLogf("quick check completed db_path=%s duration=%s", dbPath, time.Since(startedAt))
	}()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Use a timeout context to prevent hanging on corrupted databases
	ctx, cancel := context.WithTimeout(context.Background(), sqliteQuickCheckTimeout)
	defer cancel()

	rows, err := db.QueryContext(ctx, "PRAGMA quick_check")
	if err != nil {
		return fmt.Errorf("quick check failed: %w", err)
	}
	defer rows.Close()

	var results []string
	for rows.Next() {
		var result string
		if err := rows.Scan(&result); err != nil {
			return fmt.Errorf("failed to scan quick check result: %w", err)
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("quick check error: %w", err)
	}

	if len(results) == 1 && results[0] == "ok" {
		return nil
	}

	return fmt.Errorf("database corruption detected: %v", results)
}

// RecoveryResult contains information about a database recovery operation.
type RecoveryResult struct {
	// Recovered indicates if recovery was performed
	Recovered bool `json:"recovered"`
	// BackupID is the ID of the backup used for recovery
	BackupID string `json:"backup_id,omitempty"`
	// BackupTime is the timestamp of the backup used
	BackupTime string `json:"backup_time,omitempty"`
	// Error contains any error message
	Error string `json:"error,omitempty"`
	// FilesRecovered lists the files that were recovered
	FilesRecovered []string `json:"files_recovered,omitempty"`
}

type IdleCheckpointProbe func(context.Context) (idle bool, reason string, err error)

// StartIdleAwarePeriodicWALCheckpoint runs wal_checkpoint(mode) on a fixed
// interval, but only when the caller-provided probe reports the runtime is idle.
func StartIdleAwarePeriodicWALCheckpoint(ctx context.Context, db *sql.DB, interval, idleThreshold time.Duration, probe IdleCheckpointProbe, mode CheckpointMode, onError func(error)) {
	if db == nil || interval <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if onError == nil {
		onError = func(error) {}
	}
	if probe == nil {
		probe = func(context.Context) (bool, string, error) { return true, "", nil }
	}

	mode = normalizeCheckpointMode(mode)

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				probeCtx, cancelProbe := context.WithTimeout(ctx, 5*time.Second)
				idle, reason, probeErr := probe(probeCtx)
				cancelProbe()
				if probeErr != nil {
					sqliteLogf("idle-aware wal checkpoint probe failed interval=%s idle_threshold=%s error=%v", interval, idleThreshold, probeErr)
					onError(probeErr)
					continue
				}
				if !idle {
					sqliteLogf("idle-aware wal checkpoint skipped interval=%s idle_threshold=%s reason=%s", interval, idleThreshold, reason)
					continue
				}

				startedAt := time.Now()
				sqliteLogf("idle-aware wal checkpoint started interval=%s idle_threshold=%s mode=%s", interval, idleThreshold, mode)
				checkpointCtx, cancelCheckpoint := context.WithTimeout(ctx, 30*time.Second)
				err := CheckpointWAL(checkpointCtx, db, mode)
				cancelCheckpoint()
				if err != nil {
					sqliteLogf("idle-aware wal checkpoint failed interval=%s idle_threshold=%s mode=%s duration=%s error=%v", interval, idleThreshold, mode, time.Since(startedAt), err)
					onError(err)
					continue
				}
				sqliteLogf("idle-aware wal checkpoint completed interval=%s idle_threshold=%s mode=%s duration=%s", interval, idleThreshold, mode, time.Since(startedAt))
			}
		}
	}()
}
