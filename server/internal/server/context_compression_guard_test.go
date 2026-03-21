package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type countingSideEffectTool struct {
	name  string
	calls atomic.Int32
}

func (t *countingSideEffectTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        t.name,
		Description: "counting side-effect tool",
		Parameters: map[string]interface{}{
			"type":                 "object",
			"additionalProperties": true,
		},
	}
}

func (t *countingSideEffectTool) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	t.calls.Add(1)
	return map[string]interface{}{"ok": true}, nil
}

func (t *countingSideEffectTool) CallCount() int {
	return int(t.calls.Load())
}

func seedCompressedHistoryForGuardTest(t *testing.T, store *memory.Store, handler *ChatHandler, convID string) {
	t.Helper()
	seed := []memory.Message{
		{Role: "user", Content: strings.Repeat("Continue updating the old rollout checklist in docs/old_plan.md. ", 10)},
		{Role: "assistant", Content: strings.Repeat("I am still editing docs/old_plan.md and preparing the remaining changes. ", 10)},
		{Role: "user", Content: strings.Repeat("Keep the pending write task in mind and finish the old_plan migration. ", 10)},
		{Role: "assistant", Content: strings.Repeat("Noted, I will continue the write task for docs/old_plan.md unless told otherwise. ", 10)},
		{Role: "user", Content: strings.Repeat("The unfinished work is still the old_plan migration. ", 10)},
		{Role: "assistant", Content: strings.Repeat("Pending work remains in docs/old_plan.md and the old rollout document. ", 10)},
	}
	for _, msg := range seed {
		if _, err := store.AddMessage(context.Background(), convID, msg); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}
	handler.summaryCache.Put(convID, &ConversationSummary{
		Text:         "Goal\n- Continue updating docs/old_plan.md\n\nAccomplished\n- Pending: finish docs/old_plan.md migration",
		MessageCount: len(seed),
	})
}

func TestSendMessage_PausesStaleCarryOverSideEffectToolForLatestQuestion(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Guard stale carry-over")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-stale-guard",
		responses: []llm.ChatResponse{
			{
				ID:    "guard-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
					ID:        "call-write-1",
					Name:      "write",
					Arguments: `{"path":"docs/old_plan.md","content":"stale carry-over edit"}`,
				}}},
			},
			{
				ID:      "guard-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "先解释原因：我已经暂停之前那个旧的写入任务，当前优先回答你的最新问题。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	writeTool := &countingSideEffectTool{name: "write"}
	toolRegistry.Register(writeTool)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "gpt-5.3-codex-spark",
		ContextWindow: 2048,
	}}))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smartToolSelection := false
	settings.settings.SmartToolSelection = &smartToolSelection
	handler.SetSettingsHandler(settings)

	seedCompressedHistoryForGuardTest(t, store, handler, conv.ID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"先解释原因，不要继续旧任务","model":"gpt-5.3-codex-spark","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if !strings.Contains(resp.Content, "暂停之前那个旧的写入任务") {
		t.Fatalf("response content = %q, want latest-intent explanation", resp.Content)
	}
	if writeTool.CallCount() != 0 {
		t.Fatalf("write tool calls = %d, want 0 because stale carry-over should be paused", writeTool.CallCount())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("provider call count = %d, want 2 (stale tool round + redirected answer round)", scripted.CallCount())
	}
	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatal("missing second request capture")
	}
	if secondReq.PreviousResponseID != "" {
		t.Fatalf("second request previous_response_id = %q, want cleared after stale carry-over guard", secondReq.PreviousResponseID)
	}
	if latest := latestUserMessageFromLLM(secondReq.Messages); !strings.Contains(latest, "Pause any [carry-over] work") {
		t.Fatalf("expected redirect nudge in second request, got latest user message %q", latest)
	}
}

func TestStreamMessage_PausesStaleCarryOverSideEffectToolForLatestQuestion(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Stream guard stale carry-over")
	if err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-stream-stale-guard",
		responses: []llm.ChatResponse{
			{
				ID:    "stream-guard-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
					ID:        "call-write-stream-1",
					Name:      "write",
					Arguments: `{"path":"docs/old_plan.md","content":"stale stream edit"}`,
				}}},
			},
			{
				ID:      "stream-guard-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "先解释原因：我已经暂停旧任务，现在优先回答你的最新问题。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	writeTool := &countingSideEffectTool{name: "write"}
	toolRegistry.Register(writeTool)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "gpt-5.3-codex-spark",
		ContextWindow: 2048,
	}}))
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	smartToolSelection := false
	settings.settings.SmartToolSelection = &smartToolSelection
	handler.SetSettingsHandler(settings)

	seedCompressedHistoryForGuardTest(t, store, handler, conv.ID)

	body := runStreamTurn(t, handler, conv.ID, `{"message":"先解释原因，不要继续旧任务","model":"gpt-5.3-codex-spark","max_tokens":64}`)
	if writeTool.CallCount() != 0 {
		t.Fatalf("write tool calls = %d, want 0 because stale carry-over should be paused", writeTool.CallCount())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("provider call count = %d, want 2 (stale tool round + redirected answer round)", scripted.CallCount())
	}
	if !strings.Contains(body, "先解释原因：我已经暂停旧任务") {
		t.Fatalf("expected streamed answer after stale carry-over pause, body=%s", body)
	}

	events := extractJSONSSEEvents(t, body)
	for _, event := range events {
		if executing, _ := event["tool_executing"].(bool); executing {
			t.Fatalf("unexpected tool execution event after stale carry-over guard: %+v", event)
		}
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatal("missing second request capture")
	}
	if secondReq.PreviousResponseID != "" {
		t.Fatalf("second request previous_response_id = %q, want cleared after stale carry-over guard", secondReq.PreviousResponseID)
	}
}
