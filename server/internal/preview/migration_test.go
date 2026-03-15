package preview

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestMigrationService(t *testing.T) (*MigrationService, *sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create necessary tables for testing
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT,
			title TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create conversations table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS system_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("failed to create system_config table: %v", err)
	}

	migrationService := NewMigrationService(db)

	cleanup := func() {
		db.Close()
	}

	return migrationService, db, cleanup
}

func TestMigrationService_MigratePreviewData_AssignsConversations(t *testing.T) {
	migrationService, db, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	// Create some preview conversations (no user_id)
	_, err := db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv1', NULL, 'Test 1')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv2', '', 'Test 2')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv3', 'preview-user', 'Test 3')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	// Migrate
	adminUserID := "admin-user-id-123"
	result, err := migrationService.MigratePreviewData(ctx, adminUserID)
	if err != nil {
		t.Fatalf("MigratePreviewData() error = %v", err)
	}
	if got := result.MigratedCounts["conversations"]; got != 3 {
		t.Fatalf("migrated conversations = %d, want 3", got)
	}

	// Verify conversations are assigned to admin
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE user_id = ?`, adminUserID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count conversations: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 conversations assigned to admin, got %d", count)
	}
}

func TestMigrationService_MigratePreviewData_MarksMigrationComplete(t *testing.T) {
	migrationService, db, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	// Migrate
	_, err := migrationService.MigratePreviewData(ctx, "admin-user-id")
	if err != nil {
		t.Fatalf("MigratePreviewData() error = %v", err)
	}

	// Verify migration status
	var value string
	err = db.QueryRow(`SELECT value FROM system_config WHERE key = 'preview_data_migrated'`).Scan(&value)
	if err != nil {
		t.Fatalf("failed to get migration status: %v", err)
	}
	if value != "true" {
		t.Errorf("migration status = %v, want true", value)
	}
}

func TestMigrationService_GetMigrationStatus_NotMigrated(t *testing.T) {
	migrationService, _, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	status, err := migrationService.GetMigrationStatus(ctx)
	if err != nil {
		t.Fatalf("GetMigrationStatus() error = %v", err)
	}
	if status {
		t.Error("GetMigrationStatus() = true, want false when not migrated")
	}
}

func TestMigrationService_GetMigrationStatus_Migrated(t *testing.T) {
	migrationService, db, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	// Set migration status
	_, err := db.Exec(`INSERT INTO system_config (key, value) VALUES ('preview_data_migrated', 'true')`)
	if err != nil {
		t.Fatalf("failed to set migration status: %v", err)
	}

	status, err := migrationService.GetMigrationStatus(ctx)
	if err != nil {
		t.Fatalf("GetMigrationStatus() error = %v", err)
	}
	if !status {
		t.Error("GetMigrationStatus() = false, want true when migrated")
	}
}

func TestMigrationService_MigratePreviewData_Idempotent(t *testing.T) {
	migrationService, db, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a conversation
	_, err := db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv1', NULL, 'Test')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	adminUserID := "admin-user-id"

	// Migrate twice
	_, err = migrationService.MigratePreviewData(ctx, adminUserID)
	if err != nil {
		t.Fatalf("first MigratePreviewData() error = %v", err)
	}

	_, err = migrationService.MigratePreviewData(ctx, adminUserID)
	if err != nil {
		t.Fatalf("second MigratePreviewData() error = %v", err)
	}

	// Verify only one conversation exists and is assigned correctly
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE user_id = ?`, adminUserID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count conversations: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 conversation, got %d", count)
	}
}

func TestMigrationService_MigratePreviewData_PreservesExistingUserConversations(t *testing.T) {
	migrationService, db, cleanup := setupTestMigrationService(t)
	defer cleanup()

	ctx := context.Background()

	// Create conversations - one without user, one with existing user
	_, err := db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv1', NULL, 'Preview Conv')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}
	_, err = db.Exec(`INSERT INTO conversations (id, user_id, title) VALUES ('conv2', 'other-user-id', 'Other User Conv')`)
	if err != nil {
		t.Fatalf("failed to insert conversation: %v", err)
	}

	// Migrate
	adminUserID := "admin-user-id"
	_, err = migrationService.MigratePreviewData(ctx, adminUserID)
	if err != nil {
		t.Fatalf("MigratePreviewData() error = %v", err)
	}

	// Verify preview conversation is assigned to admin
	var adminCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE user_id = ?`, adminUserID).Scan(&adminCount)
	if err != nil {
		t.Fatalf("failed to count admin conversations: %v", err)
	}
	if adminCount != 1 {
		t.Errorf("expected 1 conversation for admin, got %d", adminCount)
	}

	// Verify other user's conversation is preserved
	var otherCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM conversations WHERE user_id = 'other-user-id'`).Scan(&otherCount)
	if err != nil {
		t.Fatalf("failed to count other user conversations: %v", err)
	}
	if otherCount != 1 {
		t.Errorf("expected 1 conversation for other user, got %d", otherCount)
	}
}

func TestMigrationService_MigratePreviewData_RequiresAdminUserID(t *testing.T) {
	migrationService, _, cleanup := setupTestMigrationService(t)
	defer cleanup()

	_, err := migrationService.MigratePreviewData(context.Background(), " ")
	if err == nil {
		t.Fatal("expected error for empty admin user id")
	}
}
