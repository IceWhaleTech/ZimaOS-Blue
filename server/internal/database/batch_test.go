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

func setupBatchTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	tmpFile, err := os.CreateTemp("", "batch-test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()

	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create test table
	_, err = db.Exec(`
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER NOT NULL,
			status TEXT DEFAULT 'active',
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		os.Remove(tmpFile.Name())
		t.Fatalf("Failed to create table: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(tmpFile.Name())
	}

	return db, cleanup
}

func TestBatchExecutor_BatchInsert(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultBatchConfig()
	config.BatchSize = 10 // Small batch size for testing
	executor := NewBatchExecutor(db, config, logger)

	ctx := context.Background()

	// Prepare test data
	columns := []string{"name", "value", "created_at"}
	values := make([][]interface{}, 25)
	for i := 0; i < 25; i++ {
		values[i] = []interface{}{
			"item_" + string(rune('A'+i%26)),
			i * 10,
			time.Now(),
		}
	}

	// Execute batch insert
	result, err := executor.BatchInsert(ctx, "items", columns, values)
	if err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	if result.TotalRows != 25 {
		t.Errorf("TotalRows = %d, want 25", result.TotalRows)
	}

	if result.AffectedRows != 25 {
		t.Errorf("AffectedRows = %d, want 25", result.AffectedRows)
	}

	if result.Batches != 3 { // 10 + 10 + 5
		t.Errorf("Batches = %d, want 3", result.Batches)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Errors = %v, want empty", result.Errors)
	}

	// Verify data in database
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count items: %v", err)
	}

	if count != 25 {
		t.Errorf("Database count = %d, want 25", count)
	}
}

func TestBatchExecutor_BatchInsert_Empty(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	executor := NewBatchExecutor(db, DefaultBatchConfig(), logger)

	ctx := context.Background()

	result, err := executor.BatchInsert(ctx, "items", []string{"name", "value", "created_at"}, nil)
	if err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	if result.TotalRows != 0 {
		t.Errorf("TotalRows = %d, want 0", result.TotalRows)
	}
}

func TestBatchExecutor_BatchUpdate(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultBatchConfig()
	config.BatchSize = 5
	executor := NewBatchExecutor(db, config, logger)

	ctx := context.Background()

	// Insert test data first
	for i := 0; i < 10; i++ {
		_, err := db.Exec(
			"INSERT INTO items (name, value, created_at) VALUES (?, ?, ?)",
			"item_"+string(rune('A'+i)),
			i*10,
			time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Prepare updates
	updates := make([]BatchUpdate, 10)
	for i := 0; i < 10; i++ {
		updates[i] = BatchUpdate{
			Set:   map[string]interface{}{"status": "updated", "value": i * 100},
			Where: map[string]interface{}{"id": i + 1},
		}
	}

	// Execute batch update
	result, err := executor.BatchUpdate(ctx, "items", updates)
	if err != nil {
		t.Fatalf("BatchUpdate() error = %v", err)
	}

	if result.TotalRows != 10 {
		t.Errorf("TotalRows = %d, want 10", result.TotalRows)
	}

	if result.AffectedRows != 10 {
		t.Errorf("AffectedRows = %d, want 10", result.AffectedRows)
	}

	// Verify updates
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items WHERE status = 'updated'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count updated items: %v", err)
	}

	if count != 10 {
		t.Errorf("Updated count = %d, want 10", count)
	}
}

func TestBatchExecutor_BatchDelete(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultBatchConfig()
	config.BatchSize = 5
	executor := NewBatchExecutor(db, config, logger)

	ctx := context.Background()

	// Insert test data first
	for i := 0; i < 15; i++ {
		_, err := db.Exec(
			"INSERT INTO items (name, value, created_at) VALUES (?, ?, ?)",
			"item_"+string(rune('A'+i)),
			i*10,
			time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Delete items with IDs 1-10
	ids := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		ids[i] = i + 1
	}

	result, err := executor.BatchDelete(ctx, "items", "id", ids)
	if err != nil {
		t.Fatalf("BatchDelete() error = %v", err)
	}

	if result.TotalRows != 10 {
		t.Errorf("TotalRows = %d, want 10", result.TotalRows)
	}

	if result.AffectedRows != 10 {
		t.Errorf("AffectedRows = %d, want 10", result.AffectedRows)
	}

	// Verify remaining items
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count items: %v", err)
	}

	if count != 5 {
		t.Errorf("Remaining count = %d, want 5", count)
	}
}

func TestBatchExecutor_BatchDeleteWithCondition(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	executor := NewBatchExecutor(db, DefaultBatchConfig(), logger)

	ctx := context.Background()

	// Insert test data
	for i := 0; i < 10; i++ {
		status := "active"
		if i%2 == 0 {
			status = "inactive"
		}
		_, err := db.Exec(
			"INSERT INTO items (name, value, status, created_at) VALUES (?, ?, ?, ?)",
			"item_"+string(rune('A'+i)),
			i*10,
			status,
			time.Now(),
		)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	// Delete inactive items
	conditions := []map[string]interface{}{
		{"status": "inactive"},
	}

	result, err := executor.BatchDeleteWithCondition(ctx, "items", conditions)
	if err != nil {
		t.Fatalf("BatchDeleteWithCondition() error = %v", err)
	}

	if result.AffectedRows != 5 {
		t.Errorf("AffectedRows = %d, want 5", result.AffectedRows)
	}

	// Verify remaining items
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM items WHERE status = 'active'").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count items: %v", err)
	}

	if count != 5 {
		t.Errorf("Active count = %d, want 5", count)
	}
}

func TestBatchExecutor_GetStats(t *testing.T) {
	db, cleanup := setupBatchTestDB(t)
	defer cleanup()

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	config := DefaultBatchConfig()
	config.BatchSize = 5
	executor := NewBatchExecutor(db, config, logger)

	ctx := context.Background()

	// Execute some batch operations
	columns := []string{"name", "value", "created_at"}
	values := make([][]interface{}, 12)
	for i := 0; i < 12; i++ {
		values[i] = []interface{}{
			"item_" + string(rune('A'+i)),
			i * 10,
			time.Now(),
		}
	}

	_, err := executor.BatchInsert(ctx, "items", columns, values)
	if err != nil {
		t.Fatalf("BatchInsert() error = %v", err)
	}

	stats := executor.GetStats()

	if stats.TotalBatches != 3 { // 5 + 5 + 2
		t.Errorf("TotalBatches = %d, want 3", stats.TotalBatches)
	}

	if stats.FailedBatches != 0 {
		t.Errorf("FailedBatches = %d, want 0", stats.FailedBatches)
	}
}

func BenchmarkBatchExecutor_BatchInsert(b *testing.B) {
	tmpFile, err := os.CreateTemp("", "bench-*.db")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}

	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()
	executor := NewBatchExecutor(db, DefaultBatchConfig(), logger)

	ctx := context.Background()

	// Prepare test data
	columns := []string{"name", "value", "created_at"}
	values := make([][]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		values[i] = []interface{}{
			"item_" + string(rune('A'+i%26)),
			i * 10,
			time.Now(),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear table
		db.Exec("DELETE FROM items")

		_, err := executor.BatchInsert(ctx, "items", columns, values)
		if err != nil {
			b.Fatalf("BatchInsert() error = %v", err)
		}
	}
}

func BenchmarkSingleInsert(b *testing.B) {
	tmpFile, err := os.CreateTemp("", "bench-single-*.db")
	if err != nil {
		b.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := sql.Open("sqlite", tmpFile.Name())
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			value INTEGER NOT NULL,
			created_at DATETIME NOT NULL
		);
	`)
	if err != nil {
		b.Fatalf("Failed to create table: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear table
		db.Exec("DELETE FROM items")

		// Insert 1000 items one by one
		for j := 0; j < 1000; j++ {
			_, err := db.ExecContext(ctx,
				"INSERT INTO items (name, value, created_at) VALUES (?, ?, ?)",
				"item_"+string(rune('A'+j%26)),
				j*10,
				time.Now(),
			)
			if err != nil {
				b.Fatalf("Insert error = %v", err)
			}
		}
	}
}
