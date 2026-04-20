package server

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/google/uuid"
)

type chatPersistenceMetricsStub struct {
	counts map[string]int64
}

func (m *chatPersistenceMetricsStub) RecordAPICall(string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatPersistenceMetricsStub) RecordAPICallForUser(string, string, bool, float64, int64, int64, int64, int64, string) {
}

func (m *chatPersistenceMetricsStub) RecordSpeed(string, float64, float64, float64) {}

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

func TestPersistenceCoordinatorShutdownFlushWithinTimesOut(t *testing.T) {
	coordinator := &PersistenceCoordinator{
		queue: make(chan persistenceOp, 1),
		done:  make(chan struct{}),
	}

	started := time.Now()
	ok := coordinator.ShutdownFlushWithin(20 * time.Millisecond)
	elapsed := time.Since(started)

	if ok {
		t.Fatal("ShutdownFlushWithin() = true, want false when no worker drains the queue")
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("ShutdownFlushWithin() took %s, want under 200ms", elapsed)
	}
}

func TestPendingPersistenceTakeBatchPreservesOrderAfterCoalescing(t *testing.T) {
	pending := newPendingPersistence()
	messageID := uuid.NewString()

	pending.enqueue(persistenceOp{
		kind: persistenceOpMessageUpdate,
		message: memory.Message{
			ID:             messageID,
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "draft-1",
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpAudit,
		auditEntry: sessionaudit.Entry{
			ConversationID: "conv-1",
			EventType:      "tool_result",
			Payload:        `{"step":1}`,
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpMessageUpdate,
		message: memory.Message{
			ID:             messageID,
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "draft-2",
		},
	})
	pending.enqueue(persistenceOp{
		kind:           persistenceOpPreviousResponse,
		conversationID: "conv-1",
		responseID:     "resp-1",
	})
	pending.enqueue(persistenceOp{
		kind:           persistenceOpPreviousResponse,
		conversationID: "conv-1",
		responseID:     "resp-2",
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpAudit,
		auditEntry: sessionaudit.Entry{
			ConversationID: "conv-1",
			EventType:      "assistant_message",
			Payload:        "final answer",
		},
	})

	batch := pending.takeBatch(10)

	got := make([]string, 0, len(batch.ops))
	for _, op := range batch.ops {
		switch op.kind {
		case persistenceOpMessageUpdate:
			got = append(got, "message:"+op.message.Content)
		case persistenceOpPreviousResponse:
			got = append(got, "previous:"+op.responseID)
		case persistenceOpAudit:
			got = append(got, "audit:"+op.auditEntry.EventType)
		default:
			got = append(got, string(op.kind))
		}
	}

	want := []string{
		"audit:tool_result",
		"message:draft-2",
		"previous:resp-2",
		"audit:assistant_message",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("takeBatch order = %v, want %v", got, want)
	}
	if pending.count() != 0 {
		t.Fatalf("pending.count() = %d, want 0 after taking all active ops", pending.count())
	}
}

func TestPendingPersistenceTakeBatchRetainsLaterActiveOps(t *testing.T) {
	pending := newPendingPersistence()
	firstMessageID := uuid.NewString()
	secondMessageID := uuid.NewString()

	pending.enqueue(persistenceOp{
		kind: persistenceOpMessageUpdate,
		message: memory.Message{
			ID:             firstMessageID,
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "draft-1",
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpMessageUpdate,
		message: memory.Message{
			ID:             firstMessageID,
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "draft-2",
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpAudit,
		auditEntry: sessionaudit.Entry{
			ConversationID: "conv-1",
			EventType:      "tool_result",
			Payload:        `{"step":1}`,
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpAudit,
		auditEntry: sessionaudit.Entry{
			ConversationID: "conv-1",
			EventType:      "assistant_message",
			Payload:        "answer",
		},
	})
	pending.enqueue(persistenceOp{
		kind: persistenceOpMessageUpdate,
		message: memory.Message{
			ID:             secondMessageID,
			ConversationID: "conv-1",
			Role:           "assistant",
			Content:        "tail",
		},
	})

	firstBatch := pending.takeBatch(2)
	if len(firstBatch.ops) != 2 {
		t.Fatalf("len(firstBatch.ops) = %d, want 2", len(firstBatch.ops))
	}
	firstKinds := []string{
		string(firstBatch.ops[0].kind) + ":" + firstBatch.ops[0].message.Content,
		string(firstBatch.ops[1].kind) + ":" + firstBatch.ops[1].auditEntry.EventType,
	}
	wantFirst := []string{
		"message_update:draft-2",
		"audit:tool_result",
	}
	if !reflect.DeepEqual(firstKinds, wantFirst) {
		t.Fatalf("first batch = %v, want %v", firstKinds, wantFirst)
	}

	secondBatch := pending.takeBatch(10)
	secondKinds := make([]string, 0, len(secondBatch.ops))
	for _, op := range secondBatch.ops {
		switch op.kind {
		case persistenceOpMessageUpdate:
			secondKinds = append(secondKinds, "message:"+op.message.Content)
		case persistenceOpAudit:
			secondKinds = append(secondKinds, "audit:"+op.auditEntry.EventType)
		}
	}
	wantSecond := []string{
		"audit:assistant_message",
		"message:tail",
	}
	if !reflect.DeepEqual(secondKinds, wantSecond) {
		t.Fatalf("second batch = %v, want %v", secondKinds, wantSecond)
	}
}

func TestPersistResponsePathMessageSkipsBlockingFlushWhenBarrierDisabled(t *testing.T) {
	handler := &ChatHandler{
		chatPersistAsync:           true,
		chatPersistFlushOnResponse: false,
		persistCoordinator: &PersistenceCoordinator{
			queue: make(chan persistenceOp, 1),
			done:  make(chan struct{}),
		},
	}

	start := time.Now()
	persistedID := handler.persistResponsePathMessage(memory.Message{
		ID:             uuid.NewString(),
		ConversationID: "conv-1",
		Role:           "assistant",
		Content:        "queued-only",
	})
	elapsed := time.Since(start)

	if strings.TrimSpace(persistedID) == "" {
		t.Fatal("persistResponsePathMessage() returned empty id")
	}
	if elapsed > 20*time.Millisecond {
		t.Fatalf("persistResponsePathMessage() took %s with barrier disabled, want under 20ms", elapsed)
	}
}

func TestPersistResponsePathMessageWaitsForBarrierWhenEnabled(t *testing.T) {
	coordinator := &PersistenceCoordinator{
		queue: make(chan persistenceOp, 2),
		done:  make(chan struct{}),
	}
	go func() {
		first := <-coordinator.queue
		if first.kind != persistenceOpMessageUpdate {
			return
		}
		second := <-coordinator.queue
		if second.kind != persistenceOpFlush || second.ack == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
		close(second.ack)
	}()

	handler := &ChatHandler{
		chatPersistAsync:           true,
		chatPersistFlushOnResponse: true,
		persistCoordinator:         coordinator,
	}

	start := time.Now()
	persistedID := handler.persistResponsePathMessage(memory.Message{
		ID:             uuid.NewString(),
		ConversationID: "conv-1",
		Role:           "assistant",
		Content:        "wait-for-barrier",
	})
	elapsed := time.Since(start)

	if strings.TrimSpace(persistedID) == "" {
		t.Fatal("persistResponsePathMessage() returned empty id")
	}
	if elapsed < 20*time.Millisecond {
		t.Fatalf("persistResponsePathMessage() took %s with barrier enabled, want at least 20ms", elapsed)
	}
}

func TestPersistChannelResponseMessageWaitsForBarrierWhenEnabled(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	coordinator := &PersistenceCoordinator{
		queue: make(chan persistenceOp, 2),
		done:  make(chan struct{}),
	}
	go func() {
		first := <-coordinator.queue
		if first.kind != persistenceOpMessageUpdate {
			return
		}
		select {
		case second := <-coordinator.queue:
			if second.kind != persistenceOpFlush || second.ack == nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
			close(second.ack)
		case <-time.After(100 * time.Millisecond):
			return
		}
	}()

	handler := NewChatHandler(store, nil, nil)
	handler.chatPersistAsync = true
	handler.chatPersistFlushOnResponse = true
	handler.persistCoordinator = coordinator
	defer handler.Close()

	start := time.Now()
	msg, err := handler.persistChannelResponseMessage(context.Background(), "ch:wechat_ilink:user-1", "wait-for-im-barrier")
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("persistChannelResponseMessage() error = %v", err)
	}
	if msg == nil || strings.TrimSpace(msg.ID) == "" {
		t.Fatal("persistChannelResponseMessage() returned nil or empty message id")
	}
	if elapsed < 20*time.Millisecond {
		t.Fatalf("persistChannelResponseMessage() took %s with barrier enabled, want at least 20ms", elapsed)
	}
}

func TestPersistConversationMessagesWithBarrierFlushesQueuedMessagesInOrder(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "persist-batch")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	handler := NewChatHandler(store, nil, nil)
	handler.SetPersistenceOptions(true, true, false)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	ids := handler.persistConversationMessages(conv.ID, true,
		memory.Message{Role: "user", Content: "first user"},
		memory.Message{Role: "assistant", Content: "second assistant"},
	)
	if len(ids) != 2 || strings.TrimSpace(ids[0]) == "" || strings.TrimSpace(ids[1]) == "" {
		t.Fatalf("persistConversationMessages() ids = %v, want 2 non-empty ids", ids)
	}

	msgs, err := store.GetMessages(ctx, conv.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(GetMessages) = %d, want 2", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "first user" {
		t.Fatalf("msgs[0] = %+v, want first queued user message", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "second assistant" {
		t.Fatalf("msgs[1] = %+v, want second queued assistant message", msgs[1])
	}
}

func TestChatHandlerPersistenceCoordinator_InitializesOnFirstAsyncPersist(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "lazy-persist")
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

	handler := NewChatHandler(store, nil, nil)
	defer handler.Close()

	metrics := &chatPersistenceMetricsStub{counts: make(map[string]int64)}
	handler.SetPersistenceOptions(true, true, false)
	handler.SetMetricsRecorder(metrics)
	handler.SetSessionAuditStore(auditStore)

	if handler.persistCoordinator != nil {
		t.Fatal("expected persistence coordinator to stay nil until first async persist")
	}

	ids := handler.persistConversationMessages(conv.ID, true, memory.Message{
		Role:    "assistant",
		Content: "hello from lazy persistence",
	})
	if len(ids) != 1 || strings.TrimSpace(ids[0]) == "" {
		t.Fatalf("persistConversationMessages() ids = %v, want 1 non-empty id", ids)
	}
	if handler.persistCoordinator == nil {
		t.Fatal("expected first async persist to initialize persistence coordinator")
	}
	if handler.persistCoordinator.audit != auditStore {
		t.Fatal("expected persistence coordinator to inherit audit store when lazily initialized")
	}
	if handler.persistCoordinator.metrics != metrics {
		t.Fatal("expected persistence coordinator to inherit metrics recorder when lazily initialized")
	}
}

func TestPersistBestEffortMessageContent_PersistsSearchableMessageAudit(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "persist-searchable-audit")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	auditStore, err := sessionaudit.NewSQLiteStore(filepath.Join(dbDir, sessionaudit.DefaultDBFilename), sessionaudit.StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer auditStore.Close()

	handler := NewChatHandler(store, nil, nil)
	handler.SetPersistenceOptions(true, true, true)
	handler.SetSessionAuditStore(auditStore)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	userMessageID := handler.persistBestEffortMessageContent("", conv.ID, "user", "Router reboot failed after the update", "", "", nil, false)
	assistantMessageID := handler.persistBestEffortMessageContent("", conv.ID, "assistant", "Rollback plan: restore the previous firmware and verify networking.", "", "", nil, false)
	if strings.TrimSpace(userMessageID) == "" || strings.TrimSpace(assistantMessageID) == "" {
		t.Fatalf("persistBestEffortMessageContent() returned empty message ids")
	}

	handler.persistCoordinator.FlushConversation(conv.ID)

	entries, err := auditStore.Recent(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(Recent()) = %d, want 2", len(entries))
	}

	var (
		sawUser      bool
		sawAssistant bool
	)
	for _, entry := range entries {
		switch entry.EventType {
		case "user_message":
			sawUser = entry.Role == "user" && strings.Contains(entry.Payload, "Router reboot failed")
		case "assistant_message":
			sawAssistant = entry.Role == "assistant" && strings.Contains(entry.Payload, "Rollback plan")
		}
	}
	if !sawUser {
		t.Fatalf("user_message audit entry missing or malformed: %+v", entries)
	}
	if !sawAssistant {
		t.Fatalf("assistant_message audit entry missing or malformed: %+v", entries)
	}

	results, err := auditStore.SearchConversations(ctx, sessionaudit.SearchOptions{
		Query:                "router rollback",
		Limit:                5,
		PerConversationLimit: 5,
		SnippetLength:        120,
	})
	if err != nil {
		t.Fatalf("SearchConversations() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(SearchConversations()) = %d, want 1", len(results))
	}
	if results[0].ConversationID != conv.ID {
		t.Fatalf("ConversationID = %q, want %q", results[0].ConversationID, conv.ID)
	}
	if results[0].HitCount != 2 {
		t.Fatalf("HitCount = %d, want 2", results[0].HitCount)
	}
}

func TestPersistBestEffortMessageContent_PersistsSearchableMessageAuditWhenMessagePersistenceIsSync(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "persist-sync-audit")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	auditStore, err := sessionaudit.NewSQLiteStore(filepath.Join(dbDir, sessionaudit.DefaultDBFilename), sessionaudit.StoreConfig{
		RetentionDays:    30,
		CleanupInterval:  0,
		CleanupBatchSize: 50,
	})
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer auditStore.Close()

	handler := NewChatHandler(store, nil, nil)
	handler.SetPersistenceOptions(false, true, false)
	handler.SetSessionAuditStore(auditStore)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	if messageID := handler.persistBestEffortMessageContent("", conv.ID, "user", "Audit should still use the async queue in sync message mode", "", "", nil, false); strings.TrimSpace(messageID) == "" {
		t.Fatalf("persistBestEffortMessageContent() returned empty message id")
	}
	handler.persistCoordinator.FlushConversation(conv.ID)

	entries, err := auditStore.Recent(ctx, conv.ID, 10)
	if err != nil {
		t.Fatalf("Recent() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(Recent()) = %d, want 1", len(entries))
	}
	if entries[0].EventType != "user_message" || entries[0].Role != "user" {
		t.Fatalf("unexpected audit entry: %+v", entries[0])
	}
	if !strings.Contains(entries[0].Payload, "async queue") {
		t.Fatalf("payload = %q, want async queue marker", entries[0].Payload)
	}
}
