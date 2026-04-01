package audit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestLogger(t *testing.T) (*SQLiteLogger, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	config := &LoggerConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
		BufferSize:    100,
	}

	logger, err := NewSQLiteLogger(db, config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	cleanup := func() {
		logger.Close()
		db.Close()
	}

	return logger, cleanup
}

func TestNewEntry(t *testing.T) {
	entry := NewEntry(ActionLogin, StatusSuccess)

	if entry.ID == uuid.Nil {
		t.Error("NewEntry() ID should not be nil")
	}
	if entry.Action != ActionLogin {
		t.Errorf("NewEntry() Action = %v, want %v", entry.Action, ActionLogin)
	}
	if entry.Status != StatusSuccess {
		t.Errorf("NewEntry() Status = %v, want %v", entry.Status, StatusSuccess)
	}
	if entry.Timestamp.IsZero() {
		t.Error("NewEntry() Timestamp should not be zero")
	}
}

func TestEntry_WithUser(t *testing.T) {
	entry := NewEntry(ActionLogin, StatusSuccess)
	userID := uuid.New()

	entry.WithUser(userID, "testuser")

	if entry.UserID == nil || *entry.UserID != userID {
		t.Errorf("WithUser() UserID = %v, want %v", entry.UserID, userID)
	}
	if entry.Username != "testuser" {
		t.Errorf("WithUser() Username = %v, want testuser", entry.Username)
	}
}

func TestEntry_WithResource(t *testing.T) {
	entry := NewEntry(ActionUserCreate, StatusSuccess)

	entry.WithResource("user", "123")

	if entry.ResourceType != "user" {
		t.Errorf("WithResource() ResourceType = %v, want user", entry.ResourceType)
	}
	if entry.ResourceID != "123" {
		t.Errorf("WithResource() ResourceID = %v, want 123", entry.ResourceID)
	}
}

func TestEntry_WithRequest(t *testing.T) {
	entry := NewEntry(ActionLogin, StatusSuccess)

	entry.WithRequest("192.168.1.1", "Mozilla/5.0", "req-123")

	if entry.IPAddress != "192.168.1.1" {
		t.Errorf("WithRequest() IPAddress = %v, want 192.168.1.1", entry.IPAddress)
	}
	if entry.UserAgent != "Mozilla/5.0" {
		t.Errorf("WithRequest() UserAgent = %v, want Mozilla/5.0", entry.UserAgent)
	}
	if entry.RequestID != "req-123" {
		t.Errorf("WithRequest() RequestID = %v, want req-123", entry.RequestID)
	}
}

func TestEntry_WithDetails(t *testing.T) {
	entry := NewEntry(ActionLogin, StatusSuccess)

	details := map[string]string{"method": "password"}
	entry.WithDetails(details)

	if entry.Details == nil {
		t.Error("WithDetails() Details should not be nil")
	}
}

func TestEntry_WithChange(t *testing.T) {
	entry := NewEntry(ActionUserUpdate, StatusSuccess)

	oldValue := map[string]string{"name": "old"}
	newValue := map[string]string{"name": "new"}
	entry.WithChange(oldValue, newValue)

	if entry.OldValue == nil {
		t.Error("WithChange() OldValue should not be nil")
	}
	if entry.NewValue == nil {
		t.Error("WithChange() NewValue should not be nil")
	}
}

func TestSQLiteLogger_Log(t *testing.T) {
	logger, cleanup := setupTestLogger(t)
	defer cleanup()

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess).
		WithUser(uuid.New(), "testuser").
		WithRequest("192.168.1.1", "Mozilla/5.0", "req-123")

	err := logger.Log(ctx, entry)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Query to verify
	result, err := logger.Query(ctx, &QueryParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(result.Entries) != 1 {
		t.Errorf("Query() returned %d entries, want 1", len(result.Entries))
	}
}

func TestSQLiteLogger_Query(t *testing.T) {
	logger, cleanup := setupTestLogger(t)
	defer cleanup()

	ctx := context.Background()

	// Log multiple entries
	for i := 0; i < 25; i++ {
		entry := NewEntry(ActionLogin, StatusSuccess).
			WithUser(uuid.New(), "testuser")
		_ = logger.Log(ctx, entry)
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Test pagination
	result, err := logger.Query(ctx, &QueryParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(result.Entries) != 10 {
		t.Errorf("Query() returned %d entries, want 10", len(result.Entries))
	}
	if result.Total != 25 {
		t.Errorf("Query() total = %d, want 25", result.Total)
	}
	if result.TotalPages != 3 {
		t.Errorf("Query() totalPages = %d, want 3", result.TotalPages)
	}
}

func TestSQLiteLogger_Query_Filter(t *testing.T) {
	logger, cleanup := setupTestLogger(t)
	defer cleanup()

	ctx := context.Background()

	// Log entries with different actions
	entry1 := NewEntry(ActionLogin, StatusSuccess)
	entry2 := NewEntry(ActionLogout, StatusSuccess)
	entry3 := NewEntry(ActionLogin, StatusFailure)

	_ = logger.Log(ctx, entry1)
	_ = logger.Log(ctx, entry2)
	_ = logger.Log(ctx, entry3)

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Filter by action
	action := ActionLogin
	result, err := logger.Query(ctx, &QueryParams{Action: &action})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(result.Entries) != 2 {
		t.Errorf("Query() with action filter returned %d entries, want 2", len(result.Entries))
	}

	// Filter by status
	status := StatusFailure
	result, err = logger.Query(ctx, &QueryParams{Status: &status})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(result.Entries) != 1 {
		t.Errorf("Query() with status filter returned %d entries, want 1", len(result.Entries))
	}
}

func TestSQLiteLogger_GetByID(t *testing.T) {
	logger, cleanup := setupTestLogger(t)
	defer cleanup()

	ctx := context.Background()

	entry := NewEntry(ActionLogin, StatusSuccess).
		WithUser(uuid.New(), "testuser")

	_ = logger.Log(ctx, entry)

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	// Get by ID
	found, err := logger.GetByID(ctx, entry.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if found.ID != entry.ID {
		t.Errorf("GetByID() ID = %v, want %v", found.ID, entry.ID)
	}
	if found.Action != entry.Action {
		t.Errorf("GetByID() Action = %v, want %v", found.Action, entry.Action)
	}
}

func TestSQLiteLogger_GetByID_NotFound(t *testing.T) {
	logger, cleanup := setupTestLogger(t)
	defer cleanup()

	ctx := context.Background()

	_, err := logger.GetByID(ctx, uuid.New())
	if err == nil {
		t.Error("GetByID() expected error for non-existent entry")
	}
}

func TestDefaultLoggerConfig(t *testing.T) {
	config := DefaultLoggerConfig()

	if config.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", config.BatchSize)
	}
	if config.FlushInterval != 5*time.Second {
		t.Errorf("FlushInterval = %v, want 5s", config.FlushInterval)
	}
	if config.BufferSize != 1000 {
		t.Errorf("BufferSize = %d, want 1000", config.BufferSize)
	}
}

func TestSQLiteLogger_UsesReaderDBForQueries(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "audit.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("failed to open writer database: %v", err)
	}
	defer writeDB.Close()

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("failed to open reader database: %v", err)
	}
	defer readDB.Close()

	config := &LoggerConfig{
		BatchSize:     1,
		FlushInterval: 50 * time.Millisecond,
		BufferSize:    10,
	}
	logger, err := NewSQLiteLoggerWithReadDB(writeDB, readDB, config)
	if err != nil {
		t.Fatalf("failed to create logger with read db: %v", err)
	}
	defer logger.Close()

	if logger.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if logger.readDB == logger.db {
		t.Fatal("expected logger to use a separate read db")
	}

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess)
	if err := logger.Log(ctx, entry); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	result, err := logger.Query(ctx, &QueryParams{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(result.Entries) != 1 {
		t.Fatalf("Query() returned %d entries, want 1", len(result.Entries))
	}
	if result.Entries[0].ID != entry.ID {
		t.Fatalf("Query() returned ID %v, want %v", result.Entries[0].ID, entry.ID)
	}
}
