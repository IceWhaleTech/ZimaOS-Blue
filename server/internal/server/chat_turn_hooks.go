package server

import (
	"context"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// TurnContext captures the normalized chat-turn state shared by internal hooks.
type TurnContext struct {
	ConversationID       string
	UserMessage          string
	Model                string
	ContextPackSelection *contextpack.SelectionSet
	Source               MemoryRecallSource
	RecallMode           MemoryRecallMode
	IsRegenerate         bool
	UsesContinuation     bool
}

// TurnHook defines typed chat-turn lifecycle hooks.
type TurnHook interface {
	BeforeModelCall(ctx context.Context, turn TurnContext) ([]llm.Message, error)
	AfterAssistantPersisted(ctx context.Context, turn TurnContext, assistantMsg *memory.Message) error
}

// TurnHookManager dispatches internal typed turn hooks.
type TurnHookManager struct {
	mu    sync.RWMutex
	hooks []TurnHook
}

func NewTurnHookManager() *TurnHookManager {
	return &TurnHookManager{hooks: make([]TurnHook, 0, 1)}
}

func (m *TurnHookManager) Register(hook TurnHook) {
	if m == nil || hook == nil {
		return
	}
	m.mu.Lock()
	m.hooks = append(m.hooks, hook)
	m.mu.Unlock()
}

func (m *TurnHookManager) BeforeModelCall(ctx context.Context, turn TurnContext) []llm.Message {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	hooks := append([]TurnHook(nil), m.hooks...)
	m.mu.RUnlock()

	var out []llm.Message
	for _, hook := range hooks {
		msgs, err := hook.BeforeModelCall(ctx, turn)
		if err != nil {
			logger.Warn().Err(err).Str("conv_id", turn.ConversationID).Msg("[chat] before-model turn hook failed")
			continue
		}
		if len(msgs) > 0 {
			out = append(out, msgs...)
		}
	}
	return out
}

func (m *TurnHookManager) AfterAssistantPersisted(ctx context.Context, turn TurnContext, assistantMsg *memory.Message) {
	if m == nil || assistantMsg == nil {
		return
	}
	m.mu.RLock()
	hooks := append([]TurnHook(nil), m.hooks...)
	m.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook.AfterAssistantPersisted(ctx, turn, assistantMsg); err != nil {
			logger.Warn().Err(err).Str("conv_id", turn.ConversationID).Str("message_id", assistantMsg.ID).Msg("[chat] post-persist turn hook failed")
		}
	}
}

func (h *ChatHandler) RegisterTurnHook(hook TurnHook) {
	if h == nil {
		return
	}
	if h.turnHooks == nil {
		h.turnHooks = NewTurnHookManager()
	}
	h.turnHooks.Register(hook)
}

func (h *ChatHandler) beforeModelCallHooks(ctx context.Context, turn TurnContext) []llm.Message {
	if h == nil || h.turnHooks == nil {
		return nil
	}
	return h.turnHooks.BeforeModelCall(ctx, turn)
}

func (h *ChatHandler) afterAssistantPersistedHooks(turn TurnContext, assistantMsg *memory.Message) {
	if h == nil || assistantMsg == nil {
		return
	}
	captured := *assistantMsg
	h.scheduleNextTurnWarmup(captured.ConversationID)
	if h.turnHooks == nil {
		return
	}
	h.queueEvent(func() {
		h.turnHooks.AfterAssistantPersisted(context.Background(), turn, &captured)
	})
}

// MemoryTurnHook aligns chat memory behavior with pre/post turn lifecycle hooks.
type MemoryTurnHook struct {
	handler *ChatHandler
	seenMu  sync.Mutex
	seenIDs map[string]struct{}
}

func NewMemoryTurnHook(handler *ChatHandler) *MemoryTurnHook {
	return &MemoryTurnHook{
		handler: handler,
		seenIDs: make(map[string]struct{}),
	}
}

func (h *MemoryTurnHook) BeforeModelCall(ctx context.Context, turn TurnContext) ([]llm.Message, error) {
	if h == nil || h.handler == nil {
		return nil, nil
	}
	recallMode := parseMemoryRecallMode(string(turn.RecallMode))
	shouldRecall, reason := memoryRecallDecision(turn.UserMessage, turn.UsesContinuation, turn.IsRegenerate, recallMode)
	h.handler.memoryRecallStats.RecordWithSource(shouldRecall, reason, turn.Source)
	if !shouldRecall {
		return nil, nil
	}
	memoryCtx := h.handler.recallMemories(ctx, turn.UserMessage, recallMode)
	if strings.TrimSpace(memoryCtx) == "" {
		return nil, nil
	}
	h.handler.memoryRecallStats.RecordInjectionWithSource(estimateTokens(memoryCtx), turn.Source)
	return []llm.Message{{Role: llm.RoleSystem, Content: memoryCtx}}, nil
}

func (h *MemoryTurnHook) AfterAssistantPersisted(_ context.Context, turn TurnContext, assistantMsg *memory.Message) error {
	if h == nil || h.handler == nil || assistantMsg == nil {
		return nil
	}
	messageID := strings.TrimSpace(assistantMsg.ID)
	if messageID == "" || !h.markSeen(messageID) {
		return nil
	}
	h.handler.extractMemoryAfterTurn(turn.ConversationID, memorySourceTag(turn.Source), turn.Model)
	return nil
}

func (h *MemoryTurnHook) markSeen(messageID string) bool {
	h.seenMu.Lock()
	defer h.seenMu.Unlock()
	if _, exists := h.seenIDs[messageID]; exists {
		return false
	}
	h.seenIDs[messageID] = struct{}{}
	if len(h.seenIDs) > 4096 {
		for key := range h.seenIDs {
			if key == messageID {
				continue
			}
			delete(h.seenIDs, key)
			break
		}
	}
	return true
}

func memorySourceTag(source MemoryRecallSource) string {
	switch source {
	case MemoryRecallSourceIM:
		return "im"
	default:
		return "web"
	}
}
