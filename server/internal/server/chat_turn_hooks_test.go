package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type recordingMemoryRefresher struct {
	mu    sync.Mutex
	calls []string
	ch    chan struct{}
}

func newRecordingMemoryRefresher() *recordingMemoryRefresher {
	return &recordingMemoryRefresher{ch: make(chan struct{}, 8)}
}

func (r *recordingMemoryRefresher) RefreshMemory(_ context.Context, extracted string, _ string) error {
	r.mu.Lock()
	r.calls = append(r.calls, extracted)
	r.mu.Unlock()
	select {
	case r.ch <- struct{}{}:
	default:
	}
	return nil
}

func (r *recordingMemoryRefresher) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func waitForRefreshCount(t *testing.T, refresher *recordingMemoryRefresher, want int) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for refresher.Count() < want {
		select {
		case <-refresher.ch:
		case <-deadline:
			t.Fatalf("memory refresh count = %d, want >= %d", refresher.Count(), want)
		}
	}
}

func ensureNoRefresh(t *testing.T, refresher *recordingMemoryRefresher, wait time.Duration) {
	t.Helper()
	select {
	case <-refresher.ch:
		t.Fatalf("unexpected memory refresh; count=%d", refresher.Count())
	case <-time.After(wait):
	}
}

func newLayeredMemoryServiceForTest(t *testing.T) (*memory.LayeredMemoryService, *memory.UnifiedMemoryService) {
	t.Helper()
	memDir := t.TempDir()
	mdBackend, err := memory.NewPureMarkdownBackend(memDir)
	if err != nil {
		t.Fatalf("create markdown backend: %v", err)
	}
	baseSvc := memory.NewUnifiedMemoryService(mdBackend)
	layeredSvc, err := memory.NewLayeredMemoryService(baseSvc, memory.LayeredMemoryConfig{
		BaseDir:     memDir,
		LongTermDir: memDir,
	})
	if err != nil {
		t.Fatalf("create layered memory service: %v", err)
	}
	return layeredSvc, baseSvc
}

func newMemoryCompactorForTest(refresher *recordingMemoryRefresher) *session.CompactorMemoryIntegration {
	extractor := &scriptedChatProvider{responses: []llm.ChatResponse{{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "- User prefers concise timelines."},
	}}}
	cfg := session.DefaultMemoryRefreshConfig()
	cfg.Enabled = true
	return session.NewCompactorMemoryIntegration(nil, refresher, extractor, cfg)
}

func containsMemoryContext(messages []llm.Message) bool {
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "<memory_context>") {
			return true
		}
	}
	return false
}

func TestSendMessage_AutomaticMemoryRecallAndPostTurnSave(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "memory send")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "Earlier context"}); err != nil {
		t.Fatalf("AddMessage seed: %v", err)
	}

	captureProvider := &requestCaptureProvider{}
	registry := llm.NewProviderRegistry()
	registry.Register(captureProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	defer handler.Close()
	layered, baseSvc := newLayeredMemoryServiceForTest(t)
	handler.SetLayeredMemory(layered)
	if _, err := baseSvc.Remember(context.Background(), "Alpha project timeline April 2026", []string{"project"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	refresher := newRecordingMemoryRefresher()
	handler.SetCompactorMemoryIntegration(newMemoryCompactorForTest(refresher), 8000)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"Alpha project timeline April 2026","provider":"capture","model":"capture-model"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !containsMemoryContext(captureProvider.LastRequest().Messages) {
		t.Fatalf("expected memory context in send request, got %+v", captureProvider.LastRequest().Messages)
	}
	waitForRefreshCount(t, refresher, 1)
}

func TestStreamMessage_AutomaticMemoryRecallAndPostTurnSave(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "memory stream")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "Earlier context"}); err != nil {
		t.Fatalf("AddMessage seed: %v", err)
	}

	captureProvider := &requestCaptureProvider{}
	registry := llm.NewProviderRegistry()
	registry.Register(captureProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	defer handler.Close()
	layered, baseSvc := newLayeredMemoryServiceForTest(t)
	handler.SetLayeredMemory(layered)
	if _, err := baseSvc.Remember(context.Background(), "Beta deployment two release gates", []string{"deployment"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	refresher := newRecordingMemoryRefresher()
	handler.SetCompactorMemoryIntegration(newMemoryCompactorForTest(refresher), 8000)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/stream", bytes.NewBufferString(`{"message":"Beta deployment two release gates","provider":"capture","model":"capture-model"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if !containsMemoryContext(captureProvider.LastRequest().Messages) {
		t.Fatalf("expected memory context in stream request, got %+v", captureProvider.LastRequest().Messages)
	}
	waitForRefreshCount(t, refresher, 1)
}

func TestProcessChannelMessage_FinalReplyTriggersPostTurnSaveOnly(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	captureProvider := &requestCaptureProvider{}
	registry := llm.NewProviderRegistry()
	registry.Register(captureProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	defer handler.Close()
	handler.SetIMModel("capture-model")
	refresher := newRecordingMemoryRefresher()
	handler.SetCompactorMemoryIntegration(newMemoryCompactorForTest(refresher), 8000)

	convID := channelConversationID("slack", "chat-1")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "slack chat"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), convID, memory.Message{Role: "assistant", Content: "Earlier context"}); err != nil {
		t.Fatalf("AddMessage seed: %v", err)
	}

	handler.persistChannelResponse(context.Background(), convID, "Pending browser confirmation")
	ensureNoRefresh(t, refresher, 200*time.Millisecond)

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "slack",
		ChatID:      "chat-1",
		ID:          "msg-1",
		Username:    "orca",
		Content:     "Please continue",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage: %v", err)
	}
	if strings.TrimSpace(resp) == "" {
		t.Fatal("expected non-empty IM response")
	}
	waitForRefreshCount(t, refresher, 1)
}

func TestExtractMemoryAfterTurnUsesResolvedModelSessionBudget(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "memory budget")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	totalEstimatedTokens := 0
	for i := 0; i < 12; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		content := "marker-" + strconv.Itoa(i) + " " + strings.Repeat("context ", 700)
		totalEstimatedTokens += estimateTokens(content)
		if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{Role: role, Content: content}); err != nil {
			t.Fatalf("AddMessage seed %d: %v", i, err)
		}
	}
	if totalEstimatedTokens <= session.LegacyDefaultContextTokenBudget {
		t.Fatalf("seeded conversation estimate = %d, want > %d", totalEstimatedTokens, session.LegacyDefaultContextTokenBudget)
	}

	extractor := &scriptedChatProvider{responses: []llm.ChatResponse{{
		Message: llm.Message{Role: llm.RoleAssistant, Content: "NO_MEMORY_NEEDED"},
	}}}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetProviderPool(newProviderPoolWithContextWindowModel(t, "large-context-model", 128000))
	cfg := session.DefaultMemoryRefreshConfig()
	cfg.Enabled = true
	handler.SetCompactorMemoryIntegration(session.NewCompactorMemoryIntegration(nil, newRecordingMemoryRefresher(), extractor, cfg), session.LegacyDefaultContextTokenBudget)

	if !handler.extractMemoryAfterTurn(conv.ID, "web", "large-context-model") {
		t.Fatal("expected extractMemoryAfterTurn to run")
	}

	req, ok := extractor.RequestAt(0)
	if !ok {
		t.Fatal("expected extraction request")
	}
	if len(req.Messages) < 2 {
		t.Fatalf("request messages = %d, want >= 2", len(req.Messages))
	}
	if !strings.Contains(req.Messages[1].Content, "marker-0") {
		t.Fatalf("expected earliest message to survive model-aware session budget; content=%q", req.Messages[1].Content)
	}
}
