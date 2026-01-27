package chaos

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestDiskFullSimulator_NewDiskFullSimulator(t *testing.T) {
	tempDir := t.TempDir()
	sim := NewDiskFullSimulator(tempDir)

	if sim == nil {
		t.Fatal("Expected simulator, got nil")
	}

	if sim.targetPath != tempDir {
		t.Errorf("Expected target path %s, got %s", tempDir, sim.targetPath)
	}
}

func TestDiskFullSimulator_EnableDisable(t *testing.T) {
	tempDir := t.TempDir()
	sim := NewDiskFullSimulator(tempDir)

	ctx := context.Background()

	// Enable with small percentage for testing
	err := sim.Enable(ctx, 1)
	if err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	if !sim.IsEnabled() {
		t.Error("Expected simulator to be enabled")
	}

	// Should fail if already enabled
	err = sim.Enable(ctx, 1)
	if err == nil {
		t.Error("Expected error when enabling already enabled simulator")
	}

	// Disable
	err = sim.Disable()
	if err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	if sim.IsEnabled() {
		t.Error("Expected simulator to be disabled")
	}

	// Verify filler files are cleaned up
	entries, _ := os.ReadDir(tempDir)
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("Filler file not cleaned up: %s", entry.Name())
		}
	}
}

func TestDiskFullSimulator_DisableWhenNotEnabled(t *testing.T) {
	tempDir := t.TempDir()
	sim := NewDiskFullSimulator(tempDir)

	// Should not error when disabling already disabled simulator
	err := sim.Disable()
	if err != nil {
		t.Errorf("Disable should not error when not enabled: %v", err)
	}
}

func TestDatabaseCorruptionSimulator_NewDatabaseCorruptionSimulator(t *testing.T) {
	sim := NewDatabaseCorruptionSimulator("/tmp/test.db")

	if sim == nil {
		t.Fatal("Expected simulator, got nil")
	}

	if sim.dbPath != "/tmp/test.db" {
		t.Errorf("Expected db path /tmp/test.db, got %s", sim.dbPath)
	}
}

func TestDatabaseCorruptionSimulator_CorruptHeader(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (name) VALUES ('test')")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}
	db.Close()

	// Corrupt the header
	sim := NewDatabaseCorruptionSimulator(dbPath)
	err = sim.CorruptHeader()
	if err != nil {
		t.Fatalf("CorruptHeader failed: %v", err)
	}

	// Verify backup was created
	if sim.backupPath == "" {
		t.Error("Expected backup to be created")
	}

	// Verify database is corrupted
	db, err = sql.Open("sqlite3", dbPath)
	if err == nil {
		_, err = db.Query("SELECT * FROM test")
		if err == nil {
			t.Error("Expected database to be corrupted")
		}
		db.Close()
	}

	// Restore
	err = sim.Restore()
	if err != nil {
		t.Fatalf("Restore failed: %v", err)
	}

	// Verify database is restored
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open restored database: %v", err)
	}
	defer db.Close()

	var name string
	err = db.QueryRow("SELECT name FROM test WHERE id = 1").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query restored database: %v", err)
	}

	if name != "test" {
		t.Errorf("Expected name 'test', got %s", name)
	}

	sim.Cleanup()
}

func TestDatabaseCorruptionSimulator_CorruptPage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	db.Close()

	// Corrupt page 1
	sim := NewDatabaseCorruptionSimulator(dbPath)
	err = sim.CorruptPage(1)
	if err != nil {
		t.Fatalf("CorruptPage failed: %v", err)
	}

	// Restore and cleanup
	sim.Restore()
	sim.Cleanup()
}

func TestDatabaseCorruptionSimulator_TruncateFile(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database with some data
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Insert some data to make the file larger
	for i := 0; i < 100; i++ {
		_, err = db.Exec("INSERT INTO test (data) VALUES (?)", "some test data that takes up space")
		if err != nil {
			t.Fatalf("Failed to insert data: %v", err)
		}
	}
	db.Close()

	// Get original size
	info, _ := os.Stat(dbPath)
	originalSize := info.Size()

	// Truncate to 50%
	sim := NewDatabaseCorruptionSimulator(dbPath)
	err = sim.TruncateFile(50)
	if err != nil {
		t.Fatalf("TruncateFile failed: %v", err)
	}

	// Verify file is truncated
	info, _ = os.Stat(dbPath)
	if info.Size() >= originalSize {
		t.Error("Expected file to be truncated")
	}

	// Restore
	sim.Restore()
	sim.Cleanup()
}

func TestDatabaseCorruptionSimulator_RestoreWithoutBackup(t *testing.T) {
	sim := NewDatabaseCorruptionSimulator("/tmp/nonexistent.db")

	err := sim.Restore()
	if err == nil {
		t.Error("Expected error when restoring without backup")
	}
}

func TestRecoveryTester_TestIntegrityCheck(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	db.Close()

	// Test integrity check
	tester := NewRecoveryTester(dbPath)
	result, err := tester.TestIntegrityCheck()
	if err != nil {
		t.Fatalf("TestIntegrityCheck failed: %v", err)
	}

	if !result.Passed {
		t.Errorf("Expected integrity check to pass: %s", result.Message)
	}
}

func TestRecoveryTester_TestIntegrityCheck_Corrupted(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}
	db.Close()

	// Corrupt the database
	sim := NewDatabaseCorruptionSimulator(dbPath)
	sim.CorruptHeader()

	// Test integrity check
	tester := NewRecoveryTester(dbPath)
	result, err := tester.TestIntegrityCheck()
	if err != nil {
		t.Fatalf("TestIntegrityCheck failed: %v", err)
	}

	if result.Passed {
		t.Error("Expected integrity check to fail on corrupted database")
	}

	sim.Restore()
	sim.Cleanup()
}

func TestRecoveryTester_TestRecovery(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Create a valid SQLite database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.Exec("INSERT INTO test (name) VALUES ('test')")
	if err != nil {
		t.Fatalf("Failed to insert data: %v", err)
	}
	db.Close()

	// Test recovery on valid database
	tester := NewRecoveryTester(dbPath)
	ctx := context.Background()
	result, err := tester.TestRecovery(ctx)
	if err != nil {
		t.Fatalf("TestRecovery failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected recovery to succeed: %s", result.Error)
	}

	if !result.IntegrityPassed {
		t.Error("Expected integrity check to pass")
	}
}

func TestZeroReader(t *testing.T) {
	reader := &zeroReader{}
	buf := make([]byte, 100)

	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if n != 100 {
		t.Errorf("Expected to read 100 bytes, got %d", n)
	}

	for i, b := range buf {
		if b != 0 {
			t.Errorf("Expected zero at position %d, got %d", i, b)
		}
	}
}
