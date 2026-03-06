package sessionaudit

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
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
