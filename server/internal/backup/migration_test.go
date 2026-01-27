package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationManager_NewMigrationManager(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}

	m, err := NewManager(cfg, filepath.Join(tempDir, "data"), filepath.Join(tempDir, "config"))
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mm := NewMigrationManager(m, filepath.Join(tempDir, "data"))
	if mm == nil {
		t.Fatal("Expected migration manager, got nil")
	}

	// Should have built-in migrations registered
	if len(mm.migrations) == 0 {
		t.Error("Expected built-in migrations to be registered")
	}
}

func TestMigrationManager_RegisterMigration(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}

	m, _ := NewManager(cfg, filepath.Join(tempDir, "data"), filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, filepath.Join(tempDir, "data"))

	initialCount := len(mm.migrations)

	mm.RegisterMigration(Migration{
		Version:     "0.9.0",
		Description: "Test migration",
		Up:          func(ctx context.Context, m *Manager) error { return nil },
		Down:        func(ctx context.Context, m *Manager) error { return nil },
	})

	if len(mm.migrations) != initialCount+1 {
		t.Errorf("Expected %d migrations, got %d", initialCount+1, len(mm.migrations))
	}
}

func TestMigrationManager_GetPendingMigrations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}

	m, _ := NewManager(cfg, filepath.Join(tempDir, "data"), filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, filepath.Join(tempDir, "data"))

	// With no history, all migrations up to target should be pending
	pending, err := mm.GetPendingMigrations("0.6.0")
	if err != nil {
		t.Fatalf("GetPendingMigrations failed: %v", err)
	}

	// Should include 0.5.0 and 0.6.0
	if len(pending) < 2 {
		t.Errorf("Expected at least 2 pending migrations, got %d", len(pending))
	}
}

func TestMigrationManager_Migrate(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx := context.Background()
	result, err := mm.Migrate(ctx, "0.6.0")
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %v", result.Errors)
	}

	if result.MigrationsRun < 2 {
		t.Errorf("Expected at least 2 migrations run, got %d", result.MigrationsRun)
	}

	// Verify current version
	version, err := mm.GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion failed: %v", err)
	}

	if version != "0.6.0" {
		t.Errorf("Expected version 0.6.0, got %s", version)
	}
}

func TestMigrationManager_Migrate_NoMigrations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx := context.Background()

	// First migrate to 0.6.0
	mm.Migrate(ctx, "0.6.0")

	// Try to migrate to same version
	result, err := mm.Migrate(ctx, "0.6.0")
	if err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	if result.MigrationsRun != 0 {
		t.Errorf("Expected 0 migrations run, got %d", result.MigrationsRun)
	}

	if result.Message != "No migrations to apply" {
		t.Errorf("Expected 'No migrations to apply', got '%s'", result.Message)
	}
}

func TestMigrationManager_Rollback(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx := context.Background()

	// First migrate to 0.7.0
	mm.Migrate(ctx, "0.7.0")

	// Rollback to 0.5.0
	result, err := mm.Rollback(ctx, "0.5.0")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure: %v", result.Errors)
	}

	// Verify current version
	version, err := mm.GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion failed: %v", err)
	}

	if version != "0.5.0" {
		t.Errorf("Expected version 0.5.0, got %s", version)
	}
}

func TestMigrationManager_Rollback_NoHistory(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx := context.Background()
	result, err := mm.Rollback(ctx, "0.4.0")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success")
	}

	if result.Message != "No migrations to rollback" {
		t.Errorf("Expected 'No migrations to rollback', got '%s'", result.Message)
	}
}

func TestMigrationManager_CheckCompatibility(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	tests := []struct {
		name              string
		backupVersion     string
		currentVersion    string
		expectCompatible  bool
		expectMigration   bool
	}{
		{
			name:             "same_version",
			backupVersion:    "0.5.0",
			currentVersion:   "0.5.0",
			expectCompatible: true,
			expectMigration:  false,
		},
		{
			name:             "older_backup",
			backupVersion:    "0.5.0",
			currentVersion:   "0.7.0",
			expectCompatible: true,
			expectMigration:  true,
		},
		{
			name:             "newer_backup",
			backupVersion:    "0.8.0",
			currentVersion:   "0.5.0",
			expectCompatible: false,
			expectMigration:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mm.CheckCompatibility(tt.backupVersion, tt.currentVersion)
			if err != nil {
				t.Fatalf("CheckCompatibility failed: %v", err)
			}

			if result.Compatible != tt.expectCompatible {
				t.Errorf("Expected compatible=%v, got %v", tt.expectCompatible, result.Compatible)
			}

			if result.RequiresMigration != tt.expectMigration {
				t.Errorf("Expected requiresMigration=%v, got %v", tt.expectMigration, result.RequiresMigration)
			}
		})
	}
}

func TestMigrationManager_GetHistory(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx := context.Background()
	mm.Migrate(ctx, "0.6.0")

	history, err := mm.GetHistory()
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if history.CurrentVersion != "0.6.0" {
		t.Errorf("Expected current version 0.6.0, got %s", history.CurrentVersion)
	}

	if len(history.Migrations) < 2 {
		t.Errorf("Expected at least 2 migration records, got %d", len(history.Migrations))
	}
}

func TestMigrationManager_MigrationCancellation(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := mm.Migrate(ctx, "0.8.0")
	if err == nil {
		t.Error("Expected error due to cancellation")
	}

	if result.Success {
		t.Error("Expected failure due to cancellation")
	}
}

func TestMigrationManager_InvalidVersion(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm := NewMigrationManager(m, dataDir)

	_, err := mm.GetPendingMigrations("invalid")
	if err == nil {
		t.Error("Expected error for invalid version")
	}
}

func TestMigrationManager_HistoryPersistence(t *testing.T) {
	tempDir := t.TempDir()
	cfg := Config{Path: filepath.Join(tempDir, "backups")}
	dataDir := filepath.Join(tempDir, "data")
	os.MkdirAll(dataDir, 0755)

	m, _ := NewManager(cfg, dataDir, filepath.Join(tempDir, "config"))
	mm1 := NewMigrationManager(m, dataDir)

	ctx := context.Background()
	mm1.Migrate(ctx, "0.6.0")

	// Create new migration manager (simulating restart)
	mm2 := NewMigrationManager(m, dataDir)

	version, err := mm2.GetCurrentVersion()
	if err != nil {
		t.Fatalf("GetCurrentVersion failed: %v", err)
	}

	if version != "0.6.0" {
		t.Errorf("Expected version 0.6.0 after restart, got %s", version)
	}
}
