package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var sqliteDatabaseFileExtensions = map[string]struct{}{
	".db":      {},
	".sqlite":  {},
	".sqlite3": {},
}

// IsSQLiteCorruptionError reports whether err looks like SQLite file corruption.
func IsSQLiteCorruptionError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database disk image is malformed") ||
		strings.Contains(msg, "database is malformed") ||
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

// OpenSQLiteWithRecovery opens a SQLite database and retries once after a WAL
// checkpoint when SQLite reports corruption.
func OpenSQLiteWithRecovery(dsn, dbPath string, configure func(*sql.DB) error) (*sql.DB, error) {
	if strings.TrimSpace(dbPath) == "" {
		dbPath = dsn
	}

	for attempt := 0; attempt < 2; attempt++ {
		db, err := sql.Open("sqlite3", dsn)
		if err != nil {
			return nil, WrapSQLiteOpenError(dbPath, err)
		}

		if configure == nil {
			return db, nil
		}

		if err := configure(db); err != nil {
			_ = db.Close()
			if attempt == 0 && IsSQLiteCorruptionError(err) {
				if checkpointErr := CheckpointWALForDatabase(dbPath, CheckpointTruncate); checkpointErr == nil {
					continue
				} else {
					return nil, fmt.Errorf("%w (failed to checkpoint WAL: %v)", WrapSQLiteOpenError(dbPath, err), checkpointErr)
				}
			}
			return nil, WrapSQLiteOpenError(dbPath, err)
		}

		return db, nil
	}

	return nil, fmt.Errorf("failed to open sqlite database %s", dbPath)
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
