package database

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	_ "modernc.org/sqlite"
)

func setupWALTestDB(t *testing.T) (*sql.DB, string, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "wal-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
		os.Remove(tmpFile.Name() + "-wal")
		os.Remove(tmpFile.Name() + "-shm")
	}

	return db, tmpFile.Name(), cleanup
}

func TestWALManager_Configure(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	err := manager.Configure(ctx)
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Verify WAL mode is enabled
	info, err := manager.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	if info.JournalMode != "wal" {
		t.Errorf("JournalMode = %s, want wal", info.JournalMode)
	}

	if info.SynchronousMode != "NORMAL" {
		t.Errorf("SynchronousMode = %s, want NORMAL", info.SynchronousMode)
	}

	if info.BusyTimeout != config.BusyTimeout {
		t.Errorf("BusyTimeout = %d, want %d", info.BusyTimeout, config.BusyTimeout)
	}
}

func TestWALManager_Configure_Disabled(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	config.Enabled = false
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	err := manager.Configure(ctx)
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// WAL mode should not be enabled
	info, err := manager.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	// Default is delete mode
	if info.JournalMode == "wal" {
		t.Error("JournalMode should not be wal when disabled")
	}
}

func TestWALManager_Checkpoint(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	// Configure WAL mode first
	if err := manager.Configure(ctx); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Create a table and insert some data to generate WAL entries
	_, err := db.Exec(`
		CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT);
		INSERT INTO test (value) VALUES ('test1'), ('test2'), ('test3');
	`)
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Test different checkpoint modes
	modes := []CheckpointMode{
		CheckpointPassive,
		CheckpointFull,
		CheckpointRestart,
		CheckpointTruncate,
	}

	for _, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			err := manager.Checkpoint(ctx, mode)
			if err != nil {
				t.Errorf("Checkpoint(%s) error = %v", mode, err)
			}
		})
	}

	// Verify stats
	stats := manager.GetStats()
	if stats.TotalCheckpoints != 4 {
		t.Errorf("TotalCheckpoints = %d, want 4", stats.TotalCheckpoints)
	}
	if stats.SuccessCheckpoints != 4 {
		t.Errorf("SuccessCheckpoints = %d, want 4", stats.SuccessCheckpoints)
	}
	if stats.FailedCheckpoints != 0 {
		t.Errorf("FailedCheckpoints = %d, want 0", stats.FailedCheckpoints)
	}
}

func TestWALManager_AutoCheckpoint(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	config.CheckpointInterval = 100 * time.Millisecond // Short interval for testing
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	// Configure WAL mode
	if err := manager.Configure(ctx); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Create test data
	_, err := db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Start auto checkpoint
	manager.StartAutoCheckpoint()

	// Insert some data
	for i := 0; i < 10; i++ {
		_, err := db.Exec("INSERT INTO test (value) VALUES (?)", "test")
		if err != nil {
			t.Fatalf("Failed to insert: %v", err)
		}
	}

	// Wait for at least one checkpoint
	time.Sleep(250 * time.Millisecond)

	// Stop auto checkpoint
	manager.StopAutoCheckpoint()

	// Verify at least one checkpoint occurred
	stats := manager.GetStats()
	if stats.TotalCheckpoints < 1 {
		t.Errorf("TotalCheckpoints = %d, want >= 1", stats.TotalCheckpoints)
	}
}

func TestWALManager_GetInfo(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	// Configure WAL mode
	if err := manager.Configure(ctx); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	info, err := manager.GetInfo(ctx)
	if err != nil {
		t.Fatalf("GetInfo() error = %v", err)
	}

	if info.JournalMode != "wal" {
		t.Errorf("JournalMode = %s, want wal", info.JournalMode)
	}

	if info.PageSize <= 0 {
		t.Errorf("PageSize = %d, want > 0", info.PageSize)
	}

	if info.CacheSize == 0 {
		t.Errorf("CacheSize = %d, want != 0", info.CacheSize)
	}
}

func TestWALManager_Optimize(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	// Configure WAL mode
	if err := manager.Configure(ctx); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Create and populate a table
	_, err := db.Exec(`
		CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT);
		INSERT INTO test (value) SELECT 'test' FROM (SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5);
		DELETE FROM test WHERE id > 2;
	`)
	if err != nil {
		t.Fatalf("Failed to create test data: %v", err)
	}

	// Run optimization
	err = manager.Optimize(ctx)
	if err != nil {
		t.Fatalf("Optimize() error = %v", err)
	}
}

func TestWALManager_IntegrityCheck(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	manager := NewWALManager(db, config, logger)

	ctx := context.Background()

	// Configure WAL mode
	if err := manager.Configure(ctx); err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Create a table
	_, err := db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Run integrity check
	results, err := manager.IntegrityCheck(ctx)
	if err != nil {
		t.Fatalf("IntegrityCheck() error = %v", err)
	}

	if len(results) != 1 || results[0] != "ok" {
		t.Errorf("IntegrityCheck() = %v, want [ok]", results)
	}
}

func TestWALManager_StartStopAutoCheckpoint(t *testing.T) {
	db, _, cleanup := setupWALTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultWALConfig()
	config.CheckpointInterval = time.Hour // Long interval to prevent actual checkpoints
	manager := NewWALManager(db, config, logger)

	// Start should work
	manager.StartAutoCheckpoint()

	// Starting again should be a no-op
	manager.StartAutoCheckpoint()

	// Stop should work
	manager.StopAutoCheckpoint()

	// Stopping again should be a no-op
	manager.StopAutoCheckpoint()

	// Should be able to start again after stopping
	manager.StartAutoCheckpoint()
	manager.StopAutoCheckpoint()
}
