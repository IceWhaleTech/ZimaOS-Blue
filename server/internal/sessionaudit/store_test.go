package sessionaudit

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	_ "github.com/mattn/go-sqlite3"
)

func TestStoreRecordAndRecent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session_audit_test.db")
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	entry := Entry{
		ConversationID: "conv-1",
		SessionID:      "conv-1",
		UserID:         "user-1",
		Source:         "web",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-1",
		ToolName:       "web_search",
		Payload:        `{"results":[{"title":"ok"}]}`,
	}
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	got, err := store.Recent(context.Background(), "conv-1", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Recent()) = %d, want 1", len(got))
	}
	if got[0].Payload != entry.Payload {
		t.Fatalf("payload = %q, want %q", got[0].Payload, entry.Payload)
	}
	if got[0].PayloadBytes != len(entry.Payload) {
		t.Fatalf("payload_bytes = %d, want %d", got[0].PayloadBytes, len(entry.Payload))
	}
}

func TestStorePruneExpired(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session_audit_prune.db")
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    1,
		CleanupInterval:  0,
		CleanupBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	now := timeutil.NowTime()
	if err := store.Record(context.Background(), Entry{
		ConversationID: "conv-2",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-old",
		ToolName:       "exec",
		Payload:        `{"stdout":"old"}`,
		CreatedAt:      now.Add(-48 * time.Hour),
	}); err != nil {
		t.Fatalf("Record(old) error = %v", err)
	}
	if err := store.Record(context.Background(), Entry{
		ConversationID: "conv-2",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-new",
		ToolName:       "exec",
		Payload:        `{"stdout":"new"}`,
		CreatedAt:      now.Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("Record(new) error = %v", err)
	}

	if err := store.PruneExpired(context.Background()); err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}

	got, err := store.Recent(context.Background(), "conv-2", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(Recent()) = %d, want 1", len(got))
	}
	if got[0].ToolCallID != "tc-new" {
		t.Fatalf("remaining tool_call_id = %q, want tc-new", got[0].ToolCallID)
	}

}

func TestStoreWithDBDoesNotOwnSharedConnection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared_audit.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStoreWithDB(db, StoreConfig{RetentionDays: 30, CleanupBatchSize: 100})
	if err != nil {
		t.Fatalf("NewSQLiteStoreWithDB() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := db.Exec("SELECT 1"); err != nil {
		t.Fatalf("shared DB should remain usable after store.Close(): %v", err)
	}
}

func TestStoreRecordBatch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_audit.db")
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	err = store.RecordBatch(context.Background(), []Entry{
		{
			ConversationID: "conv-batch",
			EventType:      "assistant_tool_call",
			Role:           "assistant",
			ToolCallID:     "tc-1",
			ToolName:       "web_search",
			Payload:        `{"query":"one"}`,
		},
		{
			ConversationID: "conv-batch",
			EventType:      "tool_result",
			Role:           "tool",
			ToolCallID:     "tc-1",
			ToolName:       "web_search",
			Payload:        `{"ok":true}`,
		},
	})
	if err != nil {
		t.Fatalf("RecordBatch() error = %v", err)
	}

	got, err := store.Recent(context.Background(), "conv-batch", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(Recent()) = %d, want 2", len(got))
	}
}
