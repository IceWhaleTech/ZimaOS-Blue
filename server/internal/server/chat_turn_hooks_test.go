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

func memoryContextContent(messages []llm.Message) string {
	for _, msg := range messages {
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "<memory_context>") {
			return msg.Content
		}
	}
	return ""
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

func TestSendMessage_LatestDocsSkipsMemoryRecall(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "latest docs skip memory")
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
	if _, err := baseSvc.Remember(context.Background(), "This documentation belongs to Cursor, not ZimaOS.", []string{"longterm", "docs"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"搜索 OpenAI Responses API 的最新文档。","provider":"capture","model":"capture-model"}`))
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
	if containsMemoryContext(captureProvider.LastRequest().Messages) {
		t.Fatalf("expected latest docs request to skip memory context, got %+v", captureProvider.LastRequest().Messages)
	}
}

func TestSendMessage_GenericPromptSkipsSessionCompactionMemory(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "generic skip compaction")
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
	if _, err := baseSvc.Remember(context.Background(), "Earlier session summary that may be stale.", []string{"session-compaction", "session:test"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"Explain Rust borrowing in simple terms.","provider":"capture","model":"capture-model"}`))
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
	if containsMemoryContext(captureProvider.LastRequest().Messages) {
		t.Fatalf("expected generic prompt to skip session-compaction memory, got %+v", captureProvider.LastRequest().Messages)
	}
}

func TestSendMessage_RetrospectiveWeekPromptIncludesCompressedHistoryAndSessionMemory(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "weekly retrospective memory")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	captureProvider := &requestCaptureProvider{}
	registry := llm.NewProviderRegistry()
	registry.Register(captureProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	defer handler.Close()
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "capture-model",
		ContextWindow: 768,
	}}))

	prompt := "梳理下我过去一周具体写了什么。"
	buildSeedMessages := func(repeat int) []memory.Message {
		return []memory.Message{
			{Role: "user", Content: strings.Repeat("这周我在 server/internal/server/chat.go 里处理 provider fallback 和记忆注入。 ", repeat)},
			{Role: "assistant", Content: strings.Repeat("我记录了 chat.go、chat_context.go 和 runner.go 的修改点。 ", repeat)},
			{Role: "user", Content: strings.Repeat("另外还整理了 harness 和端到端测试计划。 ", repeat)},
			{Role: "assistant", Content: strings.Repeat("好的，我会保留这些具体文件和测试场景。 ", repeat)},
			{Role: "user", Content: "继续保留这些修改上下文"},
			{Role: "assistant", Content: "已记录最近的变更脉络"},
			{Role: "user", Content: "等会儿帮我回顾"},
			{Role: "assistant", Content: "没问题"},
		}
	}

	var seed []memory.Message
	for repeat := 4; repeat <= 32; repeat++ {
		candidate := buildSeedMessages(repeat)
		candidateWithPrompt := append(append([]memory.Message{}, candidate...), memory.Message{Role: "user", Content: prompt})
		budget := handler.measurePreparedInputBudget("capture-model", 64, removeOrphanedToolResults(convertToLLMMessages(candidateWithPrompt)))
		if budget.ContextUsageRatio() >= smartContextSoftCompressionThreshold {
			seed = candidate
			break
		}
	}
	if len(seed) == 0 {
		t.Fatal("failed to build a long enough conversation fixture for compressed history")
	}
	for i, msg := range seed {
		if _, err := store.AddMessage(context.Background(), conv.ID, msg); err != nil {
			t.Fatalf("AddMessage seed %d: %v", i, err)
		}
	}
	handler.summaryCache.Put(conv.ID, &ConversationSummary{
		Text:         "Goal\n- 回顾最近一周写过的内容\n\nAccomplished\n- 完成长会话压缩、记忆召回和 provider fallback 测试",
		MessageCount: len(seed) + 1,
	})

	layered, baseSvc := newLayeredMemoryServiceForTest(t)
	handler.SetLayeredMemory(layered)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "上周主要写了 server/internal/server/chat.go 的记忆注入逻辑，以及 server/internal/server/chat_context.go 的长会话压缩。",
				Metadata: map[string]string{"tag_0": "session-compaction", "tag_1": "session:weekly-review"},
			},
			Score: 0.94,
		}},
	}
	baseSvc.SetBackend(backend)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"`+prompt+`","provider":"capture","model":"capture-model","max_tokens":64}`))
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
	if !backend.called {
		t.Fatal("expected provider send path to recall memories for retrospective week prompt")
	}

	lastReq := captureProvider.LastRequest()
	if !containsMemoryContext(lastReq.Messages) {
		t.Fatalf("expected provider request to include memory context, got %+v", lastReq.Messages)
	}
	if got := memoryContextContent(lastReq.Messages); !strings.Contains(got, "source=session_compaction") {
		t.Fatalf("memory context = %q, want session-compaction source", got)
	}

	foundAnchor := false
	foundHistorySummary := false
	for _, msg := range lastReq.Messages {
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Current-turn anchor") && strings.Contains(msg.Content, prompt) {
			foundAnchor = true
		}
		if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, historicalContextBackgroundPrefix) && strings.Contains(msg.Content, "回顾最近一周写过的内容") {
			foundHistorySummary = true
		}
	}
	if !foundAnchor {
		t.Fatalf("expected compressed history anchor in provider request, got %+v", lastReq.Messages)
	}
	if !foundHistorySummary {
		t.Fatalf("expected compressed history summary in provider request, got %+v", lastReq.Messages)
	}
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
