package server

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/inject"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type reminderToolCallProvider struct {
	name      string
	model     string
	message   string
	timeArg   string
	finalText string

	mu    sync.Mutex
	calls int
}

func newReminderToolCallProvider(name, model, message, timeArg string) *reminderToolCallProvider {
	return &reminderToolCallProvider{
		name:      name,
		model:     model,
		message:   message,
		timeArg:   timeArg,
		finalText: "Reminder has been set.",
	}
}

func (p *reminderToolCallProvider) Name() string {
	return p.name
}

func (p *reminderToolCallProvider) Models() []string {
	return []string{p.model}
}

func (p *reminderToolCallProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{
		ID:    "reminder-mock-chat",
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: p.finalText,
		},
	}, nil
}

func (p *reminderToolCallProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 8)
	go func() {
		defer close(ch)
		_ = p.ChatStreamCallback(ctx, req, func(chunk llm.StreamChunk) error {
			ch <- chunk
			return nil
		})
	}()
	return ch, nil
}

func (p *reminderToolCallProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, cb llm.StreamCallback) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	p.mu.Lock()
	p.calls++
	callNum := p.calls
	p.mu.Unlock()

	if callNum == 1 {
		args := fmt.Sprintf(`{"action":"add","message":"%s","time":"%s"}`, p.message, p.timeArg)
		if err := cb(llm.StreamChunk{
			ID:    "reminder-mock-call-1",
			Model: req.Model,
			ToolCalls: []llm.ToolCall{
				{
					ID:        "tool_reminder_add_1",
					Name:      "reminder",
					Arguments: args,
				},
			},
		}); err != nil {
			return err
		}
		return cb(llm.StreamChunk{
			ID:    "reminder-mock-call-1",
			Model: req.Model,
			Done:  true,
		})
	}

	if err := cb(llm.StreamChunk{
		ID:    "reminder-mock-call-2",
		Model: req.Model,
		Delta: p.finalText,
	}); err != nil {
		return err
	}
	return cb(llm.StreamChunk{
		ID:    "reminder-mock-call-2",
		Model: req.Model,
		Done:  true,
	})
}

type reminderCapturedEvent struct {
	UserID    string
	EventType string
	Data      any
}

type reminderEventPublisher struct {
	mu     sync.Mutex
	events []reminderCapturedEvent
}

func (p *reminderEventPublisher) Publish(userID string, eventType string, data any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, reminderCapturedEvent{
		UserID:    userID,
		EventType: eventType,
		Data:      data,
	})
}

func (p *reminderEventPublisher) snapshot() []reminderCapturedEvent {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]reminderCapturedEvent, len(p.events))
	copy(out, p.events)
	return out
}

type reminderWebPushCall struct {
	UserID string
	Title  string
	Body   string
}

type reminderWebPushSender struct {
	mu    sync.Mutex
	calls []reminderWebPushCall
}

func (s *reminderWebPushSender) SendToUser(_ context.Context, userID, title, body string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, reminderWebPushCall{
		UserID: userID,
		Title:  title,
		Body:   body,
	})
	return nil
}

func (s *reminderWebPushSender) snapshot() []reminderWebPushCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]reminderWebPushCall, len(s.calls))
	copy(out, s.calls)
	return out
}

type reminderE2EFixture struct {
	store     *memory.Store
	pushSvc   *push.Service
	pushStore *push.Store
	events    *reminderEventPublisher
	webpush   *reminderWebPushSender
	handler   *ChatHandler
	convID    string
}

func newReminderE2EFixture(t *testing.T, providers *llm.ProviderRegistry, bridge *proxybridge.Bridge) *reminderE2EFixture {
	t.Helper()

	if providers == nil {
		providers = llm.NewProviderRegistry()
	}

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	pushStore, err := push.NewStore(db)
	if err != nil {
		t.Fatalf("failed to create push store: %v", err)
	}

	pushSvc := push.NewService(pushStore, zap.NewNop())
	cronSvc := cron.NewService(cron.DefaultConfig(), zap.NewNop())
	if err := cronSvc.Start(); err != nil {
		t.Fatalf("failed to start cron service: %v", err)
	}
	t.Cleanup(func() { cronSvc.Stop(context.Background()) })

	pushSvc.SetCron(push.NewCronAdapter(func() *cron.Service { return cronSvc }))
	pushSvc.SetMessageInjector(inject.NewMemoryStoreInjector(store))

	events := &reminderEventPublisher{}
	webpush := &reminderWebPushSender{}
	pushSvc.SetEventPublisher(events)
	pushSvc.SetWebPushSender(webpush)

	toolRegistry := tools.NewRegistry()
	tools.RegisterPushTool(toolRegistry, push.NewToolsAdapter(func() *push.Service { return pushSvc }))

	handler := NewChatHandler(store, providers, toolRegistry)
	if bridge != nil {
		handler.SetProxyBridge(bridge)
	}

	conv, err := store.CreateConversation(context.Background(), "Reminder E2E")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	return &reminderE2EFixture{
		store:     store,
		pushSvc:   pushSvc,
		pushStore: pushStore,
		events:    events,
		webpush:   webpush,
		handler:   handler,
		convID:    conv.ID,
	}
}

func waitForCondition(timeout, interval time.Duration, cond func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return true
		}
		time.Sleep(interval)
	}
	return cond()
}

func hasReminderCard(messages []memory.Message, reminderMessage string) bool {
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if !strings.Contains(m.Content, "```typeless") {
			continue
		}
		if !strings.Contains(m.Content, `"type":"alert"`) {
			continue
		}
		if strings.Contains(m.Content, reminderMessage) {
			return true
		}
	}
	return false
}

func findPushEvent(events []reminderCapturedEvent) (reminderCapturedEvent, bool) {
	for _, evt := range events {
		if evt.EventType == "push" {
			return evt, true
		}
	}
	return reminderCapturedEvent{}, false
}

func buildStreamRequestBody(message, model, provider string) string {
	if provider != "" {
		return fmt.Sprintf(`{"message":%q,"provider":%q,"model":%q}`, message, provider, model)
	}
	return fmt.Sprintf(`{"message":%q,"model":%q}`, message, model)
}

func newSingleModelOpenAIProxyHandlerWithAPIKey(t *testing.T, upstreamBaseURL, providerID, modelID, apiKey string) *proxy.ProxyHandler {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "chat-stream-reminder-real-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	storage, err := providerpool.NewFileStorage(tmpDir)
	if err != nil {
		t.Fatalf("failed to create provider storage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("failed to create provider registry: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	router := providerpool.NewRouter(registry, discovery, providerpool.RoutingStrategyPriority)

	provider := &providerpool.Provider{
		ID:        providerID,
		Name:      providerID,
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstreamBaseURL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys: []providerpool.APIKey{
			{ID: "k-" + providerID, Key: apiKey, Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}

	models := []*providerpool.Model{
		{
			ID:           modelID,
			Name:         modelID,
			ProviderID:   providerID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	if err := storage.SaveModels(providerID, models); err != nil {
		t.Fatalf("failed to save models: %v", err)
	}
	router.RebuildCandidates()

	proxyHandler := proxy.NewProxyHandler(nil, proxy.NewConnectionPool(proxy.DefaultConnectionConfig()), nil)
	proxyHandler.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})

	return proxyHandler
}

func TestStreamMessageReminderFlow_EndToEnd(t *testing.T) {
	const (
		providerName    = "reminder-toolcall-mock"
		modelName       = "reminder-toolcall-model"
		reminderMessage = "10秒后喝水"
	)

	registry := llm.NewProviderRegistry()
	registry.Register(newReminderToolCallProvider(providerName, modelName, reminderMessage, "2s"))
	fixture := newReminderE2EFixture(t, registry, nil)

	body := runStreamTurn(t, fixture.handler, fixture.convID, buildStreamRequestBody("提醒我10秒钟以后喝水", modelName, providerName))
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("stream failed unexpectedly: %s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("stream response missing done marker: %s", body)
	}

	ctx := context.Background()
	created := waitForCondition(5*time.Second, 100*time.Millisecond, func() bool {
		list, err := fixture.pushSvc.List(ctx, "default")
		return err == nil && len(list) > 0
	})
	if !created {
		t.Fatalf("reminder was not created via tool call, stream body=%s", body)
	}

	fired := waitForCondition(12*time.Second, 150*time.Millisecond, func() bool {
		msgs, err := fixture.store.GetMessages(ctx, fixture.convID, 100, 0)
		if err != nil || !hasReminderCard(msgs, reminderMessage) {
			return false
		}

		evt, ok := findPushEvent(fixture.events.snapshot())
		if !ok {
			return false
		}
		data, ok := evt.Data.(map[string]any)
		if !ok {
			return false
		}
		if data["message"] != reminderMessage {
			return false
		}
		if data["conversation_id"] != fixture.convID {
			return false
		}

		wpCalls := fixture.webpush.snapshot()
		if len(wpCalls) == 0 {
			return false
		}
		call := wpCalls[0]
		return call.Body == reminderMessage && call.Title == "🔔 Reminder"
	})
	if !fired {
		msgs, _ := fixture.store.GetMessages(ctx, fixture.convID, 100, 0)
		list, _ := fixture.pushSvc.List(ctx, "default")
		t.Fatalf("reminder did not fire through all channels; reminders=%+v events=%+v webpush=%+v messages=%+v", list, fixture.events.snapshot(), fixture.webpush.snapshot(), msgs)
	}

	list, err := fixture.pushSvc.List(ctx, "default")
	if err != nil {
		t.Fatalf("failed to list reminders: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected persisted reminder, got none")
	}
	if list[0].Status != push.StatusFired {
		t.Fatalf("reminder status = %q, want %q", list[0].Status, push.StatusFired)
	}
}

func TestStreamMessageReminderFlow_RealCodex(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_RUN_REAL_CODEX")) != "1" {
		t.Skip("set ZIMA_RUN_REAL_CODEX=1 to run reminder flow against real codex provider")
	}

	baseURL := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_BASE_URL"))
	apiKey := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_API_KEY"))
	modelID := strings.TrimSpace(os.Getenv("ZIMA_REAL_CODEX_MODEL"))
	if modelID == "" {
		modelID = "gpt-5.3-codex-spark"
	}
	if baseURL == "" || apiKey == "" {
		t.Skip("missing ZIMA_REAL_CODEX_BASE_URL or ZIMA_REAL_CODEX_API_KEY")
	}

	proxyHandler := newSingleModelOpenAIProxyHandlerWithAPIKey(t, baseURL, "real-codex-reminder", modelID, apiKey)
	fixture := newReminderE2EFixture(t, llm.NewProviderRegistry(), proxybridge.NewBridge(proxyHandler))

	prompt := "提醒我10秒钟以后喝水。请直接调用 reminder 工具，参数 action=\"add\", message=\"喝水\", time=\"10秒钟以后\"。执行后简短确认。"
	body := runStreamTurn(t, fixture.handler, fixture.convID, buildStreamRequestBody(prompt, modelID, ""))
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("real codex stream failed: %s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("real codex stream missing done marker: %s", body)
	}

	ctx := context.Background()
	var reminderID string
	created := waitForCondition(15*time.Second, 200*time.Millisecond, func() bool {
		list, err := fixture.pushSvc.List(ctx, "default")
		if err != nil || len(list) == 0 {
			return false
		}
		for _, item := range list {
			if strings.Contains(item.Message, "喝水") {
				reminderID = item.ID
				return true
			}
		}
		return false
	})
	if !created {
		list, _ := fixture.pushSvc.List(ctx, "default")
		t.Fatalf("real codex did not create reminder via tool call; reminders=%+v stream=%s", list, body)
	}

	fired := waitForCondition(35*time.Second, 250*time.Millisecond, func() bool {
		msgs, err := fixture.store.GetMessages(ctx, fixture.convID, 120, 0)
		if err != nil || !hasReminderCard(msgs, "喝水") {
			return false
		}

		evt, ok := findPushEvent(fixture.events.snapshot())
		if !ok {
			return false
		}
		data, ok := evt.Data.(map[string]any)
		if !ok {
			return false
		}
		if data["conversation_id"] != fixture.convID {
			return false
		}
		msg, _ := data["message"].(string)
		if !strings.Contains(msg, "喝水") {
			return false
		}

		wpCalls := fixture.webpush.snapshot()
		if len(wpCalls) == 0 {
			return false
		}
		call := wpCalls[0]
		return strings.Contains(call.Body, "喝水") && call.Title == "🔔 Reminder"
	})
	if !fired {
		msgs, _ := fixture.store.GetMessages(ctx, fixture.convID, 120, 0)
		list, _ := fixture.pushSvc.List(ctx, "default")
		t.Fatalf("real codex reminder did not fire through all channels; reminder_id=%s reminders=%+v events=%+v webpush=%+v messages=%+v", reminderID, list, fixture.events.snapshot(), fixture.webpush.snapshot(), msgs)
	}

	stored, err := fixture.pushStore.Get(ctx, reminderID)
	if err != nil {
		t.Fatalf("failed to load stored reminder %s: %v", reminderID, err)
	}
	if stored == nil {
		t.Fatalf("stored reminder not found: %s", reminderID)
	}
	if stored.Status != push.StatusFired {
		t.Fatalf("stored reminder status = %q, want %q", stored.Status, push.StatusFired)
	}
}
