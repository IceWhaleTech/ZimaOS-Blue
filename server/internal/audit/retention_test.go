package audit

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func setupTestRetentionManager(t *testing.T) (*RetentionManager, *sql.DB, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create audit_logs table
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS audit_logs (
		id TEXT PRIMARY KEY,
		timestamp DATETIME NOT NULL,
		user_id TEXT,
		username TEXT,
		action TEXT NOT NULL,
		resource_type TEXT,
		resource_id TEXT,
		ip_address TEXT,
		user_agent TEXT,
		request_id TEXT,
		status TEXT NOT NULL,
		details TEXT,
		old_value TEXT,
		new_value TEXT
	)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	config := &RetentionConfig{
		RetentionDays:   7,
		CleanupInterval: time.Hour,
		BatchSize:       100,
	}

	manager := NewRetentionManager(db, config)

	cleanup := func() {
		manager.Stop()
		db.Close()
	}

	return manager, db, cleanup
}

func TestNewRetentionManager(t *testing.T) {
	manager, _, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	if manager == nil {
		t.Fatal("NewRetentionManager() returned nil")
	}
}

func TestNewRetentionManager_DefaultConfig(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	manager := NewRetentionManager(db, nil)
	if manager == nil {
		t.Fatal("NewRetentionManager(nil config) returned nil")
	}
	manager.Stop()
}

func TestDefaultRetentionConfig(t *testing.T) {
	config := DefaultRetentionConfig()

	if config == nil {
		t.Fatal("DefaultRetentionConfig() returned nil")
	}

	if config.RetentionDays != 90 {
		t.Errorf("DefaultRetentionConfig() RetentionDays = %d, want 90", config.RetentionDays)
	}

	if config.CleanupInterval != 24*time.Hour {
		t.Errorf("DefaultRetentionConfig() CleanupInterval = %v, want 24h", config.CleanupInterval)
	}

	if config.BatchSize != 1000 {
		t.Errorf("DefaultRetentionConfig() BatchSize = %d, want 1000", config.BatchSize)
	}
}

func TestRetentionManager_CleanupNow(t *testing.T) {
	manager, db, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	// Insert old entries
	oldTime := time.Now().AddDate(0, 0, -30) // 30 days ago
	for i := 0; i < 5; i++ {
		_, err := db.Exec(`INSERT INTO audit_logs (id, timestamp, action, status) VALUES (?, ?, ?, ?)`,
			"old-"+string(rune('0'+i)), oldTime, "login", "success")
		if err != nil {
			t.Fatalf("failed to insert old entry: %v", err)
		}
	}

	// Insert recent entries
	recentTime := time.Now()
	for i := 0; i < 3; i++ {
		_, err := db.Exec(`INSERT INTO audit_logs (id, timestamp, action, status) VALUES (?, ?, ?, ?)`,
			"recent-"+string(rune('0'+i)), recentTime, "login", "success")
		if err != nil {
			t.Fatalf("failed to insert recent entry: %v", err)
		}
	}

	// Run cleanup
	err := manager.CleanupNow()
	if err != nil {
		t.Fatalf("CleanupNow() error = %v", err)
	}

	// Check remaining entries
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&count)
	if err != nil {
		t.Fatalf("failed to count entries: %v", err)
	}

	// Old entries should be deleted (retention is 7 days)
	if count != 3 {
		t.Errorf("CleanupNow() remaining entries = %d, want 3", count)
	}
}

func TestRetentionManager_GetRetentionStats(t *testing.T) {
	manager, db, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	ctx := context.Background()

	// Insert entries
	oldTime := time.Now().AddDate(0, 0, -30)
	recentTime := time.Now()

	_, _ = db.Exec(`INSERT INTO audit_logs (id, timestamp, action, status) VALUES (?, ?, ?, ?)`,
		"old-1", oldTime, "login", "success")
	_, _ = db.Exec(`INSERT INTO audit_logs (id, timestamp, action, status) VALUES (?, ?, ?, ?)`,
		"recent-1", recentTime, "login", "success")

	stats, err := manager.GetRetentionStats(ctx)
	if err != nil {
		t.Fatalf("GetRetentionStats() error = %v", err)
	}

	if stats.TotalCount != 2 {
		t.Errorf("GetRetentionStats() TotalCount = %d, want 2", stats.TotalCount)
	}

	if stats.ExpiredCount != 1 {
		t.Errorf("GetRetentionStats() ExpiredCount = %d, want 1", stats.ExpiredCount)
	}

	if stats.OldestEntry == nil {
		t.Error("GetRetentionStats() OldestEntry should not be nil")
	}

	if stats.NewestEntry == nil {
		t.Error("GetRetentionStats() NewestEntry should not be nil")
	}

	if stats.RetentionDays != 7 {
		t.Errorf("GetRetentionStats() RetentionDays = %d, want 7", stats.RetentionDays)
	}
}

func TestRetentionManager_GetRetentionStats_Empty(t *testing.T) {
	manager, _, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	ctx := context.Background()

	stats, err := manager.GetRetentionStats(ctx)
	if err != nil {
		t.Fatalf("GetRetentionStats(empty) error = %v", err)
	}

	if stats.TotalCount != 0 {
		t.Errorf("GetRetentionStats(empty) TotalCount = %d, want 0", stats.TotalCount)
	}

	if stats.ExpiredCount != 0 {
		t.Errorf("GetRetentionStats(empty) ExpiredCount = %d, want 0", stats.ExpiredCount)
	}
}

func TestRetentionManager_Start(t *testing.T) {
	manager, _, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	// Start should not panic
	manager.Start()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)
}

func TestRetentionManager_Stop(t *testing.T) {
	manager, _, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	manager.Start()
	time.Sleep(50 * time.Millisecond)

	// Stop should not panic
	manager.Stop()
}

func TestRetentionManager_DeleteBatch(t *testing.T) {
	manager, db, cleanup := setupTestRetentionManager(t)
	defer cleanup()

	ctx := context.Background()

	// Insert old entries
	oldTime := time.Now().AddDate(0, 0, -30)
	for i := 0; i < 10; i++ {
		_, _ = db.Exec(`INSERT INTO audit_logs (id, timestamp, action, status) VALUES (?, ?, ?, ?)`,
			"batch-"+string(rune('0'+i)), oldTime, "login", "success")
	}

	cutoff := time.Now().AddDate(0, 0, -7)
	deleted, err := manager.deleteBatch(ctx, cutoff)
	if err != nil {
		t.Fatalf("deleteBatch() error = %v", err)
	}

	if deleted != 10 {
		t.Errorf("deleteBatch() deleted = %d, want 10", deleted)
	}
}

func TestDefaultArchiveConfig(t *testing.T) {
	config := DefaultArchiveConfig()

	if config == nil {
		t.Fatal("DefaultArchiveConfig() returned nil")
	}

	if config.ArchivePath == "" {
		t.Error("DefaultArchiveConfig() ArchivePath should not be empty")
	}

	if config.ArchiveAfterDays != 30 {
		t.Errorf("DefaultArchiveConfig() ArchiveAfterDays = %d, want 30", config.ArchiveAfterDays)
	}

	if !config.CompressArchives {
		t.Error("DefaultArchiveConfig() CompressArchives should be true")
	}
}

func TestRetentionStats_JSON(t *testing.T) {
	now := time.Now()
	stats := &RetentionStats{
		TotalCount:    100,
		ExpiredCount:  10,
		OldestEntry:   &now,
		NewestEntry:   &now,
		RetentionDays: 90,
		CutoffDate:    now,
	}

	if stats.TotalCount != 100 {
		t.Errorf("RetentionStats TotalCount = %d, want 100", stats.TotalCount)
	}

	if stats.ExpiredCount != 10 {
		t.Errorf("RetentionStats ExpiredCount = %d, want 10", stats.ExpiredCount)
	}
}
