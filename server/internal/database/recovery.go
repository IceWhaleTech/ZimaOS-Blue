package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CleanWALFiles removes SQLite WAL mode auxiliary files (.shm and .wal)
// for the given database path. This should be called after restoring a database
// to ensure the restored database starts fresh without stale WAL data.
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

// CleanAllWALFilesInDir removes all SQLite WAL mode auxiliary files
// (.shm and .wal) for all .db files in the given directory.
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
