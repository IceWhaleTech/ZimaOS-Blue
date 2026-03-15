package preview

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// MigrationService handles migration of preview data to admin account.
type MigrationService struct {
	db *sql.DB
}

const previewUserID = "preview-user"

var errAdminUserIDRequired = errors.New("admin user id is required")

type migrationTarget struct {
	Table string
}

var previewMigrationTargets = []migrationTarget{
	{Table: "conversations"},
	{Table: "agent_tasks"},
	{Table: "convert_tasks"},
	{Table: "convert_sources"},
	{Table: "media_tasks"},
	{Table: "push_subscriptions"},
	{Table: "user_provider_configs"},
	{Table: "user_skill_configs"},
	{Table: "session_tool_audit_logs"},
}

// MigrationResult contains per-table row counts updated during preview migration.
type MigrationResult struct {
	MigratedCounts map[string]int64 `json:"migrated_counts"`
}

// NewMigrationService creates a new MigrationService.
func NewMigrationService(db *sql.DB) *MigrationService {
	return &MigrationService{db: db}
}

// MigratePreviewData migrates all preview mode data to the new admin user.
func (s *MigrationService) MigratePreviewData(ctx context.Context, adminUserID string) (*MigrationResult, error) {
	adminUserID = strings.TrimSpace(adminUserID)
	if adminUserID == "" {
		return nil, errAdminUserIDRequired
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := ensureSystemConfigTable(ctx, tx); err != nil {
		return nil, err
	}

	result := &MigrationResult{
		MigratedCounts: make(map[string]int64, len(previewMigrationTargets)),
	}

	for _, target := range previewMigrationTargets {
		exists, err := tableExists(ctx, tx, target.Table)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}

		updated, err := migrateTableOwner(ctx, tx, target.Table, adminUserID)
		if err != nil {
			return nil, err
		}
		result.MigratedCounts[target.Table] = updated
	}

	if err := writeMigrationStatus(ctx, tx, adminUserID, result.MigratedCounts); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// GetMigrationStatus checks if preview data has been migrated.
func (s *MigrationService) GetMigrationStatus(ctx context.Context) (bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `
		SELECT value FROM system_config WHERE key = 'preview_data_migrated'
	`).Scan(&value)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, nil // Table might not exist
	}

	return value == "true", nil
}

func ensureSystemConfigTable(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure system_config table: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, tx *sql.Tx, tableName string) (bool, error) {
	var found string
	err := tx.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ? LIMIT 1`,
		tableName,
	).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", tableName, err)
	}
	return true, nil
}

func migrateTableOwner(ctx context.Context, tx *sql.Tx, tableName, adminUserID string) (int64, error) {
	query := fmt.Sprintf(`
		UPDATE %s
		SET user_id = ?
		WHERE user_id IS NULL
		   OR TRIM(user_id) = ''
		   OR user_id = ?
	`, tableName)
	res, err := tx.ExecContext(ctx, query, adminUserID, previewUserID)
	if err != nil {
		return 0, fmt.Errorf("migrate %s owner: %w", tableName, err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected for %s: %w", tableName, err)
	}
	return rows, nil
}

func writeMigrationStatus(ctx context.Context, tx *sql.Tx, adminUserID string, counts map[string]int64) error {
	countsJSON, err := json.Marshal(counts)
	if err != nil {
		return fmt.Errorf("marshal migration counts: %w", err)
	}

	pairs := [][2]string{
		{"preview_data_migrated", "true"},
		{"preview_data_migrated_admin_user_id", adminUserID},
		{"preview_data_migrated_at", timeutil.NowTime().UTC().Format(time.RFC3339Nano)},
		{"preview_data_migrated_counts", string(countsJSON)},
	}

	for _, kv := range pairs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO system_config (key, value, updated_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(key) DO UPDATE SET
				value = excluded.value,
				updated_at = CURRENT_TIMESTAMP
		`, kv[0], kv[1]); err != nil {
			return fmt.Errorf("write migration marker %s: %w", kv[0], err)
		}
	}
	return nil
}
