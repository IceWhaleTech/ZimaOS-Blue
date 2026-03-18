package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
)

var sqliteDatabaseExtensions = map[string]struct{}{
	".db":      {},
	".sqlite":  {},
	".sqlite3": {},
}

// DiscoverSQLiteDatabasePaths returns managed SQLite database files in the data directory.
//
// Only top-level files are considered so startup auto-recovery stays scoped to the
// application's own databases and does not traverse user workspace content.
func DiscoverSQLiteDatabasePaths(dataDir string) ([]string, error) {
	entries, err := os.ReadDir(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read data directory: %w", err)
	}

	paths := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := sqliteDatabaseExtensions[ext]; !ok {
			continue
		}
		paths = append(paths, filepath.Join(dataDir, entry.Name()))
	}

	sort.Strings(paths)
	return paths, nil
}

// AutoRecoveryResult contains information about an auto-recovery operation.
type AutoRecoveryResult struct {
	// Recovered indicates if recovery was performed
	Recovered bool `json:"recovered"`
	// RepairedDatabases lists databases salvaged in place without backup rollback.
	RepairedDatabases []string `json:"repaired_databases,omitempty"`
	// BackupID is the ID of the backup used for recovery
	BackupID string `json:"backup_id,omitempty"`
	// BackupTime is the timestamp of the backup used
	BackupTime time.Time `json:"backup_time,omitempty"`
	// Error contains any error message
	Error string `json:"error,omitempty"`
	// FilesRecovered is the number of files recovered
	FilesRecovered int `json:"files_recovered,omitempty"`
	// DatabasesChecked lists the databases that were checked
	DatabasesChecked []string `json:"databases_checked,omitempty"`
	// CorruptedDatabases lists the databases that were found corrupted
	CorruptedDatabases []string `json:"corrupted_databases,omitempty"`
}

// CheckAndAutoRecover checks database integrity and automatically recovers from backup if corrupted.
// This should be called during application startup before opening databases.
func (m *Manager) CheckAndAutoRecover(ctx context.Context, dbPaths []string) (*AutoRecoveryResult, error) {
	result := &AutoRecoveryResult{
		DatabasesChecked: make([]string, 0),
	}

	// Check each database for corruption
	var corruptedDBs []string
	for _, dbPath := range dbPaths {
		result.DatabasesChecked = append(result.DatabasesChecked, dbPath)

		// First, checkpoint WAL back into the main database file.
		// This preserves committed data and avoids dropping WAL content.
		if err := database.CheckpointWALForDatabase(dbPath, database.CheckpointTruncate); err != nil {
			// Log but continue - this is not fatal
			fmt.Printf("Warning: failed to checkpoint WAL for %s: %v\n", dbPath, err)
		}

		// Check database integrity
		if err := database.QuickCheckDatabase(dbPath); err != nil {
			if database.IsSQLiteCorruptionError(err) {
				if _, repairErr := database.RepairSQLiteDatabase(dbPath); repairErr == nil {
					result.RepairedDatabases = append(result.RepairedDatabases, dbPath)
					continue
				} else {
					fmt.Printf("Warning: failed to repair sqlite database %s: %v\n", dbPath, repairErr)
				}
			}
			corruptedDBs = append(corruptedDBs, dbPath)
			result.CorruptedDatabases = append(result.CorruptedDatabases, dbPath)
		}
	}

	// If no corruption found, return early
	if len(corruptedDBs) == 0 {
		return result, nil
	}

	// Find the most recent valid backup
	backups := m.List()
	if len(backups) == 0 {
		result.Error = "database corruption detected but no backups available for recovery"
		return result, fmt.Errorf("%s", result.Error)
	}

	// Sort backups by creation time (newest first)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].CreatedAt.After(backups[j].CreatedAt)
	})

	// Try to restore from the most recent valid backup
	var lastErr error
	for _, backup := range backups {
		// Verify backup integrity first
		if err := m.Verify(backup.ID); err != nil {
			// If checksum mismatch, try to repair it first
			// This can happen if the backup was created but metadata wasn't updated properly
			if repairErr := m.RepairChecksum(backup.ID); repairErr == nil {
				// Retry verification after repair
				if verifyErr := m.Verify(backup.ID); verifyErr != nil {
					lastErr = verifyErr
					continue
				}
			} else {
				lastErr = err
				continue
			}
		}

		// Perform restore
		opts := DefaultRestoreOptions()
		opts.OverwriteExisting = true

		restoreResult, err := m.Restore(ctx, backup.ID, opts)
		if err != nil {
			lastErr = err
			continue
		}

		if restoreResult.Success {
			result.Recovered = true
			result.RepairedDatabases = nil
			result.BackupID = backup.ID
			result.BackupTime = backup.CreatedAt
			result.FilesRecovered = restoreResult.FilesRestored
			return result, nil
		}

		lastErr = fmt.Errorf("restore completed with errors: %v", restoreResult.Errors)
	}

	result.Error = fmt.Sprintf("failed to recover from any backup: %v", lastErr)
	return result, fmt.Errorf("%s", result.Error)
}

// CheckDatabaseHealth checks the health of all databases in the data directory.
// Returns a list of corrupted database paths.
func (m *Manager) CheckDatabaseHealth(ctx context.Context) ([]string, error) {
	var corrupted []string

	dbPaths, err := DiscoverSQLiteDatabasePaths(m.dataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to find database files: %w", err)
	}

	for _, dbPath := range dbPaths {
		if err := database.QuickCheckDatabase(dbPath); err != nil {
			corrupted = append(corrupted, dbPath)
		}
	}

	return corrupted, nil
}

// GetLatestBackup returns the most recent backup, or nil if no backups exist.
func (m *Manager) GetLatestBackup() *BackupInfo {
	backups := m.List()
	if len(backups) == 0 {
		return nil
	}

	// List already returns sorted by creation time (newest first)
	return backups[0]
}

// RecoverFromLatestBackup attempts to recover from the most recent valid backup.
func (m *Manager) RecoverFromLatestBackup(ctx context.Context) (*AutoRecoveryResult, error) {
	result := &AutoRecoveryResult{}

	backups := m.List()
	if len(backups) == 0 {
		result.Error = "no backups available for recovery"
		return result, fmt.Errorf("%s", result.Error)
	}

	// Try each backup starting from the most recent
	for _, backup := range backups {
		// Verify backup integrity
		if err := m.Verify(backup.ID); err != nil {
			// If checksum mismatch, try to repair it first
			if repairErr := m.RepairChecksum(backup.ID); repairErr == nil {
				// Retry verification after repair
				if verifyErr := m.Verify(backup.ID); verifyErr != nil {
					continue
				}
			} else {
				continue
			}
		}

		// Perform restore
		opts := DefaultRestoreOptions()
		opts.OverwriteExisting = true

		restoreResult, err := m.Restore(ctx, backup.ID, opts)
		if err != nil {
			continue
		}

		if restoreResult.Success {
			result.Recovered = true
			result.BackupID = backup.ID
			result.BackupTime = backup.CreatedAt
			result.FilesRecovered = restoreResult.FilesRestored
			return result, nil
		}
	}

	result.Error = "failed to recover from any available backup"
	return result, fmt.Errorf("%s", result.Error)
}
