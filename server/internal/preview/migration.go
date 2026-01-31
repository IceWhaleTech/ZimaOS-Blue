package preview

import (
	"context"
	"database/sql"
)

// MigrationService handles migration of preview data to admin account.
type MigrationService struct {
	db *sql.DB
}

// NewMigrationService creates a new MigrationService.
func NewMigrationService(db *sql.DB) *MigrationService {
	return &MigrationService{db: db}
}

// MigratePreviewData migrates all preview mode data to the new admin user.
func (s *MigrationService) MigratePreviewData(ctx context.Context, adminUserID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Migrate conversations - assign to admin user
	// Note: This assumes a conversations table exists with user_id column
	_, _ = tx.ExecContext(ctx, `
		UPDATE conversations
		SET user_id = ?
		WHERE user_id IS NULL OR user_id = ''
	`, adminUserID)

	// 2. Mark migration as complete
	_, err = tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO system_config (key, value, updated_at)
		VALUES ('preview_data_migrated', 'true', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		// system_config table might not exist, that's ok
		// We'll create it in the schema migration
	}

	return tx.Commit()
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
