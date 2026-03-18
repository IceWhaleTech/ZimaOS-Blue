package database

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
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

// IsSQLiteCorruptionError reports whether err looks like SQLite file corruption.
func IsSQLiteCorruptionError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "database is malformed") ||
		strings.Contains(msg, "database corruption detected") ||
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
	BackupPath string `json:"backup_path"`
	Repaired   bool   `json:"repaired"`
}

// OpenSQLiteWithRecovery opens a SQLite database and retries after best-effort
// recovery steps when SQLite reports corruption.
func OpenSQLiteWithRecovery(dsn, dbPath string, configure func(*sql.DB) error) (*sql.DB, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = dsn
	}

	var checkpointErr error
	var repairErr error
	triedCheckpoint := false
	triedRepair := false

	for {
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, WrapSQLiteOpenError(dbPath, err)
		}

		if configure == nil {
			return db, nil
		}

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
				if !triedRepair {
					triedRepair = true
					_, repairErr = RepairSQLiteDatabase(dbPath)
					if repairErr == nil {
						continue
					}
				}

				baseErr := WrapSQLiteOpenError(dbPath, err)
				switch {
				case checkpointErr != nil && repairErr != nil:
					return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v; failed to repair database: %v)", baseErr, checkpointErr, repairErr)
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

		return db, nil
	}
}

// RepairSQLiteDatabase tries to salvage a corrupted SQLite database using the
// sqlite3 CLI's .recover command, then atomically replaces the original file
// while preserving the corrupted copy beside it for inspection.
func RepairSQLiteDatabase(dbPath string) (*SQLiteRepairResult, error) {
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

	suffix := time.Now().UTC().Format("20060102T150405.000000000")
	snapshotPath := dbPath + ".repair-src." + suffix
	recoveredPath := dbPath + ".repair-out." + suffix
	backupPath := dbPath + ".corrupt." + suffix
	replacedOriginal := false
	keepBackup := false

	defer removeSQLiteArtifacts(snapshotPath)
	defer func() {
		if !replacedOriginal {
			removeSQLiteArtifacts(recoveredPath)
		}
	}()
	defer func() {
		if replacedOriginal && !keepBackup {
			_ = restoreSQLiteArtifacts(backupPath, dbPath)
		}
	}()

	if err := copySQLiteArtifacts(dbPath, snapshotPath); err != nil {
		return nil, fmt.Errorf("failed to snapshot database before repair: %w", err)
	}
	if err := runSQLiteRecover(snapshotPath, recoveredPath); err != nil {
		return nil, fmt.Errorf("failed to recover sqlite database %s: %w", dbPath, err)
	}
	if info, err := os.Stat(recoveredPath); err != nil {
		return nil, fmt.Errorf("sqlite recovery did not produce %s: %w", recoveredPath, err)
	} else if info.Size() == 0 {
		return nil, fmt.Errorf("sqlite recovery produced an empty database file")
	}
	if err := CheckDatabaseIntegrity(recoveredPath); err != nil {
		return nil, fmt.Errorf("recovered sqlite database failed integrity check: %w", err)
	}
	if err := rotateSQLiteArtifacts(dbPath, backupPath); err != nil {
		return nil, fmt.Errorf("failed to rotate corrupted database out of the way: %w", err)
	}
	replacedOriginal = true
	if err := os.Rename(recoveredPath, dbPath); err != nil {
		return nil, fmt.Errorf("failed to install repaired database %s: %w", dbPath, err)
	}
	keepBackup = true
	replacedOriginal = false

	return &SQLiteRepairResult{
		BackupPath: backupPath,
		Repaired:   true,
	}, nil
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
	_ = os.Remove(dstPath)

	ctx, cancel := context.WithTimeout(context.Background(), sqliteRecoverTimeout)
	defer cancel()

	recoverCmd := exec.CommandContext(ctx, "sqlite3", srcPath, ".recover")
	importCmd := exec.CommandContext(ctx, "sqlite3", dstPath)

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
		return fmt.Errorf(strings.Join(details, "; "))
	}

	return nil
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
func CheckpointWALForDatabase(dbPath string, mode CheckpointMode) error {
	if strings.TrimSpace(dbPath) == "" {
		return fmt.Errorf("database path is empty")
	}
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to stat database %s: %w", dbPath, err)
	}

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
				checkpointCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
				err := CheckpointWAL(checkpointCtx, db, mode)
				cancel()
				if err != nil {
					onError(err)
				}
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

	// Use a timeout context to prevent hanging on corrupted databases
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run integrity check
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

	// Check results - "ok" means database is healthy
	if len(results) == 1 && results[0] == "ok" {
		return nil
	}

	// Database is corrupted
	return fmt.Errorf("database corruption detected: %v", results)
}

// QuickCheckDatabase performs a quick integrity check on the database.
// This is faster than full integrity check but may miss some issues.
// Uses a timeout to prevent hanging on severely corrupted databases.
func QuickCheckDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Use a timeout context to prevent hanging on corrupted databases
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
