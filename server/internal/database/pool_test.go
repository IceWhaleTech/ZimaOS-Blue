package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	_ "github.com/mattn/go-sqlite3"
)

func setupPoolTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "pool-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestPoolManager_Configure(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.MaxOpenConns = 5
	config.MaxIdleConns = 2

	manager := NewPoolManager(db, config, logger)
	manager.Configure()

	stats := manager.GetStats()

	if stats.MaxOpenConnections != 5 {
		t.Errorf("MaxOpenConnections = %d, want 5", stats.MaxOpenConnections)
	}
}

func TestPoolManager_HealthCheck(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.HealthCheckInterval = 100 * time.Millisecond

	manager := NewPoolManager(db, config, logger)
	manager.Configure()

	// Start health check
	manager.StartHealthCheck()

	// Wait for at least one health check
	time.Sleep(250 * time.Millisecond)

	// Should be healthy
	if !manager.IsHealthy() {
		t.Error("IsHealthy() = false, want true")
	}

	stats := manager.GetStats()
	if stats.HealthChecksPassed < 1 {
		t.Errorf("HealthChecksPassed = %d, want >= 1", stats.HealthChecksPassed)
	}

	// Stop health check
	manager.StopHealthCheck()
}

func TestPoolManager_GetStats(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.MaxOpenConns = 10
	config.MaxIdleConns = 5

	manager := NewPoolManager(db, config, logger)
	manager.Configure()

	stats := manager.GetStats()

	if stats.MaxOpenConnections != 10 {
		t.Errorf("MaxOpenConnections = %d, want 10", stats.MaxOpenConnections)
	}

	// Initially should have no connections in use
	if stats.InUse != 0 {
		t.Errorf("InUse = %d, want 0", stats.InUse)
	}
}

func TestPoolManager_GetUtilization(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.MaxOpenConns = 10

	manager := NewPoolManager(db, config, logger)
	manager.Configure()

	// Initially should be 0%
	util := manager.GetUtilization()
	if util != 0 {
		t.Errorf("GetUtilization() = %f, want 0", util)
	}
}

func TestPoolManager_WithConnection(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	manager := NewPoolManager(db, DefaultPoolConfig(), logger)
	manager.Configure()

	ctx := context.Background()

	err := manager.WithConnection(ctx, func(conn *sql.Conn) error {
		var result int
		return conn.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	})

	if err != nil {
		t.Errorf("WithConnection() error = %v", err)
	}
}

func TestPoolManager_WithTransaction(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	// Create test table
	_, err := db.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, value TEXT)`)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	manager := NewPoolManager(db, DefaultPoolConfig(), logger)
	manager.Configure()

	ctx := context.Background()

	// Test successful transaction
	err = manager.WithTransaction(ctx, nil, func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO test (value) VALUES (?)", "test")
		return err
	})

	if err != nil {
		t.Errorf("WithTransaction() error = %v", err)
	}

	// Verify data was inserted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}

	// Test failed transaction (should rollback)
	err = manager.WithTransaction(ctx, nil, func(tx *sql.Tx) error {
		tx.Exec("INSERT INTO test (value) VALUES (?)", "test2")
		return errors.New("intentional error")
	})

	if err == nil {
		t.Error("WithTransaction() should have returned error")
	}

	// Verify rollback (count should still be 1)
	db.QueryRow("SELECT COUNT(*) FROM test").Scan(&count)
	if count != 1 {
		t.Errorf("count after rollback = %d, want 1", count)
	}
}

func TestPoolManager_Warmup(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.MaxOpenConns = 10
	config.MaxIdleConns = 5

	manager := NewPoolManager(db, config, logger)
	manager.Configure()

	ctx := context.Background()

	err := manager.Warmup(ctx, 3)
	if err != nil {
		t.Errorf("Warmup() error = %v", err)
	}

	// After warmup, should have idle connections
	stats := manager.GetStats()
	if stats.Idle < 1 {
		t.Errorf("Idle = %d, want >= 1", stats.Idle)
	}
}

func TestPoolManager_StartStopHealthCheck(t *testing.T) {
	db, cleanup := setupPoolTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultPoolConfig()
	config.HealthCheckInterval = time.Hour // Long interval

	manager := NewPoolManager(db, config, logger)

	// Start should work
	manager.StartHealthCheck()

	// Starting again should be a no-op
	manager.StartHealthCheck()

	// Stop should work
	manager.StopHealthCheck()

	// Stopping again should be a no-op
	manager.StopHealthCheck()

	// Should be able to start again
	manager.StartHealthCheck()
	manager.StopHealthCheck()
}

func TestRecommendedPoolSize(t *testing.T) {
	tests := []struct {
		name         string
		cpuCores     int
		ioMultiplier float64
		wantMin      int
		wantMax      int
	}{
		{
			name:         "single core",
			cpuCores:     1,
			ioMultiplier: 2.0,
			wantMin:      2,
			wantMax:      10,
		},
		{
			name:         "quad core",
			cpuCores:     4,
			ioMultiplier: 2.0,
			wantMin:      10,
			wantMax:      20,
		},
		{
			name:         "many cores",
			cpuCores:     32,
			ioMultiplier: 2.0,
			wantMin:      50,
			wantMax:      100,
		},
		{
			name:         "zero cores",
			cpuCores:     0,
			ioMultiplier: 2.0,
			wantMin:      2,
			wantMax:      10,
		},
		{
			name:         "negative multiplier",
			cpuCores:     4,
			ioMultiplier: -1.0,
			wantMin:      10,
			wantMax:      20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RecommendedPoolSize(tt.cpuCores, tt.ioMultiplier)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("RecommendedPoolSize(%d, %f) = %d, want between %d and %d",
					tt.cpuCores, tt.ioMultiplier, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
