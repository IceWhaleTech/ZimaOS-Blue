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
	z "github.com/IceWhaleTech/zorm"
)

// MigrationService handles migration of preview data to admin account.
type MigrationService struct {
	db     *sql.DB
	readDB *sql.DB
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
	return NewMigrationServiceWithReadDB(db, db)
}

func NewMigrationServiceWithReadDB(writeDB, readDB *sql.DB) *MigrationService {
	if readDB == nil {
		readDB = writeDB
	}
	return &MigrationService{db: writeDB, readDB: readDB}
}

func (s *MigrationService) reader() *sql.DB {
	if s != nil && s.readDB != nil {
		return s.readDB
	}
	if s == nil {
		return nil
	}
	return s.db
}

func (s *MigrationService) readTable(ctx context.Context, name string) *z.ZormTable {
	return z.TableContext(ctx, s.reader(), name)
}

type systemConfigRow struct {
	Key       string `json:"key" zorm:"key"`
	Value     string `json:"value" zorm:"value"`
	UpdatedAt string `json:"updated_at" zorm:"updated_at"`
}

type sqliteMasterRow struct {
	Name string `json:"name" zorm:"name"`
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
	var rows []systemConfigRow
	_, err := s.readTable(ctx, "system_config").Select(
		&rows,
		z.Fields("key", "value", "updated_at"),
		z.Where(z.Eq("key", "preview_data_migrated")),
		z.Limit(1),
	)
	if len(rows) == 0 {
		return false, nil
	}
	if err != nil {
		return false, nil // Table might not exist
	}
	return rows[0].Value == "true", nil
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
	var rows []sqliteMasterRow
	_, err := z.TableContext(ctx, tx, "sqlite_master").Select(
		&rows,
		z.Fields("name"),
		z.Where(z.Eq("type", "table"), z.Eq("name", tableName)),
		z.Limit(1),
	)
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", tableName, err)
	}
	return len(rows) > 0, nil
}

func migrateTableOwner(ctx context.Context, tx *sql.Tx, tableName, adminUserID string) (int64, error) {
	updated, err := z.TableContext(ctx, tx, tableName).Update(
		z.V{"user_id": adminUserID},
		z.Fields("user_id"),
		z.Where(
			z.Expr("user_id IS NULL OR TRIM(user_id) = '' OR user_id = ?", previewUserID),
		),
	)
	if err != nil {
		return 0, fmt.Errorf("migrate %s owner: %w", tableName, err)
	}
	return int64(updated), nil
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
		if _, err := z.TableContext(ctx, tx, "system_config").Insert(
			z.V{
				"key":        kv[0],
				"value":      kv[1],
				"updated_at": timeutil.NowTime().UTC().Format(time.RFC3339Nano),
			},
			z.OnConflictDoUpdateSet([]string{"key"}, []string{"value", "updated_at"}),
		); err != nil {
			return fmt.Errorf("write migration marker %s: %w", kv[0], err)
		}
	}
	return nil
}
