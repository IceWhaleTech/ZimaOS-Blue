package sessionaudit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	rawLogPath := store.rawLogPath("conv-1")
	rawBytes, err := os.ReadFile(rawLogPath)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", rawLogPath, err)
	}
	var rawEntry Entry
	if err := json.Unmarshal(rawBytes, &rawEntry); err != nil {
		t.Fatalf("json.Unmarshal(jsonl sidecar) error = %v", err)
	}
	if rawEntry.Payload != entry.Payload {
		t.Fatalf("jsonl payload = %q, want %q", rawEntry.Payload, entry.Payload)
	}
}

func TestStoreDeleteConversation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session_audit_delete.db")
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Record(ctx, Entry{
		ConversationID: "conv-delete",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-delete",
		ToolName:       "exec",
		Payload:        `{"stdout":"gone"}`,
	}); err != nil {
		t.Fatalf("Record(delete) error = %v", err)
	}
	if err := store.Record(ctx, Entry{
		ConversationID: "conv-keep",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-keep",
		ToolName:       "exec",
		Payload:        `{"stdout":"stay"}`,
	}); err != nil {
		t.Fatalf("Record(keep) error = %v", err)
	}

	if err := store.DeleteConversation(ctx, "conv-delete"); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	deleted, err := store.Recent(ctx, "conv-delete", 10)
	if err != nil {
		t.Fatalf("Recent(deleted) error = %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("len(Recent(deleted)) = %d, want 0", len(deleted))
	}
	if _, err := os.Stat(store.rawLogPath("conv-delete")); !os.IsNotExist(err) {
		t.Fatalf("expected conv-delete raw log to be removed, stat err = %v", err)
	}

	kept, err := store.Recent(ctx, "conv-keep", 10)
	if err != nil {
		t.Fatalf("Recent(kept) error = %v", err)
	}
	if len(kept) != 1 || kept[0].ToolCallID != "tc-keep" {
		t.Fatalf("unexpected kept rows: %+v", kept)
	}
}

func TestStoreDeleteConversationRemovesEmptyRawLogDir(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewJSONLStore(baseDir, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	defer store.Close()

	if err := store.Record(context.Background(), Entry{
		ConversationID: "conv-only",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "only raw log entry",
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := store.DeleteConversation(context.Background(), "conv-only"); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	if _, err := os.Stat(store.rawLogDir); !os.IsNotExist(err) {
		t.Fatalf("expected raw log dir to be removed when empty, stat err = %v", err)
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

	rawBytes, err := os.ReadFile(store.rawLogPath("conv-2"))
	if err != nil {
		t.Fatalf("ReadFile(pruned raw log) error = %v", err)
	}
	rawText := string(rawBytes)
	if strings.Contains(rawText, "tc-old") {
		t.Fatalf("pruned jsonl still contains old entry: %q", rawText)
	}
	if !strings.Contains(rawText, "tc-new") {
		t.Fatalf("pruned jsonl missing new entry: %q", rawText)
	}

}

func TestStorePruneExpiredRemovesEmptyRawLogDirInJSONLMode(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewJSONLStore(baseDir, StoreConfig{
		RetentionDays:    1,
		CleanupInterval:  0,
		CleanupBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	defer store.Close()

	if err := store.Record(context.Background(), Entry{
		ConversationID: "conv-expired",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "expired only entry",
		CreatedAt:      timeutil.NowTime().Add(-72 * time.Hour),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := store.PruneExpired(context.Background()); err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}

	if _, err := os.Stat(store.rawLogDir); !os.IsNotExist(err) {
		t.Fatalf("expected raw log dir to be removed after pruning last file, stat err = %v", err)
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

func TestStoreSubscribeReceivesNewEntriesForConversation(t *testing.T) {
	store, err := NewJSONLStore(t.TempDir(), StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	defer store.Close()

	ch, cleanup := store.Subscribe("conv-watch")
	defer cleanup()

	if err := store.Record(context.Background(), Entry{
		ConversationID: "conv-other",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "should stay hidden",
		CreatedAt:      time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Record(other) error = %v", err)
	}
	if err := store.Record(context.Background(), Entry{
		ID:             "entry-watch",
		ConversationID: "conv-watch",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "watch me",
		CreatedAt:      time.Date(2026, 4, 6, 9, 0, 1, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Record(watch) error = %v", err)
	}

	select {
	case entry, ok := <-ch:
		if !ok {
			t.Fatal("subscription channel closed unexpectedly")
		}
		if entry.ConversationID != "conv-watch" {
			t.Fatalf("conversation_id = %q, want conv-watch", entry.ConversationID)
		}
		if entry.ID != "entry-watch" {
			t.Fatalf("id = %q, want entry-watch", entry.ID)
		}
		if entry.Payload != "watch me" {
			t.Fatalf("payload = %q, want watch me", entry.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for subscribed audit entry")
	}
}

func TestStoreSubscribeDoesNotBlockSlowSubscribers(t *testing.T) {
	store, err := NewJSONLStore(t.TempDir(), StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	defer store.Close()

	ch, cleanup := store.Subscribe("")
	defer cleanup()
	_ = ch

	entries := make([]Entry, 0, 2048)
	base := time.Date(2026, 4, 6, 9, 30, 0, 0, time.UTC)
	for i := 0; i < 2048; i++ {
		entries = append(entries, Entry{
			ID:             fmt.Sprintf("entry-%d", i),
			ConversationID: "conv-slow",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        fmt.Sprintf("payload-%d", i),
			CreatedAt:      base.Add(time.Duration(i) * time.Millisecond),
		})
	}

	done := make(chan error, 1)
	go func() {
		done <- store.RecordBatch(context.Background(), entries)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RecordBatch() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RecordBatch() blocked behind a slow audit subscriber")
	}
}

func TestNewJSONLStoreRecordAndSearchWithoutDB(t *testing.T) {
	baseDir := t.TempDir()
	store, err := NewJSONLStore(baseDir, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewJSONLStore() error = %v", err)
	}
	defer store.Close()

	entry := Entry{
		ConversationID: "conv-jsonl-only",
		SessionID:      "conv-jsonl-only",
		EventType:      "assistant_tool_call",
		Role:           "assistant",
		ToolCallID:     "tc-jsonl",
		ToolName:       "web_search",
		Payload:        `{"query":"jsonl only recall needle"}`,
	}
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	results, err := store.SearchConversations(context.Background(), SearchOptions{
		Query:                "recall needle",
		Limit:                5,
		PerConversationLimit: 3,
		SnippetLength:        120,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1", len(results))
	}
	if results[0].ConversationID != "conv-jsonl-only" {
		t.Fatalf("ConversationID = %q, want conv-jsonl-only", results[0].ConversationID)
	}

	if _, err := os.Stat(filepath.Join(baseDir, DefaultDBFilename)); !os.IsNotExist(err) {
		t.Fatalf("session_audit.db should not exist in jsonl-only mode, stat err = %v", err)
	}
	if _, err := os.Stat(store.rawLogPath("conv-jsonl-only")); err != nil {
		t.Fatalf("expected raw log file to exist, stat err = %v", err)
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

func TestStoreRecordBatchRollsBackOnValidationError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "batch_audit_rollback.db")
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
			ConversationID: "conv-batch-rollback",
			EventType:      "assistant_tool_call",
			Role:           "assistant",
			ToolCallID:     "tc-ok",
			ToolName:       "web_search",
			Payload:        `{"query":"ok"}`,
		},
		{
			EventType:  "tool_result",
			Role:       "tool",
			ToolCallID: "tc-bad",
			ToolName:   "web_search",
			Payload:    `{"ok":false}`,
		},
	})
	if err == nil {
		t.Fatal("RecordBatch() error = nil, want validation failure")
	}

	got, err := store.Recent(context.Background(), "conv-batch-rollback", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("len(Recent()) = %d, want 0 after rollback", len(got))
	}
}

func TestStoreUsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "session_audit_reader.db")
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	if store.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected file-backed session audit store to use a separate read db")
	}

	ctx := context.Background()
	if err := store.Record(ctx, Entry{
		ConversationID: "conv-reader",
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-reader",
		ToolName:       "web_search",
		Payload:        `{"results":[{"title":"reader"}]}`,
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := store.Recent(ctx, "conv-reader", 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(got) != 1 || got[0].ToolCallID != "tc-reader" {
		t.Fatalf("unexpected recent rows via reader: %+v", got)
	}
}

func TestStoreSearchConversationsAggregatesProjectedEntries(t *testing.T) {
	dbDir := t.TempDir()
	dbPath := filepath.Join(dbDir, DefaultDBFilename)
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	base := timeutil.NowTime().UTC()
	err = store.RecordBatch(ctx, []Entry{
		{
			ID:             "user-hit",
			ConversationID: "conv-search",
			SessionID:      "conv-search",
			UserID:         "user-1",
			EventType:      "user_message",
			Role:           "user",
			Payload:        "Need a recovery plan for the router reboot issue",
			CreatedAt:      base.Add(-4 * time.Minute),
		},
		{
			ID:             "assistant-hit",
			ConversationID: "conv-search",
			SessionID:      "conv-search",
			UserID:         "user-1",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "The recovery plan should include diagnostics and rollback steps.",
			CreatedAt:      base.Add(-3 * time.Minute),
		},
		{
			ID:             "tool-hit",
			ConversationID: "conv-search",
			SessionID:      "conv-search",
			UserID:         "user-1",
			EventType:      "tool_result",
			Role:           "tool",
			ToolName:       "web_search",
			Payload:        `{"summary":"router recovery plan checklist","status":"ok"}`,
			CreatedAt:      base.Add(-2 * time.Minute),
		},
		{
			ID:             "context-hit",
			ConversationID: "conv-search",
			SessionID:      "conv-search",
			UserID:         "user-1",
			EventType:      "context_pack",
			Role:           "system",
			Payload:        `{"summary":"Recovery playbook for router outage incident"}`,
			CreatedAt:      base.Add(-1 * time.Minute),
		},
		{
			ID:             "other-conv",
			ConversationID: "conv-other",
			SessionID:      "conv-other",
			UserID:         "user-1",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "Nothing about the outage appears here",
			CreatedAt:      base,
		},
	})
	if err != nil {
		t.Fatalf("RecordBatch() error = %v", err)
	}

	results, err := store.SearchConversations(ctx, SearchOptions{
		Query:                "recovery plan router",
		Limit:                5,
		PerConversationLimit: 4,
		SnippetLength:        80,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1", len(results))
	}
	if results[0].ConversationID != "conv-search" {
		t.Fatalf("ConversationID = %q, want conv-search", results[0].ConversationID)
	}
	if results[0].HitCount != 4 {
		t.Fatalf("HitCount = %d, want 4", results[0].HitCount)
	}
	if len(results[0].Snippets) != 4 {
		t.Fatalf("len(Snippets) = %d, want 4", len(results[0].Snippets))
	}
	if results[0].Snippets[0].CreatedAt.Before(results[0].Snippets[1].CreatedAt) {
		t.Fatalf("snippets not sorted desc by CreatedAt: %+v", results[0].Snippets)
	}
	if results[0].Snippets[0].EventType != "context_pack" {
		t.Fatalf("latest snippet event_type = %q, want context_pack", results[0].Snippets[0].EventType)
	}
	if !strings.Contains(strings.ToLower(results[0].Snippets[0].Snippet), "recovery") {
		t.Fatalf("snippet = %q, want recovery context", results[0].Snippets[0].Snippet)
	}

	rawDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer rawDB.Close()

	var count int
	if err := rawDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='session_search_projections'`).Scan(&count); err != nil {
		t.Fatalf("query session_search_projections schema: %v", err)
	}
	if count != 1 {
		t.Fatalf("session_search_projections count = %d, want 1", count)
	}
	if _, err := os.Stat(filepath.Join(dbDir, "session_search.db")); !os.IsNotExist(err) {
		t.Fatalf("session_search.db should not exist, stat err = %v", err)
	}
}

func TestStoreSearchConversationsFallsBackToRawLogForLargePayloads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), DefaultDBFilename)
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	largePayload := strings.Repeat("x", searchProjectionMaxBytes) + " hidden-needle"
	entry := Entry{
		ID:             "too-large",
		ConversationID: "conv-large",
		SessionID:      "conv-large",
		EventType:      "tool_result",
		Role:           "tool",
		ToolName:       "read_file",
		Payload:        largePayload,
	}
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	results, err := store.SearchConversations(context.Background(), SearchOptions{
		Query:         "hidden-needle",
		Limit:         5,
		SnippetLength: 80,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1 via raw log fallback", len(results))
	}
	if results[0].ConversationID != "conv-large" {
		t.Fatalf("ConversationID = %q, want conv-large", results[0].ConversationID)
	}
	if results[0].HitCount != 1 {
		t.Fatalf("HitCount = %d, want 1", results[0].HitCount)
	}
	if len(results[0].Snippets) != 1 || !strings.Contains(results[0].Snippets[0].Snippet, "hidden-needle") {
		t.Fatalf("unexpected raw fallback snippets: %+v", results[0].Snippets)
	}

	recent, err := store.Recent(context.Background(), "conv-large", 5)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(recent) != 1 {
		t.Fatalf("len(Recent()) = %d, want 1", len(recent))
	}
	if recent[0].Payload != largePayload {
		t.Fatalf("Recent payload mismatch")
	}
}

func TestStoreSearchConversationsFallsBackToRawLogForUnprojectedEventType(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), DefaultDBFilename)
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
		ID:             "tool-call-only",
		ConversationID: "conv-tool-call",
		SessionID:      "conv-tool-call",
		UserID:         "user-1",
		EventType:      "assistant_tool_call",
		Role:           "assistant",
		ToolCallID:     "tc-rg",
		ToolName:       "web_search",
		Payload:        `{"query":"nightly regression recall needle"}`,
	}
	if err := store.Record(context.Background(), entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	results, err := store.SearchConversations(context.Background(), SearchOptions{
		Query:                "regression needle",
		Limit:                5,
		PerConversationLimit: 3,
		SnippetLength:        120,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1 via raw log fallback", len(results))
	}
	if results[0].ConversationID != "conv-tool-call" {
		t.Fatalf("ConversationID = %q, want conv-tool-call", results[0].ConversationID)
	}
	if len(results[0].Snippets) != 1 {
		t.Fatalf("len(Snippets) = %d, want 1", len(results[0].Snippets))
	}
	if results[0].Snippets[0].EventType != "assistant_tool_call" {
		t.Fatalf("EventType = %q, want assistant_tool_call", results[0].Snippets[0].EventType)
	}
	if !strings.Contains(strings.ToLower(results[0].Snippets[0].Snippet), "needle") {
		t.Fatalf("snippet = %q, want raw payload hit", results[0].Snippets[0].Snippet)
	}
}

func TestStoreDeleteConversationRemovesSearchProjection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), DefaultDBFilename)
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if err := store.Record(ctx, Entry{
		ID:             "delete-search-hit",
		ConversationID: "conv-delete-search",
		SessionID:      "conv-delete-search",
		EventType:      "assistant_message",
		Role:           "assistant",
		Payload:        "delete me from searchable projections",
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	results, err := store.SearchConversations(ctx, SearchOptions{Query: "searchable", Limit: 5})
	if err != nil {
		t.Fatalf("SearchConversations(before delete) error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations(before delete)) = %d, want 1", len(results))
	}

	if err := store.DeleteConversation(ctx, "conv-delete-search"); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}

	results, err = store.SearchConversations(ctx, SearchOptions{Query: "searchable", Limit: 5})
	if err != nil {
		t.Fatalf("SearchConversations(after delete) error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("len(SearchConversations(after delete)) = %d, want 0", len(results))
	}
}

func TestStorePruneExpiredRemovesSearchProjection(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), DefaultDBFilename)
	store, err := NewSQLiteStore(dbPath, StoreConfig{
		RetentionDays:    1,
		CleanupInterval:  0,
		CleanupBatchSize: 100,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore() error = %v", err)
	}
	defer store.Close()

	now := timeutil.NowTime()
	if err := store.RecordBatch(context.Background(), []Entry{
		{
			ID:             "old-projection",
			ConversationID: "conv-prune",
			SessionID:      "conv-prune",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "obsolete searchable snippet",
			CreatedAt:      now.Add(-72 * time.Hour),
		},
		{
			ID:             "new-projection",
			ConversationID: "conv-prune",
			SessionID:      "conv-prune",
			EventType:      "assistant_message",
			Role:           "assistant",
			Payload:        "fresh searchable snippet",
			CreatedAt:      now.Add(-1 * time.Hour),
		},
	}); err != nil {
		t.Fatalf("RecordBatch() error = %v", err)
	}

	if err := store.PruneExpired(context.Background()); err != nil {
		t.Fatalf("PruneExpired() error = %v", err)
	}

	results, err := store.SearchConversations(context.Background(), SearchOptions{
		Query:                "searchable snippet",
		Limit:                5,
		PerConversationLimit: 5,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1", len(results))
	}
	if results[0].HitCount != 1 {
		t.Fatalf("HitCount = %d, want 1 after prune", results[0].HitCount)
	}
	if len(results[0].Snippets) != 1 || !strings.Contains(results[0].Snippets[0].Snippet, "fresh") {
		t.Fatalf("unexpected snippets after prune: %+v", results[0].Snippets)
	}
}
