package server

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/google/uuid"
)

type chatPersistenceMetricsStub struct {
	counts map[string]int64
}

func (m *chatPersistenceMetricsStub) RecordCounter(name string, value int64, _ map[string]string) {
	if m.counts == nil {
		m.counts = make(map[string]int64)
	}
	m.counts[name] += value
}

func TestPersistenceCoordinatorCoalescesAndFlushes(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "persist")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	auditStore, err := sessionaudit.NewSQLiteStore(filepath.Join(dbDir, "audit.db"), sessionaudit.StoreConfig{
		RetentionDays:    30,
		CleanupBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer auditStore.Close()

	metrics := &chatPersistenceMetricsStub{counts: make(map[string]int64)}
	coordinator := NewPersistenceCoordinator(store, auditStore, metrics)

	messageID := uuid.NewString()
	coordinator.EnqueueMessageUpdate(memory.Message{
		ID:             messageID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "draft-1",
	})
	coordinator.EnqueueMessageUpdate(memory.Message{
		ID:             messageID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "draft-2",
	})
	coordinator.EnqueuePreviousResponseID(conv.ID, "resp-1")
	coordinator.EnqueuePreviousResponseID(conv.ID, "resp-2")
	coordinator.EnqueueAudit(sessionaudit.Entry{
		ConversationID: conv.ID,
		EventType:      "assistant_tool_call",
		Role:           "assistant",
		ToolCallID:     "tc-1",
		ToolName:       "web_search",
		Payload:        `{"query":"hello"}`,
	})
	coordinator.EnqueueAudit(sessionaudit.Entry{
		ConversationID: conv.ID,
		EventType:      "tool_result",
		Role:           "tool",
		ToolCallID:     "tc-1",
		ToolName:       "web_search",
		Payload:        `{"ok":true}`,
	})

	coordinator.FlushMessage(messageID)

	msgs, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages after FlushMessage: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(GetMessages) = %d, want 1", len(msgs))
	}
	if msgs[0].Content != "draft-2" {
		t.Fatalf("message content = %q, want draft-2", msgs[0].Content)
	}

	prevID, err := store.GetConversationPreviousResponseID(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationPreviousResponseID: %v", err)
	}
	if prevID != "resp-2" {
		t.Fatalf("previous_response_id = %q, want resp-2", prevID)
	}

	auditEntries, err := auditStore.Recent(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("Recent audit entries: %v", err)
	}
	if len(auditEntries) != 2 {
		t.Fatalf("len(audit entries) = %d, want 2", len(auditEntries))
	}

	coordinator.EnqueueMessageUpdate(memory.Message{
		ID:             messageID,
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "draft-3",
	})
	coordinator.ShutdownFlush()

	msgs, err = store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages after ShutdownFlush: %v", err)
	}
	if msgs[0].Content != "draft-3" {
		t.Fatalf("message content after ShutdownFlush = %q, want draft-3", msgs[0].Content)
	}
	if metrics.counts["chat_persist_flush_total"] == 0 {
		t.Fatalf("chat_persist_flush_total not recorded")
	}
}
