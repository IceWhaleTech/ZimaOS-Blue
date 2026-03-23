package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/google/uuid"
)

const (
	chatPersistQueueDepth   = 4096
	chatPersistFlushEvery   = 25 * time.Millisecond
	chatPersistMaxBatchSize = 64
	chatPersistRetryEvery   = 30 * time.Second
)

type persistenceOpKind string

const (
	persistenceOpPreviousResponse persistenceOpKind = "previous_response_id"
	persistenceOpMessageUpdate    persistenceOpKind = "message_update"
	persistenceOpAudit            persistenceOpKind = "audit"
	persistenceOpFlush            persistenceOpKind = "flush"
	persistenceOpShutdown         persistenceOpKind = "shutdown"
)

type persistenceOp struct {
	kind           persistenceOpKind
	conversationID string
	message        memory.Message
	responseID     string
	auditEntry     sessionaudit.Entry
	ack            chan struct{}
}

type pendingPersistence struct {
	previousResponse map[string]persistenceOp
	messages         map[string]persistenceOp
	audit            []persistenceOp
}

func newPendingPersistence() pendingPersistence {
	return pendingPersistence{
		previousResponse: make(map[string]persistenceOp),
		messages:         make(map[string]persistenceOp),
	}
}

func (p pendingPersistence) count() int {
	return len(p.previousResponse) + len(p.messages) + len(p.audit)
}

type PersistenceCoordinator struct {
	store *memory.Store
	audit *sessionaudit.Store
	queue chan persistenceOp
	done  chan struct{}
	once  sync.Once

	mu            sync.RWMutex
	metrics       runtimeCounterRecorder
	storeDegraded bool
	auditDegraded bool
}

func NewPersistenceCoordinator(store *memory.Store, audit *sessionaudit.Store, metrics runtimeCounterRecorder) *PersistenceCoordinator {
	pc := &PersistenceCoordinator{
		store:   store,
		audit:   audit,
		queue:   make(chan persistenceOp, chatPersistQueueDepth),
		done:    make(chan struct{}),
		metrics: metrics,
	}
	go pc.run()
	return pc
}

func (p *PersistenceCoordinator) SetMetrics(metrics runtimeCounterRecorder) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.metrics = metrics
	p.mu.Unlock()
}

func (p *PersistenceCoordinator) SetAuditStore(store *sessionaudit.Store) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.audit = store
	p.auditDegraded = false
	p.mu.Unlock()
}

func (p *PersistenceCoordinator) EnqueuePreviousResponseID(conversationID, responseID string) {
	p.enqueue(persistenceOp{
		kind:           persistenceOpPreviousResponse,
		conversationID: strings.TrimSpace(conversationID),
		responseID:     strings.TrimSpace(responseID),
	})
}

func (p *PersistenceCoordinator) EnqueueMessageUpdate(msg memory.Message) {
	if strings.TrimSpace(msg.ID) == "" || strings.TrimSpace(msg.ConversationID) == "" {
		return
	}
	p.enqueue(persistenceOp{
		kind:    persistenceOpMessageUpdate,
		message: msg,
	})
}

func (p *PersistenceCoordinator) EnqueueAudit(entry sessionaudit.Entry) {
	p.enqueue(persistenceOp{
		kind:       persistenceOpAudit,
		auditEntry: entry,
	})
}

func (p *PersistenceCoordinator) FlushConversation(_ string) {
	p.flush()
}

func (p *PersistenceCoordinator) FlushMessage(_ string) {
	p.flush()
}

func (p *PersistenceCoordinator) ShutdownFlush() {
	if p == nil {
		return
	}
	ack := make(chan struct{})
	p.enqueueBlocking(persistenceOp{kind: persistenceOpShutdown, ack: ack})
	<-ack
	p.once.Do(func() {
		close(p.done)
	})
}

func (p *PersistenceCoordinator) flush() {
	if p == nil {
		return
	}
	ack := make(chan struct{})
	p.enqueueBlocking(persistenceOp{kind: persistenceOpFlush, ack: ack})
	<-ack
}

func (p *PersistenceCoordinator) enqueue(op persistenceOp) {
	if p == nil {
		return
	}
	select {
	case p.queue <- op:
		p.recordCounterValue("chat_persist_queue_depth", int64(len(p.queue)), nil)
	default:
		p.recordCounterValue("chat_db_dropped_ops_total", 1, map[string]string{"reason": "queue_full"})
		logger.Warn().Str("kind", string(op.kind)).Msg("[chat] persistence queue full; dropped best-effort op")
	}
}

func (p *PersistenceCoordinator) enqueueBlocking(op persistenceOp) {
	if p == nil {
		if op.ack != nil {
			close(op.ack)
		}
		return
	}
	p.queue <- op
	p.recordCounterValue("chat_persist_queue_depth", int64(len(p.queue)), nil)
}

func (p *PersistenceCoordinator) run() {
	ticker := time.NewTicker(chatPersistFlushEvery)
	retryTicker := time.NewTicker(chatPersistRetryEvery)
	defer ticker.Stop()
	defer retryTicker.Stop()

	pending := newPendingPersistence()
	for {
		select {
		case op := <-p.queue:
			switch op.kind {
			case persistenceOpPreviousResponse:
				if strings.TrimSpace(op.conversationID) != "" && strings.TrimSpace(op.responseID) != "" {
					pending.previousResponse[op.conversationID] = op
				}
			case persistenceOpMessageUpdate:
				if strings.TrimSpace(op.message.ID) != "" {
					pending.messages[op.message.ID] = op
				}
			case persistenceOpAudit:
				pending.audit = append(pending.audit, op)
			case persistenceOpFlush:
				p.flushAll(&pending)
				close(op.ack)
			case persistenceOpShutdown:
				p.flushAll(&pending)
				close(op.ack)
				return
			}
			if pending.count() >= chatPersistMaxBatchSize {
				p.flushOnce(&pending)
			}
		case <-ticker.C:
			p.flushOnce(&pending)
		case <-retryTicker.C:
			p.retryDegraded()
		}
	}
}

func (p *PersistenceCoordinator) flushAll(pending *pendingPersistence) {
	for pending != nil && pending.count() > 0 {
		p.flushOnce(pending)
	}
}

func (p *PersistenceCoordinator) flushOnce(pending *pendingPersistence) {
	if p == nil || pending == nil || pending.count() == 0 {
		return
	}

	start := time.Now()
	batch := takePendingBatch(pending, chatPersistMaxBatchSize)
	flushed := 0

	for _, op := range batch.previousResponse {
		if p.isStoreDegraded() {
			p.recordCounterValue("chat_db_dropped_ops_total", 1, map[string]string{"kind": string(op.kind), "db": "chat"})
			continue
		}
		if err := p.store.SetConversationPreviousResponseID(context.Background(), op.conversationID, op.responseID); err != nil {
			if !p.handleStoreError(err, 1) || p.store.SetConversationPreviousResponseID(context.Background(), op.conversationID, op.responseID) != nil {
				p.recordCounterValue("chat_db_dropped_ops_total", 1, map[string]string{"kind": string(op.kind), "db": "chat"})
				continue
			}
		}
		flushed++
	}

	for _, op := range batch.messages {
		if p.isStoreDegraded() {
			p.recordCounterValue("chat_db_dropped_ops_total", 1, map[string]string{"kind": string(op.kind), "db": "chat"})
			continue
		}
		if err := p.store.UpsertMessageContentFullTrusted(context.Background(), op.message); err != nil {
			if !p.handleStoreError(err, 1) || p.store.UpsertMessageContentFullTrusted(context.Background(), op.message) != nil {
				p.recordCounterValue("chat_db_dropped_ops_total", 1, map[string]string{"kind": string(op.kind), "db": "chat"})
				continue
			}
		}
		flushed++
	}

	if len(batch.audit) > 0 {
		if p.isAuditDegraded() || p.currentAuditStore() == nil {
			p.recordCounterValue("chat_db_dropped_ops_total", int64(len(batch.audit)), map[string]string{"kind": string(persistenceOpAudit), "db": "audit"})
		} else {
			entries := make([]sessionaudit.Entry, 0, len(batch.audit))
			for _, op := range batch.audit {
				entries = append(entries, op.auditEntry)
			}
			if err := p.currentAuditStore().RecordBatch(context.Background(), entries); err != nil {
				if !p.handleAuditError(err, len(entries)) || p.currentAuditStore() == nil || p.currentAuditStore().RecordBatch(context.Background(), entries) != nil {
					p.recordCounterValue("chat_db_dropped_ops_total", int64(len(entries)), map[string]string{"kind": string(persistenceOpAudit), "db": "audit"})
				} else {
					flushed += len(entries)
				}
			} else {
				flushed += len(entries)
			}
		}
	}

	p.recordCounterValue("chat_persist_flush_total", 1, nil)
	p.recordCounterValue("chat_persist_batch_size", int64(flushed), nil)
	p.recordCounterValue("chat_persist_flush_duration_ms", time.Since(start).Milliseconds(), nil)
}

type pendingBatch struct {
	previousResponse []persistenceOp
	messages         []persistenceOp
	audit            []persistenceOp
}

func takePendingBatch(pending *pendingPersistence, max int) pendingBatch {
	batch := pendingBatch{
		previousResponse: make([]persistenceOp, 0, max),
		messages:         make([]persistenceOp, 0, max),
		audit:            make([]persistenceOp, 0, max),
	}
	if pending == nil || max <= 0 {
		return batch
	}

	remaining := max
	for key, op := range pending.previousResponse {
		if remaining == 0 {
			return batch
		}
		batch.previousResponse = append(batch.previousResponse, op)
		delete(pending.previousResponse, key)
		remaining--
	}
	for key, op := range pending.messages {
		if remaining == 0 {
			return batch
		}
		batch.messages = append(batch.messages, op)
		delete(pending.messages, key)
		remaining--
	}
	if remaining > 0 && len(pending.audit) > 0 {
		if remaining > len(pending.audit) {
			remaining = len(pending.audit)
		}
		batch.audit = append(batch.audit, pending.audit[:remaining]...)
		pending.audit = pending.audit[remaining:]
	}
	return batch
}

func (p *PersistenceCoordinator) handleStoreError(err error, batchSize int) bool {
	if err == nil {
		return false
	}
	if dbutil.IsSQLiteCorruptionError(err) || strings.Contains(strings.ToLower(err.Error()), "recover failed") {
		p.recordCounterValue("chat_db_recovery_total", 1, map[string]string{"db": "chat"})
		if recoverErr := p.store.Recover(); recoverErr == nil {
			p.clearStoreDegraded()
			p.recordCounterValue("chat_db_replayed_ops_total", int64(batchSize), map[string]string{"db": "chat"})
			return true
		}
		p.setStoreDegraded()
	}
	logger.Warn().Err(err).Msg("[chat] persistence flush failed")
	return false
}

func (p *PersistenceCoordinator) handleAuditError(err error, batchSize int) bool {
	if err == nil {
		return false
	}
	if dbutil.IsSQLiteCorruptionError(err) || strings.Contains(strings.ToLower(err.Error()), "recover failed") {
		p.recordCounterValue("chat_db_recovery_total", 1, map[string]string{"db": "audit"})
		if audit := p.currentAuditStore(); audit != nil {
			if recoverErr := audit.Recover(); recoverErr == nil {
				p.clearAuditDegraded()
				p.recordCounterValue("chat_db_replayed_ops_total", int64(batchSize), map[string]string{"db": "audit"})
				return true
			}
		}
		p.setAuditDegraded()
	}
	logger.Warn().Err(err).Msg("[chat] session audit flush failed")
	return false
}

func (p *PersistenceCoordinator) retryDegraded() {
	if p == nil {
		return
	}
	if p.isStoreDegraded() && p.store != nil {
		if err := p.store.Recover(); err == nil {
			p.clearStoreDegraded()
		}
	}
	if p.isAuditDegraded() {
		if audit := p.currentAuditStore(); audit != nil {
			if err := audit.Recover(); err == nil {
				p.clearAuditDegraded()
			}
		}
	}
}

func (p *PersistenceCoordinator) currentAuditStore() *sessionaudit.Store {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.audit
}

func (p *PersistenceCoordinator) isStoreDegraded() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.storeDegraded
}

func (p *PersistenceCoordinator) isAuditDegraded() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.auditDegraded
}

func (p *PersistenceCoordinator) setStoreDegraded() {
	p.mu.Lock()
	if p.storeDegraded {
		p.mu.Unlock()
		return
	}
	p.storeDegraded = true
	p.mu.Unlock()
	p.recordCounterValue("chat_db_degraded", 1, map[string]string{"db": "chat", "state": "enter"})
}

func (p *PersistenceCoordinator) clearStoreDegraded() {
	p.mu.Lock()
	if !p.storeDegraded {
		p.mu.Unlock()
		return
	}
	p.storeDegraded = false
	p.mu.Unlock()
	p.recordCounterValue("chat_db_degraded", 1, map[string]string{"db": "chat", "state": "exit"})
}

func (p *PersistenceCoordinator) setAuditDegraded() {
	p.mu.Lock()
	if p.auditDegraded {
		p.mu.Unlock()
		return
	}
	p.auditDegraded = true
	p.mu.Unlock()
	p.recordCounterValue("chat_db_degraded", 1, map[string]string{"db": "audit", "state": "enter"})
}

func (p *PersistenceCoordinator) clearAuditDegraded() {
	p.mu.Lock()
	if !p.auditDegraded {
		p.mu.Unlock()
		return
	}
	p.auditDegraded = false
	p.mu.Unlock()
	p.recordCounterValue("chat_db_degraded", 1, map[string]string{"db": "audit", "state": "exit"})
}

func (p *PersistenceCoordinator) recordCounterValue(name string, value int64, tags map[string]string) {
	if p == nil || value == 0 {
		return
	}
	p.mu.RLock()
	recorder := p.metrics
	p.mu.RUnlock()
	if recorder == nil {
		return
	}
	if tags == nil {
		tags = map[string]string{}
	}
	if _, ok := tags["route_kind"]; !ok {
		tags["route_kind"] = "chat"
	}
	recorder.RecordCounter(name, value, tags)
}

func (h *ChatHandler) ensurePersistenceCoordinator() {
	if h == nil || !h.chatPersistAsync || h.store == nil {
		return
	}
	if h.persistCoordinator != nil {
		h.persistCoordinator.SetAuditStore(h.sessionAuditStore)
		if metrics, ok := h.metricsRecorder.(runtimeCounterRecorder); ok {
			h.persistCoordinator.SetMetrics(metrics)
		}
		return
	}
	var metrics runtimeCounterRecorder
	if recorder, ok := h.metricsRecorder.(runtimeCounterRecorder); ok {
		metrics = recorder
	}
	h.persistCoordinator = NewPersistenceCoordinator(h.store, h.sessionAuditStore, metrics)
}

func (h *ChatHandler) persistAsyncMessage(msg memory.Message, forceFlush bool) string {
	if h == nil {
		return ""
	}
	h.ensurePersistenceCoordinator()
	if strings.TrimSpace(msg.ID) == "" {
		msg.ID = generateMessageID()
	}
	if h.chatPersistAsync && h.persistCoordinator != nil {
		h.persistCoordinator.EnqueueMessageUpdate(msg)
		if forceFlush {
			h.persistCoordinator.FlushMessage(msg.ID)
		}
		return msg.ID
	}

	if strings.TrimSpace(msg.ConversationID) == "" {
		return ""
	}
	if err := h.store.UpsertMessageContentFullTrusted(context.Background(), msg); err != nil {
		logger.Warn().Err(err).Str("message_id", msg.ID).Msg("[chat] failed to persist assistant message")
		return ""
	}
	return msg.ID
}

func (h *ChatHandler) persistBestEffortMessageContent(messageID, conversationID, role, content, provider, model string, stats *memory.MessageStats, forceFlush bool) string {
	if h == nil || h.store == nil || strings.TrimSpace(conversationID) == "" {
		return ""
	}
	if strings.TrimSpace(messageID) == "" {
		messageID = generateMessageID()
	}
	msg := memory.Message{
		ID:             messageID,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		Provider:       provider,
		Model:          model,
		Stats:          stats,
	}
	return h.persistAsyncMessage(msg, forceFlush)
}

func (h *ChatHandler) updateMessageBestEffort(messageID, conversationID, role, content, provider, model string, stats *memory.MessageStats) {
	if h == nil || h.store == nil || strings.TrimSpace(messageID) == "" {
		return
	}
	h.ensurePersistenceCoordinator()
	if h.chatPersistAsync && h.persistCoordinator != nil {
		h.persistCoordinator.EnqueueMessageUpdate(memory.Message{
			ID:             messageID,
			ConversationID: conversationID,
			Role:           role,
			Content:        content,
			Provider:       provider,
			Model:          model,
			Stats:          stats,
		})
		return
	}
	if err := h.store.UpdateMessageContentFull(context.Background(), messageID, content, provider, model, stats); err != nil {
		logger.Warn().Err(err).Str("message_id", messageID).Msg("[chat] failed to update persisted message")
	}
}

func (h *ChatHandler) flushPersistedMessage(messageID string) {
	if h == nil || !h.chatPersistAsync || h.persistCoordinator == nil || strings.TrimSpace(messageID) == "" {
		return
	}
	h.persistCoordinator.FlushMessage(messageID)
}

func (h *ChatHandler) persistAuditEntry(entry sessionaudit.Entry) {
	if h == nil || h.sessionAuditStore == nil {
		return
	}
	h.ensurePersistenceCoordinator()
	if h.chatPersistAsync && h.persistCoordinator != nil {
		h.persistCoordinator.EnqueueAudit(entry)
		return
	}
	auditCtx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	if err := h.sessionAuditStore.Record(auditCtx, entry); err != nil {
		logger.Warn().
			Err(err).
			Str("conv_id", entry.ConversationID).
			Str("event", entry.EventType).
			Str("tool", entry.ToolName).
			Msg("failed to persist tool payload audit log")
	}
}

func (h *ChatHandler) getRecentMessagesForContext(ctx context.Context, conversationID string, limit int) ([]memory.Message, error) {
	if h == nil || h.store == nil {
		return nil, fmt.Errorf("memory store is not initialized")
	}
	if h.chatReadLite {
		return h.store.GetRecentMessagesLite(ctx, conversationID, limit)
	}
	return h.store.GetRecentMessages(ctx, conversationID, limit)
}

func (h *ChatHandler) queuePreviousResponseID(conversationID, responseID string) {
	if h == nil || h.store == nil {
		return
	}
	h.ensurePersistenceCoordinator()
	if h.chatPersistAsync && h.persistCoordinator != nil {
		h.persistCoordinator.EnqueuePreviousResponseID(conversationID, responseID)
		return
	}
	if err := h.store.SetConversationPreviousResponseID(context.Background(), conversationID, responseID); err != nil {
		logger.Warn().Err(err).Str("conv_id", conversationID).Msg("[chat] failed to persist previous_response_id")
	}
}

func generateMessageID() string {
	return uuid.NewString()
}
