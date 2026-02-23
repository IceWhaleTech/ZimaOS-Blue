package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// Migration represents a database/config migration.
type Migration struct {
	Version     string                                       `json:"version"`
	Description string                                       `json:"description"`
	Up          func(ctx context.Context, m *Manager) error `json:"-"`
	Down        func(ctx context.Context, m *Manager) error `json:"-"`
}

// MigrationRecord tracks applied migrations.
type MigrationRecord struct {
	Version   string `json:"version"`
	AppliedAt string `json:"applied_at"`
	Success   bool   `json:"success"`
}

// MigrationHistory stores the history of applied migrations.
type MigrationHistory struct {
	CurrentVersion string            `json:"current_version"`
	Migrations     []MigrationRecord `json:"migrations"`
}

// MigrationManager handles version migrations.
type MigrationManager struct {
	manager     *Manager
	migrations  []Migration
	historyPath string
}

// NewMigrationManager creates a new migration manager.
func NewMigrationManager(m *Manager, dataDir string) *MigrationManager {
	mm := &MigrationManager{
		manager:     m,
		migrations:  make([]Migration, 0),
		historyPath: filepath.Join(dataDir, "migration_history.json"),
	}

	// Register built-in migrations
	mm.registerBuiltinMigrations()

	return mm
}

// RegisterMigration adds a new migration.
func (mm *MigrationManager) RegisterMigration(m Migration) {
	mm.migrations = append(mm.migrations, m)
	// Sort migrations by version
	sort.Slice(mm.migrations, func(i, j int) bool {
		vi, _ := semver.NewVersion(mm.migrations[i].Version)
		vj, _ := semver.NewVersion(mm.migrations[j].Version)
		if vi == nil || vj == nil {
			return mm.migrations[i].Version < mm.migrations[j].Version
		}
		return vi.LessThan(vj)
	})
}

// GetPendingMigrations returns migrations that need to be applied.
func (mm *MigrationManager) GetPendingMigrations(targetVersion string) ([]Migration, error) {
	history, err := mm.loadHistory()
	if err != nil {
		return nil, err
	}

	currentVersion := history.CurrentVersion
	if currentVersion == "" {
		currentVersion = "0.0.0"
	}

	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	target, err := semver.NewVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid target version: %w", err)
	}

	var pending []Migration
	for _, m := range mm.migrations {
		v, err := semver.NewVersion(m.Version)
		if err != nil {
			continue
		}

		// Include migrations between current and target
		if v.GreaterThan(current) && (v.LessThan(target) || v.Equal(target)) {
			pending = append(pending, m)
		}
	}

	return pending, nil
}

// Migrate runs all pending migrations up to the target version.
func (mm *MigrationManager) Migrate(ctx context.Context, targetVersion string) (*MigrationResult, error) {
	pending, err := mm.GetPendingMigrations(targetVersion)
	if err != nil {
		return nil, err
	}

	result := &MigrationResult{
		Success:         true,
		MigrationsRun:   0,
		MigrationsTotal: len(pending),
		AppliedVersions: make([]string, 0),
		Errors:          make([]string, 0),
	}

	if len(pending) == 0 {
		result.Message = "No migrations to apply"
		return result, nil
	}

	history, _ := mm.loadHistory()

	for _, m := range pending {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Errors = append(result.Errors, "migration cancelled")
			return result, ctx.Err()
		default:
		}

		if m.Up == nil {
			continue
		}

		if err := m.Up(ctx, mm.manager); err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("migration %s failed: %v", m.Version, err))

			// Record failed migration
			history.Migrations = append(history.Migrations, MigrationRecord{
				Version:   m.Version,
				AppliedAt: timeutil.NowTime().Format(time.RFC3339),
				Success:   false,
			})
			mm.saveHistory(history)

			return result, fmt.Errorf("migration %s failed: %w", m.Version, err)
		}

		// Record successful migration
		history.Migrations = append(history.Migrations, MigrationRecord{
			Version:   m.Version,
			AppliedAt: timeutil.NowTime().Format(time.RFC3339),
			Success:   true,
		})
		history.CurrentVersion = m.Version

		result.MigrationsRun++
		result.AppliedVersions = append(result.AppliedVersions, m.Version)
	}

	if err := mm.saveHistory(history); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to save migration history: %v", err))
	}

	result.Message = fmt.Sprintf("Successfully applied %d migrations", result.MigrationsRun)
	return result, nil
}

// Rollback rolls back to a specific version.
func (mm *MigrationManager) Rollback(ctx context.Context, targetVersion string) (*MigrationResult, error) {
	history, err := mm.loadHistory()
	if err != nil {
		return nil, err
	}

	currentVersion := history.CurrentVersion
	if currentVersion == "" {
		return &MigrationResult{
			Success: true,
			Message: "No migrations to rollback",
		}, nil
	}

	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	target, err := semver.NewVersion(targetVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid target version: %w", err)
	}

	if target.GreaterThan(current) || target.Equal(current) {
		return &MigrationResult{
			Success: true,
			Message: "Target version is not lower than current version",
		}, nil
	}

	// Find migrations to rollback (in reverse order)
	var toRollback []Migration
	for i := len(mm.migrations) - 1; i >= 0; i-- {
		m := mm.migrations[i]
		v, err := semver.NewVersion(m.Version)
		if err != nil {
			continue
		}

		if (v.LessThan(current) || v.Equal(current)) && v.GreaterThan(target) {
			toRollback = append(toRollback, m)
		}
	}

	result := &MigrationResult{
		Success:         true,
		MigrationsRun:   0,
		MigrationsTotal: len(toRollback),
		AppliedVersions: make([]string, 0),
		Errors:          make([]string, 0),
	}

	for _, m := range toRollback {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Errors = append(result.Errors, "rollback cancelled")
			return result, ctx.Err()
		default:
		}

		if m.Down == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("migration %s has no rollback", m.Version))
			continue
		}

		if err := m.Down(ctx, mm.manager); err != nil {
			result.Success = false
			result.Errors = append(result.Errors, fmt.Sprintf("rollback %s failed: %v", m.Version, err))
			return result, fmt.Errorf("rollback %s failed: %w", m.Version, err)
		}

		result.MigrationsRun++
		result.AppliedVersions = append(result.AppliedVersions, m.Version)
	}

	// Update history
	history.CurrentVersion = targetVersion
	if err := mm.saveHistory(history); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to save migration history: %v", err))
	}

	result.Message = fmt.Sprintf("Successfully rolled back %d migrations", result.MigrationsRun)
	return result, nil
}

// GetCurrentVersion returns the current schema version.
func (mm *MigrationManager) GetCurrentVersion() (string, error) {
	history, err := mm.loadHistory()
	if err != nil {
		return "", err
	}
	if history.CurrentVersion == "" {
		return "0.0.0", nil
	}
	return history.CurrentVersion, nil
}

// GetHistory returns the migration history.
func (mm *MigrationManager) GetHistory() (*MigrationHistory, error) {
	return mm.loadHistory()
}

// CheckCompatibility checks if a backup is compatible with the current version.
func (mm *MigrationManager) CheckCompatibility(backupVersion, currentVersion string) (*CompatibilityResult, error) {
	backup, err := semver.NewVersion(backupVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid backup version: %w", err)
	}

	current, err := semver.NewVersion(currentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}

	result := &CompatibilityResult{
		BackupVersion:     backupVersion,
		CurrentVersion:    currentVersion,
		Compatible:        true,
		RequiresMigration: false,
	}

	if backup.Equal(current) {
		result.Message = "Versions match, no migration needed"
		return result, nil
	}

	if backup.LessThan(current) {
		// Backup is older, need to migrate up
		pending, err := mm.GetPendingMigrations(currentVersion)
		if err != nil {
			return nil, err
		}

		// Filter to only migrations after backup version
		var needed []Migration
		for _, m := range pending {
			v, _ := semver.NewVersion(m.Version)
			if v != nil && v.GreaterThan(backup) {
				needed = append(needed, m)
			}
		}

		result.RequiresMigration = len(needed) > 0
		result.MigrationsNeeded = len(needed)
		result.MigrationVersions = make([]string, len(needed))
		for i, m := range needed {
			result.MigrationVersions[i] = m.Version
		}
		result.Message = fmt.Sprintf("Backup is from older version, %d migrations needed", len(needed))
	} else {
		// Backup is newer than current
		result.Compatible = false
		result.Message = "Backup is from a newer version, cannot restore without upgrading"
	}

	return result, nil
}

// MigrationResult contains the result of a migration operation.
type MigrationResult struct {
	Success         bool     `json:"success"`
	Message         string   `json:"message"`
	MigrationsRun   int      `json:"migrations_run"`
	MigrationsTotal int      `json:"migrations_total"`
	AppliedVersions []string `json:"applied_versions"`
	Errors          []string `json:"errors,omitempty"`
}

// CompatibilityResult contains the result of a compatibility check.
type CompatibilityResult struct {
	BackupVersion     string   `json:"backup_version"`
	CurrentVersion    string   `json:"current_version"`
	Compatible        bool     `json:"compatible"`
	RequiresMigration bool     `json:"requires_migration"`
	MigrationsNeeded  int      `json:"migrations_needed"`
	MigrationVersions []string `json:"migration_versions,omitempty"`
	Message           string   `json:"message"`
}

func (mm *MigrationManager) loadHistory() (*MigrationHistory, error) {
	data, err := os.ReadFile(mm.historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &MigrationHistory{
				CurrentVersion: "",
				Migrations:     make([]MigrationRecord, 0),
			}, nil
		}
		return nil, err
	}

	var history MigrationHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return &history, nil
}

func (mm *MigrationManager) saveHistory(history *MigrationHistory) error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(mm.historyPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mm.historyPath, data, 0644)
}

func (mm *MigrationManager) registerBuiltinMigrations() {
	// Migration from 0.4.0 to 0.5.0
	mm.RegisterMigration(Migration{
		Version:     "0.5.0",
		Description: "Add grayscale configuration support",
		Up: func(ctx context.Context, m *Manager) error {
			// This migration adds support for grayscale config
			// No data changes needed, just version bump
			return nil
		},
		Down: func(ctx context.Context, m *Manager) error {
			// Rollback is a no-op for this migration
			return nil
		},
	})

	// Migration from 0.5.0 to 0.6.0
	mm.RegisterMigration(Migration{
		Version:     "0.6.0",
		Description: "Add message channel support",
		Up: func(ctx context.Context, m *Manager) error {
			// Future migration for channel support
			return nil
		},
		Down: func(ctx context.Context, m *Manager) error {
			return nil
		},
	})

	// Migration from 0.6.0 to 0.7.0
	mm.RegisterMigration(Migration{
		Version:     "0.7.0",
		Description: "Add security enhancements (OIDC, MFA)",
		Up: func(ctx context.Context, m *Manager) error {
			// Future migration for security features
			return nil
		},
		Down: func(ctx context.Context, m *Manager) error {
			return nil
		},
	})

	// Migration from 0.7.0 to 0.8.0
	mm.RegisterMigration(Migration{
		Version:     "0.8.0",
		Description: "Add RAG and knowledge base support",
		Up: func(ctx context.Context, m *Manager) error {
			// Future migration for RAG features
			return nil
		},
		Down: func(ctx context.Context, m *Manager) error {
			return nil
		},
	})
}

// RestoreWithMigration restores a backup and applies necessary migrations.
func (mm *MigrationManager) RestoreWithMigration(ctx context.Context, backupID string, opts RestoreOptions, currentVersion string) (*RestoreWithMigrationResult, error) {
	// Get backup info
	info, err := mm.manager.Get(backupID)
	if err != nil {
		return nil, err
	}

	// Check compatibility
	compat, err := mm.CheckCompatibility(info.Version, currentVersion)
	if err != nil {
		return nil, err
	}

	result := &RestoreWithMigrationResult{
		Compatibility: compat,
	}

	if !compat.Compatible {
		result.Success = false
		result.Message = compat.Message
		return result, fmt.Errorf("backup not compatible: %s", compat.Message)
	}

	// Perform restore
	restoreResult, err := mm.manager.Restore(ctx, backupID, opts)
	if err != nil {
		result.Success = false
		result.Message = fmt.Sprintf("restore failed: %v", err)
		result.RestoreResult = restoreResult
		return result, err
	}
	result.RestoreResult = restoreResult

	// Apply migrations if needed
	if compat.RequiresMigration {
		migrationResult, err := mm.Migrate(ctx, currentVersion)
		if err != nil {
			result.Success = false
			result.Message = fmt.Sprintf("migration failed after restore: %v", err)
			result.MigrationResult = migrationResult
			return result, err
		}
		result.MigrationResult = migrationResult
	}

	result.Success = true
	if compat.RequiresMigration {
		result.Message = fmt.Sprintf("Restored %d files and applied %d migrations",
			restoreResult.FilesRestored, result.MigrationResult.MigrationsRun)
	} else {
		result.Message = fmt.Sprintf("Restored %d files, no migrations needed", restoreResult.FilesRestored)
	}

	return result, nil
}

// RestoreWithMigrationResult contains the result of a restore with migration.
type RestoreWithMigrationResult struct {
	Success         bool                 `json:"success"`
	Message         string               `json:"message"`
	Compatibility   *CompatibilityResult `json:"compatibility"`
	RestoreResult   *RestoreResult       `json:"restore_result,omitempty"`
	MigrationResult *MigrationResult     `json:"migration_result,omitempty"`
}
