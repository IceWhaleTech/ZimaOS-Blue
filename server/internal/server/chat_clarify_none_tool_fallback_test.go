package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

const clarifyNoneMixedPrompt = "看下 workspace 里的 README，还是搜一下最新 OpenAI Responses API 文档，你觉得该先做哪个？"

func newClarifyNoneSelectionChatHandler(t *testing.T, store *memory.Store, registry *llm.ProviderRegistry, toolRegistry *tools.Registry) *ChatHandler {
	t.Helper()

	if toolRegistry == nil {
		toolRegistry = tools.NewRegistry()
	}
	toolRegistry.Register(tools.NewToolSearchTool(toolRegistry))
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "ask", Description: "Ask the user clarifying questions"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "browser", Description: "Open and interact with web pages"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "deep_research", Description: "Run deep research"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "exec", Description: "Execute skill and shell commands"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "config", Description: "Manage runtime settings and providers"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "read", Description: "Read workspace files"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "web_query", Description: "Search the web"})
	toolRegistry.ExposeDefinition(tools.ToolDefinition{Name: "write", Description: "Write workspace files"})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetToolSelector(tools.DefaultToolSelector())
	handler.SetToolRouter(tools.DefaultToolRouter())

	cfg := &config.Config{
		ToolCalling: *config.DefaultToolCallingConfig(),
		Agents:      *config.DefaultAgentsConfig(),
	}
	handler.SetToolPolicyResolver(tools.NewToolPolicyResolver(cfg))

	workspaceDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	writeSettingsSelectorCanonicalWebQuerySkill(t, workspaceDir, "search the web for latest docs and official references", `blue web_query input="OpenAI Responses API docs"`, "search", "web", "docs", "latest")
	writeSettingsSelectorSkill(t, workspaceDir, "ask", "ask the user clarifying questions and wait for an answer", `blue ask q="Choose a deploy strategy" a='["Canary","Blue-Green"]'`, "clarify", "interactive", "question")
	writeSettingsSelectorSkill(t, workspaceDir, "browser", "browse urls and interact with web pages after login or click flows", "blue browser.navigate url=https://example.com", "browser", "login", "click", "page")
	writeSettingsSelectorSkill(t, workspaceDir, "analyze", "analyze multiple links and synthesize a report", `blue analyze topic="multi-link report" --json`, "analysis", "report", "summary", "link", "url")
	writeSettingsSelectorSkill(t, workspaceDir, "deep_research", "perform cited timeline comparisons and deep research", `blue deep_research query="OpenAI vs Anthropic agent runtime"`, "research", "citation", "timeline", "compare")
	writeSettingsSelectorSkill(t, workspaceDir, "ui_reviewer", "review screenshots and UI layouts for accessibility and visual issues", `blue ui_reviewer target="https://example.com"`, "ui", "review", "screenshot", "layout", "accessibility")
	writeSettingsSelectorSkill(t, workspaceDir, "config", "manage providers settings channels skills tools health and proxy diagnostics", "blue config.providers.list", "admin", "settings", "providers", "diagnostics")

	handler.SetSkillSelector(agentcore.NewSkillSelector(workspaceDir, agentcore.NewHeuristicSkillReranker()))
	handler.ConfigureToolSearchRuntime(workspaceDir, cfg)

	return handler
}

func assertClarifyNoneSelectionForPrompt(t *testing.T, handler *ChatHandler, convID, prompt string) {
	t.Helper()

	selection := handler.selectChatToolSurfacesForRequest(context.Background(), prompt, tools.ToolPolicyRequest{
		Model:     "gpt-5.3-codex-spark",
		SessionID: convID,
		RouteKind: tools.ToolRouteKindChat,
	}, nil, nil)
	if selection.NativeMode != chatNativeToolSurfaceModeClarifyNone {
		t.Fatalf("NativeMode = %q, want %q", selection.NativeMode, chatNativeToolSurfaceModeClarifyNone)
	}
	if len(selection.NativeDefs) != 0 {
		t.Fatalf("NativeDefs = %v, want none", selectedToolNames(selection.NativeDefs))
	}
	if selection.DiscoveryDecision == nil || !selection.DiscoveryDecision.NeedClarify {
		t.Fatalf("DiscoveryDecision = %#v, want clarify decision", selection.DiscoveryDecision)
	}
}

func TestChatHandlerSendMessage_ClarifyNonePseudoToolLeakFallsBackToClarification(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Clarify none send message pseudo tool leak")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "clarify-none-pseudo-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: `{"cmd":"ls -la"}我先去读 README，再搜最新 OpenAI Responses API 文档。<exec>{"cmd":"ls -la"}</exec>`,
				},
				Usage: llm.Usage{PromptTokens: 40, CompletionTokens: 40, TotalTokens: 80},
			},
		},
	}
	registry.Register(scripted)

	handler := newClarifyNoneSelectionChatHandler(t, store, registry, tools.NewRegistry())
	assertClarifyNoneSelectionForPrompt(t, handler, conv.ID, clarifyNoneMixedPrompt)

	e := echo.New()
	reqBody := `{"message":"` + clarifyNoneMixedPrompt + `","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if strings.Contains(content, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(content), "<exec>") {
		t.Fatalf("expected pseudo tool-call leakage to be removed from final response, got=%q", content)
	}
	if !strings.Contains(content, "先不执行任何工具") || !strings.Contains(content, "请先明确告诉我") {
		t.Fatalf("expected clarify-none fallback reply, got=%q", content)
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected only one LLM round for clarify-none pseudo leak, got %d", scripted.CallCount())
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestStreamMessage_ClarifyNonePseudoToolLeakFallsBackToClarification(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Clarify none stream pseudo tool leak")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := newClarifyNoneSelectionChatHandler(t, store, llm.NewProviderRegistry(), tools.NewRegistry())
	assertClarifyNoneSelectionForPrompt(t, handler, conv.ID, clarifyNoneMixedPrompt)

	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent:  `{"cmd":"ls -la"}我先去读 README，再搜最新 OpenAI Responses API 文档。<exec>{"cmd":"ls -la"}</exec>`,
		secondRoundContent: "unexpected second round",
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"`+clarifyNoneMixedPrompt+`","model":"gpt-5.3-codex-spark"}`)
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if strings.Contains(body, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(body), "<exec>") {
		t.Fatalf("expected pseudo tool-call leakage to be suppressed from streamed body, got=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, "先不执行任何工具") || !strings.Contains(body, "请先明确告诉我") {
		t.Fatalf("expected clarify-none fallback in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 1 {
		t.Fatalf("expected only one proxy call for clarify-none pseudo leak, got %d", fakeProxy.callCount)
	}
	if fakeProxy.sawExecutionNudge || fakeProxy.sawPseudoToolNudge || fakeProxy.sawAgentLoopNudge {
		t.Fatalf("expected no continuation nudge for clarify-none pseudo leak, last=%q", fakeProxy.lastRequestMessage)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestProcessChannelMessage_ClarifyNonePseudoToolLeakFallsBackToClarification(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_clarify_none_pseudo")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "manual title"); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-clarify-none",
		responses: []llm.ChatResponse{
			{
				ID:    "im-clarify-none-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: `{"cmd":"ls -la"}我先去读 README，再搜最新 OpenAI Responses API 文档。<exec>{"cmd":"ls -la"}</exec>`,
				},
			},
		},
	}
	registry.Register(scripted)

	handler := newClarifyNoneSelectionChatHandler(t, store, registry, tools.NewRegistry())
	assertClarifyNoneSelectionForPrompt(t, handler, convID, clarifyNoneMixedPrompt)

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_clarify_none_pseudo",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     clarifyNoneMixedPrompt,
		Metadata: map[string]interface{}{
			"language": "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if strings.Contains(resp, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(resp), "<exec>") {
		t.Fatalf("expected pseudo tool-call leakage to be removed from IM response, got %q", resp)
	}
	if !strings.Contains(resp, "先不执行任何工具") || !strings.Contains(resp, "请先明确告诉我") {
		t.Fatalf("expected clarify-none fallback reply, got %q", resp)
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected only one IM LLM round for clarify-none pseudo leak, got %d", scripted.CallCount())
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first IM request capture")
	}
	if len(firstReq.Tools) != 0 {
		t.Fatalf("expected clarify-none IM request to expose no tools, got %v", firstReq.Tools)
	}

	messages, err := store.GetMessages(context.Background(), convID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from persisted IM assistant messages, got=%q", m.Content)
		}
	}
}

func TestProcessChannelMessage_CheckpointResume_ClarifyNonePseudoToolLeakFallsBackToClarification(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_clarify_none_resume")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "manual title"); err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-clarify-none-resume",
		responses: []llm.ChatResponse{
			{
				ID:    "im-clarify-none-resume-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: `{"cmd":"ls -la"}我先去读 README，再搜最新 OpenAI Responses API 文档。<exec>{"cmd":"ls -la"}</exec>`,
				},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name: "web_query",
		result: map[string]interface{}{
			"status":      "ok",
			"mode":        "search_read",
			"input":       "OpenAI Responses API docs",
			"query":       "OpenAI Responses API docs",
			"title":       "OpenAI Responses API docs",
			"target_url":  "https://platform.openai.com/docs/api-reference/responses",
			"final_url":   "https://platform.openai.com/docs/api-reference/responses",
			"content":     "mock docs result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := newClarifyNoneSelectionChatHandler(t, store, registry, toolRegistry)
	assertClarifyNoneSelectionForPrompt(t, handler, convID, clarifyNoneMixedPrompt)
	checkpointMgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(checkpointMgr)

	checkpoint := checkpointMgr.Create(tools.BrowserCheckpointRequest{
		ID:        "cp-clarify-none-resume",
		SessionID: convID,
		Channel:   "feishu",
		Required:  true,
		RiskLevel: "high",
		Step:      "research",
		Action:    "search",
		Timeout:   time.Minute,
	})
	pendingTool := llm.ToolCall{
		ID:        "call_web_query_resume",
		Name:      "web_query",
		Arguments: `{"input":"OpenAI Responses API docs"}`,
	}
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_clarify_none_resume",
		ReplyToID:       "msg_prev",
		Lang:            i18n.ParseLanguage("zh-CN"),
		AgentMode:       false,
		RoutingMessage:  clarifyNoneMixedPrompt,
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark", Messages: []llm.Message{{Role: llm.RoleUser, Content: clarifyNoneMixedPrompt}}},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先查一下。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_clarify_none_resume",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
		Metadata: map[string]interface{}{
			"language": "zh-CN",
		},
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}
	if strings.Contains(resp, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(resp), "<exec>") {
		t.Fatalf("expected pseudo tool-call leakage to be removed from resumed IM response, got %q", resp)
	}
	if !strings.Contains(resp, "先不执行任何工具") || !strings.Contains(resp, "请先明确告诉我") {
		t.Fatalf("expected clarify-none fallback reply after checkpoint resume, got %q", resp)
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected only one resumed IM LLM round for clarify-none pseudo leak, got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "OpenAI Responses API docs" {
		t.Fatalf("web_query input = %q, want OpenAI Responses API docs", input)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing resumed IM request capture")
	}
	if len(firstReq.Tools) != 0 {
		t.Fatalf("expected resumed clarify-none IM request to expose no tools, got %v", firstReq.Tools)
	}
	sawToolResult := false
	for _, msg := range firstReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "mock docs result") {
			sawToolResult = true
			break
		}
	}
	if !sawToolResult {
		t.Fatalf("expected resumed IM request to include executed tool result, got %#v", firstReq.Messages)
	}

	messages, err := store.GetMessages(context.Background(), convID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"cmd":"ls -la"}`) || strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from persisted resumed IM assistant messages, got=%q", m.Content)
		}
	}
}
