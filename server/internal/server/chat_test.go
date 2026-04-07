package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pruner"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/session"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

type requestCaptureProvider struct {
	lastReq llm.ChatRequest
	mu      sync.Mutex
}

type contextTooLongThenSuccessProxyHandler struct {
	mu            sync.Mutex
	requestModels []string
	successText   string
	providerID    string
	providerName  string
	failAll       bool
	relayWrapped  bool
}

type scriptedChatProvider struct {
	name       string
	models     []string
	responses  []llm.ChatResponse
	callErrors []error

	mu        sync.Mutex
	callCount int
	requests  []llm.ChatRequest
}

type deepResearchExecMock struct {
	result map[string]interface{}
	err    error
	mu     sync.Mutex
	calls  int
	last   map[string]interface{}
}

type researchServiceMock struct {
	createJob *tools.ResearchJob
	getJob    *tools.ResearchJob
	err       error

	mu          sync.Mutex
	createCalls int
	statusCalls int
	lastCreate  tools.ResearchCreateJobRequest
	lastJobID   string
	lastUserID  string
}

func (m *deepResearchExecMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.mu.Lock()
	m.calls++
	m.last = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.last[k] = v
	}
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func (m *researchServiceMock) CreateJob(_ context.Context, req tools.ResearchCreateJobRequest) (*tools.ResearchJob, error) {
	m.mu.Lock()
	m.createCalls++
	m.lastCreate = req
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.createJob != nil {
		return m.createJob, nil
	}
	return &tools.ResearchJob{
		ID:             "job-default",
		ConversationID: req.ConversationID,
		Status:         "completed",
		Query:          req.Query,
		Mode:           req.Mode,
		Answer:         "default research answer",
	}, nil
}

func (m *researchServiceMock) GetJobForUser(id, userID string) (*tools.ResearchJob, error) {
	m.mu.Lock()
	m.statusCalls++
	m.lastJobID = id
	m.lastUserID = userID
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.getJob != nil {
		return m.getJob, nil
	}
	return &tools.ResearchJob{
		ID:       id,
		Status:   "completed",
		Query:    "default query",
		Answer:   "default research answer",
		Mode:     "deep",
		Progress: 100,
	}, nil
}

type smallModelRuntimeMock struct {
	respText string
	err      error
	ready    *bool
	calls    int
	calledCh chan struct{}
	lastReq  smallmodel.GenerateRequest
}

type staticToolMock struct {
	def    tools.ToolDefinition
	result interface{}
	err    error
}

type imageInputCaptureTool struct {
	mu     sync.Mutex
	inputs []tools.ToolImageInput
}

type webSearchToolMock struct {
	name   string
	result interface{}
	err    error
	calls  int
	last   map[string]interface{}
	mu     sync.Mutex
}

func (m *staticToolMock) Definition() tools.ToolDefinition {
	return m.def
}

func (m *staticToolMock) Execute(context.Context, map[string]interface{}) (interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return map[string]interface{}{"ok": true}, nil
}

func (m *imageInputCaptureTool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "image",
		Description: "mock image tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{"type": "string"},
				"prompt": map[string]interface{}{"type": "string"},
			},
			"additionalProperties": true,
		},
	}
}

func (m *imageInputCaptureTool) Execute(ctx context.Context, _ map[string]interface{}) (interface{}, error) {
	m.mu.Lock()
	m.inputs = tools.GetImageInputs(ctx)
	m.mu.Unlock()
	return map[string]interface{}{"ok": true}, nil
}

func (m *imageInputCaptureTool) CapturedInputs() []tools.ToolImageInput {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]tools.ToolImageInput, len(m.inputs))
	copy(out, m.inputs)
	return out
}

func inlinePNGBase64ForChatTest() string {
	return "ZGF0YQ=="
}

func TestBuildDirectoryWhitelistPromptHintUsesSecuritySettings(t *testing.T) {
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(`{"directory_whitelist_enabled":true,"directory_whitelist":[{"path":"/tmp/project","alias":"proj"}]}`),
	)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	if err := settings.Patch(e.NewContext(req, rec)); err != nil {
		t.Fatalf("settings Patch() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("settings Patch() status = %d, want %d", rec.Code, http.StatusOK)
	}

	handler := &ChatHandler{}
	handler.SetSettingsHandler(settings)

	hint := handler.buildDirectoryWhitelistPromptHint()
	if !strings.Contains(hint, "@proj => /tmp/project") {
		t.Fatalf("hint = %q, want alias mapping", hint)
	}
	if !strings.Contains(hint, "Prefer relative paths for files in the current workspace.") {
		t.Fatalf("hint = %q, want relative path guidance", hint)
	}

	roots, aliases := handler.directoryWhitelistScope()
	if len(roots) != 1 || roots[0] != "/tmp/project" {
		t.Fatalf("directoryWhitelistScope() roots = %v, want [/tmp/project]", roots)
	}
	if got := aliases["proj"]; got != "/tmp/project" {
		t.Fatalf("directoryWhitelistScope() alias proj = %q, want /tmp/project", got)
	}
}

func TestGetProviderFromPool_MiniMaxKeepsConfiguredBaseURL(t *testing.T) {
	storage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}

	provider := &providerpool.Provider{
		ID:        "minimax",
		Name:      "MiniMax",
		Type:      providerpool.ProviderTypeBuiltin,
		BaseURL:   "https://api.minimax.io/anthropic",
		APIFormat: providerpool.APIFormatAnthropic,
		Enabled:   true,
		APIKeys: []providerpool.APIKey{{
			ID:      "key-main",
			Key:     "sk-test",
			Enabled: true,
		}},
	}
	if err := storage.SaveProvider(provider); err != nil {
		t.Fatalf("SaveProvider failed: %v", err)
	}

	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	handler := &ChatHandler{
		providerPool: &providerpool.Pool{Registry: registry},
	}

	got, err := handler.getProviderFromPool("minimax")
	if err != nil {
		t.Fatalf("getProviderFromPool returned error: %v", err)
	}

	claudeProvider, ok := got.(*llm.ClaudeProvider)
	if !ok {
		t.Fatalf("provider type = %T, want *llm.ClaudeProvider", got)
	}

	baseURL := reflect.ValueOf(claudeProvider).Elem().FieldByName("baseURL").String()
	if baseURL != "https://api.minimax.io/anthropic" {
		t.Fatalf("claude baseURL = %q, want %q", baseURL, "https://api.minimax.io/anthropic")
	}
}

func (m *webSearchToolMock) Definition() tools.ToolDefinition {
	name := strings.TrimSpace(m.name)
	if name == "" {
		name = "web_search"
	}
	return tools.ToolDefinition{
		Name:        name,
		Description: "mock web tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"input":       map[string]interface{}{"type": "string"},
				"query":       map[string]interface{}{"type": "string"},
				"q":           map[string]interface{}{"type": "string"},
				"format":      map[string]interface{}{"type": "string"},
				"max_results": map[string]interface{}{"type": "integer"},
			},
			"additionalProperties": true,
		},
	}
}

func (m *webSearchToolMock) Execute(_ context.Context, args map[string]interface{}) (interface{}, error) {
	m.mu.Lock()
	m.calls++
	m.last = make(map[string]interface{}, len(args))
	for k, v := range args {
		m.last[k] = v
	}
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return map[string]interface{}{}, nil
}

func (m *smallModelRuntimeMock) Ready() bool {
	if m.ready == nil {
		return true
	}
	return *m.ready
}

func (m *smallModelRuntimeMock) Generate(_ context.Context, req smallmodel.GenerateRequest) (*smallmodel.GenerateResponse, error) {
	m.calls++
	m.lastReq = req
	if m.calledCh != nil {
		select {
		case m.calledCh <- struct{}{}:
		default:
		}
	}
	if m.err != nil {
		return nil, m.err
	}
	return &smallmodel.GenerateResponse{Text: m.respText}, nil
}

func (p *requestCaptureProvider) Name() string { return "capture" }

func (p *requestCaptureProvider) Models() []string { return []string{"capture-model"} }

func (p *requestCaptureProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.lastReq = req
	p.mu.Unlock()
	return &llm.ChatResponse{
		ID:      "capture-resp",
		Model:   req.Model,
		Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		Usage:   llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12},
	}, nil
}

func (p *requestCaptureProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	close(ch)
	return ch, nil
}

func (p *requestCaptureProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	p.mu.Lock()
	p.lastReq = req
	p.mu.Unlock()
	return callback(llm.StreamChunk{Delta: "ok", Done: true, Usage: &llm.Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}})
}

func (p *requestCaptureProvider) LastRequest() llm.ChatRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.lastReq
}

func (h *contextTooLongThenSuccessProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	h.mu.Lock()
	h.requestModels = append(h.requestModels, strings.TrimSpace(body.Model))
	h.mu.Unlock()

	providerID := strings.TrimSpace(h.providerID)
	if providerID == "" {
		providerID = "p-context"
	}
	providerName := strings.TrimSpace(h.providerName)
	if providerName == "" {
		providerName = "MockProxy"
	}
	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = providerName
		rr.ProviderID = providerID
		rr.Model = strings.TrimSpace(body.Model)
	}

	if h.failAll || strings.EqualFold(strings.TrimSpace(body.Model), "small-model") {
		w.Header().Set("Content-Type", "application/json")
		if h.relayWrapped {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"Context window is full. Reduce conversation history, system prompt, or tools.","type":"upstream_error"}}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"This model's maximum context length is 4096 tokens. However, your messages resulted in 5000 tokens.","type":"invalid_request_error","code":"context_length_exceeded"}}`))
		return
	}

	if body.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"id":"stream-ok","choices":[{"delta":{"content":%q},"finish_reason":"stop"}],"model":%q}`, h.successContent(), strings.TrimSpace(body.Model)))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"resp-ok","model":%q,"choices":[{"message":{"role":"assistant","content":%q},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`, strings.TrimSpace(body.Model), h.successContent())))
}

func (h *contextTooLongThenSuccessProxyHandler) successContent() string {
	if strings.TrimSpace(h.successText) != "" {
		return h.successText
	}
	return "recovered after fallback"
}

func (h *contextTooLongThenSuccessProxyHandler) RequestModels() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]string, len(h.requestModels))
	copy(out, h.requestModels)
	return out
}

func cloneChatRequestForTest(req llm.ChatRequest) llm.ChatRequest {
	raw, err := json.Marshal(req)
	if err != nil {
		return req
	}
	var copied llm.ChatRequest
	if err := json.Unmarshal(raw, &copied); err != nil {
		return req
	}
	return copied
}

func (p *scriptedChatProvider) Name() string {
	if strings.TrimSpace(p.name) != "" {
		return p.name
	}
	return "scripted"
}

func (p *scriptedChatProvider) Models() []string {
	if len(p.models) > 0 {
		return p.models
	}
	return []string{"gpt-5.3-codex-spark", "gpt-5.3-codex"}
}

func (p *scriptedChatProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	p.mu.Lock()
	p.callCount++
	p.requests = append(p.requests, cloneChatRequestForTest(req))
	rawIdx := p.callCount - 1
	idx := rawIdx
	if idx >= len(p.responses) {
		idx = len(p.responses) - 1
	}
	p.mu.Unlock()

	if idx < 0 {
		return &llm.ChatResponse{
			ID:      "scripted-default",
			Model:   req.Model,
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}, nil
	}
	if rawIdx >= 0 && rawIdx < len(p.callErrors) && p.callErrors[rawIdx] != nil {
		return nil, p.callErrors[rawIdx]
	}
	resp := p.responses[idx]
	return &resp, nil
}

func (p *scriptedChatProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	}
	close(ch)
	return ch, nil
}

func (p *scriptedChatProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:        resp.ID,
		Model:     resp.Model,
		Delta:     resp.Message.Content,
		Done:      true,
		Usage:     &resp.Usage,
		ToolCalls: resp.Message.ToolCalls,
	})
}

func (p *scriptedChatProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.callCount
}

func (p *scriptedChatProvider) RequestAt(idx int) (llm.ChatRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx < 0 || idx >= len(p.requests) {
		return llm.ChatRequest{}, false
	}
	return p.requests[idx], true
}

func buildFileReadRegressionContent(lineCount int, token string) string {
	lines := make([]string, 0, lineCount)
	for i := 1; i <= lineCount; i++ {
		lines = append(lines, fmt.Sprintf("line %03d %s", i, strings.Repeat(token, 2)))
	}
	return strings.Join(lines, "\n")
}

func requireToolPayloadMessage(t *testing.T, req llm.ChatRequest, toolName string) (llm.Message, map[string]interface{}) {
	t.Helper()

	want := normalizeFileToolCompatName(toolName)
	if want == "" {
		want = strings.ToLower(strings.TrimSpace(toolName))
	}

	for _, msg := range req.Messages {
		if msg.Role != llm.RoleTool {
			continue
		}
		got := normalizeFileToolCompatName(msg.ToolName)
		if got == "" {
			got = strings.ToLower(strings.TrimSpace(msg.ToolName))
		}
		if got != want {
			continue
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(msg.Content), &payload); err != nil {
			t.Fatalf("decode %s tool payload: %v content=%q", toolName, err, msg.Content)
		}
		return msg, payload
	}

	t.Fatalf("expected request to include %s tool result, got %#v", toolName, req.Messages)
	return llm.Message{}, nil
}

func hasSystemAnchor(messages []llm.Message, title, goal string) bool {
	for _, m := range messages {
		if m.Role != llm.RoleSystem {
			continue
		}
		if strings.Contains(m.Content, "Conversation title: "+title) && strings.Contains(m.Content, "Initial user goal: "+goal) {
			return true
		}
	}
	return false
}

func TestIsAwaitingUserInput_ProtocolMarker(t *testing.T) {
	content := "Need your decision.\n<awaiting_user_input>true</awaiting_user_input>"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected protocol marker to be detected as awaiting input")
	}
}

func TestIsAwaitingUserInput_StructuredAskGate(t *testing.T) {
	content := "请选择执行策略\nA. 快速方案\nB. 平衡方案\nC. 工程化方案"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected structured options ask gate to be detected as awaiting input")
	}
}

func TestIsAwaitingUserInput_AskGateTag(t *testing.T) {
	content := "<ask_gate>one-line question\nA. yes\nB. no</ask_gate>"
	if !isAwaitingUserInput(content) {
		t.Fatal("expected ask_gate tag to be detected as awaiting input")
	}
}

func TestShouldAutoContinueForTodo_StopsOnAwaitingInput(t *testing.T) {
	current := "- [ ] implement feature\n<awaiting_user_input>true</awaiting_user_input>"
	if shouldAutoContinueForTodo(current, "") {
		t.Fatal("auto-continue should stop when awaiting user input marker is present")
	}
}

func TestSyncTrackedTodoAfterToollessReply_CompletesSinglePendingSummaryItem(t *testing.T) {
	tracked := "- [x] 执行导出脚本\n- [ ] 提供最终总结"
	current := "任务已完成。最终总结：Apple Notes 已全部导出。"

	got, changed := syncTrackedTodoAfterToollessReply(tracked, current)
	if !changed {
		t.Fatal("expected implicit final-summary todo completion")
	}
	if strings.Contains(got, "- [ ] 提供最终总结") {
		t.Fatalf("expected summary item to be checked, got %q", got)
	}
	if !strings.Contains(got, "- [x] 提供最终总结") {
		t.Fatalf("expected checked summary item, got %q", got)
	}
}

func TestSyncTrackedTodoAfterToollessReply_DoesNotCompleteNonSummaryTodo(t *testing.T) {
	tracked := "- [x] 执行导出脚本\n- [ ] 验证导出文件存在"
	current := "任务已完成。最终总结：Apple Notes 已全部导出。"

	got, changed := syncTrackedTodoAfterToollessReply(tracked, current)
	if changed {
		t.Fatalf("expected non-summary todo to remain pending, got %q", got)
	}
	if got != tracked {
		t.Fatalf("expected tracked todo unchanged, got %q", got)
	}
}

func TestSyncTrackedTodoAfterToollessReply_CompletesSinglePendingArtifactDeliveryItem(t *testing.T) {
	tracked := "- [x] 提取页面实际色值\n- [ ] 将完整报告（问题标注 + 对比度建议 + Hero HTML/CSS 示例）写入文件并输出"
	current := "已将完整报告写入 `ui-review/完整报告.md`，可以直接查看。"

	got, changed := syncTrackedTodoAfterToollessReply(tracked, current)
	if !changed {
		t.Fatal("expected artifact-delivery todo completion")
	}
	if strings.Contains(got, "- [ ] 将完整报告") {
		t.Fatalf("expected report delivery item to be checked, got %q", got)
	}
	if !strings.Contains(got, "- [x] 将完整报告") {
		t.Fatalf("expected checked report delivery item, got %q", got)
	}
}

func TestSyncTrackedTodoAfterToollessReply_DoesNotCompleteUnrelatedTodoOnArtifactDeliveryCue(t *testing.T) {
	tracked := "- [x] 生成报告草稿\n- [ ] 校验移动端 Hero 间距"
	current := "已将完整报告写入 `ui-review/完整报告.md`，可以直接查看。"

	got, changed := syncTrackedTodoAfterToollessReply(tracked, current)
	if changed {
		t.Fatalf("expected unrelated todo to remain pending, got %q", got)
	}
	if got != tracked {
		t.Fatalf("expected tracked todo unchanged, got %q", got)
	}
}

func TestSyncTrackedTodoAfterToollessReply_DoesNotCompleteWhenMultiplePendingItemsRemain(t *testing.T) {
	tracked := "- [ ] 收集信息\n- [ ] 提供最终总结"
	current := "任务已完成。最终总结：资料已经整理完成。"

	got, changed := syncTrackedTodoAfterToollessReply(tracked, current)
	if changed {
		t.Fatalf("expected checklist to stay unchanged with multiple pending items, got %q", got)
	}
	if got != tracked {
		t.Fatalf("expected tracked todo unchanged, got %q", got)
	}
}

func TestDeriveContinuationContext_AffirmativeWithDefaultPlan(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "你可以帮我查一下 ZimaOS 的信息吗"},
		{
			Role:    llm.RoleAssistant,
			Content: "- [ ] 产品定位与功能概览\n- [ ] 最新动态\n\n如果你不想选，我可以默认按「功能概览 + 最新动态」先查一版。",
		},
		{Role: llm.RoleUser, Content: "好的"},
	}

	cc := deriveContinuationContext("好的", msgs)
	if strings.TrimSpace(cc.Hint) == "" {
		t.Fatal("expected continuation hint for short affirmative reply with default plan")
	}
	if cc.ToolQuery != "你可以帮我查一下 ZimaOS 的信息吗" {
		t.Fatalf("unexpected tool query: %q", cc.ToolQuery)
	}
}

func TestDeriveContinuationContext_AffirmativeWithSoftConsentOffer(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "帮我查 BlueAgent 最近动态"},
		{
			Role:    llm.RoleAssistant,
			Content: "为了避免误导，我建议只看高可信来源。如果你同意，我下一步会按这个范围整理（1-2分钟）：\n1) 官方公告\n2) 主流科技媒体",
		},
		{Role: llm.RoleUser, Content: "继续"},
	}

	cc := deriveContinuationContext("继续", msgs)
	if strings.TrimSpace(cc.Hint) == "" {
		t.Fatal("expected continuation hint for short affirmative reply after soft consent offer")
	}
	if cc.ToolQuery != "帮我查 BlueAgent 最近动态" {
		t.Fatalf("unexpected tool query: %q", cc.ToolQuery)
	}
}

func TestProcessChannelMessage_AffirmativeContinuationUsesStoredObjective(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_affirmative_continue")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "帮我查询一下伊朗今天的战况"},
		{Role: "assistant", Content: "为了不给你错误信息，我建议按高可信来源整理。如果你同意，我下一步会按这个范围整理（1-2分钟）：\n1) 官方渠道\n2) 主流媒体"},
	}
	for _, msg := range seed {
		if _, err := store.AddMessage(context.Background(), convID, msg); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-affirmative-continue",
		responses: []llm.ChatResponse{{
			ID:      "im-continue-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "已继续整理。"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_affirmative_continue",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "同意",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if got := strings.TrimSpace(resp); got != "已继续整理。" {
		t.Fatalf("unexpected response: %q", got)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}

	foundTarget := false
	for _, msg := range firstReq.Messages {
		if msg.Role != llm.RoleSystem {
			continue
		}
		if strings.Contains(msg.Content, "Continuation target: resume the user's original objective: 帮我查询一下伊朗今天的战况") {
			foundTarget = true
			break
		}
	}
	if !foundTarget {
		t.Fatalf("expected continuation target system message in request, got %+v", firstReq.Messages)
	}
}

func TestDeriveContinuationContext_DoesNotForceWhenAwaitingWithoutDefault(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "查天气"},
		{Role: llm.RoleAssistant, Content: "- [ ] 确认城市\n- [ ] 查询天气\n你要查哪个城市？"},
		{Role: llm.RoleUser, Content: "好的"},
	}

	cc := deriveContinuationContext("好的", msgs)
	if cc.Hint != "" || cc.ToolQuery != "" {
		t.Fatalf("expected no continuation context, got hint=%q tool_query=%q", cc.Hint, cc.ToolQuery)
	}
}

func TestDeriveContinuationContextFromRecentMessages_RejectsInterveningObjective(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	msgs := []memory.Message{
		{Role: "user", Content: "请帮我评审 zimaspace.com/zimaos 的 UI", CreatedAt: now.Add(-10 * time.Minute)},
		{Role: "assistant", Content: "- [ ] 访问 zimaspace.com/zimaos 并截取页面快照\n- [ ] 生成 UI 质量报告", CreatedAt: now.Add(-9 * time.Minute)},
		{Role: "user", Content: "今天天气怎么样", CreatedAt: now.Add(-4 * time.Minute)},
		{Role: "user", Content: "好的", CreatedAt: now},
	}

	cc := deriveContinuationContextFromRecentMessages("好的", msgs, now)
	if cc.Hint != "" || cc.ToolQuery != "" {
		t.Fatalf("expected stale continuation to be rejected, got hint=%q tool_query=%q", cc.Hint, cc.ToolQuery)
	}
}

func TestDeriveContinuationContextFromRecentMessages_RejectsExpiredTask(t *testing.T) {
	now := time.Date(2026, 3, 12, 10, 0, 0, 0, time.UTC)
	msgs := []memory.Message{
		{Role: "user", Content: "请帮我评审 zimaspace.com/zimaos 的 UI", CreatedAt: now.Add(-(shortAffirmativeContinuationTTL + 5*time.Minute))},
		{Role: "assistant", Content: "- [ ] 访问 zimaspace.com/zimaos 并截取页面快照\n- [ ] 生成 UI 质量报告", CreatedAt: now.Add(-(shortAffirmativeContinuationTTL + 4*time.Minute))},
		{Role: "user", Content: "继续", CreatedAt: now},
	}

	cc := deriveContinuationContextFromRecentMessages("继续", msgs, now)
	if cc.Hint != "" || cc.ToolQuery != "" {
		t.Fatalf("expected expired continuation to be rejected, got hint=%q tool_query=%q", cc.Hint, cc.ToolQuery)
	}
}

func TestProcessChannelMessage_AffirmativeContinuationDoesNotReviveOldTaskAfterNewUserGoals(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_affirmative_ignore_old_task")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "请帮我评审 zimaspace.com/zimaos 的 UI"},
		{Role: "assistant", Content: "- [ ] 访问 zimaspace.com/zimaos 并截取页面快照\n- [ ] 生成 UI 质量报告"},
	}
	for i := 1; i <= 12; i++ {
		seed = append(seed, memory.Message{Role: "user", Content: fmt.Sprintf("later-user-%02d", i)})
	}
	for _, msg := range seed {
		if _, err := store.AddMessage(context.Background(), convID, msg); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-ignore-old-task",
		responses: []llm.ChatResponse{{
			ID:      "im-ignore-old-task-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "收到。"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())

	if _, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_affirmative_ignore_old_task",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "好的",
	}); err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	for _, msg := range firstReq.Messages {
		if msg.Role != llm.RoleSystem {
			continue
		}
		if strings.Contains(msg.Content, "Continuation target: resume the user's original objective: 请帮我评审 zimaspace.com/zimaos 的 UI") {
			t.Fatalf("expected old UI review objective to be ignored after newer user goals, got %+v", firstReq.Messages)
		}
	}
}

func TestIsAffirmativeContinuationMessage(t *testing.T) {
	cases := map[string]bool{
		"好的":        true,
		"继续吧":       true,
		"ok":        true,
		"sure":      true,
		"请继续执行":     true,
		"sí":        true,
		"continue":  true,
		"ja":        true,
		"sim":       true,
		"да":        true,
		"はい":        true,
		"계속":        true,
		"我想换个主题聊电影": false,
	}
	for input, want := range cases {
		if got := isAffirmativeContinuationMessage(input); got != want {
			t.Fatalf("input=%q got=%v want=%v", input, got, want)
		}
	}
}

// Test ChatHandler creation
func TestNewChatHandler(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()

	handler := NewChatHandler(store, registry, toolRegistry)
	if handler == nil {
		t.Fatal("expected handler, got nil")
	}
}

func TestDefaultModelForRuntime(t *testing.T) {
	h := &ChatHandler{}
	if got := h.defaultModelForRuntime(""); got != defaultRuntimeModel {
		t.Fatalf("default model without routing metadata = %q, want %q", got, defaultRuntimeModel)
	}
	if got := h.defaultModelForRuntime("auto"); got != "auto" {
		t.Fatalf("auto model without routing metadata = %q, want %q", got, "auto")
	}
	if got := h.defaultModelForRuntime("gpt-4o"); got != "gpt-4o" {
		t.Fatalf("explicit model = %q, want %q", got, "gpt-4o")
	}

	newPoolWithModel := func(t *testing.T, providerEnabled, modelEnabled bool) *providerpool.Pool {
		t.Helper()

		storage, err := providerpool.NewFileStorage(t.TempDir())
		if err != nil {
			t.Fatalf("create provider storage: %v", err)
		}
		registry, err := providerpool.NewRegistry(storage)
		if err != nil {
			t.Fatalf("create provider registry: %v", err)
		}
		provider := &providerpool.Provider{
			ID:      "p-test",
			Name:    "p-test",
			Type:    providerpool.ProviderTypeCustom,
			Enabled: true,
			Status:  providerpool.ProviderStatusActive,
			BaseURL: "https://example.com/v1",
		}
		if err := registry.Register(provider); err != nil {
			t.Fatalf("register provider: %v", err)
		}
		if !providerEnabled {
			if err := registry.Disable(provider.ID); err != nil {
				t.Fatalf("disable provider: %v", err)
			}
		}
		if err := storage.SaveModels(provider.ID, []*providerpool.Model{
			{
				ID:         defaultRuntimeModel,
				Name:       defaultRuntimeModel,
				ProviderID: provider.ID,
				Enabled:    modelEnabled,
			},
		}); err != nil {
			t.Fatalf("save models: %v", err)
		}

		discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
		return &providerpool.Pool{
			Registry:  registry,
			Discovery: discovery,
		}
	}

	t.Run("fallbacks to auto when default model provider is disabled", func(t *testing.T) {
		h2 := &ChatHandler{}
		h2.SetProviderPool(newPoolWithModel(t, false, true))
		if got := h2.defaultModelForRuntime(""); got != "auto" {
			t.Fatalf("empty model with disabled provider = %q, want auto", got)
		}
	})

	t.Run("fallbacks to auto when default model probe marked unavailable", func(t *testing.T) {
		h2 := &ChatHandler{}
		h2.SetProviderPool(newPoolWithModel(t, true, false))
		if got := h2.defaultModelForRuntime(""); got != "auto" {
			t.Fatalf("empty model with disabled default model = %q, want auto", got)
		}
	})

	t.Run("uses default model when provider and model are available", func(t *testing.T) {
		h2 := &ChatHandler{}
		h2.SetProviderPool(newPoolWithModel(t, true, true))
		if got := h2.defaultModelForRuntime(""); got != defaultRuntimeModel {
			t.Fatalf("empty model with available default model = %q, want %q", got, defaultRuntimeModel)
		}
		if got := h2.defaultModelForRuntime("auto"); got != "auto" {
			t.Fatalf("auto model with available default model = %q, want auto", got)
		}
	})
}

func TestGenerateTitleWithLLM_MarksBackgroundTask(t *testing.T) {
	var gotBackground string
	h := &ChatHandler{
		proxyBridge: proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBackground = r.Header.Get(proxy.BackgroundTaskHeader)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"id":"1","model":"","choices":[{"message":{"role":"assistant","content":"简短标题"},"finish_reason":"stop"}]}`)
		})),
	}

	title := h.generateTitleWithLLM(strings.Repeat("这是一个很长的问题。", 8), "zh")
	if title != "简短标题" {
		t.Fatalf("title = %q, want %q", title, "简短标题")
	}
	if gotBackground != "true" {
		t.Fatalf("expected %s header=true, got %q", proxy.BackgroundTaskHeader, gotBackground)
	}
}

func TestExtractConversationTitleFromAIResponse(t *testing.T) {
	t.Run("keeps semantic markdown heading", func(t *testing.T) {
		content := "## BlueAgent 最新动态\n\n这里是本轮总结。"
		if got := extractConversationTitleFromAIResponse(content); got != "BlueAgent 最新动态" {
			t.Fatalf("title = %q, want %q", got, "BlueAgent 最新动态")
		}
	})

	t.Run("ignores checklist heading", func(t *testing.T) {
		content := "## TODO清单\n- [ ] 收集官方资料\n- [ ] 汇总结果"
		if got := extractConversationTitleFromAIResponse(content); got != "" {
			t.Fatalf("title = %q, want empty", got)
		}
	})
}

func TestGenerateConversationTitle_IgnoresChecklistHeading(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "新对话")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	userMessage := "查一下 BlueAgent 最新动态"
	aiResponse := "## TODO清单\n- [ ] 收集官方资料\n- [ ] 汇总关键变化"

	handler.generateConversationTitle(conv.ID, "", userMessage, aiResponse, "zh")

	updatedConv, err := store.GetConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}
	if updatedConv.Title != userMessage {
		t.Fatalf("conversation title = %q, want %q", updatedConv.Title, userMessage)
	}
}

func TestGenerateConversationTitle_FinalizesOnlyOnce(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "New Conversation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.generateConversationTitle(conv.ID, "", "First short title", "ok", "en")
	handler.generateConversationTitle(conv.ID, "", "Second short title", "## Better Replacement", "en")

	updatedConv, err := store.GetConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}
	if updatedConv.Title != "First short title" {
		t.Fatalf("conversation title = %q, want %q", updatedConv.Title, "First short title")
	}
	if !updatedConv.AutoTitleFinalized {
		t.Fatal("expected auto title to be finalized after first successful update")
	}
}

func TestChatOnce_InjectsLocaleFromSettingsToProxyBridge(t *testing.T) {
	var gotLocale string
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLocale = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_1","model":"auto","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)
	handler.SetProxyBridge(bridge)

	_, err := handler.chatOnce(context.Background(), llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if gotLocale != "fr-FR" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "fr-FR")
	}
}

func TestChatOnce_DoesNotOverrideContextLocale(t *testing.T) {
	var gotLocale string
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotLocale = r.Header.Get("Accept-Language")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_2","model":"auto","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))

	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)
	handler.SetProxyBridge(bridge)

	ctx := proxy.WithLocale(context.Background(), "ja-JP")
	_, err := handler.chatOnce(ctx, llm.ChatRequest{
		Model: "auto",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
	})
	if err != nil {
		t.Fatalf("chatOnce() error = %v", err)
	}
	if gotLocale != "ja-JP" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "ja-JP")
	}
}

func TestProcessChannelMessage_DefaultModel502RollsBackToAuto(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	registry.Register(&scriptedChatProvider{
		name:   "model-catalog",
		models: []string{defaultRuntimeModel},
	})
	handler := NewChatHandler(store, registry, tools.NewRegistry())

	var requestModels []string
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		requestModels = append(requestModels, strings.TrimSpace(body.Model))
		if len(requestModels) == 1 {
			http.Error(w, `upstream 502: {"error":{"message":"Upstream request failed","type":"upstream_error"}}`, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_ok","model":"auto","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	handler.SetProxyBridge(bridge)

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_fallback_auto",
		ID:          "msg_1",
		UserID:      "user_1",
		Content:     "hello",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v (models=%v)", err, requestModels)
	}
	if strings.TrimSpace(resp) == "" {
		t.Fatalf("ProcessChannelMessage() returned empty response")
	}
	if len(requestModels) != 2 {
		t.Fatalf("proxy calls = %d, want 2; models=%v", len(requestModels), requestModels)
	}
	if requestModels[0] != defaultRuntimeModel {
		t.Fatalf("first request model = %q, want %q", requestModels[0], defaultRuntimeModel)
	}
	if requestModels[1] != "auto" {
		t.Fatalf("second request model = %q, want auto", requestModels[1])
	}
}

func TestProcessChannelMessage_GeneratesIMConversationTitleOnlyOnce(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())

	var mu sync.Mutex
	titleRequests := 0
	chatRequests := 0
	bridge := proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		isTitleRequest := false
		for _, msg := range req.Messages {
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Generate a very short title") {
				isTitleRequest = true
				break
			}
		}

		mu.Lock()
		if isTitleRequest {
			titleRequests++
		} else {
			chatRequests++
		}
		currentTitleRequests := titleRequests
		currentChatRequests := chatRequests
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if isTitleRequest {
			title := "OpenClaw 更新"
			if currentTitleRequests > 1 {
				title = "不应覆盖"
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"title_%d","model":"auto","choices":[{"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}]}`, currentTitleRequests, title)))
			return
		}

		content := "好的，我来整理 OpenClaw 最近更新。"
		if currentChatRequests > 1 {
			content = "继续补充一些最新变化。"
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"resp_%d","model":"auto","choices":[{"message":{"role":"assistant","content":"%s"},"finish_reason":"stop"}]}`, currentChatRequests, content)))
	}))
	handler.SetProxyBridge(bridge)

	convID := channelConversationID("feishu", "chat_im_title_once")
	firstResp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_title_once",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "alice",
		Content:     "帮我看一下 OpenClaw 最近更新",
	})
	if err != nil {
		t.Fatalf("first ProcessChannelMessage() error = %v", err)
	}
	if !strings.Contains(firstResp, "OpenClaw") {
		t.Fatalf("unexpected first response %q", firstResp)
	}

	deadline := time.Now().Add(2 * time.Second)
	for {
		conv, err := store.GetConversation(context.Background(), convID)
		if err == nil && conv != nil && conv.Title == "OpenClaw 更新" && conv.AutoTitleFinalized {
			break
		}
		if time.Now().After(deadline) {
			conv, convErr := store.GetConversation(context.Background(), convID)
			if convErr != nil {
				t.Fatalf("timed out waiting for IM title generation; get conversation: %v", convErr)
			}
			t.Fatalf("timed out waiting for IM title generation; title=%q finalized=%v", conv.Title, conv.AutoTitleFinalized)
		}
		time.Sleep(20 * time.Millisecond)
	}

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_title_once",
		ID:          "msg_2",
		UserID:      "user_1",
		Username:    "alice",
		Content:     "继续",
	})
	if err != nil {
		t.Fatalf("second ProcessChannelMessage() error = %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	conv, err := store.GetConversation(context.Background(), convID)
	if err != nil {
		t.Fatalf("failed to load conversation: %v", err)
	}
	if conv.Title != "OpenClaw 更新" {
		t.Fatalf("conversation title = %q, want %q", conv.Title, "OpenClaw 更新")
	}

	mu.Lock()
	defer mu.Unlock()
	if titleRequests != 1 {
		t.Fatalf("title requests = %d, want 1", titleRequests)
	}
	if chatRequests != 2 {
		t.Fatalf("chat requests = %d, want 2", chatRequests)
	}
}

func TestChatHandlerSendMessage_WriteToWhitelistAliasE2E(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Whitelist Alias Write E2E")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	workspaceRoot := t.TempDir()
	whitelistRoot := t.TempDir()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "write-whitelist-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_write_whitelist_1",
							Name:      "write",
							Arguments: `{"path":"@docs/notes/e2e.txt","content":"hello whitelist alias"}`,
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 80, CompletionTokens: 18, TotalTokens: 98},
			},
			{
				ID:    "write-whitelist-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已完成写入。",
				},
				Usage: llm.Usage{PromptTokens: 92, CompletionTokens: 12, TotalTokens: 104},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))

	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	e := echo.New()
	settingsReq := httptest.NewRequest(
		http.MethodPatch,
		"/api/settings",
		strings.NewReader(fmt.Sprintf(`{"directory_whitelist_enabled":true,"directory_whitelist":[{"path":%q,"alias":"docs"}]}`, whitelistRoot)),
	)
	settingsReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	settingsRec := httptest.NewRecorder()
	if err := settings.Patch(e.NewContext(settingsReq, settingsRec)); err != nil {
		t.Fatalf("settings Patch() error = %v", err)
	}
	if settingsRec.Code != http.StatusOK {
		t.Fatalf("settings Patch() status = %d, want 200", settingsRec.Code)
	}

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(settings)

	reqBody := `{"message":"请写入白名单目录文件","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (tool + final), got %d", scripted.CallCount())
	}

	target := filepath.Join(whitelistRoot, "notes", "e2e.txt")
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("expected whitelist file to be written, read error: %v", readErr)
	}
	if string(data) != "hello whitelist alias" {
		t.Fatalf("unexpected whitelist file content: %q", string(data))
	}

	workspaceTarget := filepath.Join(workspaceRoot, "notes", "e2e.txt")
	if _, statErr := os.Stat(workspaceTarget); !os.IsNotExist(statErr) {
		t.Fatalf("expected no mirrored file in workspace root, got stat=%v", statErr)
	}
}

func TestProcessChannelMessage_AutoContinueRetriesEmptyReplyAfterToolRound(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im",
		responses: []llm.ChatResponse{
			{
				ID:    "im-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "我先查一下最近一周的动态。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_1",
						Name:      "web_search",
						Arguments: `{"query":"OpenClaw recent updates"}`,
					}},
				},
			},
			{
				ID:      "im-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: ""},
			},
			{
				ID:      "im-round-3",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{result: map[string]interface{}{
		"query": "OpenClaw recent updates",
		"results": []map[string]interface{}{
			{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "description": "recent release notes"},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_post_tool",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我调研一下最近一周 openclaw 的动向吧",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if !strings.Contains(resp, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final summary, got %q", resp)
	}
	if strings.Contains(resp, "最终总结生成失败") || strings.Contains(resp, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording, got %q", resp)
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + empty + retry), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}
}

func TestChatHandlerSendMessage_PreservesHistoricalFileReadToolResultInPreparedContext(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "File read follow-up send")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	workspaceRoot := t.TempDir()
	sourcePath := filepath.Join(workspaceRoot, "notes.txt")
	rawContent := buildFileReadRegressionContent(240, "payload ")
	if err := os.WriteFile(sourcePath, []byte(rawContent), 0o644); err != nil {
		t.Fatalf("write source file: %v", err)
	}

	readTool := tools.NewFileReadTool([]string{workspaceRoot}, 0)
	readResult, err := readTool.Execute(context.Background(), map[string]interface{}{"path": sourcePath})
	if err != nil {
		t.Fatalf("execute file_read tool: %v", err)
	}
	readPayload, ok := readResult.(string)
	if !ok {
		t.Fatalf("file_read result type = %T, want string", readResult)
	}

	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "请读取 notes.txt 并检查内容",
	}); err != nil {
		t.Fatalf("seed user message: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role: "assistant",
		ToolCalls: []memory.ToolCall{{
			ID:        "call_file_read_seed_1",
			Name:      "file_read",
			Arguments: fmt.Sprintf(`{"path":%q}`, sourcePath),
		}},
	}); err != nil {
		t.Fatalf("seed assistant tool call: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:       "tool",
		Content:    readPayload,
		ToolCallID: "call_file_read_seed_1",
		ToolName:   "file_read",
	}); err != nil {
		t.Fatalf("seed tool result: %v", err)
	}

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"基于刚才读到的本地文件内容，直接告诉我第120行写了什么。不要创建或修改任何文件。","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	lastReq := capture.LastRequest()
	toolMsg, payload := requireToolPayloadMessage(t, lastReq, "file_read")
	if len(toolMsg.Content) > maxLLMToolOutputBytes {
		t.Fatalf("file_read tool message too large: %d", len(toolMsg.Content))
	}

	content, _ := payload["content"].(string)
	if len(content) <= 512 {
		t.Fatalf("expected file_read follow-up content > 512 bytes, got %d", len(content))
	}
	if !strings.Contains(content, "line 120 payload payload ") {
		t.Fatalf("expected preserved mid-file content in follow-up payload, got=%q", content)
	}
	if truncated, _ := payload["truncated"].(bool); truncated {
		t.Fatalf("expected truncated=false for under-budget file_read payload, got true")
	}
}

func TestChatHandlerSendMessage_WebQueryToolFollowUpKeepsNonEmptySummary(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Web query summary send")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-web-query-summary-send",
		responses: []llm.ChatResponse{
			{
				ID:    "web-query-summary-send-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_query_send_summary_1",
						Name:      "web_query",
						Arguments: `{"input":"blue release notes"}`,
					}},
				},
			},
			{
				ID:      "web-query-summary-send-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "我已经整理好 Blue 的更新摘要。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{
		name:   "web_query",
		result: buildWebQueryToolPayloadForLLMTests(strings.Repeat("On April 4, 2026, version 1.2.3 shipped 12 improvements, 4 fixes, and 2 migrations. ", 180)),
	})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"请查一下 Blue 的最近更新，然后给我一个简短总结。","provider":"scripted-web-query-summary-send","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() < 2 {
		t.Fatalf("expected at least 2 LLM rounds (web_query + follow-up), got %d", scripted.CallCount())
	}

	var followUpReq llm.ChatRequest
	foundFollowUp := false
	for idx := 0; idx < scripted.CallCount(); idx++ {
		req, ok := scripted.RequestAt(idx)
		if !ok {
			continue
		}
		for _, msg := range req.Messages {
			if msg.Role == llm.RoleTool && msg.ToolName == "web_query" {
				followUpReq = req
				foundFollowUp = true
				break
			}
		}
		if foundFollowUp {
			break
		}
	}
	if !foundFollowUp {
		t.Fatalf("missing follow-up request carrying web_query tool result")
	}

	toolMsg, payload := requireToolPayloadMessage(t, followUpReq, "web_query")
	if len(toolMsg.Content) > maxLLMToolOutputBytes {
		t.Fatalf("web_query tool message too large: %d", len(toolMsg.Content))
	}
	if got, ok := payload["has_results"].(bool); !ok || !got {
		t.Fatalf("has_results = %#v, want true", payload["has_results"])
	}
	if _, ok := payload["selected_result"].(map[string]interface{}); !ok {
		t.Fatalf("selected_result = %#v, want object", payload["selected_result"])
	}
	facts, ok := payload["key_facts"].([]interface{})
	if !ok || len(facts) == 0 {
		t.Fatalf("key_facts = %#v, want non-empty array", payload["key_facts"])
	}
	if got, ok := payload["llm_compacted"].(bool); !ok || !got {
		t.Fatalf("llm_compacted = %#v, want true", payload["llm_compacted"])
	}
}

func TestProcessChannelMessage_AutoContinueRetriesEmptyReplyAfterWebQueryToolRound(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-web-query",
		responses: []llm.ChatResponse{
			{
				ID:    "im-web-query-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "我先查一下最近一周的动态。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_query_1",
						Name:      "web_query",
						Arguments: `{"input":"OpenClaw recent updates"}`,
					}},
				},
			},
			{
				ID:      "im-web-query-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: ""},
			},
			{
				ID:      "im-web-query-round-3",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name: "web_query",
		result: map[string]interface{}{
			"status":     "ok",
			"mode":       "search_read",
			"input":      "OpenClaw recent updates",
			"query":      "OpenClaw recent updates",
			"title":      "OpenClaw Release Notes",
			"target_url": "https://github.com/opendungeons/openclaw/releases",
			"final_url":  "https://github.com/opendungeons/openclaw/releases",
			"sources": []map[string]interface{}{
				{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "snippet": "recent release notes", "selected": true},
			},
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_post_web_query_tool",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我调研一下最近一周 openclaw 的动向吧",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if !strings.Contains(resp, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final summary, got %q", resp)
	}
	if strings.Contains(resp, "最终总结生成失败") || strings.Contains(resp, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording, got %q", resp)
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + empty + retry), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "OpenClaw recent updates" {
		t.Fatalf("web_query input = %q, want OpenClaw recent updates", input)
	}
}

func TestProcessChannelMessage_RecoversPseudoDirectXMLToolCallIntoRealToolExecution(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-pseudo-xml",
		responses: []llm.ChatResponse{
			{
				ID:    "im-pseudo-xml-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: `<web_query><input>Apple AAPL stock price today2026</input></web_query>`,
				},
			},
			{
				ID:      "im-pseudo-xml-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "AAPL 当前股价页面已获取，并已基于真实工具结果完成总结。"},
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
			"input":       "Apple AAPL stock price today2026",
			"query":       "Apple AAPL stock price today2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "im mock finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_pseudo_xml",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "Apple AAPL stock price today2026",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if !strings.Contains(resp, "AAPL 当前股价页面已获取，并已基于真实工具结果完成总结。") {
		t.Fatalf("expected final summary, got %q", resp)
	}
	if strings.Contains(resp, "<web_query>") || strings.Contains(resp, "<input>Apple AAPL stock price today2026</input>") {
		t.Fatalf("expected recovered pseudo xml to be removed from IM response, got %q", resp)
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "Apple AAPL stock price today2026" {
		t.Fatalf("web_query input = %q, want Apple AAPL stock price today2026", input)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "im mock finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered IM tool execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered IM web_query tool result, got %#v", secondReq.Messages)
	}

	messages, err := store.GetMessages(context.Background(), "feishu:chat_im_pseudo_xml", 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "<web_query>") || strings.Contains(m.Content, "<input>Apple AAPL stock price today2026</input>") {
			t.Fatalf("expected recovered pseudo xml to be discarded from persisted IM assistant messages, got=%q", m.Content)
		}
	}
}

func TestProcessChannelMessage_AutoContinueRetriesToollessPendingTodoInAgentMode(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-agent-todo",
		responses: []llm.ChatResponse{
			{
				ID:    "im-agent-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "开始处理。\n\n- [ ] 搜索伊朗今天的战况\n- [ ] 汇总关键进展\n\n我先去搜索一下。",
				},
			},
			{
				ID:    "im-agent-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "先检索一下最新战况。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_ir_1",
						Name:      "web_search",
						Arguments: `{"query":"Iran latest battlefield updates today"}`,
					}},
				},
			},
			{
				ID:    "im-agent-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "任务已完成。伊朗今天的战况要点包括：前线动态、袭击通报和主要来源。",
				},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{result: map[string]interface{}{
		"query": "Iran latest battlefield updates today",
		"results": []map[string]interface{}{
			{"title": "Live updates", "url": "https://example.com/live", "description": "latest updates"},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_agent_pending_todo",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我查询一下伊朗今天的战况",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}
	if !strings.Contains(resp, "任务已完成") {
		t.Fatalf("expected final summary after IM auto-continue, got %q", resp)
	}
	if scripted.CallCount() != 5 {
		t.Fatalf("expected 5 LLM rounds with todo reconciliation and final completion, got %d", scripted.CallCount())
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	last := secondReq.Messages[len(secondReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "A canonical TODO checklist already exists") {
		t.Fatalf("expected pending_todo continuation nudge in second request, got role=%s content=%q", last.Role, last.Content)
	}
}

func TestProcessChannelMessage_FreshStandaloneTopicKeepsOldIMHistoryInAgentMode(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_fresh_topic")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "请帮我评审 zimaspace.com/zimaos 的 UI"},
		{Role: "assistant", Content: "- [ ] 访问 zimaspace.com/zimaos 并截取页面快照\n- [ ] 生成 UI 质量报告"},
	}
	for _, m := range seed {
		if _, err := store.AddMessage(context.Background(), convID, m); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-fresh-topic",
		responses: []llm.ChatResponse{{
			ID:      "fresh-topic-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_fresh_topic",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我查询一下伊朗今天的战况",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	nonSystem := make([]llm.Message, 0, len(firstReq.Messages))
	foundOldHistory := false
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "zimaspace.com/zimaos") {
			foundOldHistory = true
		}
		if msg.Role != llm.RoleSystem {
			nonSystem = append(nonSystem, msg)
		}
	}
	if !foundOldHistory {
		t.Fatalf("expected prior IM history to remain when auto fresh-topic switching is disabled, messages=%+v", firstReq.Messages)
	}
	if len(nonSystem) < 3 {
		t.Fatalf("expected prior history plus current user turn in non-system context, got %d messages", len(nonSystem))
	}
	last := nonSystem[len(nonSystem)-1]
	if last.Role != llm.RoleUser || last.Content != "帮我查询一下伊朗今天的战况" {
		t.Fatalf("expected current user turn at the end, got %+v", last)
	}
}

func TestProcessChannelMessage_IMHistoryLimitZeroDoesNotForceHistoryDrop(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_history_limit_zero")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "Q1 old"},
		{Role: "assistant", Content: "A1 old"},
		{Role: "user", Content: "Q2 old"},
		{Role: "assistant", Content: "A2 old"},
	}
	for _, m := range seed {
		if _, err := store.AddMessage(context.Background(), convID, m); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-history-limit-zero",
		responses: []llm.ChatResponse{{
			ID:      "history-limit-zero-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	zero := 0
	settingsHandler.settings.IMHistoryLimit = &zero
	handler.SetSettingsHandler(settingsHandler)

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_history_limit_zero",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "今天天气怎么样",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	nonSystem := make([]llm.Message, 0, len(firstReq.Messages))
	foundOldHistory := false
	for _, msg := range firstReq.Messages {
		if strings.Contains(msg.Content, "Q1 old") || strings.Contains(msg.Content, "Q2 old") ||
			strings.Contains(msg.Content, "A1 old") || strings.Contains(msg.Content, "A2 old") {
			foundOldHistory = true
		}
		if msg.Role != llm.RoleSystem {
			nonSystem = append(nonSystem, msg)
		}
	}
	if !foundOldHistory {
		t.Fatalf("expected IM history to remain below the pressure threshold even when im_history_limit=0, messages=%+v", firstReq.Messages)
	}
	last := nonSystem[len(nonSystem)-1]
	if last.Role != llm.RoleUser || last.Content != "今天天气怎么样" {
		t.Fatalf("expected current user turn at the end, got %+v", last)
	}
}

func TestProcessChannelMessage_IMHistoryLimitMetadataOverrideDoesNotForceHistoryDrop(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_history_limit_override")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "Q1 old"},
		{Role: "assistant", Content: "A1 old"},
		{Role: "user", Content: "Q2 old"},
		{Role: "assistant", Content: "A2 old"},
	}
	for _, m := range seed {
		if _, err := store.AddMessage(context.Background(), convID, m); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-history-limit-override",
		responses: []llm.ChatResponse{{
			ID:      "history-limit-override-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	zero := 0
	settingsHandler.settings.IMHistoryLimit = &zero
	handler.SetSettingsHandler(settingsHandler)

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_history_limit_override",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "今天天气怎么样",
		Metadata: map[string]interface{}{
			"im_history_limit": 2,
		},
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	nonSystem := make([]llm.Message, 0, len(firstReq.Messages))
	for _, msg := range firstReq.Messages {
		if msg.Role != llm.RoleSystem {
			nonSystem = append(nonSystem, msg)
		}
	}
	if len(nonSystem) != 5 {
		t.Fatalf("expected full non-system history to remain with metadata override below threshold, got %d", len(nonSystem))
	}
	if nonSystem[0].Role != llm.RoleUser || nonSystem[0].Content != "Q1 old" {
		t.Fatalf("messages[0] = %+v, want user/Q1 old", nonSystem[0])
	}
	if nonSystem[1].Role != llm.RoleAssistant || nonSystem[1].Content != "A1 old" {
		t.Fatalf("messages[1] = %+v, want assistant/A1 old", nonSystem[1])
	}
	if nonSystem[4].Role != llm.RoleUser || nonSystem[4].Content != "今天天气怎么样" {
		t.Fatalf("messages[4] = %+v, want user/current", nonSystem[4])
	}
}

func TestProcessChannelMessage_IMHistoryLimitDynamicBoostForPendingContinuation(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_history_limit_dynamic")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}
	seed := []memory.Message{
		{Role: "user", Content: "Q1 old"},
		{Role: "assistant", Content: "A1 old"},
		{Role: "user", Content: "Q2 old"},
		{Role: "assistant", Content: "A2 old"},
		{Role: "user", Content: "Q3 old"},
		{Role: "assistant", Content: "A3 old"},
		{Role: "user", Content: "Q4 old"},
		{Role: "assistant", Content: "A4 old"},
		{Role: "user", Content: "Q5 old"},
		{Role: "assistant", Content: "- [ ] 调研结果复核\n- [ ] 输出最终结论"},
	}
	for _, m := range seed {
		if _, err := store.AddMessage(context.Background(), convID, m); err != nil {
			t.Fatalf("AddMessage: %v", err)
		}
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-history-limit-dynamic",
		responses: []llm.ChatResponse{{
			ID:      "history-limit-dynamic-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	// Keep settings default (no explicit im_history_limit) so dynamic strategy applies.
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_history_limit_dynamic",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "好",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	nonSystem := make([]llm.Message, 0, len(firstReq.Messages))
	for _, msg := range firstReq.Messages {
		if msg.Role != llm.RoleSystem {
			nonSystem = append(nonSystem, msg)
		}
	}
	containsQ1 := false
	for _, msg := range nonSystem {
		if msg.Role == llm.RoleUser && msg.Content == "Q1 old" {
			containsQ1 = true
			break
		}
	}
	if !containsQ1 {
		t.Fatalf("expected dynamic IM history window to include older round Q1 old, got non-system=%+v", nonSystem)
	}
}

func TestProcessChannelMessage_NoAutoFreshStandaloneTopicKeepsPreviousResponseID(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	convID := channelConversationID("feishu", "chat_im_prev_resp_reset")
	if _, err := store.CreateConversationWithID(context.Background(), convID, "feishu - user_1"); err != nil {
		t.Fatalf("CreateConversationWithID: %v", err)
	}

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-prev-id-reset",
		responses: []llm.ChatResponse{{
			ID:      "fresh-topic-prev-id-round-1",
			Model:   "gpt-5.3-codex-spark",
			Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"},
		}},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)
	handler.imModel = "gpt-5.3-codex-spark"
	handler.setPreviousResponseID(convID, "resp_old_im_1")

	_, err = handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_prev_resp_reset",
		ID:          "msg_1",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "帮我查询一下伊朗今天的战况",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() error = %v", err)
	}

	firstReq, ok := scripted.RequestAt(0)
	if !ok {
		t.Fatalf("missing first request capture")
	}
	if firstReq.PreviousResponseID == "" {
		t.Fatalf("expected previous_response_id to be preserved when auto fresh-topic switching is disabled")
	}
}

func TestProcessChannelMessage_CheckpointResumeRetriesEmptyReplyAfterToolRound(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-im-resume",
		responses: []llm.ChatResponse{
			{
				ID:      "resume-round-1",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: ""},
			},
			{
				ID:      "resume-round-2",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。"},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{result: map[string]interface{}{
		"query": "OpenClaw recent updates",
		"results": []map[string]interface{}{
			{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "description": "recent release notes"},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	mgr := tools.NewBrowserCheckpointManager(2 * time.Minute)
	handler.SetBrowserCheckpointManager(mgr)

	convID := channelConversationID("feishu", "chat_im_resume")
	checkpoint := mgr.Create(tools.BrowserCheckpointRequest{
		SessionID: convID,
		Required:  true,
		RiskLevel: "high",
		Step:      "research",
		Action:    "search",
	})
	pendingTool := llm.ToolCall{ID: "call_web_resume_1", Name: "web_search", Arguments: `{"query":"OpenClaw recent updates"}`}
	handler.setIMCheckpointState(convID, &imCheckpointResumeState{
		CheckpointID:    checkpoint.ID,
		ConversationID:  convID,
		ChannelName:     "feishu",
		ChatID:          "chat_im_resume",
		ReplyToID:       "msg_prev",
		Lang:            i18n.DefaultLanguage,
		AgentMode:       false,
		RoutingMessage:  "帮我调研一下最近一周 openclaw 的动向吧",
		CreatedAt:       time.Now(),
		ResumeReq:       llm.ChatRequest{Model: "gpt-5.3-codex-spark"},
		AssistantMsg:    llm.Message{Role: llm.RoleAssistant, Content: "我先查一下最近一周的动态。", ToolCalls: []llm.ToolCall{pendingTool}},
		PendingToolCall: pendingTool,
	})

	resp, err := handler.ProcessChannelMessage(context.Background(), channel.Message{
		ChannelName: "feishu",
		ChatID:      "chat_im_resume",
		ID:          "msg_confirm",
		UserID:      "user_1",
		Username:    "user_1",
		Content:     "1",
	})
	if err != nil {
		t.Fatalf("ProcessChannelMessage() resume error = %v", err)
	}
	if !strings.Contains(resp, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final summary, got %q", resp)
	}
	if strings.Contains(resp, "最终总结生成失败") || strings.Contains(resp, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording, got %q", resp)
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (empty + retry) after resumed tool execution, got %d", scripted.CallCount())
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	last := secondReq.Messages[len(secondReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in resumed request, got role=%s content=%q", last.Role, last.Content)
	}
}

func TestShouldRollbackIMDefaultModelToAuto(t *testing.T) {
	t.Run("returns true for default model on 5xx", func(t *testing.T) {
		err := &proxybridge.ProxyError{StatusCode: http.StatusBadGateway, Body: "upstream 502"}
		if !shouldRollbackIMDefaultModelToAuto(defaultRuntimeModel, err) {
			t.Fatalf("shouldRollbackIMDefaultModelToAuto() = false, want true")
		}
	})

	t.Run("returns true for default model on overload", func(t *testing.T) {
		err := &proxybridge.ProxyError{StatusCode: http.StatusTooManyRequests, Body: "rate limit"}
		if !shouldRollbackIMDefaultModelToAuto(defaultRuntimeModel, err) {
			t.Fatalf("shouldRollbackIMDefaultModelToAuto() = false, want true")
		}
	})

	t.Run("returns false for non-default model", func(t *testing.T) {
		err := &proxybridge.ProxyError{StatusCode: http.StatusBadGateway, Body: "upstream 502"}
		if shouldRollbackIMDefaultModelToAuto("auto", err) {
			t.Fatalf("shouldRollbackIMDefaultModelToAuto() = true, want false")
		}
	})

	t.Run("returns true for default model on no-provider error", func(t *testing.T) {
		err := &proxybridge.ProxyError{StatusCode: http.StatusServiceUnavailable, Body: "no available provider"}
		if !shouldRollbackIMDefaultModelToAuto(defaultRuntimeModel, err) {
			t.Fatalf("shouldRollbackIMDefaultModelToAuto() = false, want true")
		}
	})
}

func TestSendMessage_PropagatesSettingsLocaleToUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "nonstream locale from settings"
	)

	var gotLocale string
	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload)
		if bytes.Contains(raw, []byte(msgText)) {
			gotLocale = strings.TrimSpace(r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_nonstream_locale","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "nonstream-locale-settings-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Non-stream locale settings")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "zh-CN"
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"`+msgText+`","model":"`+modelID+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotLocale != "zh-CN" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "zh-CN")
	}
}

func TestSendMessage_ContextLocaleOverridesSettingsForUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "nonstream locale from context"
	)

	var gotLocale string
	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		raw, _ := json.Marshal(payload)
		if bytes.Contains(raw, []byte(msgText)) {
			gotLocale = strings.TrimSpace(r.Header.Get("Accept-Language"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_nonstream_locale_ctx","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "nonstream-locale-context-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Non-stream locale context")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"`+msgText+`","model":"`+modelID+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(proxy.WithLocale(req.Context(), "ja-JP"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotLocale != "ja-JP" {
		t.Fatalf("Accept-Language = %q, want %q", gotLocale, "ja-JP")
	}
}

// Test CreateConversation endpoint
func TestChatHandlerCreateConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"title": "Test Conversation"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] == nil {
		t.Error("expected id in response")
	}
	if resp["title"] != "Test Conversation" {
		t.Errorf("expected title 'Test Conversation', got '%v'", resp["title"])
	}
}

// Test ListConversations endpoint
func TestChatHandlerListConversations(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	// Create some conversations
	store.CreateConversation(context.Background(), "Conv 1")
	store.CreateConversation(context.Background(), "Conv 2")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListConversations(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(resp))
	}
}

func TestChatHandlerListConversationsPreviewIncludesLegacyData(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	if _, err := store.CreateConversation(context.Background(), "Legacy Conv"); err != nil {
		t.Fatalf("CreateConversation legacy: %v", err)
	}
	if _, err := store.CreateConversation(context.Background(), "Preview Conv", "preview-user"); err != nil {
		t.Fatalf("CreateConversation preview: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "preview-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListConversations(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(resp))
	}

	titles := map[string]bool{}
	for _, conv := range resp {
		if title, _ := conv["title"].(string); title != "" {
			titles[title] = true
		}
	}
	if !titles["Legacy Conv"] || !titles["Preview Conv"] {
		t.Fatalf("expected preview list to include legacy and preview conversations, got %#v", titles)
	}
}

func TestChatHandlerListConversationsQueryIncludesSessionAuditRecall(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "Maintenance Notes")
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

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetPersistenceOptions(true, true, true)
	handler.SetSessionAuditStore(auditStore)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	if msgID := handler.persistBestEffortMessageContent("", conv.ID, "user", "Router rollback failed after the firmware update", "", "", nil, false); strings.TrimSpace(msgID) == "" {
		t.Fatalf("persistBestEffortMessageContent() returned empty message id")
	}
	handler.persistCoordinator.FlushConversation(conv.ID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?q=rollback", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListConversations(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []memory.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(resp))
	}
	if resp[0].ID != conv.ID {
		t.Fatalf("response[0].ID = %q, want %q", resp[0].ID, conv.ID)
	}
	if resp[0].Title != conv.Title {
		t.Fatalf("response[0].Title = %q, want %q", resp[0].Title, conv.Title)
	}
}

func TestChatHandlerListConversationsQueryMergesTitleAndAuditMatchesWithoutDuplicates(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	titleMatch, err := store.CreateConversation(ctx, "Rollback Tracker")
	if err != nil {
		t.Fatalf("CreateConversation titleMatch: %v", err)
	}
	auditMatch, err := store.CreateConversation(ctx, "Maintenance Notes")
	if err != nil {
		t.Fatalf("CreateConversation auditMatch: %v", err)
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

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetPersistenceOptions(true, true, true)
	handler.SetSessionAuditStore(auditStore)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	if msgID := handler.persistBestEffortMessageContent("", titleMatch.ID, "assistant", "Rollback plan captured in the last run", "", "", nil, false); strings.TrimSpace(msgID) == "" {
		t.Fatalf("persistBestEffortMessageContent(titleMatch) returned empty message id")
	}
	if msgID := handler.persistBestEffortMessageContent("", auditMatch.ID, "user", "Router rollback failed after the firmware update", "", "", nil, false); strings.TrimSpace(msgID) == "" {
		t.Fatalf("persistBestEffortMessageContent(auditMatch) returned empty message id")
	}
	handler.persistCoordinator.FlushConversation(titleMatch.ID)
	handler.persistCoordinator.FlushConversation(auditMatch.ID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?q=rollback", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListConversations(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []memory.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp) != 2 {
		t.Fatalf("len(response) = %d, want 2", len(resp))
	}

	seen := map[string]int{}
	for _, conv := range resp {
		seen[conv.ID]++
	}
	if seen[titleMatch.ID] != 1 {
		t.Fatalf("titleMatch occurrences = %d, want 1; response = %+v", seen[titleMatch.ID], resp)
	}
	if seen[auditMatch.ID] != 1 {
		t.Fatalf("auditMatch occurrences = %d, want 1; response = %+v", seen[auditMatch.ID], resp)
	}
}

func TestChatHandlerListConversationsQueryIncludesSessionAuditRecallForScopedUser(t *testing.T) {
	ctx := context.Background()
	dbDir := t.TempDir()

	store, err := memory.NewStoreWithOptions(filepath.Join(dbDir, "chat.db"), memory.DefaultChatStoreOptions(filepath.Join(dbDir, "chat.db")))
	if err != nil {
		t.Fatalf("NewStoreWithOptions: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(ctx, "Maintenance Notes", "scoped-user")
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

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetPersistenceOptions(true, true, true)
	handler.SetSessionAuditStore(auditStore)
	defer handler.Close()
	if handler.persistCoordinator != nil {
		defer handler.persistCoordinator.ShutdownFlush()
	}

	if msgID := handler.persistBestEffortMessageContent("", conv.ID, "user", "Rollback checklist for scoped user conversation", "", "", nil, false); strings.TrimSpace(msgID) == "" {
		t.Fatalf("persistBestEffortMessageContent() returned empty message id")
	}
	handler.persistCoordinator.FlushConversation(conv.ID)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations?q=rollback", nil)
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "scoped-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListConversations(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []memory.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(resp) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(resp))
	}
	if resp[0].ID != conv.ID {
		t.Fatalf("response[0].ID = %q, want %q", resp[0].ID, conv.ID)
	}
}

// Test GetConversation endpoint
func TestChatHandlerGetConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["id"] != conv.ID {
		t.Errorf("expected id '%s', got '%v'", conv.ID, resp["id"])
	}
}

// Test GetConversation not found
func TestChatHandlerGetConversationNotFound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("nonexistent")

	err := handler.GetConversation(c)
	if err == nil {
		t.Error("expected error for nonexistent conversation")
	}
}

// Test DeleteConversation endpoint
func TestChatHandlerDeleteConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "To Delete")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/conversations/"+conv.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.DeleteConversation(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}
}

// Test GetMessages endpoint
func TestChatHandlerGetMessages(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Hello"})
	store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "Hi!"})

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetMessages(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Errorf("expected 2 messages, got %d", len(resp))
	}
}

func TestChatHandlerGetMessages_PaginatesFromLatest(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	for i := 1; i <= 5; i++ {
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: fmt.Sprintf("m%d", i)})
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	run := func(offset int) []memory.Message {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/conversations/%s/messages?limit=2&offset=%d", conv.ID, offset), nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)

		err := handler.GetMessages(c)
		if err != nil {
			t.Fatalf("handler error (offset=%d): %v", offset, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status (offset=%d) = %d, want 200", offset, rec.Code)
		}

		var resp []memory.Message
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response (offset=%d): %v", offset, err)
		}
		return resp
	}

	page0 := run(0)
	if len(page0) != 2 || page0[0].Content != "m4" || page0[1].Content != "m5" {
		t.Fatalf("page0 = %#v, want [m4 m5]", page0)
	}

	page1 := run(2)
	if len(page1) != 2 || page1[0].Content != "m2" || page1[1].Content != "m3" {
		t.Fatalf("page1 = %#v, want [m2 m3]", page1)
	}

	page2 := run(4)
	if len(page2) != 1 || page2[0].Content != "m1" {
		t.Fatalf("page2 = %#v, want [m1]", page2)
	}

	page3 := run(6)
	if len(page3) != 0 {
		t.Fatalf("page3 len = %d, want 0", len(page3))
	}
}

func TestChatHandlerGetMessages_PaginatesFromLatest_LargeConversation(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Large Conv")
	for i := 1; i <= 120; i++ {
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: fmt.Sprintf("m%03d", i)})
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	run := func(limit, offset int) []memory.Message {
		req := httptest.NewRequest(
			http.MethodGet,
			fmt.Sprintf("/api/v1/conversations/%s/messages?limit=%d&offset=%d", conv.ID, limit, offset),
			nil,
		)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)

		err := handler.GetMessages(c)
		if err != nil {
			t.Fatalf("handler error (offset=%d): %v", offset, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status (offset=%d) = %d, want 200", offset, rec.Code)
		}

		var resp []memory.Message
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response (offset=%d): %v", offset, err)
		}
		return resp
	}

	page0 := run(50, 0)
	if len(page0) != 50 || page0[0].Content != "m071" || page0[49].Content != "m120" {
		t.Fatalf("page0 bounds = [%s ... %s], len=%d, want [m071 ... m120], len=50", page0[0].Content, page0[len(page0)-1].Content, len(page0))
	}

	page1 := run(50, 50)
	if len(page1) != 50 || page1[0].Content != "m021" || page1[49].Content != "m070" {
		t.Fatalf("page1 bounds = [%s ... %s], len=%d, want [m021 ... m070], len=50", page1[0].Content, page1[len(page1)-1].Content, len(page1))
	}

	page2 := run(50, 100)
	if len(page2) != 20 || page2[0].Content != "m001" || page2[19].Content != "m020" {
		t.Fatalf("page2 bounds = [%s ... %s], len=%d, want [m001 ... m020], len=20", page2[0].Content, page2[len(page2)-1].Content, len(page2))
	}

	page3 := run(50, 150)
	if len(page3) != 0 {
		t.Fatalf("page3 len = %d, want 0", len(page3))
	}
}

// Test SendMessage endpoint with mock provider
func TestChatHandlerRegisterRoutes_SendMessageBodyTooLarge(t *testing.T) {
	e := echo.New()
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.RegisterRoutes(e.Group("/api/v1"))

	payload := `{"message":"` + strings.Repeat("a", int(chatRequestBodyLimitBytes)) + `x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/conv-1/messages", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func TestChatHandlerRegisterRoutes_StreamMessageBodyTooLarge(t *testing.T) {
	e := echo.New()
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.RegisterRoutes(e.Group("/api/v1"))

	payload := `{"message":"` + strings.Repeat("a", int(chatRequestBodyLimitBytes)) + `x"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/conv-1/messages/stream", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusRequestEntityTooLarge, rec.Body.String())
	}
}

func newProviderPoolWithContextWindowModel(t *testing.T, modelID string, contextWindow int) *providerpool.Pool {
	t.Helper()

	storage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("create provider storage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("create provider registry: %v", err)
	}
	provider := &providerpool.Provider{
		ID:      "p-context",
		Name:    "p-context",
		Type:    providerpool.ProviderTypeCustom,
		Enabled: true,
		Status:  providerpool.ProviderStatusActive,
		BaseURL: "https://example.com/v1",
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	if err := storage.SaveModels(provider.ID, []*providerpool.Model{{
		ID:            modelID,
		Name:          modelID,
		ProviderID:    provider.ID,
		Enabled:       true,
		ContextWindow: contextWindow,
	}}); err != nil {
		t.Fatalf("save models: %v", err)
	}
	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	return &providerpool.Pool{Registry: registry, Discovery: discovery}
}

type contextWindowModelSpec struct {
	ProviderID    string
	ModelID       string
	ContextWindow int
	InputPrice    float64
	Priority      int
}

func newProviderPoolWithContextWindowModels(t *testing.T, specs []contextWindowModelSpec) *providerpool.Pool {
	t.Helper()

	storage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("create provider storage: %v", err)
	}
	registry, err := providerpool.NewRegistry(storage)
	if err != nil {
		t.Fatalf("create provider registry: %v", err)
	}

	modelsByProvider := make(map[string][]*providerpool.Model)
	for _, spec := range specs {
		priority := spec.Priority
		if priority == 0 {
			priority = 50
		}
		if _, err := registry.Get(spec.ProviderID); err != nil {
			if regErr := registry.Register(&providerpool.Provider{
				ID:       spec.ProviderID,
				Name:     spec.ProviderID,
				Type:     providerpool.ProviderTypeCustom,
				Enabled:  true,
				Status:   providerpool.ProviderStatusActive,
				BaseURL:  "https://example.com/v1",
				Priority: priority,
			}); regErr != nil {
				t.Fatalf("register provider %s: %v", spec.ProviderID, regErr)
			}
		}
		modelsByProvider[spec.ProviderID] = append(modelsByProvider[spec.ProviderID], &providerpool.Model{
			ID:            spec.ModelID,
			Name:          spec.ModelID,
			ProviderID:    spec.ProviderID,
			Enabled:       true,
			ContextWindow: spec.ContextWindow,
			InputPrice:    spec.InputPrice,
		})
	}

	for providerID, models := range modelsByProvider {
		if err := storage.SaveModels(providerID, models); err != nil {
			t.Fatalf("save models for %s: %v", providerID, err)
		}
	}

	discovery := providerpool.NewModelDiscovery(registry, storage, time.Hour)
	return &providerpool.Pool{Registry: registry, Discovery: discovery}
}

func findRepeatCountForBudget(t *testing.T, handler *ChatHandler, smallModel string, fallbackModels []string, base string) string {
	t.Helper()

	for repeat := 200; repeat <= 4000; repeat += 100 {
		content := strings.Repeat(base, repeat)
		messages := []llm.Message{{Role: llm.RoleUser, Content: content}}
		smallBudget := handler.measurePreparedInputBudget(smallModel, 64, messages)
		if !smallBudget.Exceeds() {
			continue
		}
		allFit := true
		for _, modelID := range fallbackModels {
			if handler.measurePreparedInputBudget(modelID, 64, messages).Exceeds() {
				allFit = false
				break
			}
		}
		if allFit {
			return content
		}
	}
	t.Fatalf("failed to find repeat count for %s -> %v", smallModel, fallbackModels)
	return ""
}

func TestChatHandlerSendMessageRejectsEstimatedContextOverflow(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModel(t, "tiny-context-model", 1024))
	if got := handler.resolveContextWindowForModel("tiny-context-model"); got != 1024 {
		t.Fatalf("resolveContextWindowForModel() = %d, want 1024", got)
	}
	if estimate := handler.estimatePreparedInputBudget("tiny-context-model", 64, []llm.Message{{Role: llm.RoleUser, Content: strings.Repeat("Long context block. ", 500)}}); estimate == nil {
		t.Fatal("expected direct input budget estimate to exceed context window")
	}

	e := echo.New()
	body := fmt.Sprintf(`{"message":%q,"model":"tiny-context-model","max_tokens":64}`, strings.Repeat("Long context block. ", 500))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if exceeded, _ := resp["context_window_exceeded"].(bool); !exceeded {
		t.Fatalf("context_window_exceeded = %#v, want true", resp["context_window_exceeded"])
	}
	if got, _ := resp["model"].(string); got != "tiny-context-model" {
		t.Fatalf("model = %q, want %q", got, "tiny-context-model")
	}
	maxInput, _ := resp["max_input_tokens"].(float64)
	estimated, _ := resp["estimated_input_tokens"].(float64)
	if estimated <= maxInput {
		t.Fatalf("estimated_input_tokens = %v, want > max_input_tokens = %v", estimated, maxInput)
	}
}

func TestChatHandlerStreamMessageRejectsEstimatedContextOverflow(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModel(t, "tiny-context-model", 1024))
	if got := handler.resolveContextWindowForModel("tiny-context-model"); got != 1024 {
		t.Fatalf("resolveContextWindowForModel() = %d, want 1024", got)
	}
	if estimate := handler.estimatePreparedInputBudget("tiny-context-model", 64, []llm.Message{{Role: llm.RoleUser, Content: strings.Repeat("Long context block. ", 500)}}); estimate == nil {
		t.Fatal("expected direct input budget estimate to exceed context window")
	}

	e := echo.New()
	body := fmt.Sprintf(`{"message":%q,"model":"tiny-context-model","max_tokens":64}`, strings.Repeat("Long context block. ", 500))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage() error = %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if exceeded, _ := resp["context_window_exceeded"].(bool); !exceeded {
		t.Fatalf("context_window_exceeded = %#v, want true", resp["context_window_exceeded"])
	}
}

func TestResolveSessionTokenBudgetForModelUsesDetectedContextWindow(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModel(t, "large-context-model", 128000))
	handler.SetCompactorMemoryIntegration(nil, session.LegacyDefaultContextTokenBudget)

	want := 128000 - estimateOutputReserveTokens(128000, 0)
	if got := handler.resolveSessionTokenBudgetForModel("large-context-model"); got != want {
		t.Fatalf("resolveSessionTokenBudgetForModel() = %d, want %d", got, want)
	}
}

func TestResolveSessionTokenBudgetForModelHonorsExplicitOverride(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModel(t, "large-context-model", 128000))
	handler.SetCompactorMemoryIntegration(nil, 64000)

	if got := handler.resolveSessionTokenBudgetForModel("large-context-model"); got != 64000 {
		t.Fatalf("resolveSessionTokenBudgetForModel() = %d, want 64000", got)
	}
}

func TestFitPreparedMessagesToBudgetStage1Compaction(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())

	toolJSON, err := json.Marshal(map[string]interface{}{
		"query":   "budget fit",
		"results": []map[string]string{{"title": "Result", "description": strings.Repeat("search evidence ", 300)}},
	})
	if err != nil {
		t.Fatalf("marshal tool payload: %v", err)
	}
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "Investigate the logs"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "web_search", Arguments: `{"query":"budget fit"}`}}},
		{Role: llm.RoleTool, ToolCallID: "call-1", ToolName: "web_search", Content: string(toolJSON)},
		{Role: llm.RoleAssistant, Content: strings.Repeat("analysis ", 120)},
		{Role: llm.RoleUser, Content: "Give me the concise answer"},
	}

	for _, window := range []int{1024, 1280, 1536, 1792, 2048, 2304, 2560} {
		handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
			ProviderID:    "p-context",
			ModelID:       "stage1-model",
			ContextWindow: window,
		}}))
		plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
			ConvID:    "conv-stage1",
			Model:     "stage1-model",
			MaxTokens: 64,
			Messages:  messages,
		})
		current := plan.Current()
		if current == nil || current.Stage != 1 {
			continue
		}
		if current.ContextTrim == nil || current.ContextTrim.Type != "progressive_compaction" {
			t.Fatalf("context trim = %#v, want progressive_compaction", current.ContextTrim)
		}
		if current.Budget.Exceeds() {
			t.Fatalf("stage1 budget still exceeds: %+v", current.Budget)
		}
		if !handler.measurePreparedInputBudget("stage1-model", 64, messages).Exceeds() {
			t.Fatal("expected original prepared budget to exceed")
		}
		return
	}
	t.Fatal("expected at least one context window to fit at stage 1")
}

func TestFitPreparedMessagesToBudgetStage2Summary(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.summaryCache.Put("conv-stage2", &ConversationSummary{
		Text:         "- Cached summary\n- Keep the latest ask concise",
		MessageCount: 8,
	})

	messages := []llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: strings.Repeat(fmt.Sprintf("user-%d background ", i), 120)},
			llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat(fmt.Sprintf("assistant-%d details ", i), 140)},
		)
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "Answer the latest question briefly"})

	for _, window := range []int{1024, 1280, 1536, 1792, 2048, 2304, 2560, 3072, 3584, 4096, 4608, 5120} {
		handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
			ProviderID:    "p-context",
			ModelID:       "stage2-model",
			ContextWindow: window,
		}}))
		plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
			ConvID:    "conv-stage2",
			Model:     "stage2-model",
			MaxTokens: 64,
			Messages:  messages,
		})
		current := plan.Current()
		if current == nil || current.Stage != 2 {
			continue
		}
		if current.ContextTrim == nil || current.ContextTrim.Type != "progressive_compaction" {
			t.Fatalf("context trim = %#v, want progressive_compaction", current.ContextTrim)
		}
		if current.ContextTrim.Stage != 2 {
			t.Fatalf("context trim stage = %d, want 2", current.ContextTrim.Stage)
		}
		foundAnchor := false
		foundSummary := false
		for _, msg := range current.Messages {
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Current-turn anchor") &&
				strings.Contains(msg.Content, "Answer the latest question briefly") {
				foundAnchor = true
			}
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Cached summary") {
				foundSummary = true
			}
		}
		if !foundAnchor {
			t.Fatalf("stage2 messages did not include latest-intent anchor: %#v", current.Messages)
		}
		if !foundSummary {
			t.Fatalf("stage2 messages did not include cached summary: %#v", current.Messages)
		}
		return
	}
	t.Fatal("expected at least one context window to fit at stage 2")
}

func TestPreparedBudgetStagesWrapHistoricalSummaryAsBackgroundOnly(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.summaryCache.Put("conv-stage-anchor", &ConversationSummary{
		Text:         "Goal\n- Continue the old implementation\n\nAccomplished\n- Pending: finish the migration",
		MessageCount: 8,
	})

	messages := []llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: strings.Repeat(fmt.Sprintf("user-%d background ", i), 40)},
			llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat(fmt.Sprintf("assistant-%d details ", i), 40)},
		)
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "先解释原因，不要继续旧任务"})

	stage2Messages, stageSummary := handler.buildPreparedBudgetStage2(context.Background(), "conv-stage-anchor", "stage-anchor-model", messages)
	stage3Messages, _ := handler.buildPreparedBudgetStage3(context.Background(), "conv-stage-anchor", "stage-anchor-model", messages, stageSummary)

	for _, stageMsgs := range [][]llm.Message{stage2Messages, stage3Messages} {
		foundAnchor := false
		foundWrappedSummary := false
		for _, msg := range stageMsgs {
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Current-turn anchor") &&
				strings.Contains(msg.Content, "先解释原因，不要继续旧任务") {
				foundAnchor = true
			}
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, historicalContextBackgroundPrefix) &&
				strings.Contains(msg.Content, historicalCarryOverReminder) {
				foundWrappedSummary = true
			}
		}
		if !foundAnchor {
			t.Fatalf("expected latest-intent anchor in prepared stage messages: %#v", stageMsgs)
		}
		if !foundWrappedSummary {
			t.Fatalf("expected wrapped historical summary in prepared stage messages: %#v", stageMsgs)
		}
	}

	if cached, ok := handler.summaryCache.Get("conv-stage-anchor"); !ok {
		t.Fatal("expected cached summary to remain present")
	} else if summary := cached.(*ConversationSummary); strings.Contains(summary.Text, historicalContextBackgroundPrefix) || strings.Contains(summary.Text, "Current-turn anchor") {
		t.Fatalf("cached summary should remain raw, got %q", summary.Text)
	}
}

func TestFitPreparedMessagesToBudgetKeepsOriginalBelowPressureThreshold(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "Summarize the last deployment issue"},
		{Role: llm.RoleAssistant, Content: strings.Repeat("I traced the deploy logs and captured the root cause. ", 16)},
		{Role: llm.RoleUser, Content: "Keep the answer concise"},
	}

	for _, window := range []int{4096, 6144, 8192, 12288, 16384} {
		handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
			ProviderID:    "p-context",
			ModelID:       "below-threshold-model",
			ContextWindow: window,
		}}))
		budget := handler.measurePreparedInputBudget("below-threshold-model", 64, messages)
		if budget.Exceeds() || budget.ContextUsageRatio() >= smartContextSoftCompressionThreshold {
			continue
		}
		plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
			ConvID:    "conv-below-threshold",
			Model:     "below-threshold-model",
			MaxTokens: 64,
			Messages:  messages,
		})
		current := plan.Current()
		if current == nil {
			t.Fatalf("expected original attempt below threshold, got failure: %#v", plan.Failure())
		}
		if current.Stage != 0 {
			t.Fatalf("current stage = %d, want 0 below threshold", current.Stage)
		}
		if current.ContextTrim != nil {
			t.Fatalf("context trim = %#v, want nil below threshold", current.ContextTrim)
		}
		return
	}
	t.Fatal("expected at least one below-threshold fixture")
}

func TestFitPreparedMessagesToBudgetProactivelyCompactsAtPressureThreshold(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.summaryCache.Put("conv-pressure-stage2", &ConversationSummary{
		Text:         "Goal\n- Keep the latest ask concise\n\nRelevant Files\n- server/internal/server/chat.go",
		MessageCount: 8,
	})

	messages := []llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: strings.Repeat(fmt.Sprintf("user-%d background ", i), 120)},
			llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat(fmt.Sprintf("assistant-%d details ", i), 140)},
		)
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "Answer the latest question briefly"})

	for _, window := range []int{4608, 5120, 5632, 6144, 6656, 7168, 7680, 8192} {
		handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
			ProviderID:    "p-context",
			ModelID:       "pressure-stage2-model",
			ContextWindow: window,
		}}))
		originalBudget := handler.measurePreparedInputBudget("pressure-stage2-model", 64, messages)
		if originalBudget.Exceeds() || originalBudget.ContextUsageRatio() < smartContextSoftCompressionThreshold {
			continue
		}
		plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
			ConvID:    "conv-pressure-stage2",
			Model:     "pressure-stage2-model",
			MaxTokens: 64,
			Messages:  messages,
		})
		current := plan.Current()
		if current == nil {
			t.Fatalf("expected proactive compaction attempt, got failure: %#v", plan.Failure())
		}
		if current.Stage != 2 {
			t.Fatalf("current stage = %d, want 2 when pressure >= 75%% and summary is available", current.Stage)
		}
		if current.ContextTrim == nil || current.ContextTrim.Type != "progressive_compaction" || current.ContextTrim.Stage != 2 {
			t.Fatalf("context trim = %#v, want stage-2 progressive_compaction", current.ContextTrim)
		}
		foundSummary := false
		for _, msg := range current.Messages {
			if msg.Role == llm.RoleSystem && strings.Contains(msg.Content, "Keep the latest ask concise") {
				foundSummary = true
				break
			}
		}
		if !foundSummary {
			t.Fatalf("expected stage2 summary to be injected under pressure, messages=%#v", current.Messages)
		}
		return
	}
	t.Fatal("expected at least one >=75% fixture that still fits without hard overflow")
}

func TestPreparedBudgetStagesDoNotTrimToolResultsBeforeFinalFallback(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.summaryCache.Put("conv-stage-no-trim", &ConversationSummary{
		Text:         "Goal\n- Preserve the current task\n\nRelevant Files\n- auth/middleware.go",
		MessageCount: 4,
	})

	longTool := strings.Repeat("tool-output ", 1200)
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "Inspect the deployment logs"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "web_search", Arguments: `{"query":"deploy logs"}`}}},
		{Role: llm.RoleTool, ToolCallID: "call-1", ToolName: "web_search", Content: longTool},
		{Role: llm.RoleUser, Content: "Now answer briefly"},
	}

	stage1Messages := handler.buildPreparedBudgetStage1("stage-no-trim-model", messages)
	stage2Messages, _ := handler.buildPreparedBudgetStage2(context.Background(), "conv-stage-no-trim", "stage-no-trim-model", messages)
	stage3Messages, _ := handler.buildPreparedBudgetStage3(context.Background(), "conv-stage-no-trim", "stage-no-trim-model", messages, "")

	for _, stageMsgs := range [][]llm.Message{stage1Messages, stage2Messages, stage3Messages} {
		found := false
		for _, msg := range stageMsgs {
			if msg.Role == llm.RoleTool && msg.ToolCallID == "call-1" {
				found = true
				if !strings.Contains(msg.Content, "tool-output") {
					t.Fatalf("tool result was trimmed too early: %q", msg.Content)
				}
			}
		}
		if !found {
			t.Fatalf("expected tool result to remain present before final trim fallback: %#v", stageMsgs)
		}
	}
}

func TestPreparedBudgetStagesIgnoreStaleSummaryCache(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	summaryEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "- Goal: keep context fresh\n- Pending: regenerate stale summaries"}
	handler.SetSmallModelRuntime(sm)

	messages := []llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}}
	for i := 0; i < 4; i++ {
		messages = append(messages,
			llm.Message{Role: llm.RoleUser, Content: strings.Repeat(fmt.Sprintf("user-%d background ", i), 40)},
			llm.Message{Role: llm.RoleAssistant, Content: strings.Repeat(fmt.Sprintf("assistant-%d details ", i), 40)},
		)
	}
	messages = append(messages, llm.Message{Role: llm.RoleUser, Content: "Continue with the latest task"})

	handler.summaryCache.Put("conv-stage-stale", &ConversationSummary{
		Text:         "stale summary should be ignored",
		MessageCount: 2,
	})
	stage2Messages, stage2Summary := handler.buildPreparedBudgetStage2(context.Background(), "conv-stage-stale", "stage2-model", messages)
	if strings.Contains(stage2Summary, "stale summary") {
		t.Fatalf("stage2 reused stale summary: %q", stage2Summary)
	}
	if !strings.Contains(stage2Summary, "Accomplished") {
		t.Fatalf("stage2 summary = %q, want regenerated canonical summary", stage2Summary)
	}
	if len(stage2Messages) == 0 {
		t.Fatal("expected stage2 to produce messages")
	}

	handler.summaryCache.Put("conv-stage-stale", &ConversationSummary{
		Text:         "stale summary should be ignored",
		MessageCount: 1,
	})
	stage3Messages, stage3Summary := handler.buildPreparedBudgetStage3(context.Background(), "conv-stage-stale", "stage2-model", messages, "")
	if strings.Contains(stage3Summary, "stale summary") {
		t.Fatalf("stage3 reused stale summary: %q", stage3Summary)
	}
	if !strings.Contains(stage3Summary, "Goal") || !strings.Contains(stage3Summary, "Accomplished") {
		t.Fatalf("stage3 summary = %q, want regenerated canonical summary", stage3Summary)
	}
	if len(stage3Messages) == 0 {
		t.Fatal("expected stage3 to produce messages")
	}
	if sm.calls < 2 {
		t.Fatalf("small model calls = %d, want >= 2 for stale stage2/stage3 regeneration", sm.calls)
	}
}

func TestFitPreparedMessagesToBudgetPrefersSameProviderFallback(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-explicit", ModelID: "small-model", ContextWindow: 1024, InputPrice: 10, Priority: 20},
		{ProviderID: "p-explicit", ModelID: "same-provider-large", ContextWindow: 4096, InputPrice: 5, Priority: 20},
		{ProviderID: "p-other", ModelID: "cross-provider-cheap", ContextWindow: 4096, InputPrice: 0.1, Priority: 100},
	}))

	content := findRepeatCountForBudget(t, handler, "small-model", []string{"same-provider-large", "cross-provider-cheap"}, "fallback budget ")
	plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
		ConvID:             "conv-fallback-same",
		Model:              "small-model",
		MaxTokens:          64,
		Messages:           []llm.Message{{Role: llm.RoleUser, Content: content}},
		ExplicitProviderID: "p-explicit",
		ProviderExplicit:   true,
	})

	current := plan.Current()
	if current == nil {
		t.Fatalf("expected fallback attempt, got failure: %#v", plan.Failure())
	}
	if current.Model != "same-provider-large" {
		t.Fatalf("fallback model = %q, want same-provider-large", current.Model)
	}
	if current.ProviderID != "p-explicit" {
		t.Fatalf("fallback provider = %q, want p-explicit", current.ProviderID)
	}
	if current.ContextTrim == nil || current.ContextTrim.FallbackProvider != "p-explicit" {
		t.Fatalf("context trim = %#v, want same-provider fallback metadata", current.ContextTrim)
	}
}

func TestFitPreparedMessagesToBudgetFallsBackAcrossProvidersWhenNeeded(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-explicit", ModelID: "small-model", ContextWindow: 1024, InputPrice: 10, Priority: 20},
		{ProviderID: "p-other", ModelID: "cross-provider-large", ContextWindow: 4096, InputPrice: 1, Priority: 100},
	}))

	content := findRepeatCountForBudget(t, handler, "small-model", []string{"cross-provider-large"}, "cross provider budget ")
	plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
		ConvID:             "conv-fallback-cross",
		Model:              "small-model",
		MaxTokens:          64,
		Messages:           []llm.Message{{Role: llm.RoleUser, Content: content}},
		ExplicitProviderID: "p-explicit",
		ProviderExplicit:   true,
	})

	current := plan.Current()
	if current == nil {
		t.Fatalf("expected cross-provider fallback attempt, got failure: %#v", plan.Failure())
	}
	if current.Model != "cross-provider-large" {
		t.Fatalf("fallback model = %q, want cross-provider-large", current.Model)
	}
	if current.ProviderID != "p-other" {
		t.Fatalf("fallback provider = %q, want p-other", current.ProviderID)
	}
	if current.ContextTrim == nil || current.ContextTrim.FallbackProvider != "p-other" {
		t.Fatalf("context trim = %#v, want cross-provider fallback metadata", current.ContextTrim)
	}
}

func TestFitPreparedMessagesToBudgetFailureIncludesMetadata(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "small-model",
		ContextWindow: 1024,
	}}))

	content := findRepeatCountForBudget(t, handler, "small-model", nil, "too large ")
	plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
		ConvID:    "conv-failure",
		Model:     "small-model",
		MaxTokens: 64,
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: content}},
	})

	if current := plan.Current(); current != nil {
		t.Fatalf("expected failure, got attempt: %#v", current)
	}
	failure := plan.Failure()
	if failure == nil {
		t.Fatal("expected failure metadata")
	}
	if failure.OriginalModel != "small-model" {
		t.Fatalf("original model = %q, want small-model", failure.OriginalModel)
	}
	if !failure.FallbackAttempted {
		t.Fatal("fallback_attempted = false, want true")
	}
	if len(failure.CompressionStagesAttempted) != 3 {
		t.Fatalf("compression stages = %#v, want 3 entries", failure.CompressionStagesAttempted)
	}
	if !failure.Budget.Exceeds() {
		t.Fatalf("final budget = %+v, want exceeded", failure.Budget)
	}
}

func TestFitPreparedMessagesToBudgetFallsBackWhenToolSchemaConsumesBudget(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-context", ModelID: "small-model", ContextWindow: 1024, InputPrice: 5, Priority: 20},
		{ProviderID: "p-context", ModelID: "large-model", ContextWindow: 8192, InputPrice: 8, Priority: 20},
	}))

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "system prompt"},
		{Role: llm.RoleUser, Content: "Use the available tools and keep the answer short."},
	}
	toolDefs := []llm.Tool{{
		Name:        "web_search",
		Description: strings.Repeat("Search across provider-visible sources and preserve citations. ", 40),
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": strings.Repeat("Detailed query guidance. ", 220),
				},
			},
			"required": []string{"query"},
		},
	}}

	messageOnlyBudget := handler.measurePreparedInputBudget("small-model", 64, messages)
	if messageOnlyBudget.Exceeds() {
		t.Fatalf("message-only budget should fit, got %+v", messageOnlyBudget)
	}
	toolAwareBudget := handler.measurePreparedInputBudgetWithTools("small-model", 64, messages, toolDefs)
	if !toolAwareBudget.Exceeds() {
		t.Fatalf("tool-aware budget should exceed after schema accounting, got %+v", toolAwareBudget)
	}

	plan := handler.fitPreparedMessagesToBudget(context.Background(), preparedBudgetFitParams{
		ConvID:    "conv-tool-budget",
		Model:     "small-model",
		MaxTokens: 64,
		Messages:  messages,
		Tools:     toolDefs,
	})
	current := plan.Current()
	if current == nil {
		t.Fatalf("expected fallback attempt, got failure: %#v", plan.Failure())
	}
	if current.Model != "large-model" {
		t.Fatalf("fallback model = %q, want large-model", current.Model)
	}
	if current.ContextTrim == nil || current.ContextTrim.FallbackModel != "large-model" {
		t.Fatalf("context trim = %#v, want fallback metadata for large-model", current.ContextTrim)
	}
}

func TestStripDuplicateTodoChecklistFromTypelessCards(t *testing.T) {
	content := "done\n\n```typeless\n" +
		`{"type":"result","title":"plan_create","details":[{"label":"checklist","value":"- [ ] task one\n- [ ] task two"},{"label":"pending_count","value":"2"}]}` +
		"\n```"
	trimmed := stripDuplicateTodoChecklistFromTypelessCards(content, "- [x] task one\n- [ ] task two")
	if strings.Contains(trimmed, `"label":"checklist"`) {
		t.Fatalf("expected duplicate checklist detail to be removed, got %q", trimmed)
	}
	if !strings.Contains(trimmed, `"label":"pending_count"`) {
		t.Fatalf("expected non-checklist details to remain, got %q", trimmed)
	}
}

func TestTodoAwarePersistedContentPrefersSummaryOverTrackedChecklist(t *testing.T) {
	got := todoAwarePersistedContent("总结：任务已完成。", "- [ ] task one\n- [ ] task two", true)
	if got != "总结：任务已完成。" {
		t.Fatalf("todoAwarePersistedContent() = %q, want summary content", got)
	}
}

func TestChatHandlerSendMessageRetriesContextTooLongWithLargerModel(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Retry context too long")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-context", ModelID: "small-model", ContextWindow: 4096, InputPrice: 5, Priority: 20},
		{ProviderID: "p-context", ModelID: "large-model", ContextWindow: 8192, InputPrice: 8, Priority: 20},
	}))
	fakeProxy := &contextTooLongThenSuccessProxyHandler{providerID: "p-context"}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"hello","model":"small-model","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if resp.Model != "large-model" {
		t.Fatalf("response model = %q, want large-model", resp.Model)
	}
	if resp.ContextTrim == nil {
		t.Fatal("expected context trim metadata")
	}
	if resp.ContextTrim.Type != "progressive_compaction" {
		t.Fatalf("context trim type = %q, want progressive_compaction", resp.ContextTrim.Type)
	}
	if resp.ContextTrim.FallbackModel != "large-model" {
		t.Fatalf("fallback_model = %q, want large-model", resp.ContextTrim.FallbackModel)
	}
	if resp.ContextTrim.FallbackProvider != "p-context" {
		t.Fatalf("fallback_provider = %q, want p-context", resp.ContextTrim.FallbackProvider)
	}
	models := fakeProxy.RequestModels()
	if len(models) < 2 {
		t.Fatalf("request models = %#v, want at least [small-model ... large-model]", models)
	}
	if models[0] != "small-model" {
		t.Fatalf("first request model = %q, want small-model; models=%#v", models[0], models)
	}
	if models[len(models)-1] != "large-model" {
		t.Fatalf("final request model = %q, want large-model; models=%#v", models[len(models)-1], models)
	}
	for i := 0; i < len(models)-1; i++ {
		if models[i] != "small-model" {
			t.Fatalf("intermediate request model = %q, want small-model before fallback; models=%#v", models[i], models)
		}
	}
}

func TestChatHandlerSendMessageRetriesRelayWrappedContextTooLongWithLargerModel(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Retry relay-wrapped context too long")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-context", ModelID: "small-model", ContextWindow: 4096, InputPrice: 5, Priority: 20},
		{ProviderID: "p-context", ModelID: "large-model", ContextWindow: 8192, InputPrice: 8, Priority: 20},
	}))
	fakeProxy := &contextTooLongThenSuccessProxyHandler{
		providerID:   "p-context",
		relayWrapped: true,
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"hello","model":"small-model","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if resp.Model != "large-model" {
		t.Fatalf("response model = %q, want large-model", resp.Model)
	}
	if resp.ContextTrim == nil {
		t.Fatal("expected context trim metadata")
	}
	if resp.ContextTrim.FallbackModel != "large-model" {
		t.Fatalf("fallback_model = %q, want large-model", resp.ContextTrim.FallbackModel)
	}
	models := fakeProxy.RequestModels()
	if len(models) < 2 {
		t.Fatalf("request models = %#v, want at least [small-model ... large-model]", models)
	}
	if models[0] != "small-model" {
		t.Fatalf("first request model = %q, want small-model; models=%#v", models[0], models)
	}
	if models[len(models)-1] != "large-model" {
		t.Fatalf("final request model = %q, want large-model; models=%#v", models[len(models)-1], models)
	}
}

func TestChatHandlerSendMessageReturnsBudgetFailureAfterAllUpstreamContextTooLongAttempts(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Retry context too long failure")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{
		{ProviderID: "p-context", ModelID: "small-model", ContextWindow: 4096, InputPrice: 5, Priority: 20},
		{ProviderID: "p-context", ModelID: "large-model", ContextWindow: 8192, InputPrice: 8, Priority: 20},
	}))
	handler.SetProxyBridge(proxybridge.NewBridge(&contextTooLongThenSuccessProxyHandler{
		providerID: "p-context",
		failAll:    true,
	}))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"hello","model":"small-model","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if exceeded, _ := resp["context_window_exceeded"].(bool); !exceeded {
		t.Fatalf("context_window_exceeded = %#v, want true", resp["context_window_exceeded"])
	}
	if got, _ := resp["original_model"].(string); got != "small-model" {
		t.Fatalf("original_model = %q, want small-model", got)
	}
	if attempted, _ := resp["fallback_attempted"].(bool); !attempted {
		t.Fatalf("fallback_attempted = %#v, want true", resp["fallback_attempted"])
	}
	stages, ok := resp["compression_stages_attempted"].([]interface{})
	if !ok || len(stages) != 3 {
		t.Fatalf("compression_stages_attempted = %#v, want 3 entries", resp["compression_stages_attempted"])
	}
	if got, _ := resp["model"].(string); got != "large-model" {
		t.Fatalf("model = %q, want large-model", got)
	}
}

func TestChatHandlerSendMessage(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-123",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Hello! How can I help?"},
	})
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "mock", "model": "mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if resp["content"] != "Hello! How can I help?" {
		t.Errorf("expected content 'Hello! How can I help?', got '%v'", resp["content"])
	}
}

func TestChatHandlerSendMessage_PropagatesProxyStatus(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Proxy Error Conv")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:      "unused-response",
				Model:   "gpt-5.3-codex-spark",
				Message: llm.Message{Role: llm.RoleAssistant, Content: "unused"},
			},
		},
		callErrors: []error{
			&proxybridge.ProxyError{
				StatusCode: http.StatusForbidden,
				Body:       `provider prov_4959c73f638d5562 auth error (403): {"error":{"message":"用户额度不足","code":"insufficient_user_quota"}}`,
			},
		},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())

	e := echo.New()
	reqBody := `{"message":"Hello!","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err == nil {
		t.Fatal("expected proxy error")
	}

	e.DefaultHTTPErrorHandler(err, c)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "proxy returned 403") {
		t.Fatalf("body = %s, want proxy status detail", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "insufficient_user_quota") {
		t.Fatalf("body = %s, want quota detail", rec.Body.String())
	}
}

func TestChatHandlerSendMessageAutoContinue_PseudoToolCallCommandWorkdirJSON(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Pseudo Tool Call SendMessage")

	registry := llm.NewProviderRegistry()
	pseudoContent := "好的，我来给你设一个 10 秒后的提醒。to=functions.exec {\"command\":\"blue reminder add message=\\\"喝水\\\" time=10s\",\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}{\"command\":\"...\"}\n```\nLet's do that exactly.{\"command\":\"blue help reminder\",...}\n```"
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "scripted-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 50, CompletionTokens: 90, TotalTokens: 140},
			},
			{
				ID:    "scripted-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "提醒已创建。",
				},
				Usage: llm.Usage{PromptTokens: 70, CompletionTokens: 8, TotalTokens: 78},
			},
		},
	}
	registry.Register(scripted)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"提醒我10秒后喝水","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); got != "提醒已创建。" {
		t.Fatalf("expected retried final content, got %q", got)
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected two LLM rounds (pseudo + retry), got %d", scripted.CallCount())
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	if len(secondReq.Messages) < 2 {
		t.Fatalf("expected second request to include continuation messages, got %d", len(secondReq.Messages))
	}
	last := secondReq.Messages[len(secondReq.Messages)-1]
	if last.Role != llm.RoleUser || !(strings.Contains(last.Content, "Now actually execute by calling available tools") || strings.Contains(last.Content, "A canonical TODO checklist already exists")) {
		t.Fatalf("expected generic execution nudge in second request, got role=%s content=%q", last.Role, last.Content)
	}
	if strings.Contains(last.Content, "fake tool-call text") {
		t.Fatalf("expected pseudo-specific fake-tool wording removed from continuation prompt, got content=%q", last.Content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, `{"command":"blue reminder add`) || strings.Contains(m.Content, "Let's do that exactly") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestChatHandlerSendMessage_RecoversPseudoFunctionCallsIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `<function_calls>
<invoke name="$blue">
<parameter name="command">web_query</parameter>
<parameter name="args">
<parameter name="input">Apple AAPL stock price today April 2026</parameter>
</parameter>
</invoke>
</function_calls>
正在查询最新股价信息...`
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-pseudo-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 50, CompletionTokens: 90, TotalTokens: 140},
			},
			{
				ID:    "recovered-pseudo-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已查询 Apple AAPL 股价，并整理成最终结果。",
				},
				Usage: llm.Usage{PromptTokens: 70, CompletionTokens: 8, TotalTokens: 78},
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
			"input":       "Apple AAPL stock price today April 2026",
			"query":       "Apple AAPL stock price today April 2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "mock finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "Apple AAPL stock price today April 2026" {
		t.Fatalf("web_query input = %q, want Apple AAPL stock price today April 2026", input)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "mock finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered tool execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已查询 Apple AAPL 股价，并整理成最终结果。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "<function_calls>") || strings.Contains(content, "<invoke name=\"$blue\">") {
		t.Fatalf("expected recovered pseudo tool-call text to be removed from response body, got %q", content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "<function_calls>") || strings.Contains(m.Content, "<invoke name=\"$blue\">") {
			t.Fatalf("expected recovered pseudo tool-call text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestChatHandlerSendMessage_PreservesXMLToolExampleProseWithoutExecutingTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "XML tool example prose send message")

	registry := llm.NewProviderRegistry()
	exampleContent := "下面是 XML 工具调用示例：<web_query><input>Apple AAPL stock price today2026</input></web_query>，不要实际执行。"
	scripted := &scriptedChatProvider{
		name: "scripted-xml-example-prose",
		responses: []llm.ChatResponse{
			{
				ID:    "xml-example-prose-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: exampleContent,
				},
				Usage: llm.Usage{PromptTokens: 36, CompletionTokens: 38, TotalTokens: 74},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name: "web_query",
		result: map[string]interface{}{
			"status": "ok",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"给我一个 xml 工具调用示例","provider":"scripted-xml-example-prose","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected exactly 1 LLM round for example prose, got %d", scripted.CallCount())
	}
	if _, ok := scripted.RequestAt(1); ok {
		t.Fatal("expected no second request capture for example prose")
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	webQueryMock.mu.Unlock()
	if calls != 0 {
		t.Fatalf("web_query calls = %d, want 0", calls)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if content != exampleContent {
		t.Fatalf("expected example prose preserved, got %q", content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	var assistantContent string
	for _, m := range messages {
		if m.Role == "assistant" {
			assistantContent = m.Content
		}
	}
	if assistantContent != exampleContent {
		t.Fatalf("expected persisted example prose preserved, got %q", assistantContent)
	}
}

func TestChatHandlerSendMessage_PreservesFormatGuidanceJSONProseWithoutExecutingTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Format guidance JSON prose send message")

	registry := llm.NewProviderRegistry()
	exampleContent := `工具调用格式如下：{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}`
	scripted := &scriptedChatProvider{
		name: "scripted-format-guidance-json-prose",
		responses: []llm.ChatResponse{
			{
				ID:    "format-guidance-json-prose-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: exampleContent,
				},
				Usage: llm.Usage{PromptTokens: 34, CompletionTokens: 32, TotalTokens: 66},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name:   "web_query",
		result: map[string]interface{}{"status": "ok"},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"给我一个 json 工具调用格式","provider":"scripted-format-guidance-json-prose","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected exactly 1 LLM round for format guidance prose, got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	webQueryMock.mu.Unlock()
	if calls != 0 {
		t.Fatalf("web_query calls = %d, want 0", calls)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if content != exampleContent {
		t.Fatalf("expected format guidance prose preserved, got %q", content)
	}
}

func TestChatHandlerSendMessage_PreservesBlockquoteGuidanceJSONProseWithoutExecutingTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Blockquote guidance JSON prose send message")

	registry := llm.NewProviderRegistry()
	exampleContent := "文档说明：\n> {\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}"
	scripted := &scriptedChatProvider{
		name: "scripted-blockquote-guidance-json-prose",
		responses: []llm.ChatResponse{
			{
				ID:    "blockquote-guidance-json-prose-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: exampleContent,
				},
				Usage: llm.Usage{PromptTokens: 34, CompletionTokens: 34, TotalTokens: 68},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name:   "web_query",
		result: map[string]interface{}{"status": "ok"},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"给我一个 blockquote 里的 json 工具调用说明","provider":"scripted-blockquote-guidance-json-prose","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected exactly 1 LLM round for blockquote guidance prose, got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	webQueryMock.mu.Unlock()
	if calls != 0 {
		t.Fatalf("web_query calls = %d, want 0", calls)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if content != exampleContent {
		t.Fatalf("expected blockquote guidance prose preserved, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversScalarDirectXMLPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered scalar direct xml pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `<web_query>Apple AAPL stock price today April 2026</web_query>
正在查询最新股价信息...`
	scripted := &scriptedChatProvider{
		name: "scripted-scalar-xml",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-scalar-xml-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 50, CompletionTokens: 72, TotalTokens: 122},
			},
			{
				ID:    "recovered-scalar-xml-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已查询 Apple AAPL 股价，并基于真实 web_query 结果完成总结。",
				},
				Usage: llm.Usage{PromptTokens: 68, CompletionTokens: 10, TotalTokens: 78},
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
			"input":       "Apple AAPL stock price today April 2026",
			"query":       "Apple AAPL stock price today April 2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "scalar direct xml finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted-scalar-xml","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "Apple AAPL stock price today April 2026" {
		t.Fatalf("web_query input = %q, want Apple AAPL stock price today April 2026", input)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "scalar direct xml finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered scalar direct xml execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已查询 Apple AAPL 股价，并基于真实 web_query 结果完成总结。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "<web_query>") {
		t.Fatalf("expected recovered scalar direct xml text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversToolCallWrapperPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered tool_call wrapper pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `<tool_call>
{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}
</tool_call>
我整理好后发你。`
	scripted := &scriptedChatProvider{
		name: "scripted-tool-call-wrapper",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-tool-call-wrapper-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 44, CompletionTokens: 66, TotalTokens: 110},
			},
			{
				ID:    "recovered-tool-call-wrapper-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好小米手机官网信息。",
				},
				Usage: llm.Usage{PromptTokens: 58, CompletionTokens: 12, TotalTokens: 70},
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
			"input":       "小米 官网 手机",
			"query":       "小米 官网 手机",
			"title":       "小米手机官网",
			"target_url":  "https://www.mi.com/",
			"final_url":   "https://www.mi.com/",
			"content":     "tool_call wrapper finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"小米 官网 手机","provider":"scripted-tool-call-wrapper","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	query, _ := webQueryMock.last["query"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if query != "小米 官网 手机" {
		t.Fatalf("web_query query = %q, want 小米 官网 手机", query)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "tool_call wrapper finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered tool_call wrapper execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已基于真实 web_query 结果整理好小米手机官网信息。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "<tool_call>") || strings.Contains(content, "tzkz0_web_search") {
		t.Fatalf("expected recovered tool_call wrapper text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversBracketedToolCallPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered bracketed tool_call pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `[TOOL_CALL]
{tool => "web_query", args => {
  --input "Apple AAPL stock price today April 2026"
  --max-results 5
}}
[/TOOL_CALL]
我整理好后发你。`
	scripted := &scriptedChatProvider{
		name: "scripted-bracketed-tool-call",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-bracketed-tool-call-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 46, CompletionTokens: 68, TotalTokens: 114},
			},
			{
				ID:    "recovered-bracketed-tool-call-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好 AAPL 股价信息。",
				},
				Usage: llm.Usage{PromptTokens: 60, CompletionTokens: 12, TotalTokens: 72},
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
			"input":       "Apple AAPL stock price today April 2026",
			"query":       "Apple AAPL stock price today April 2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "bracketed tool_call finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted-bracketed-tool-call","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	maxResults, _ := webQueryMock.last["max_results"].(float64)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "Apple AAPL stock price today April 2026" {
		t.Fatalf("web_query input = %q, want Apple AAPL stock price today April 2026", input)
	}
	if maxResults != 5 {
		t.Fatalf("web_query max_results = %v, want 5", maxResults)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "bracketed tool_call finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered bracketed tool_call execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已基于真实 web_query 结果整理好 AAPL 股价信息。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "[TOOL_CALL]") || strings.Contains(content, "--input") {
		t.Fatalf("expected recovered bracketed tool_call text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversNestedFunctionWrapperPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered nested function wrapper pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `<tool_call>
{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}
</tool_call>
我整理好后发你。`
	scripted := &scriptedChatProvider{
		name: "scripted-nested-function-wrapper",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-nested-function-wrapper-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 44, CompletionTokens: 64, TotalTokens: 108},
			},
			{
				ID:    "recovered-nested-function-wrapper-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好 AAPL 股价信息。",
				},
				Usage: llm.Usage{PromptTokens: 58, CompletionTokens: 12, TotalTokens: 70},
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
			"input":       "Apple AAPL stock price today April 2026",
			"query":       "Apple AAPL stock price today April 2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "nested function wrapper finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted-nested-function-wrapper","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	query, _ := webQueryMock.last["query"].(string)
	maxResults, _ := webQueryMock.last["max_results"].(float64)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if query != "Apple AAPL stock price today April 2026" {
		t.Fatalf("web_query query = %q, want Apple AAPL stock price today April 2026", query)
	}
	if maxResults != 5 {
		t.Fatalf("web_query max_results = %v, want 5", maxResults)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "nested function wrapper finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered nested function wrapper execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已基于真实 web_query 结果整理好 AAPL 股价信息。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "<tool_call>") || strings.Contains(content, `"type":"function"`) {
		t.Fatalf("expected recovered nested function wrapper text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversBareJSONWrapperPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered bare json wrapper pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}`
	scripted := &scriptedChatProvider{
		name: "scripted-bare-json-wrapper",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-bare-json-wrapper-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 40, CompletionTokens: 52, TotalTokens: 92},
			},
			{
				ID:    "recovered-bare-json-wrapper-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好小米手机官网信息。",
				},
				Usage: llm.Usage{PromptTokens: 54, CompletionTokens: 12, TotalTokens: 66},
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
			"input":       "小米 官网 手机",
			"query":       "小米 官网 手机",
			"title":       "小米手机官网",
			"target_url":  "https://www.mi.com/",
			"final_url":   "https://www.mi.com/",
			"content":     "bare json wrapper finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"小米 官网 手机","provider":"scripted-bare-json-wrapper","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	query, _ := webQueryMock.last["query"].(string)
	maxResults, _ := webQueryMock.last["max_results"].(float64)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if query != "小米 官网 手机" {
		t.Fatalf("web_query query = %q, want 小米 官网 手机", query)
	}
	if maxResults != 10 {
		t.Fatalf("web_query max_results = %v, want 10", maxResults)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "bare json wrapper finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered bare json wrapper execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已基于真实 web_query 结果整理好小米手机官网信息。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "tzkz0_web_search") || strings.Contains(content, `"arguments"`) {
		t.Fatalf("expected recovered bare json wrapper text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversBareJSONToolCallsArrayPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered bare json tool_calls array pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `{"tool_calls":[{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}]}`
	scripted := &scriptedChatProvider{
		name: "scripted-bare-json-tool-calls-array",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-bare-json-tool-calls-array-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 42, CompletionTokens: 54, TotalTokens: 96},
			},
			{
				ID:    "recovered-bare-json-tool-calls-array-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好 AAPL 股价信息。",
				},
				Usage: llm.Usage{PromptTokens: 56, CompletionTokens: 12, TotalTokens: 68},
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
			"input":       "Apple AAPL stock price today April 2026",
			"query":       "Apple AAPL stock price today April 2026",
			"title":       "Apple Inc. (AAPL) Stock Price",
			"target_url":  "https://example.com/aapl",
			"final_url":   "https://example.com/aapl",
			"content":     "bare json tool_calls array finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted-bare-json-tool-calls-array","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	query, _ := webQueryMock.last["query"].(string)
	maxResults, _ := webQueryMock.last["max_results"].(float64)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if query != "Apple AAPL stock price today April 2026" {
		t.Fatalf("web_query query = %q, want Apple AAPL stock price today April 2026", query)
	}
	if maxResults != 5 {
		t.Fatalf("web_query max_results = %v, want 5", maxResults)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "bare json tool_calls array finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered bare json tool_calls array execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}
}

func TestChatHandlerSendMessage_RecoversEmbeddedBareJSONWrapperPseudoToolCallIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered embedded bare json wrapper pseudo tool call send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `我先查一下。{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}我整理好后发你。`
	scripted := &scriptedChatProvider{
		name: "scripted-embedded-bare-json-wrapper",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-embedded-bare-json-wrapper-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 44, CompletionTokens: 60, TotalTokens: 104},
			},
			{
				ID:    "recovered-embedded-bare-json-wrapper-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已基于真实 web_query 结果整理好小米手机官网信息。",
				},
				Usage: llm.Usage{PromptTokens: 58, CompletionTokens: 12, TotalTokens: 70},
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
			"input":       "小米 官网 手机",
			"query":       "小米 官网 手机",
			"title":       "小米手机官网",
			"target_url":  "https://www.mi.com/",
			"final_url":   "https://www.mi.com/",
			"content":     "embedded bare json wrapper finance result",
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"小米 官网 手机","provider":"scripted-embedded-bare-json-wrapper","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	query, _ := webQueryMock.last["query"].(string)
	maxResults, _ := webQueryMock.last["max_results"].(float64)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if query != "小米 官网 手机" {
		t.Fatalf("web_query query = %q, want 小米 官网 手机", query)
	}
	if maxResults != 10 {
		t.Fatalf("web_query max_results = %v, want 10", maxResults)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "web_query" && strings.Contains(msg.Content, "embedded bare json wrapper finance result") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered embedded bare json wrapper execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered web_query tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if got := strings.TrimSpace(content); !strings.Contains(got, "已基于真实 web_query 结果整理好小米手机官网信息。") {
		t.Fatalf("expected final content from second round, got %q", got)
	}
	if strings.Contains(content, "tzkz0_web_search") || strings.Contains(content, `"arguments"`) {
		t.Fatalf("expected recovered embedded bare json wrapper text to be removed from response body, got %q", content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to read stored messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "tzkz0_web_search") || strings.Contains(m.Content, `"arguments"`) {
			t.Fatalf("expected recovered embedded bare json wrapper text to be discarded from stored assistant message, got=%q", m.Content)
		}
	}
}

func TestChatHandlerSendMessage_RecoversPseudoFileWriteInvokeIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered pseudo file write send message")
	workspaceRoot := t.TempDir()
	targetPath := filepath.Join(workspaceRoot, "stock_report.txt")
	reportContent := "苹果公司(AAPL)股票报告\n生成日期：2026年4月3日\n\n当前股价：[待查询]\n市场概况：[待查询]\n"

	registry := llm.NewProviderRegistry()
	pseudoContent := `<function_calls>
<invoke name="file_write">
<parameter name="path">` + targetPath + `</parameter>
<parameter name="content">` + reportContent + `</parameter>
</invoke>
</function_calls>`
	scripted := &scriptedChatProvider{
		name: "scripted-write",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-file-write-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 48, CompletionTokens: 65, TotalTokens: 113},
			},
			{
				ID:    "recovered-file-write-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已完成写入 stock_report.txt。",
				},
				Usage: llm.Usage{PromptTokens: 60, CompletionTokens: 10, TotalTokens: 70},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"请写入白名单目录文件","provider":"scripted-write","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-write summary), got %d", scripted.CallCount())
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("expected recovered file write to create target file, read error: %v", err)
	}
	if strings.TrimRight(string(data), "\n") != strings.TrimRight(reportContent, "\n") {
		t.Fatalf("unexpected recovered file content: %q", string(data))
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && (msg.ToolName == "write" || msg.ToolName == "file_write") && strings.Contains(msg.Content, "stock_report.txt") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered file write execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered write tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "已完成写入 stock_report.txt。") {
		t.Fatalf("expected final write summary, got %q", content)
	}
	if strings.Contains(content, "<invoke name=\"file_write\">") {
		t.Fatalf("expected recovered pseudo file-write text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessage_RecoversPseudoBlueDeepResearchIntoRealToolExecution(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Recovered pseudo deep research send message")

	registry := llm.NewProviderRegistry()
	pseudoContent := `<function_calls>
<invoke name="$blue">
<parameter name="command">deep_research</parameter>
<parameter name="args">
<parameter name="query">Apple AAPL stock price today April 2026</parameter>
<parameter name="mode">deep</parameter>
</parameter>
</invoke>
</function_calls>
正在查询最新股价信息...`
	scripted := &scriptedChatProvider{
		name: "scripted-deep-research",
		responses: []llm.ChatResponse{
			{
				ID:    "recovered-deep-research-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: pseudoContent,
				},
				Usage: llm.Usage{PromptTokens: 52, CompletionTokens: 72, TotalTokens: 124},
			},
			{
				ID:    "recovered-deep-research-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已完成 Apple AAPL 深度调研，并整理好结果摘要。",
				},
				Usage: llm.Usage{PromptTokens: 66, CompletionTokens: 10, TotalTokens: 76},
			},
		},
	}
	registry.Register(scripted)

	researchSvc := &researchServiceMock{
		createJob: &tools.ResearchJob{
			ID:                 "job-aapl-1",
			ConversationID:     conv.ID,
			Status:             "completed",
			Query:              "Apple AAPL stock price today April 2026",
			Mode:               "deep",
			RequestedRouteMode: "web",
			EffectiveRouteMode: "web",
			Progress:           100,
			EvidenceCount:      4,
			Answer:             "AAPL mock deep research answer",
			Confidence:         0.91,
		},
		getJob: &tools.ResearchJob{
			ID:                 "job-aapl-1",
			ConversationID:     conv.ID,
			Status:             "completed",
			Query:              "Apple AAPL stock price today April 2026",
			Mode:               "deep",
			RequestedRouteMode: "web",
			EffectiveRouteMode: "web",
			Progress:           100,
			EvidenceCount:      4,
			Answer:             "AAPL mock deep research answer",
			Confidence:         0.91,
		},
	}

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewDeepResearchTool(researchSvc))

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"Apple AAPL stock price today April 2026","provider":"scripted-deep-research","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 2 {
		t.Fatalf("expected 2 LLM rounds (pseudo + post-tool summary), got %d", scripted.CallCount())
	}

	researchSvc.mu.Lock()
	createCalls := researchSvc.createCalls
	statusCalls := researchSvc.statusCalls
	lastCreate := researchSvc.lastCreate
	lastJobID := researchSvc.lastJobID
	researchSvc.mu.Unlock()
	if createCalls != 1 {
		t.Fatalf("deep_research create calls = %d, want 1", createCalls)
	}
	if statusCalls != 1 {
		t.Fatalf("deep_research status calls = %d, want 1", statusCalls)
	}
	if lastCreate.Query != "Apple AAPL stock price today April 2026" {
		t.Fatalf("deep_research query = %q, want Apple AAPL stock price today April 2026", lastCreate.Query)
	}
	if lastCreate.Mode != "deep" {
		t.Fatalf("deep_research mode = %q, want deep", lastCreate.Mode)
	}
	if lastJobID != "job-aapl-1" {
		t.Fatalf("deep_research status job_id = %q, want job-aapl-1", lastJobID)
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	var sawToolResult bool
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleTool && msg.ToolName == "research" && strings.Contains(msg.Content, "AAPL mock deep research answer") {
			sawToolResult = true
		}
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "Now actually execute by calling available tools") {
			t.Fatalf("expected recovered deep_research execution instead of generic execution nudge, got user message %q", msg.Content)
		}
	}
	if !sawToolResult {
		t.Fatalf("expected second request to include recovered deep_research tool result, got %#v", secondReq.Messages)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "已完成 Apple AAPL 深度调研，并整理好结果摘要。") {
		t.Fatalf("expected final deep research summary, got %q", content)
	}
	if strings.Contains(content, "<invoke name=\"$blue\">") {
		t.Fatalf("expected recovered pseudo deep research text to be removed from response body, got %q", content)
	}
}

func TestChatHandlerSendMessageAutoContinue_RetriesEmptyReplyAfterToolRound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Post Tool Empty Reply Non-Stream")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "post-tool-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "我先查一下最近一周的动态。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_1",
						Name:      "web_search",
						Arguments: `{"query":"OpenClaw recent updates"}`,
					}},
				},
				Usage: llm.Usage{PromptTokens: 60, CompletionTokens: 18, TotalTokens: 78},
			},
			{
				ID:    "post-tool-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "",
				},
				Usage: llm.Usage{PromptTokens: 82, CompletionTokens: 1, TotalTokens: 83},
			},
			{
				ID:    "post-tool-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。",
				},
				Usage: llm.Usage{PromptTokens: 96, CompletionTokens: 28, TotalTokens: 124},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{result: map[string]interface{}{
		"query": "OpenClaw recent updates",
		"results": []map[string]interface{}{
			{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "description": "recent release notes"},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我调研一下最近一周 openclaw 的动向吧","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + empty + retry), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final summary, got %q", content)
	}
	if strings.Contains(content, "最终总结生成失败") || strings.Contains(content, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording, got %q", content)
	}
}

func TestRecoverPseudoToolCallsFromContent(t *testing.T) {
	ensureChatMiscRegexes()

	t.Run("blue wrapper with nested args", func(t *testing.T) {
		content := `<function_calls>
<invoke name="$blue">
<parameter name="command">deep_research</parameter>
<parameter name="args">
<parameter name="query">Apple AAPL stock price today April 2026</parameter>
<parameter name="iterations">3</parameter>
</parameter>
</invoke>
</function_calls>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "deep_research"}})
		if !ok {
			t.Fatal("expected pseudo tool-call recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "deep_research" {
			t.Fatalf("call name = %q, want deep_research", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["iterations"].(float64); got != 3 {
			t.Fatalf("iterations = %v, want 3", args["iterations"])
		}
	})

	t.Run("direct xml tool tag", func(t *testing.T) {
		content := `<web_query><input>Apple AAPL stock price today2026</input><max_results>5</max_results></web_query>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected pseudo xml tool tag recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Apple AAPL stock price today2026" {
			t.Fatalf("input = %q, want Apple AAPL stock price today2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("direct xml scalar tool tag", func(t *testing.T) {
		content := `<web_query>Apple AAPL stock price today2026</web_query>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected scalar pseudo xml tool tag recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Apple AAPL stock price today2026" {
			t.Fatalf("input = %q, want Apple AAPL stock price today2026", got)
		}
	})

	t.Run("direct invoke scalar body", func(t *testing.T) {
		content := `<function_calls>
<invoke name="web_query">Apple AAPL stock price today2026</invoke>
</function_calls>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected scalar invoke recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Apple AAPL stock price today2026" {
			t.Fatalf("input = %q, want Apple AAPL stock price today2026", got)
		}
	})

	t.Run("xml tool_call wrapper with json body", func(t *testing.T) {
		content := `<tool_call>
{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}
</tool_call>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected xml tool_call wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "小米 官网 手机" {
			t.Fatalf("query = %q, want 小米 官网 手机", got)
		}
		if got, _ := args["max_results"].(float64); got != 10 {
			t.Fatalf("max_results = %v, want 10", args["max_results"])
		}
	})

	t.Run("xml function_call wrapper with child arguments", func(t *testing.T) {
		content := `<function_call>
<name>web_query</name>
<arguments>{"input":"Apple AAPL stock price today2026","max_results":5}</arguments>
</function_call>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected xml function_call wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Apple AAPL stock price today2026" {
			t.Fatalf("input = %q, want Apple AAPL stock price today2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("tool_code wrapper with inline cli-style args", func(t *testing.T) {
		content := `<tool_code>web_query input="Aristotle Nicomachean Ethics virtue mean doctrine golden mean practical wisdom phronesis"</tool_code>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected tool_code wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Aristotle Nicomachean Ethics virtue mean doctrine golden mean practical wisdom phronesis" {
			t.Fatalf("input = %q, want Aristotle Nicomachean Ethics virtue mean doctrine golden mean practical wisdom phronesis", got)
		}
	})

	t.Run("bracketed tool_call wrapper with json body", func(t *testing.T) {
		content := `[TOOL_CALL]
{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}
[/TOOL_CALL]`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bracketed tool_call wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "小米 官网 手机" {
			t.Fatalf("query = %q, want 小米 官网 手机", got)
		}
		if got, _ := args["max_results"].(float64); got != 10 {
			t.Fatalf("max_results = %v, want 10", args["max_results"])
		}
	})

	t.Run("bracketed tool_call wrapper with cli args body", func(t *testing.T) {
		content := `[TOOL_CALL]
{tool => "web_query", args => {
  --input "Apple AAPL stock price today April 2026"
  --max-results 5
}}
[/TOOL_CALL]`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bracketed cli tool_call wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["input"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("input = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("xml tool_call wrapper with nested function object body", func(t *testing.T) {
		content := `<tool_call>
{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}
</tool_call>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected nested function wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("bracketed tool_call wrapper with nested function object body", func(t *testing.T) {
		content := `[TOOL_CALL]
{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}
[/TOOL_CALL]`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bracketed nested function wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("bare json tool_call wrapper body", func(t *testing.T) {
		content := `{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bare json wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "小米 官网 手机" {
			t.Fatalf("query = %q, want 小米 官网 手机", got)
		}
		if got, _ := args["max_results"].(float64); got != 10 {
			t.Fatalf("max_results = %v, want 10", args["max_results"])
		}
	})

	t.Run("bare json nested function wrapper body", func(t *testing.T) {
		content := `{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bare json nested function wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("bare json tool_calls array body", func(t *testing.T) {
		content := `[{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}}]`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bare json tool_calls array recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "小米 官网 手机" {
			t.Fatalf("query = %q, want 小米 官网 手机", got)
		}
		if got, _ := args["max_results"].(float64); got != 10 {
			t.Fatalf("max_results = %v, want 10", args["max_results"])
		}
	})

	t.Run("embedded bare json tool_call wrapper inside prose", func(t *testing.T) {
		content := `我先查一下。{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}我整理好后发你。`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected embedded bare json wrapper recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "小米 官网 手机" {
			t.Fatalf("query = %q, want 小米 官网 手机", got)
		}
		if got, _ := args["max_results"].(float64); got != 10 {
			t.Fatalf("max_results = %v, want 10", args["max_results"])
		}
	})

	t.Run("embedded bare json tool_calls array inside prose", func(t *testing.T) {
		content := `先查一下：{"tool_calls":[{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}]}我整理好后发你。`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected embedded bare json tool_calls array recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("does not recover embedded bare json wrapper inside explicit example prose", func(t *testing.T) {
		content := `你可以参考这个 JSON 示例：{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}` + "\n" + `实际回答里不要真的调用。`
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected explicit JSON example prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover embedded bare json wrapper inside code fence", func(t *testing.T) {
		content := "下面是工具调用示例：\n```json\n" +
			`{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}` +
			"\n```\n请按需改写。"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected code-fenced JSON example not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover direct xml tool tag inside code fence", func(t *testing.T) {
		content := "下面是 XML 工具调用示例：\n```xml\n<web_query><input>Apple AAPL stock price today2026</input></web_query>\n```\n不要实际执行。"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected code-fenced XML example not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover bracketed tool_call inside code fence", func(t *testing.T) {
		content := "下面是 bracketed 工具调用示例：\n```text\n[TOOL_CALL]\n{\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}\n[/TOOL_CALL]\n```\n不要实际执行。"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected code-fenced bracketed example not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover direct xml tool tag inside explicit example prose", func(t *testing.T) {
		content := "下面是 XML 工具调用示例：<web_query><input>Apple AAPL stock price today2026</input></web_query>，不要实际执行。"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected explicit XML example prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover bracketed tool_call inside explicit example prose", func(t *testing.T) {
		content := "下面是 bracketed 工具调用示例：[TOOL_CALL]{\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}[/TOOL_CALL]，不要实际执行。"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected explicit bracketed example prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover embedded bare json wrapper inside format guidance prose", func(t *testing.T) {
		content := `工具调用格式如下：{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}`
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected format guidance JSON prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover direct xml tool tag inside return-guidance prose", func(t *testing.T) {
		content := "你可以返回以下 XML：<web_query><input>Apple AAPL stock price today2026</input></web_query>"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected return-guidance XML prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover bracketed tool_call inside write-guidance prose", func(t *testing.T) {
		content := "可写成 [TOOL_CALL]{\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}[/TOOL_CALL]"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected write-guidance bracketed prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover blockquote json guidance prose", func(t *testing.T) {
		content := "文档说明：\n> {\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected blockquote guidance prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover list item xml guidance prose", func(t *testing.T) {
		content := "可选写法：\n- <web_query><input>Apple AAPL stock price today2026</input></web_query>"
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected list-item XML guidance prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("does not recover quoted bracketed guidance prose", func(t *testing.T) {
		content := "响应格式可写成 \"[TOOL_CALL]{\\\"name\\\":\\\"tzkz0_web_search\\\",\\\"arguments\\\":\\\"{\\\\\\\"query\\\\\\\":\\\\\\\"小米 官网 手机\\\\\\\",\\\\\\\"max_results\\\\\\\":10}\\\"}[/TOOL_CALL]\""
		if calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}}); ok || len(calls) > 0 {
			t.Fatalf("expected quoted bracketed guidance prose not to recover tool calls, got %#v", calls)
		}
	})

	t.Run("bare json choices delta tool_calls response fragment body", func(t *testing.T) {
		content := `{"choices":[{"delta":{"tool_calls":[{"type":"function","function":{"name":"tzkz0_web_search","arguments":"{\"query\":\"Apple AAPL stock price today April 2026\",\"max_results\":5}"}}]}}]}`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "web_query"}})
		if !ok {
			t.Fatal("expected bare json choices delta tool_calls recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "web_query" {
			t.Fatalf("call name = %q, want web_query", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["query"].(string); got != "Apple AAPL stock price today April 2026" {
			t.Fatalf("query = %q, want Apple AAPL stock price today April 2026", got)
		}
		if got, _ := args["max_results"].(float64); got != 5 {
			t.Fatalf("max_results = %v, want 5", args["max_results"])
		}
	})

	t.Run("direct invoke tool name alias", func(t *testing.T) {
		content := `<function_calls>
<invoke name="file_write">
<parameter name="path">stock_report.txt</parameter>
<parameter name="content">苹果公司(AAPL)股票报告</parameter>
</invoke>
</function_calls>`
		calls, ok := recoverPseudoToolCallsFromContent(content, []llm.Tool{{Name: "write"}})
		if !ok {
			t.Fatal("expected direct invoke alias recovery to succeed")
		}
		if len(calls) != 1 {
			t.Fatalf("recovered calls = %d, want 1", len(calls))
		}
		if calls[0].Name != "write" {
			t.Fatalf("call name = %q, want write", calls[0].Name)
		}
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(calls[0].Arguments), &args); err != nil {
			t.Fatalf("unmarshal arguments: %v", err)
		}
		if got, _ := args["path"].(string); got != "stock_report.txt" {
			t.Fatalf("path = %q, want stock_report.txt", got)
		}
		if got, _ := args["content"].(string); got != "苹果公司(AAPL)股票报告" {
			t.Fatalf("content = %q, want 苹果公司(AAPL)股票报告", got)
		}
	})
}

func TestPseudoJSONToolCallStartIndex_EmbeddedBareJSONWrapperInsideProse(t *testing.T) {
	delta := `我先查一下。{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"format\":\"json\",\"max_results\":10}"}我整理好后发你。`
	got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "web_query"}})
	want := strings.Index(delta, `{"name":"tzkz0_web_search"`)
	if got != want {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want %d", got, want)
	}
}

func TestPseudoJSONToolCallStartIndex_DoesNotFlagExplicitExampleProse(t *testing.T) {
	delta := `你可以参考这个 JSON 示例：{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}`
	if got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want -1", got)
	}
}

func TestPseudoJSONToolCallStartIndex_DoesNotFlagCodeFenceExample(t *testing.T) {
	delta := "下面是工具调用示例：\n```json\n" +
		`{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}` +
		"\n```"
	if got := pseudoJSONToolCallStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoJSONToolCallStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagExplicitXMLExampleProse(t *testing.T) {
	delta := "下面是 XML 工具调用示例：<web_query><input>Apple AAPL stock price today2026</input></web_query>，不要实际执行。"
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagExplicitBracketedExampleProse(t *testing.T) {
	delta := "下面是 bracketed 工具调用示例：[TOOL_CALL]{\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}[/TOOL_CALL]，不要实际执行。"
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagFormatGuidanceJSONProse(t *testing.T) {
	delta := `工具调用格式如下：{"name":"tzkz0_web_search","arguments":"{\"query\":\"小米 官网 手机\",\"max_results\":10}"}`
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagReturnGuidanceXMLProse(t *testing.T) {
	delta := "你可以返回以下 XML：<web_query><input>Apple AAPL stock price today2026</input></web_query>"
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagBlockquoteGuidanceJSONProse(t *testing.T) {
	delta := "文档说明：\n> {\"name\":\"tzkz0_web_search\",\"arguments\":\"{\\\"query\\\":\\\"小米 官网 手机\\\",\\\"max_results\\\":10}\"}"
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestPseudoDirectiveStartIndex_DoesNotFlagListItemGuidanceXMLProse(t *testing.T) {
	delta := "可选写法：\n- <web_query><input>Apple AAPL stock price today2026</input></web_query>"
	if got := pseudoDirectiveStartIndex(delta, []llm.Tool{{Name: "web_query"}}); got != -1 {
		t.Fatalf("pseudoDirectiveStartIndex() = %d, want -1", got)
	}
}

func TestChatHandlerSendMessageAutoContinue_RetriesEmptyReplyAfterWebQueryToolRound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Post Web Query Empty Reply Non-Stream")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-web-query",
		responses: []llm.ChatResponse{
			{
				ID:    "post-web-query-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "我先查一下最近一周的动态。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_query_1",
						Name:      "web_query",
						Arguments: `{"input":"OpenClaw recent updates"}`,
					}},
				},
				Usage: llm.Usage{PromptTokens: 60, CompletionTokens: 18, TotalTokens: 78},
			},
			{
				ID:    "post-web-query-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "",
				},
				Usage: llm.Usage{PromptTokens: 82, CompletionTokens: 1, TotalTokens: 83},
			},
			{
				ID:    "post-web-query-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。",
				},
				Usage: llm.Usage{PromptTokens: 96, CompletionTokens: 28, TotalTokens: 124},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	webQueryMock := &webSearchToolMock{
		name: "web_query",
		result: map[string]interface{}{
			"status":     "ok",
			"mode":       "search_read",
			"input":      "OpenClaw recent updates",
			"query":      "OpenClaw recent updates",
			"title":      "OpenClaw Release Notes",
			"target_url": "https://github.com/opendungeons/openclaw/releases",
			"final_url":  "https://github.com/opendungeons/openclaw/releases",
			"sources": []map[string]interface{}{
				{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "snippet": "recent release notes", "selected": true},
			},
			"next_action": "none",
		},
	}
	toolRegistry.Register(webQueryMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我调研一下最近一周 openclaw 的动向吧","provider":"scripted-web-query","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + empty + retry), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	webQueryMock.mu.Lock()
	calls := webQueryMock.calls
	input, _ := webQueryMock.last["input"].(string)
	webQueryMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "OpenClaw recent updates" {
		t.Fatalf("web_query input = %q, want OpenClaw recent updates", input)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final summary, got %q", content)
	}
	if strings.Contains(content, "最终总结生成失败") || strings.Contains(content, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording, got %q", content)
	}
}

func TestChatHandlerSendMessagePassesImageAttachmentsIntoToolContext(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Image Attachment Tool Context")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "image-tool-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_image_1",
						Name:      "image",
						Arguments: `{"action":"review","prompt":"describe this upload"}`,
					}},
				},
			},
			{
				ID:    "image-tool-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "done",
				},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	imageTool := &imageInputCaptureTool{}
	toolRegistry.Register(imageTool)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := fmt.Sprintf(`{"message":"帮我看看这张图","provider":"scripted","model":"gpt-5.3-codex-spark","attachments":[{"type":"image","name":"upload.png","mime_type":"image/png","data":"%s"}]}`, inlinePNGBase64ForChatTest())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	inputs := imageTool.CapturedInputs()
	if len(inputs) != 1 {
		t.Fatalf("captured inputs = %#v, want 1 image input", inputs)
	}
	if inputs[0].Name != "upload.png" {
		t.Fatalf("image input name = %q, want upload.png", inputs[0].Name)
	}
	if inputs[0].MimeType != "image/png" {
		t.Fatalf("image input mime_type = %q, want image/png", inputs[0].MimeType)
	}
	if inputs[0].Data == "" {
		t.Fatal("expected image input data to be forwarded")
	}
}

func TestChatHandlerSendMessageAutoContinue_RetriesErrorAfterToolRound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Post Tool Error Reply Non-Stream")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "task-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "- [ ] 写服务端\n- [ ] 写前端\n- [ ] 启动并输出 localhost 地址",
				},
			},
			{
				ID:    "task-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_exec_1",
						Name:      "exec",
						Arguments: `{"cmd":"cat > server.js <<'EOF'\nconsole.log('ok')\nEOF"}`,
					}},
				},
			},
			{
				ID:    "task-round-4",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已完成，游戏可直接运行，地址：http://localhost:3000",
				},
			},
		},
		callErrors: []error{nil, nil, errors.New("provider returned 500: follow-up summary failed"), nil},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{Name: "exec"}, result: map[string]interface{}{"ok": true}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	reqBody := `{"message":"帮我写一个坦克大战的 web 小游戏，要有服务端并启动后给我 localhost 地址","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds (plan + tool + failed follow-up + retry), got %d", scripted.CallCount())
	}

	fourthReq, ok := scripted.RequestAt(3)
	if !ok {
		t.Fatalf("missing fourth request capture")
	}
	last := fourthReq.Messages[len(fourthReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in fourth request, got role=%s content=%q", last.Role, last.Content)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "http://localhost:3000") {
		t.Fatalf("expected final localhost address, got %q", content)
	}
	if strings.Contains(content, "详细执行记录见上方工具卡片") || strings.Contains(content, "Tool execution completed") {
		t.Fatalf("expected no fallback-only summary wording, got %q", content)
	}
}

func TestChatHandlerSendMessageAutoContinue_TracksPlanStateAcrossToolAndToollessRounds(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Plan State Non-Stream Auto Continue")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "plan-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_plan_create_1",
							Name:      "plan_create",
							Arguments: `{"tasks":["实现2048网页游戏","本地运行并输出localhost地址"]}`,
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 80, CompletionTokens: 20, TotalTokens: 100},
			},
			{
				ID:    "plan-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "第一步已完成，我继续执行下一步并补充验证。",
				},
				Usage: llm.Usage{PromptTokens: 110, CompletionTokens: 30, TotalTokens: 140},
			},
			{
				ID:    "plan-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "已完成，游戏可直接运行，地址：http://localhost:3000。如果你愿意，我还可以帮你：1. 如果你愿意，我可以帮你访问并验证游戏功能。2. 如果你希望，我也可以帮你运行一轮回归测试。",
				},
				Usage: llm.Usage{PromptTokens: 130, CompletionTokens: 35, TotalTokens: 165},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{
		def: tools.ToolDefinition{
			Name:        "plan_create",
			Description: "plan create mock",
		},
		result: map[string]interface{}{
			"operation":       "create",
			"task_count":      2,
			"completed_count": 0,
			"pending_count":   2,
			"all_completed":   false,
			"checklist":       "- [ ] 实现2048网页游戏\n- [ ] 本地运行并输出localhost地址",
		},
	})
	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	reqBody := `{"message":"帮我做完2048并给localhost地址","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + toolless progress + completion), got %d", scripted.CallCount())
	}

	secondReq, ok := scripted.RequestAt(1)
	if !ok {
		t.Fatalf("missing second request capture")
	}
	sawTodoProgress := false
	for _, msg := range secondReq.Messages {
		if msg.Role == llm.RoleUser && strings.Contains(msg.Content, "<tp>") && strings.Contains(msg.Content, "Remaining:") {
			sawTodoProgress = true
			break
		}
	}
	if !sawTodoProgress {
		t.Fatalf("expected second request to include TODO progress context, got messages=%d", len(secondReq.Messages))
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !(strings.Contains(last.Content, "Now actually execute by calling available tools") || strings.Contains(last.Content, "A canonical TODO checklist already exists")) {
		t.Fatalf("expected non-stream pending_todo continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "http://localhost:3000") {
		t.Fatalf("expected final completion content with localhost address, got %q", content)
	}
}

func TestChatHandlerSendMessage_StripsDuplicateChecklistFromFinalResponse(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Plan State Non-Stream Checklist Dedupe")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "plan-dedupe-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_plan_create_dedupe_1",
							Name:      "plan_create",
							Arguments: `{"tasks":["实现2048网页游戏","本地运行并输出localhost地址"]}`,
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 80, CompletionTokens: 20, TotalTokens: 100},
			},
			{
				ID:    "plan-dedupe-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					Content: `- [ ] 实现2048网页游戏
- [ ] 本地运行并输出localhost地址

我先完成实现，再做本地验证。`,
				},
				Usage: llm.Usage{PromptTokens: 110, CompletionTokens: 30, TotalTokens: 140},
			},
			{
				ID:    "plan-dedupe-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					Content: `- [x] 实现2048网页游戏
- [x] 本地运行并输出localhost地址

已完成，游戏可直接运行，地址：http://localhost:3000`,
				},
				Usage: llm.Usage{PromptTokens: 130, CompletionTokens: 35, TotalTokens: 165},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{
		def: tools.ToolDefinition{
			Name:        "plan_create",
			Description: "plan create mock",
		},
		result: map[string]interface{}{
			"operation":       "create",
			"task_count":      2,
			"completed_count": 0,
			"pending_count":   2,
			"all_completed":   false,
			"checklist":       "- [ ] 实现2048网页游戏\n- [ ] 本地运行并输出localhost地址",
		},
	})
	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	reqBody := `{"message":"帮我做完2048并给localhost地址","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + duplicated checklist progress + duplicated checklist completion), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "A canonical TODO checklist already exists") {
		t.Fatalf("expected pending_todo continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "http://localhost:3000") {
		t.Fatalf("expected final completion content with localhost address, got %q", content)
	}
	if strings.Contains(content, "- [x]") || strings.Contains(content, "- [ ]") {
		t.Fatalf("expected final response content to strip duplicated checklist, got %q", content)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 50, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	assistantCount := 0
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		assistantCount++
		if strings.Contains(m.Content, "- [x]") || strings.Contains(m.Content, "- [ ]") {
			t.Fatalf("expected persisted assistant content to strip duplicated checklist, got %q", m.Content)
		}
	}
	if assistantCount != 1 {
		t.Fatalf("expected exactly one persisted assistant message, got %d", assistantCount)
	}
}

func TestChatHandlerSendMessage_DeepSearchGuardForcesSecondSearchRound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Deep Search Guard Non-Stream")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "deep-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_search_1",
							Name:      "web_search",
							Arguments: `{"query":"BlueAgent latest release notes"}`,
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 90, CompletionTokens: 20, TotalTokens: 110},
			},
			{
				ID:    "deep-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "先给你初步结论：BlueAgent 最近有更新。",
				},
				Usage: llm.Usage{PromptTokens: 120, CompletionTokens: 28, TotalTokens: 148},
			},
			{
				ID:    "deep-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{
							ID:        "call_search_2",
							Name:      "web_search",
							Arguments: `{"query":"BlueAgent changelog migration guide"}`,
						},
					},
				},
				Usage: llm.Usage{PromptTokens: 140, CompletionTokens: 24, TotalTokens: 164},
			},
			{
				ID:    "deep-round-4",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "执行摘要：BlueAgent 近期版本更新集中在工具链与文档。\n关键发现：发布说明与文档更新一致。\n风险与不确定性：社区二手信息存在时效偏差。\n来源：https://github.com/blueagent/blueagent/releases",
				},
				Usage: llm.Usage{PromptTokens: 170, CompletionTokens: 48, TotalTokens: 218},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{
		result: map[string]interface{}{
			"query": "BlueAgent latest release notes",
			"results": []map[string]interface{}{
				{
					"title":       "BlueAgent Releases",
					"url":         "https://github.com/blueagent/blueagent/releases",
					"description": "official release notes",
				},
			},
		},
	})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"请深度搜索 BlueAgent 最新新闻并给我完整报告附来源","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 4 {
		t.Fatalf("expected 4 LLM rounds (search + premature summary + forced search + final report), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	if len(thirdReq.Messages) < 2 {
		t.Fatalf("expected third request to include continuation nudge, got %d messages", len(thirdReq.Messages))
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "Deep-search guard: do not finalize yet") {
		t.Fatalf("expected deep-search guard nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "执行摘要") || !strings.Contains(content, "来源：") {
		t.Fatalf("expected final report content, got %q", content)
	}
}

func TestChatHandlerSendMessage_AutoContinuesRecoveryRedirectAfterRepeatedOverwriteLoop(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Repeated Overwrite Recovery")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	workspaceRoot := t.TempDir()
	targetPath := filepath.Join(workspaceRoot, "export_notes.applescript")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "overwrite-loop-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_write_1",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 1"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-loop-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_write_2",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 2"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-loop-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_write_3",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 3"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-loop-recovery",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "I noticed I was repeatedly overwriting the same file without making progress, so I stopped before damaging it further. The next step should be to inspect or validate the existing file, or switch to a different approach.",
				},
			},
			{
				ID:    "overwrite-loop-final-summary",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "I inspected the existing file instead of overwriting it again. The current script still contains the round 3 payload, so I stopped there to avoid more damage.",
				},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我导出 Apple Notes","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 5 {
		t.Fatalf("expected 5 LLM rounds (3 writes + recovery redirect + auto-continued summary), got %d", scripted.CallCount())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "I inspected the existing file instead of overwriting it again") {
		t.Fatalf("expected auto-continued recovery summary content, got %q", content)
	}

	b, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "-- broken script round 3" {
		t.Fatalf("final file content = %q, want round 3 payload", got)
	}
}

func TestChatHandlerSendMessage_StopsAfterRepeatedOverwriteRecoveryFails(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Repeated Overwrite Abort")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	workspaceRoot := t.TempDir()
	targetPath := filepath.Join(workspaceRoot, "export_notes.applescript")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted",
		responses: []llm.ChatResponse{
			{
				ID:    "overwrite-abort-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_abort_write_1",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 1"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-abort-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_abort_write_2",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 2"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-abort-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_abort_write_3",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 3"}`, targetPath),
					}},
				},
			},
			{
				ID:    "overwrite-abort-round-4",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_abort_write_4",
						Name:      "write",
						Arguments: fmt.Sprintf(`{"path":%q,"content":"-- broken script round 4"}`, targetPath),
					}},
				},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(tools.NewFileWriteTool([]string{workspaceRoot}, 0))

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我导出 Apple Notes","provider":"scripted","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() < 4 || scripted.CallCount() > 12 {
		t.Fatalf("expected bounded repeated-write recovery attempts before abort, got %d", scripted.CallCount())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if strings.TrimSpace(content) != "" && !strings.Contains(strings.ToLower(content), "overwrite") {
		t.Fatalf("expected either an empty completion or overwrite-related abort content, got %q", content)
	}

	b, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read final file: %v", err)
	}
	if got := strings.TrimSpace(string(b)); got != "-- broken script round 4" {
		t.Fatalf("final file content = %q, want round 4 payload", got)
	}
}

func TestChatHandlerSendMessageInjectsConversationAnchor(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Anchor Test Title")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Initial objective: fix context continuity"})
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSystemPromptBuilder(agentcore.NewSystemPromptBuilder(&agentcore.Config{}))

	e := echo.New()
	reqBody := `{"message":"B","provider":"capture","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	lastReq := capture.LastRequest()
	if !hasSystemAnchor(lastReq.Messages, "Anchor Test Title", "Initial objective: fix context continuity") {
		t.Fatalf("expected conversation anchor in system messages, got %d messages", len(lastReq.Messages))
	}
}

func TestBuildConversationAnchorPromptEdgeCases(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())

	t.Run("empty_title_uses_fallback_title", func(t *testing.T) {
		conv, _ := store.CreateConversation(context.Background(), "")
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Goal A"})
		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)
		if !strings.Contains(prompt, "Conversation title: Untitled conversation") {
			t.Fatalf("expected fallback title, got: %q", prompt)
		}
		if !strings.Contains(prompt, "Initial user goal: Goal A") {
			t.Fatalf("expected initial goal in prompt, got: %q", prompt)
		}
	})

	t.Run("long_title_and_goal_are_truncated", func(t *testing.T) {
		longTitle := strings.Repeat("测", 130)
		longGoal := strings.Repeat("g", 230)
		conv, _ := store.CreateConversation(context.Background(), longTitle)
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: longGoal})

		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)
		expectedTitle := "Conversation title: " + strings.Repeat("测", 120) + "..."
		expectedGoal := "Initial user goal: " + strings.Repeat("g", 220) + "..."

		if !strings.Contains(prompt, expectedTitle) {
			t.Fatalf("expected truncated title in prompt, got: %q", prompt)
		}
		if !strings.Contains(prompt, expectedGoal) {
			t.Fatalf("expected truncated goal in prompt, got: %q", prompt)
		}
	})

	t.Run("empty_user_content_falls_back_to_scope_only", func(t *testing.T) {
		conv, _ := store.CreateConversation(context.Background(), "Only Scope")
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: ""})
		_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})
		prompt := handler.buildConversationAnchorPrompt(context.Background(), conv.ID)

		if !strings.Contains(prompt, "Conversation title: Only Scope") {
			t.Fatalf("expected title in prompt, got: %q", prompt)
		}
		if strings.Contains(prompt, "Initial user goal:") {
			t.Fatalf("did not expect initial goal line for empty user content, got: %q", prompt)
		}
		if !strings.Contains(prompt, "Keep replies aligned with this conversation scope") {
			t.Fatalf("expected scope fallback guidance, got: %q", prompt)
		}
	})
}

// Test SendMessage with invalid provider
func TestChatHandlerSendMessageInvalidProvider(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "nonexistent", "model": "model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err == nil {
		t.Error("expected error for invalid provider")
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToDeepResearch(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Fallback Conv")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.Locale = "ja-JP"
	handler.SetSettingsHandler(settings)
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "Fallback answer from deep research.",
			"citations": []map[string]interface{}{
				{"title": "Doc A", "url": "https://example.com/a"},
			},
			"search_cards": []map[string]interface{}{
				{
					"type":       "search",
					"query":      "zimaos latest updates",
					"totalCount": 1,
					"results": []map[string]interface{}{
						{
							"title":       "Release notes",
							"url":         "https://example.com/release",
							"description": "latest update log",
						},
					},
				},
			},
			"confidence":     0.9,
			"evidence_count": 1,
			"support_count":  3,
			"conflict_count": 1,
			"has_conflict":   true,
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"please do deep research on zimaos latest updates with sources","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "deepresearch" {
		t.Fatalf("provider = %v, want deepresearch", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "Fallback answer from deep research.") {
		t.Fatalf("content = %q, want fallback deep research answer", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "情報源:") {
		t.Fatalf("content = %q, want localized sources label", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "```typeless") {
		t.Fatalf("content = %q, want typeless card block", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "\"type\":\"deep-research\"") {
		t.Fatalf("content = %q, want deep-research typeless card", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "\"has_conflict\":true") {
		t.Fatalf("content = %q, want conflict fields in typeless card", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "\"search_cards\"") {
		t.Fatalf("content = %q, want preserved search_cards in typeless card", got)
	}
	execMock.mu.Lock()
	gotLang, _ := execMock.last["lang"].(string)
	execMock.mu.Unlock()
	if gotLang != "ja-JP" {
		t.Fatalf("fallback lang = %q, want ja-JP", gotLang)
	}
}

func TestChatHandlerSendMessage_NoProviderNonResearchSkipsDeepResearchFallback(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Fallback Conv Non Research")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "should not be used",
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"hello","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err == nil {
		t.Fatalf("expected error when no provider and no research intent")
	}
	if !strings.Contains(err.Error(), "no proxy bridge configured") {
		t.Fatalf("error = %v, want no proxy bridge configured", err)
	}
	execMock.mu.Lock()
	calls := execMock.calls
	execMock.mu.Unlock()
	if calls != 0 {
		t.Fatalf("deep research fallback calls = %d, want 0", calls)
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToDeepResearchWithV2Fields(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Fallback Conv V2")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "按时间线整理后的研究结论。",
			"citations": []map[string]interface{}{
				{"title": "蓝驰创投采访", "url": "https://example.com/a"},
			},
			"citation_coverage": 0.93,
			"entity_disambiguation": map[string]interface{}{
				"enabled":        true,
				"threshold":      0.72,
				"filtered_count": 2,
			},
			"timeline_sections": []map[string]interface{}{
				{"label": "2023-2024", "summary": "中期观点"},
				{"label": "2025-至今", "summary": "近期观点"},
			},
			"stage_errors": []string{"query retry exhausted: xxx"},
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"请深度调研蓝驰付强观点演变并附来源","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	content, _ := resp["content"].(string)
	for _, token := range []string{
		"citation_coverage",
		"entity_disambiguation",
		"timeline_sections",
		"stage_errors",
		"https://example.com/a",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("content missing %q: %s", token, content)
		}
	}
}

func TestApplyDeepResearchPreference_NilKeepsTool(t *testing.T) {
	defs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "deep_research"},
		{Name: "deep-research"},
	}
	got := applyDeepResearchPreference(defs, nil)
	if len(got) != len(defs) {
		t.Fatalf("got %d defs, want %d", len(got), len(defs))
	}
}

func TestApplyDeepResearchPreference_FalseKeepsTool(t *testing.T) {
	disabled := false
	defs := []tools.ToolDefinition{
		{Name: "web_search"},
		{Name: "deep_research"},
		{Name: "deep-research"},
		{Name: "research_run"},
		{Name: "research_status"},
	}
	got := applyDeepResearchPreference(defs, &disabled)
	if len(got) != len(defs) {
		t.Fatalf("got %d defs, want %d", len(got), len(defs))
	}
}

func TestLocalizedSourcesLabelForLang_CoversAllSupportedLocales(t *testing.T) {
	cases := []struct {
		lang string
		want string
	}{
		{lang: "en-US", want: "Sources"},
		{lang: "en-GB", want: "Sources"},
		{lang: "zh-CN", want: "来源"},
		{lang: "zh-TW", want: "來源"},
		{lang: "ja-JP", want: "情報源"},
		{lang: "ko-KR", want: "출처"},
		{lang: "de-DE", want: "Quellen"},
		{lang: "fr-FR", want: "Sources"},
		{lang: "es-ES", want: "Fuentes"},
		{lang: "it-IT", want: "Fonti"},
		{lang: "pt-BR", want: "Fontes"},
		{lang: "pt-PT", want: "Fontes"},
		{lang: "ru-RU", want: "Источники"},
		{lang: "pl-PL", want: "Źródła"},
		{lang: "nl-NL", want: "Bronnen"},
		{lang: "sv-SE", want: "Källor"},
		{lang: "da-DK", want: "Kilder"},
		{lang: "nb-NO", want: "Kilder"},
		{lang: "cs-CZ", want: "Zdroje"},
		{lang: "sk-SK", want: "Zdroje"},
		{lang: "hu-HU", want: "Források"},
		{lang: "ro-RO", want: "Surse"},
		{lang: "hr-HR", want: "Izvori"},
		{lang: "el-GR", want: "Πηγές"},
		{lang: "ca-ES", want: "Fonts"},
		{lang: "ga-IE", want: "Foinsí"},
		{lang: "ml-IN", want: "ഉറവിടങ്ങൾ"},
	}
	for _, tc := range cases {
		if got := localizedSourcesLabelForLang(tc.lang); got != tc.want {
			t.Fatalf("localizedSourcesLabelForLang(%q)=%q, want %q", tc.lang, got, tc.want)
		}
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToIROnlyWhenDeepResearchUnavailable(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "IR fallback Conv")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	settings.settings.OfflineIRFallbackEnabled = &enabled
	handler.SetSettingsHandler(settings)

	e := echo.New()
	reqBody := `{"message":"please deep research zimaos latest updates","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-only-fallback" {
		t.Fatalf("model = %v, want ir-only-fallback", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "IR-only fallback") {
		t.Fatalf("content = %q, want IR-only fallback marker", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[fallbackReasonDeepResearchUnavailable] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonDeepResearchUnavailable, stats.FallbackReasons[fallbackReasonDeepResearchUnavailable])
	}
}

func TestChatHandlerSendMessage_NoProviderFallsBackToWebQueryTool(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Web query fallback Conv")
	registry := llm.NewProviderRegistry() // intentionally empty
	toolRegistry := tools.NewRegistry()
	webSearchMock := &webSearchToolMock{
		name: "web_query",
		result: map[string]interface{}{
			"status":     "ok",
			"mode":       "search_read",
			"input":      "zimaos release notes",
			"query":      "zimaos release notes",
			"title":      "ZimaOS Release Notes",
			"target_url": "https://example.com/release",
			"final_url":  "https://example.com/release",
			"content":    "release summary",
			"sources": []map[string]interface{}{
				{"title": "ZimaOS Release Notes", "url": "https://example.com/release", "snippet": "release summary", "selected": true},
				{"title": "ZimaOS Docs", "url": "https://example.com/docs", "snippet": "documentation"},
			},
			"next_action": "none",
		},
	}
	toolRegistry.Register(webSearchMock)

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"zimaos release notes","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "web_query" {
		t.Fatalf("provider = %v, want web_query", got)
	}
	if got := resp["model"]; got != "web-query-fallback" {
		t.Fatalf("model = %v, want web-query-fallback", got)
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, `Web query fallback results for "zimaos release notes":`) {
		t.Fatalf("content = %q, want web-query fallback summary", content)
	}
	if !strings.Contains(content, "ZimaOS Release Notes") {
		t.Fatalf("content = %q, want result title", content)
	}
	if !strings.Contains(content, "```typeless") {
		t.Fatalf("content = %q, want typeless card block", content)
	}
	if !strings.Contains(content, `"type":"result"`) {
		t.Fatalf("content = %q, want web-query typeless card", content)
	}

	webSearchMock.mu.Lock()
	calls := webSearchMock.calls
	input, _ := webSearchMock.last["input"].(string)
	format, _ := webSearchMock.last["format"].(string)
	webSearchMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_query calls = %d, want 1", calls)
	}
	if input != "zimaos release notes" {
		t.Fatalf("web_query input = %q, want zimaos release notes", input)
	}
	if format != "json" {
		t.Fatalf("web_query format = %q, want json", format)
	}
}

func TestChatHandlerSendMessage_NoProviderIROnlyUsesLayeredMemoryRecall(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "IR layered memory fallback")
	registry := llm.NewProviderRegistry() // intentionally empty
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	settings.settings.OfflineIRFallbackEnabled = &enabled
	handler.SetSettingsHandler(settings)

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
	handler.SetLayeredMemory(layeredSvc)
	if _, err := baseSvc.Remember(context.Background(), "Alpha project timeline is April 2026 with two release gates.", []string{"project", "timeline"}); err != nil {
		t.Fatalf("seed memory: %v", err)
	}

	e := echo.New()
	reqBody := `{"message":"Please deep research Alpha project timeline with local references","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-only-fallback" {
		t.Fatalf("model = %v, want ir-only-fallback", got)
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, "Alpha project timeline is April 2026") {
		t.Fatalf("content = %q, want layered memory snippet", content)
	}
}

func TestChatHandlerSendMessage_ShortQARoutesToSmallModel(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA")
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	offlineIRFallbackEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.OfflineIRFallbackEnabled = &offlineIRFallbackEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "small model answer"}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"今天上海天气怎么样？","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls == 0 {
		t.Fatal("expected small model runtime to be called")
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "smallmodel" {
		t.Fatalf("provider = %v, want smallmodel", got)
	}
	if got := resp["content"]; got != "small model answer" {
		t.Fatalf("content = %v, want small model answer", got)
	}
}

func TestChatHandlerSendMessage_ShortQAWithImageAttachmentRoutesToSmallModel(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA Image")
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	offlineIRFallbackEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.OfflineIRFallbackEnabled = &offlineIRFallbackEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "image small model answer"}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{
		"message":"这张图里有什么？",
		"provider":"",
		"model":"",
		"attachments":[
			{
				"type":"image",
				"name":"a.png",
				"mime_type":"image/png",
				"data":"aGVsbG8="
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls == 0 {
		t.Fatal("expected small model runtime to be called")
	}
	if len(sm.lastReq.Images) != 1 {
		t.Fatalf("small model images = %d, want 1", len(sm.lastReq.Images))
	}
	if sm.lastReq.Images[0].MimeType != "image/png" {
		t.Fatalf("image mime_type = %q, want image/png", sm.lastReq.Images[0].MimeType)
	}
	if sm.lastReq.Images[0].Data != "aGVsbG8=" {
		t.Fatalf("image data mismatch")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "smallmodel" {
		t.Fatalf("provider = %v, want smallmodel", got)
	}
	if got := resp["content"]; got != "image small model answer" {
		t.Fatalf("content = %v, want image small model answer", got)
	}
}

func TestChatHandlerSendMessage_ShortQANotReadySkipsSmallModelRoute(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA Not Ready")
	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-shortqa-not-ready",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "llm fallback answer"},
	})
	registry.Register(mockProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	ready := false
	sm := &smallModelRuntimeMock{ready: &ready, respText: "should not be used"}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"timeline?","provider":"","model":"mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when runtime is not ready", sm.calls)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["content"]; got != "llm fallback answer" {
		t.Fatalf("content = %v, want llm fallback answer", got)
	}
}

func TestChatHandlerSendMessage_ShortQAFallbackIRFirst(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA IR First")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "Project X timeline is planned for Q4 delivery.",
	})
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	offlineIRFallbackEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.OfflineIRFallbackEnabled = &offlineIRFallbackEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{err: smallmodel.ErrNotReady}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"Project X timeline?","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if sm.calls == 0 {
		t.Fatal("expected small model runtime to be called")
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := resp["model"]; got != "ir-first-fallback" {
		t.Fatalf("model = %v, want ir-first-fallback", got)
	}
	if got, _ := resp["content"].(string); !strings.Contains(got, "Project X timeline is planned for Q4") {
		t.Fatalf("content = %q, want local IR snippet", got)
	}
}

func TestChatHandlerSendMessage_ShortQAFallbackIROfflineSwitchDisabledGoesToLLM(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA IR Disabled")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "Project Z timeline is planned for next month.",
	})

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-ir-disabled",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "llm fallback answer"},
	})
	registry.Register(mockProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	offlineIRFallbackEnabled := false
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.OfflineIRFallbackEnabled = &offlineIRFallbackEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{err: smallmodel.ErrNotReady}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"Project Z timeline?","provider":"","model":"mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got == "ir" {
		t.Fatalf("provider = %v, want non-IR LLM fallback", got)
	}
	if got := resp["content"]; got != "llm fallback answer" {
		t.Fatalf("content = %v, want llm fallback answer", got)
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[fallbackReasonIRNoSignal] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonIRNoSignal, stats.FallbackReasons[fallbackReasonIRNoSignal])
	}
}

func TestChatHandlerSendMessage_ShortQAFallbackIRMissGoesToLLM(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA IR Miss")
	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-ir-miss",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "llm fallback answer"},
	})
	registry.Register(mockProvider)

	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{err: smallmodel.ErrNotReady}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	reqBody := `{"message":"timeline?","provider":"","model":"mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got := resp["provider"]; got == "ir" {
		t.Fatalf("provider = %v, want non-IR LLM fallback", got)
	}
	if got := resp["content"]; got != "llm fallback answer" {
		t.Fatalf("content = %v, want llm fallback answer", got)
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[fallbackReasonIRNoSignal] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonIRNoSignal, stats.FallbackReasons[fallbackReasonIRNoSignal])
	}
}

func TestChatHandlerSendMessage_ShortQACircuitBreakerFallsBackToIR(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Short QA Circuit")
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "Project Y milestone is still targeted for this quarter.",
	})
	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	offlineIRFallbackEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	settings.settings.OfflineIRFallbackEnabled = &offlineIRFallbackEnabled
	handler.SetSettingsHandler(settings)
	handler.smallModelBreaker = newSmallModelCircuitBreaker(2, time.Minute)
	sm := &smallModelRuntimeMock{err: context.DeadlineExceeded}
	handler.SetSmallModelRuntime(sm)

	e := echo.New()
	send := func(msg string) map[string]interface{} {
		reqBody := `{"message":"` + msg + `","provider":"","model":""}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)
		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}

	_ = send("Project Y milestone?")
	_ = send("Project Y milestone?")
	resp3 := send("Project Y milestone?")
	if got := resp3["provider"]; got != "ir" {
		t.Fatalf("provider = %v, want ir", got)
	}
	if got := sm.calls; got != 2 {
		t.Fatalf("small model calls = %d, want 2 (third request should be circuit-open bypass)", got)
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.FallbackReasons[smallmodel.FallbackReasonTimeout] != 2 {
		t.Fatalf("fallback reason %q = %d, want 2", smallmodel.FallbackReasonTimeout, stats.FallbackReasons[smallmodel.FallbackReasonTimeout])
	}
	if stats.FallbackReasons[smallmodel.FallbackReasonCircuitOpen] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", smallmodel.FallbackReasonCircuitOpen, stats.FallbackReasons[smallmodel.FallbackReasonCircuitOpen])
	}
}

func TestTrySmallModelShortQA_CircuitBreakerRecoversAfterWindow(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.smallModelBreaker = newSmallModelCircuitBreaker(1, 25*time.Millisecond)
	sm := &smallModelRuntimeMock{err: context.DeadlineExceeded, respText: "ok after recover"}
	handler.SetSmallModelRuntime(sm)

	if _, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2); err == nil {
		t.Fatal("expected first call to fail")
	}
	if _, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2); !errors.Is(err, smallmodel.ErrCircuitOpen) {
		t.Fatalf("expected circuit-open error, got %v", err)
	}
	if sm.calls != 1 {
		t.Fatalf("small model calls = %d, want 1 before recovery window", sm.calls)
	}

	time.Sleep(40 * time.Millisecond)
	sm.err = nil
	resp, err := handler.trySmallModelShortQA(context.Background(), "Q?", 32, 0.2)
	if err != nil {
		t.Fatalf("expected recovery success, got %v", err)
	}
	if resp == nil || resp.Message.Content != "ok after recover" {
		t.Fatalf("unexpected response after recovery: %+v", resp)
	}
}

func TestMaybeAutoRollbackShortQARoute_DisablesRouteOnHighFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}

	handler.maybeAutoRollbackShortQARoute()
	if settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route to be auto-disabled")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
	if stats.FallbackReasons[fallbackReasonAutoRollback] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonAutoRollback, stats.FallbackReasons[fallbackReasonAutoRollback])
	}
}

func TestMaybeAutoRollbackShortQARoute_DoesNotDisableBelowThreshold(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 39; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}

	handler.maybeAutoRollbackShortQARoute()
	if !settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route to stay enabled below sample threshold")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 0 {
		t.Fatalf("AutoRollbackTotal = %d, want 0", stats.AutoRollbackTotal)
	}
}

func TestMaybeAutoRollbackShortQARoute_UsesWindowedFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	shortQAEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &shortQAEnabled
	handler.SetSettingsHandler(settings)

	// Healthy first window should not disable and should advance baseline.
	for i := 0; i < 400; i++ {
		handler.smallModelStats.RecordShortQARoute(true)
	}
	handler.maybeAutoRollbackShortQARoute()
	if !settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route still enabled after healthy window")
	}

	// Next window is unhealthy. Cumulative fail rate is still low (40 / 440 < 0.15),
	// so this would fail if logic were not windowed.
	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordShortQARoute(false)
	}
	handler.maybeAutoRollbackShortQARoute()
	if settings.GetSmallModelRouteShortQAEnabled() {
		t.Fatal("expected short-qa route auto-disabled based on windowed failure rate")
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
}

func TestChatHandlerSendMessage_ImageArtifactSuccessSkipsChecklistBootstrap(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Image Artifact Completion")
	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-image",
		responses: []llm.ChatResponse{
			{
				ID:    "image-round-1",
				Model: "claude-opus-4-6",
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{{
						ID:        "call_image_1",
						Name:      "generateImage",
						Arguments: `{"prompt":"A friendly robot sitting in a cozy coffee shop, reading a book.","path":"robot_cafe.png"}`,
					}},
				},
				Usage: llm.Usage{PromptTokens: 48, CompletionTokens: 12, TotalTokens: 60},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{
		def: tools.ToolDefinition{
			Name:        "image",
			Description: "mock image generation",
			Parameters: map[string]interface{}{
				"type":                 "object",
				"properties":           map[string]interface{}{},
				"additionalProperties": true,
			},
		},
		result: map[string]interface{}{
			"status":  "succeeded",
			"message": "saved generated image to robot_cafe.png",
			"request": map[string]interface{}{"path": "robot_cafe.png"},
			"outputs": []map[string]interface{}{
				{"url": "/api/media/generated/images/robot_cafe.png"},
			},
		},
	})

	handler := NewChatHandler(store, registry, toolRegistry)
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settings.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settings)

	e := echo.New()
	reqBody := `{"message":"Generate an image of a friendly robot sitting in a cozy coffee shop, reading a book. Save it as \"robot_cafe.png\" in the current directory.","provider":"scripted-image","model":"claude-opus-4-6"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 1 {
		t.Fatalf("expected exactly 1 LLM round after deterministic image completion, got %d", scripted.CallCount())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v body=%s", err, rec.Body.String())
	}
	content, _ := resp["content"].(string)
	if !strings.Contains(content, `Generated the requested image and saved it to "robot_cafe.png".`) {
		t.Fatalf("expected deterministic completion message, got %q", content)
	}
	if strings.Contains(content, "bootstrap an agent mode checklist") || strings.Contains(content, "haven't provided a specific task yet") {
		t.Fatalf("expected checklist bootstrap path to be skipped, got %q", content)
	}
}

func TestChatHandlerStreamMessageAutoContinue_RetriesEmptyReplyAfterToolRound(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Post Tool Empty Reply Stream")

	registry := llm.NewProviderRegistry()
	scripted := &scriptedChatProvider{
		name: "scripted-stream",
		responses: []llm.ChatResponse{
			{
				ID:    "stream-post-tool-round-1",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "我先查一下最近一周的动态。",
					ToolCalls: []llm.ToolCall{{
						ID:        "call_web_stream_1",
						Name:      "web_search",
						Arguments: `{"query":"OpenClaw recent updates"}`,
					}},
				},
				Usage: llm.Usage{PromptTokens: 60, CompletionTokens: 18, TotalTokens: 78},
			},
			{
				ID:    "stream-post-tool-round-2",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "",
				},
				Usage: llm.Usage{PromptTokens: 82, CompletionTokens: 1, TotalTokens: 83},
			},
			{
				ID:    "stream-post-tool-round-3",
				Model: "gpt-5.3-codex-spark",
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "最近一周 OpenClaw 主要动态集中在 GitHub 发布说明和文档更新；我已经整理完关键变化与来源。",
				},
				Usage: llm.Usage{PromptTokens: 96, CompletionTokens: 28, TotalTokens: 124},
			},
		},
	}
	registry.Register(scripted)

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&webSearchToolMock{result: map[string]interface{}{
		"query": "OpenClaw recent updates",
		"results": []map[string]interface{}{
			{"title": "OpenClaw Release Notes", "url": "https://github.com/opendungeons/openclaw/releases", "description": "recent release notes"},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"帮我调研一下最近一周 openclaw 的动向吧","provider":"scripted-stream","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if scripted.CallCount() != 3 {
		t.Fatalf("expected 3 LLM rounds (tool + empty + retry), got %d", scripted.CallCount())
	}

	thirdReq, ok := scripted.RequestAt(2)
	if !ok {
		t.Fatalf("missing third request capture")
	}
	last := thirdReq.Messages[len(thirdReq.Messages)-1]
	if last.Role != llm.RoleUser || !strings.Contains(last.Content, "The tools above have been executed successfully") {
		t.Fatalf("expected post-tool continuation nudge in third request, got role=%s content=%q", last.Role, last.Content)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "最近一周 OpenClaw 主要动态") {
		t.Fatalf("expected retried final stream summary, got: %s", body)
	}
	if strings.Contains(body, "最终总结生成失败") || strings.Contains(body, "Tool execution completed") {
		t.Fatalf("expected no tool-fallback failure wording in stream body, got: %s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", body)
	}
}

func TestMaybeAutoRollbackSummaryRoute_DisablesRouteOnHighFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	summaryEnabled := true
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
	}

	handler.maybeAutoRollbackSummaryRoute()
	if settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route to be auto-disabled")
	}
	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
	if stats.FallbackReasons[fallbackReasonAutoRollbackSummary] != 1 {
		t.Fatalf("fallback reason %q = %d, want 1", fallbackReasonAutoRollbackSummary, stats.FallbackReasons[fallbackReasonAutoRollbackSummary])
	}
}

func TestGenerateConversationSummaryWithSmallModel_NotReadySkipsRuntimeCall(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	summaryEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)
	ready := false
	sm := &smallModelRuntimeMock{ready: &ready, respText: "summary should not be used"}
	handler.SetSmallModelRuntime(sm)

	summary := handler.generateConversationSummaryWithSmallModel(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
		{Role: llm.RoleAssistant, Content: "world"},
	})
	if summary != "" {
		t.Fatalf("summary = %q, want empty when runtime is not ready", summary)
	}
	if sm.calls != 0 {
		t.Fatalf("small model calls = %d, want 0 when runtime is not ready", sm.calls)
	}
}

func TestGenerateConversationSummaryWithSmallModel_PreservesMultilineBullets(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	enabled := true
	summaryEnabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)
	sm := &smallModelRuntimeMock{respText: "- Goal: fix login\n- Pending: add regression test\n- File Paths: auth/middleware.go"}
	handler.SetSmallModelRuntime(sm)

	summary := handler.generateConversationSummaryWithSmallModel(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "Login keeps failing after refresh"},
		{Role: llm.RoleAssistant, Content: "I will inspect the auth middleware ordering."},
	})
	if !strings.Contains(summary, "\n") {
		t.Fatalf("summary = %q, want multiline bullets preserved", summary)
	}
	if !strings.Contains(summary, "Goal") || !strings.Contains(summary, "Accomplished") || !strings.Contains(summary, "Relevant Files") {
		t.Fatalf("summary = %q, want canonical structured sections preserved", summary)
	}
}

func TestMaybeAutoRollbackSummaryRoute_UsesWindowedFailureRate(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	summaryEnabled := true
	settings.settings.SmallModelSummaryEnabled = &summaryEnabled
	handler.SetSettingsHandler(settings)

	for i := 0; i < 400; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
		handler.smallModelStats.RecordSummarySuccess()
	}
	handler.maybeAutoRollbackSummaryRoute()
	if !settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route still enabled after healthy window")
	}

	for i := 0; i < 40; i++ {
		handler.smallModelStats.RecordSummaryAttempt()
	}
	handler.maybeAutoRollbackSummaryRoute()
	if settings.GetSmallModelSummaryEnabled() {
		t.Fatal("expected summary route auto-disabled based on windowed failure rate")
	}

	stats := handler.smallModelStats.Snapshot()
	if stats.AutoRollbackTotal != 1 {
		t.Fatalf("AutoRollbackTotal = %d, want 1", stats.AutoRollbackTotal)
	}
}

func TestChatHandlerShouldDisableProxyPruner(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetSettingsHandler(settings)

	if handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner enabled by default")
	}

	contextPruneEnabled := false
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if !handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner disabled when context prune switch is explicitly off")
	}

	smallModelEnabled := true
	settings.settings.SmallModelEnabled = &smallModelEnabled
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if !handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner disabled when small model enabled and context prune switch off")
	}

	contextPruneEnabled = true
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if handler.shouldDisableProxyPruner() {
		t.Fatal("expected pruner enabled when context prune switch on")
	}
}

func TestChatHandlerShouldDisableProxyPrunerForAttempt(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetSettingsHandler(settings)

	belowPressure := &preparedBudgetAttempt{
		Budget: chatInputBudgetEstimate{
			ContextWindow:        32000,
			EstimatedInputTokens: 12000,
		},
	}
	if !handler.shouldDisableProxyPrunerForAttempt(belowPressure) {
		t.Fatal("expected proxy pruner disabled below pressure threshold when no explicit setting is present")
	}

	abovePressure := &preparedBudgetAttempt{
		Budget: chatInputBudgetEstimate{
			ContextWindow:        32000,
			EstimatedInputTokens: 26000,
		},
	}
	if !handler.shouldDisableProxyPrunerForAttempt(abovePressure) {
		t.Fatal("expected proxy pruner to stay disabled until the higher soft threshold is reached")
	}

	highPressure := &preparedBudgetAttempt{
		Budget: chatInputBudgetEstimate{
			ContextWindow:        32000,
			EstimatedInputTokens: 28000,
		},
	}
	if handler.shouldDisableProxyPrunerForAttempt(highPressure) {
		t.Fatal("expected proxy pruner enabled once the higher soft threshold is reached")
	}

	contextPruneEnabled := true
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	if handler.shouldDisableProxyPrunerForAttempt(belowPressure) {
		t.Fatal("expected explicit context prune enable to keep proxy pruner active")
	}
}

func TestChatHandlerSendMessageDisablesProxyPrunerBelowPressureThreshold(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Below-pressure pruner disable")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "below-threshold-model",
		ContextWindow: 32000,
	}}))

	var gotPrunerDisabled bool
	var gotRequestModel string
	handler.SetProxyBridge(proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrunerDisabled = pruner.IsPrunerDisabled(r.Context())
		var body struct {
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotRequestModel = strings.TrimSpace(body.Model)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"id":"resp-below-pressure","model":%q,"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`, gotRequestModel)))
	})))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"hello","model":"below-threshold-model","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotRequestModel != "below-threshold-model" {
		t.Fatalf("request model = %q, want below-threshold-model", gotRequestModel)
	}
	if !gotPrunerDisabled {
		t.Fatal("expected proxy pruner disabled to be propagated to proxy bridge below pressure threshold")
	}

	var resp SendMessageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v body=%s", err, rec.Body.String())
	}
	if resp.ContextTrim != nil {
		t.Fatalf("context trim = %#v, want nil when request stays below pressure threshold", resp.ContextTrim)
	}
}

func TestChatHandlerSendMessageKeepsProxyPrunerWhenExplicitlyEnabledBelowPressureThreshold(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Below-pressure explicit pruner enable")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	contextPruneEnabled := true
	settings.settings.SmallModelContextPruneEnabled = &contextPruneEnabled
	handler.SetSettingsHandler(settings)
	handler.SetProviderPool(newProviderPoolWithContextWindowModels(t, []contextWindowModelSpec{{
		ProviderID:    "p-context",
		ModelID:       "below-threshold-model",
		ContextWindow: 32000,
	}}))

	var gotPrunerDisabled bool
	handler.SetProxyBridge(proxybridge.NewBridge(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPrunerDisabled = pruner.IsPrunerDisabled(r.Context())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp-explicit-enable","model":"below-threshold-model","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`))
	})))

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"hello","model":"below-threshold-model","max_tokens":64}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if gotPrunerDisabled {
		t.Fatal("expected explicit context prune enable to keep proxy pruner active below pressure threshold")
	}
}

func TestSmallModelStatsHandlers(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.smallModelStats.RecordShortQARoute(true)
	handler.smallModelStats.RecordImageQARoute(true)
	handler.smallModelStats.RecordFallback(fallbackReasonDeepResearchUnavailable)
	handler.smallModelStats.RecordFallback("timeout")
	handler.smallModelStats.RecordLatencyWithScene("short_qa", 10*time.Millisecond)
	handler.smallModelStats.RecordLatencyWithScene("image_qa", 15*time.Millisecond)
	handler.smallModelStats.RecordLatencyWithScene("summary", 20*time.Millisecond)
	handler.smallModelStats.RecordAutoRollback()
	handler.smallModelStats.RecordIRTakeover()

	e := echo.New()

	statsReq := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/stats", nil)
	statsRec := httptest.NewRecorder()
	statsCtx := e.NewContext(statsReq, statsRec)
	if err := handler.SmallModelStatsHandler(statsCtx); err != nil {
		t.Fatalf("SmallModelStatsHandler failed: %v", err)
	}
	if statsRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", statsRec.Code)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(statsRec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if got := payload["short_qa_route_attempts"]; got != float64(1) {
		t.Fatalf("short_qa_route_attempts = %v, want 1", got)
	}
	if got := payload["image_qa_route_attempts"]; got != float64(1) {
		t.Fatalf("image_qa_route_attempts = %v, want 1", got)
	}
	if got := payload["ir_takeover_total"]; got != float64(1) {
		t.Fatalf("ir_takeover_total = %v, want 1", got)
	}
	if got := payload["auto_rollback_total"]; got != float64(1) {
		t.Fatalf("auto_rollback_total = %v, want 1", got)
	}
	if got := payload["small_model_fallback_total"]; got != float64(2) {
		t.Fatalf("small_model_fallback_total = %v, want 2", got)
	}
	if got := payload["small_model_timeout_total"]; got != float64(1) {
		t.Fatalf("small_model_timeout_total = %v, want 1", got)
	}
	if got := payload["small_model_latency_samples"]; got != float64(3) {
		t.Fatalf("small_model_latency_samples = %v, want 3", got)
	}
	if got := payload["small_model_latency_ms"]; got != float64(15) {
		t.Fatalf("small_model_latency_ms = %v, want 15", got)
	}
	if got := payload["short_qa_latency_ms"]; got != float64(10) {
		t.Fatalf("short_qa_latency_ms = %v, want 10", got)
	}
	if got := payload["image_qa_latency_ms"]; got != float64(15) {
		t.Fatalf("image_qa_latency_ms = %v, want 15", got)
	}
	if got := payload["summary_latency_ms"]; got != float64(20) {
		t.Fatalf("summary_latency_ms = %v, want 20", got)
	}

	resetReq := httptest.NewRequest(http.MethodPost, "/api/v1/small-model/stats/reset", nil)
	resetRec := httptest.NewRecorder()
	resetCtx := e.NewContext(resetReq, resetRec)
	if err := handler.ResetSmallModelStatsHandler(resetCtx); err != nil {
		t.Fatalf("ResetSmallModelStatsHandler failed: %v", err)
	}
	if resetRec.Code != http.StatusOK {
		t.Fatalf("reset status = %d, want 200", resetRec.Code)
	}

	statsReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/small-model/stats", nil)
	statsRec2 := httptest.NewRecorder()
	statsCtx2 := e.NewContext(statsReq2, statsRec2)
	if err := handler.SmallModelStatsHandler(statsCtx2); err != nil {
		t.Fatalf("SmallModelStatsHandler after reset failed: %v", err)
	}
	var payload2 map[string]interface{}
	if err := json.Unmarshal(statsRec2.Body.Bytes(), &payload2); err != nil {
		t.Fatalf("decode stats after reset: %v", err)
	}
	if got := payload2["short_qa_route_attempts"]; got != float64(0) {
		t.Fatalf("short_qa_route_attempts = %v, want 0", got)
	}
	if got := payload2["image_qa_route_attempts"]; got != float64(0) {
		t.Fatalf("image_qa_route_attempts = %v, want 0", got)
	}
	if got := payload2["ir_takeover_total"]; got != float64(0) {
		t.Fatalf("ir_takeover_total = %v, want 0", got)
	}
	if got := payload2["auto_rollback_total"]; got != float64(0) {
		t.Fatalf("auto_rollback_total = %v, want 0", got)
	}
	if got := payload2["small_model_fallback_total"]; got != float64(0) {
		t.Fatalf("small_model_fallback_total = %v, want 0", got)
	}
	if got := payload2["small_model_timeout_total"]; got != float64(0) {
		t.Fatalf("small_model_timeout_total = %v, want 0", got)
	}
	if got := payload2["small_model_latency_samples"]; got != float64(0) {
		t.Fatalf("small_model_latency_samples = %v, want 0", got)
	}
	if got := payload2["short_qa_latency_samples"]; got != float64(0) {
		t.Fatalf("short_qa_latency_samples = %v, want 0", got)
	}
	if got := payload2["image_qa_latency_samples"]; got != float64(0) {
		t.Fatalf("image_qa_latency_samples = %v, want 0", got)
	}
}

func TestShouldRouteImageQA_SeparatesImageTrafficFromShortQA(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	handler.SetSettingsHandler(settings)
	handler.smallModel = &smallModelRuntimeMock{respText: "cat"}

	enabled := true
	settings.settings.SmallModelEnabled = &enabled
	settings.settings.SmallModelRouteShortQAEnabled = &enabled

	req := SendMessageRequest{
		Message: "What is in this image?",
		Attachments: []MessageAttachment{{
			Type:     "image",
			MimeType: "image/png",
			Data:     "ZGF0YQ==",
		}},
	}

	if !handler.shouldRouteImageQA(req, req.Message) {
		t.Fatal("expected image QA route to inherit short QA enablement for image-only requests")
	}
	if handler.shouldRouteShortQA(req, req.Message) {
		t.Fatal("expected short QA route to skip image requests once image QA route is available")
	}

	disabled := false
	settings.settings.SmallModelRouteImageQAEnabled = &disabled
	if handler.shouldRouteImageQA(req, req.Message) {
		t.Fatal("expected explicit image QA disable to override inherited short QA setting")
	}
}

func TestChatHandlerSendMessageSlashCommandsAndOffline(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	callSend := func(body string) map[string]interface{} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)

		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("SendMessage error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return resp
	}

	resp := callSend(`{"message":"/model test-model","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /model response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/model","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /model query response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/status","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "test-model") {
		t.Fatalf("unexpected /status response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/ping","provider":"","model":""}`)
	if resp["content"] != "pong" {
		t.Fatalf("unexpected /ping response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/title Renamed Conv","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "Renamed Conv") {
		t.Fatalf("unexpected /title response: %v", resp["content"])
	}
	updatedConv, err := store.GetConversation(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("failed to get conversation after /title: %v", err)
	}
	if updatedConv.Title != "Renamed Conv" {
		t.Fatalf("conversation title = %q, want %q", updatedConv.Title, "Renamed Conv")
	}

	resp = callSend(`{"message":"/models","provider":"","model":""}`)
	modelsContent := resp["content"].(string)
	if !strings.Contains(modelsContent, "Available models") && !strings.Contains(modelsContent, "No model list") && !strings.Contains(modelsContent, "Providers:") && !strings.Contains(modelsContent, "No models available") {
		t.Fatalf("unexpected /models response: %v", resp["content"])
	}

	resp = callSend(`{"message":"/offline on","provider":"","model":""}`)
	if !strings.Contains(strings.ToLower(resp["content"].(string)), "offline mode is now on") {
		t.Fatalf("unexpected /offline on response: %v", resp["content"])
	}

	resp = callSend(`{"message":"hello offline","provider":"","model":""}`)
	content := resp["content"].(string)
	if !strings.Contains(content, "Offline mode response") {
		t.Fatalf("expected offline response, got: %s", content)
	}
	if got := resp["provider"]; got != "local" {
		t.Fatalf("provider = %v, want local", got)
	}
	if got := resp["model"]; got != "offline" {
		t.Fatalf("model = %v, want offline", got)
	}

	resp = callSend(`{"message":"/clear","provider":"","model":""}`)
	if !strings.Contains(resp["content"].(string), "Conversation cleared") {
		t.Fatalf("unexpected /clear response: %v", resp["content"])
	}

	msgs, err := store.GetMessages(context.Background(), conv.ID, 1000, 0)
	if err != nil {
		t.Fatalf("failed to list messages after clear: %v", err)
	}
	// /clear command stores one user + one assistant confirmation after wiping old history.
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages after clear command, got %d", len(msgs))
	}
}

func TestChatHandlerStreamMessageOfflineMode(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	e := echo.New()

	// Enable offline mode first.
	{
		req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(`{"message":"/offline on","provider":"","model":""}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("id")
		c.SetParamValues(conv.ID)
		if err := handler.SendMessage(c); err != nil {
			t.Fatalf("enable offline failed: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(`{"message":"stream hello","provider":"","model":""}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Offline mode response") {
		t.Fatalf("expected offline stream content, got: %s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected done chunk in stream, got: %s", body)
	}
}

// Test ListProviders endpoint
func TestChatHandlerListProviders(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	registry.Register(llm.NewMockProvider())

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListProviders(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 1 {
		t.Errorf("expected 1 provider, got %d", len(resp))
	}
}

// Test ListTools endpoint
func TestChatHandlerListTools(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	tools.RegisterBuiltinTools(toolRegistry)

	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListTools(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	for _, item := range resp {
		if item["name"] == "analyze" {
			t.Fatalf("expected analyze to stay hidden from chat /tools surface, got %#v", item)
		}
	}
}

// Test StreamMessage with Provider Pool ID mapping
func TestChatHandlerStreamMessageWithProviderPoolID(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	// Use CustomProvider which has Name() = "custom"
	// Provider Pool IDs that don't match known mappings will map to "custom"
	customProvider := llm.NewCustomProvider("test-key", "http://localhost")
	registry.Register(customProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	// Use a Provider Pool ID format (prov_<hex>)
	// This should be mapped to "custom" by mapProviderID()
	reqBody := `{"message": "Hello!", "provider": "prov_992ef6e8c8ad938a", "model": "test-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.StreamMessage(c)
	// The provider should be found (mapped to "custom")
	// We expect an error from the actual API call (since we're using a fake endpoint),
	// but NOT a "provider not found" error
	if err != nil {
		httpErr, ok := err.(*echo.HTTPError)
		if ok && httpErr.Code == http.StatusBadRequest {
			// Check if it's the "provider not found" error
			if msg, ok := httpErr.Message.(string); ok && bytes.Contains([]byte(msg), []byte("provider not found")) {
				t.Fatalf("StreamMessage should map Provider Pool ID to LLM provider name, but got error: %v", err)
			}
		}
		// Other errors are acceptable (e.g., API call failures, streaming setup issues)
		// The important thing is that the provider was found
	}
}

// MockMetricsRecorder implements MetricsRecorder for testing
type MockMetricsRecorder struct {
	APICalls   []MockAPICallRecord
	SpeedCalls []MockSpeedRecord
}

type MockAPICallRecord struct {
	Model        string
	Success      bool
	LatencyMs    float64
	InputTokens  int64
	OutputTokens int64
	CacheRead    int64
	CacheWrite   int64
	ErrorType    string
}

type MockSpeedRecord struct {
	Model           string
	TokensPerSecond float64
	TTFTMs          float64
	DecodeSpeed     float64
}

func (m *MockMetricsRecorder) RecordAPICall(model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
	m.APICalls = append(m.APICalls, MockAPICallRecord{
		Model:        model,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		ErrorType:    errorType,
	})
}

func (m *MockMetricsRecorder) RecordAPICallForUser(userID, model string, success bool, latencyMs float64, inputTokens, outputTokens, cacheRead, cacheWrite int64, errorType string) {
	m.APICalls = append(m.APICalls, MockAPICallRecord{
		Model:        model,
		Success:      success,
		LatencyMs:    latencyMs,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		CacheRead:    cacheRead,
		CacheWrite:   cacheWrite,
		ErrorType:    errorType,
	})
}

func (m *MockMetricsRecorder) RecordSpeed(model string, tokensPerSecond, ttftMs, decodeSpeed float64) {
	m.SpeedCalls = append(m.SpeedCalls, MockSpeedRecord{
		Model:           model,
		TokensPerSecond: tokensPerSecond,
		TTFTMs:          ttftMs,
		DecodeSpeed:     decodeSpeed,
	})
}

// Test SetMetricsRecorder
func TestChatHandlerSetMetricsRecorder(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	mockRecorder := &MockMetricsRecorder{}
	handler.SetMetricsRecorder(mockRecorder)

	if handler.metricsRecorder == nil {
		t.Error("expected metricsRecorder to be set")
	}
}

// Test SendMessage records metrics
func TestChatHandlerSendMessageRecordsMetrics(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	registry := llm.NewProviderRegistry()
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse(llm.ChatResponse{
		ID:      "resp-123",
		Model:   "mock-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Hello!"},
		Usage: llm.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	})
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	mockRecorder := &MockMetricsRecorder{}
	handler.SetMetricsRecorder(mockRecorder)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "mock", "model": "mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.SendMessage(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	// Verify metrics were recorded
	if len(mockRecorder.APICalls) != 1 {
		t.Errorf("expected 1 API call recorded, got %d", len(mockRecorder.APICalls))
	}

	if len(mockRecorder.APICalls) > 0 {
		call := mockRecorder.APICalls[0]
		if !call.Success {
			t.Error("expected successful API call")
		}
		if call.InputTokens != 10 {
			t.Errorf("expected 10 input tokens, got %d", call.InputTokens)
		}
		if call.OutputTokens != 5 {
			t.Errorf("expected 5 output tokens, got %d", call.OutputTokens)
		}
		// Latency can be 0 for mock provider (instant response)
		if call.LatencyMs < 0 {
			t.Error("expected non-negative latency")
		}
	}
}

func TestChatHandlerSendMessageTrialUsageAggregatesToolRoundsWithoutMetricsRecorder(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Trial Conv")

	registry := llm.NewProviderRegistry()
	registry.Register(&scriptedChatProvider{
		name:   "scripted-trial",
		models: []string{"gpt-5.3-codex"},
		responses: []llm.ChatResponse{
			{
				ID:         "round-1",
				Model:      "gpt-5.3-codex",
				Provider:   "trial",
				ProviderID: providerpool.TrialProviderID,
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "tc-1", Name: "dummy_tool", Arguments: "{}"},
					},
				},
				Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			},
			{
				ID:         "round-2",
				Model:      "gpt-5.3-codex",
				Provider:   "trial",
				ProviderID: providerpool.TrialProviderID,
				Message: llm.Message{
					Role:    llm.RoleAssistant,
					Content: "done",
				},
				Usage: llm.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
			},
		},
	})

	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{
		def:    tools.ToolDefinition{Name: "dummy_tool"},
		result: map[string]interface{}{"ok": true},
	})

	handler := NewChatHandler(store, registry, toolRegistry)
	ppStorage, err := providerpool.NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("create providerpool storage: %v", err)
	}
	ppRegistry, err := providerpool.NewRegistry(ppStorage)
	if err != nil {
		t.Fatalf("create providerpool registry: %v", err)
	}
	handler.providerPool = &providerpool.Pool{
		Registry:          ppRegistry,
		TrialQuotaManager: &providerpool.TrialQuotaManager{},
	}

	e := echo.New()
	reqBody := `{"message":"run tool flow","provider":"scripted-trial","model":"gpt-5.3-codex"}`
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
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	status := handler.providerPool.TrialQuotaManager.GetStatus()
	if status.TokensUsed != 25 {
		t.Fatalf("trial tokens_used = %d, want 25", status.TokensUsed)
	}
}

// Test mapProviderID function
func TestMapProviderID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"anthropic", "claude"},
		{"openai", "openai"},
		{"ollama", "ollama"},
		{"custom", "custom"},
		{"grok", "grok"},
		{"qwen", "qwen"},
		{"venice", "venice"},
		{"bedrock", "bedrock"},
		{"glm", "glm"},
		{"claude", "claude"},
		// Unknown providers return as-is
		{"prov_abc123", "prov_abc123"},
		{"unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := mapProviderID(tt.input)
			if result != tt.expected {
				t.Errorf("mapProviderID(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test message stats persistence
func TestMessageStatsPersistence(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	// Add a message with stats
	stats := &memory.MessageStats{
		InputTokens:     100,
		OutputTokens:    50,
		TotalTokens:     150,
		LatencyMs:       1234,
		TTFTMs:          567,
		TokensPerSecond: 25.5,
	}

	msg, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Test response",
		Provider: "openai",
		Model:    "gpt-4",
		Stats:    stats,
	})
	if err != nil {
		t.Fatalf("failed to add message: %v", err)
	}

	// Retrieve messages and verify stats
	messages, err := store.GetMessages(context.Background(), conv.ID, 100, 0)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	retrieved := messages[0]

	// Verify basic fields
	if retrieved.ID != msg.ID {
		t.Errorf("expected ID %s, got %s", msg.ID, retrieved.ID)
	}
	if retrieved.Provider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", retrieved.Provider)
	}
	if retrieved.Model != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%s'", retrieved.Model)
	}

	// Verify stats
	if retrieved.Stats == nil {
		t.Fatal("expected stats to be present")
	}
	if retrieved.Stats.InputTokens != 100 {
		t.Errorf("expected input_tokens 100, got %d", retrieved.Stats.InputTokens)
	}
	if retrieved.Stats.OutputTokens != 50 {
		t.Errorf("expected output_tokens 50, got %d", retrieved.Stats.OutputTokens)
	}
	if retrieved.Stats.TotalTokens != 150 {
		t.Errorf("expected total_tokens 150, got %d", retrieved.Stats.TotalTokens)
	}
	if retrieved.Stats.LatencyMs != 1234 {
		t.Errorf("expected latency_ms 1234, got %d", retrieved.Stats.LatencyMs)
	}
	if retrieved.Stats.TTFTMs != 567 {
		t.Errorf("expected ttft_ms 567, got %d", retrieved.Stats.TTFTMs)
	}
	if retrieved.Stats.TokensPerSecond != 25.5 {
		t.Errorf("expected tokens_per_second 25.5, got %f", retrieved.Stats.TokensPerSecond)
	}
}

// Test GetMessages returns stats in JSON response
func TestChatHandlerGetMessagesWithStats(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	conv, _ := store.CreateConversation(context.Background(), "Test Conv")

	// Add user message
	store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "Hello",
	})

	// Add assistant message with stats
	store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:     "assistant",
		Content:  "Hi there!",
		Provider: "openai",
		Model:    "gpt-4",
		Stats: &memory.MessageStats{
			InputTokens:     10,
			OutputTokens:    5,
			TotalTokens:     15,
			LatencyMs:       500,
			TTFTMs:          100,
			TokensPerSecond: 10.0,
		},
	})

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/conversations/"+conv.ID+"/messages", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err := handler.GetMessages(c)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	if len(resp) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(resp))
	}

	// Check assistant message has stats
	assistantMsg := resp[1]
	if assistantMsg["provider"] != "openai" {
		t.Errorf("expected provider 'openai', got '%v'", assistantMsg["provider"])
	}
	if assistantMsg["model"] != "gpt-4" {
		t.Errorf("expected model 'gpt-4', got '%v'", assistantMsg["model"])
	}

	stats, ok := assistantMsg["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stats to be present in response")
	}

	// JSON numbers are float64
	if stats["input_tokens"].(float64) != 10 {
		t.Errorf("expected input_tokens 10, got %v", stats["input_tokens"])
	}
	if stats["output_tokens"].(float64) != 5 {
		t.Errorf("expected output_tokens 5, got %v", stats["output_tokens"])
	}
	if stats["ttft_ms"].(float64) != 100 {
		t.Errorf("expected ttft_ms 100, got %v", stats["ttft_ms"])
	}
	if stats["tokens_per_second"].(float64) != 10.0 {
		t.Errorf("expected tokens_per_second 10.0, got %v", stats["tokens_per_second"])
	}
}

// Test token estimation functions
func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		minTokens int
		maxTokens int
	}{
		{
			name:      "empty string",
			text:      "",
			minTokens: 0,
			maxTokens: 0,
		},
		{
			name:      "short English text",
			text:      "Hello world",
			minTokens: 2,
			maxTokens: 4,
		},
		{
			name:      "longer English text",
			text:      "The quick brown fox jumps over the lazy dog",
			minTokens: 8,
			maxTokens: 15,
		},
		{
			name:      "Chinese text",
			text:      "你好世界",
			minTokens: 2,
			maxTokens: 4,
		},
		{
			name:      "mixed Chinese and English",
			text:      "Hello 你好 World 世界",
			minTokens: 4,
			maxTokens: 8,
		},
		{
			name:      "Japanese text",
			text:      "こんにちは世界",
			minTokens: 3,
			maxTokens: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := estimateTokens(tt.text)
			if result < tt.minTokens || result > tt.maxTokens {
				t.Errorf("estimateTokens(%q) = %d, want between %d and %d", tt.text, result, tt.minTokens, tt.maxTokens)
			}
		})
	}
}

// Test CJK character detection
func TestIsCJK(t *testing.T) {
	tests := []struct {
		char     rune
		expected bool
	}{
		{'A', false},
		{'a', false},
		{'1', false},
		{' ', false},
		{'中', true},
		{'国', true},
		{'あ', true}, // Hiragana
		{'ア', true}, // Katakana
		{'한', true}, // Hangul
	}

	for _, tt := range tests {
		t.Run(string(tt.char), func(t *testing.T) {
			result := isCJK(tt.char)
			if result != tt.expected {
				t.Errorf("isCJK(%q) = %v, want %v", tt.char, result, tt.expected)
			}
		})
	}
}

// Test input token estimation from messages
func TestEstimateInputTokens(t *testing.T) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.RoleUser, Content: "Hello!"},
		{Role: llm.RoleAssistant, Content: "Hi there! How can I help you today?"},
	}

	result := estimateInputTokens(messages)

	// Should be at least 12 tokens (4 per message overhead) + content tokens
	if result < 15 {
		t.Errorf("estimateInputTokens() = %d, expected at least 15", result)
	}

	// Should be reasonable (not too high)
	if result > 50 {
		t.Errorf("estimateInputTokens() = %d, expected at most 50", result)
	}
}

func TestBrowserCheckpointRequesterWeb_DeniesWhenSilent(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	// No active SSE client => AskQuestionsWithContext returns silent fallback.
	questionMgr := tools.NewQuestionManager(sse.NewBroker(), func() bool { return false }, 2*time.Minute)
	questionMgr.SetTimeoutActionFunc(func() string { return "default" })
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-1",
		"session-1",
		"",
		"",
		i18n.DefaultLanguage,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      "act",
		Action:    "click",
	})
	if err != nil {
		t.Fatalf("requester returned error: %v", err)
	}
	if result.Decision != tools.BrowserCheckpointDeny {
		t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointDeny)
	}
	if pending := handler.browserCheckpointMgr.GetPendingBySession("session-1"); pending != nil {
		t.Fatalf("expected checkpoint resolved, got pending %+v", pending)
	}
}

func TestBrowserCheckpointRequesterWeb_DismissDenies(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	broker := sse.NewBroker()
	client := broker.Subscribe("user-2")
	defer broker.Unsubscribe("user-2", client)

	questionMgr := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-2",
		"session-2",
		"",
		"",
		i18n.DefaultLanguage,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	resultCh := make(chan tools.BrowserCheckpointResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
			Required:  true,
			RiskLevel: "high",
			Step:      "act",
			Action:    "click",
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	pending := waitPendingQuestionForUser(t, questionMgr, "user-2", 2*time.Second)
	if !questionMgr.DismissQuestion(pending.ID) {
		t.Fatalf("failed to dismiss question id=%s", pending.ID)
	}

	select {
	case err := <-errCh:
		t.Fatalf("requester returned error: %v", err)
	case result := <-resultCh:
		if result.Decision != tools.BrowserCheckpointDeny {
			t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointDeny)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for requester result")
	}
}

func TestBrowserCheckpointRequesterWeb_ContinueApproves(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	broker := sse.NewBroker()
	client := broker.Subscribe("user-3")
	defer broker.Unsubscribe("user-3", client)

	questionMgr := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-3",
		"session-3",
		"",
		"",
		i18n.DefaultLanguage,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	resultCh := make(chan tools.BrowserCheckpointResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
			Required:  true,
			RiskLevel: "high",
			Step:      "act",
			Action:    "click",
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	pending := waitPendingQuestionForUser(t, questionMgr, "user-3", 2*time.Second)
	if !questionMgr.ResolveAnswer(pending.ID, []tools.QuestionAnswerResult{{
		QuestionID: "browser_checkpoint",
		Selected:   []string{"continue"},
	}}) {
		t.Fatalf("failed to resolve question id=%s", pending.ID)
	}

	select {
	case err := <-errCh:
		t.Fatalf("requester returned error: %v", err)
	case result := <-resultCh:
		if result.Decision != tools.BrowserCheckpointApprove {
			t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointApprove)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for requester result")
	}
}

func TestBrowserCheckpointRequesterWeb_TrustedSiteAutoApproves(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	siteStore, err := tools.NewBrowserSiteAllowlistStore(db)
	if err != nil {
		t.Fatalf("new browser site allowlist store: %v", err)
	}
	if err := siteStore.Add("https://example.com/login", "user-site"); err != nil {
		t.Fatalf("seed allowlist: %v", err)
	}
	handler.SetBrowserSiteAllowlistStore(siteStore)

	broker := sse.NewBroker()
	client := broker.Subscribe("user-site")
	defer broker.Unsubscribe("user-site", client)

	questionMgr := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-site",
		"session-site",
		"",
		"",
		i18n.DefaultLanguage,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      "recipe",
		Action:    "login",
		URL:       "https://example.com/account",
	})
	if err != nil {
		t.Fatalf("requester returned error: %v", err)
	}
	if result.Decision != tools.BrowserCheckpointApprove {
		t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointApprove)
	}
	if pending := questionMgr.GetPending("user-site"); pending != nil {
		t.Fatalf("expected no pending question for trusted site, got %+v", pending)
	}
}

func TestBrowserCheckpointRequesterWeb_AllowSitePersistsFutureApproval(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	siteStore, err := tools.NewBrowserSiteAllowlistStore(db)
	if err != nil {
		t.Fatalf("new browser site allowlist store: %v", err)
	}
	handler.SetBrowserSiteAllowlistStore(siteStore)

	broker := sse.NewBroker()
	client := broker.Subscribe("user-allow")
	defer broker.Unsubscribe("user-allow", client)

	questionMgr := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-allow",
		"session-allow",
		"",
		"",
		i18n.DefaultLanguage,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	resultCh := make(chan tools.BrowserCheckpointResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
			Required:  true,
			RiskLevel: "high",
			Step:      "recipe",
			Action:    "login",
			URL:       "https://example.com/login",
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	pending := waitPendingQuestionForUser(t, questionMgr, "user-allow", 2*time.Second)
	if len(pending.Questions) != 1 {
		t.Fatalf("questions len = %d, want 1", len(pending.Questions))
	}
	if len(pending.Questions[0].Options) != 3 {
		t.Fatalf("options len = %d, want 3", len(pending.Questions[0].Options))
	}
	if got := pending.Questions[0].Options[1].Value; got != "allow_site" {
		t.Fatalf("site allow option value = %q, want allow_site", got)
	}
	if !questionMgr.ResolveAnswer(pending.ID, []tools.QuestionAnswerResult{{
		QuestionID: "browser_checkpoint",
		Selected:   []string{"allow_site"},
	}}) {
		t.Fatalf("failed to resolve question id=%s", pending.ID)
	}

	select {
	case err := <-errCh:
		t.Fatalf("requester returned error: %v", err)
	case result := <-resultCh:
		if result.Decision != tools.BrowserCheckpointApprove {
			t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointApprove)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for requester result")
	}

	if matched := siteStore.Match("https://example.com/account", "user-allow"); matched == nil {
		t.Fatal("expected trusted site to be persisted after allow_site answer")
	}

	result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      "recipe",
		Action:    "login",
		URL:       "https://example.com/account",
	})
	if err != nil {
		t.Fatalf("second requester returned error: %v", err)
	}
	if result.Decision != tools.BrowserCheckpointApprove {
		t.Fatalf("second decision = %s, want %s", result.Decision, tools.BrowserCheckpointApprove)
	}
	if pending := questionMgr.GetPending("user-allow"); pending != nil {
		t.Fatalf("expected no pending question after allowlist hit, got %+v", pending)
	}
}

func waitPendingQuestionForUser(t *testing.T, mgr *tools.QuestionManager, userID string, timeout time.Duration) *tools.QuestionRequest {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if pending := mgr.GetPending(userID); pending != nil {
			return pending
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for pending question for user %s", userID)
	return nil
}

func TestFormatIMBrowserProgressLocalized(t *testing.T) {
	card := map[string]interface{}{
		"type":   "browser-progress",
		"step":   "screenshot",
		"name":   "Capturing screenshot",
		"status": "success",
		"url":    "https://example.com",
	}

	got := formatIMBrowserProgress(card, i18n.LangZhCN)
	want := "✅ 浏览器 正在截取屏幕截图 (https://example.com)"
	if got != want {
		t.Fatalf("formatIMBrowserProgress() = %q, want %q", got, want)
	}
}

func TestBuildIMCardEmitterLocalizesRecipeProgress(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent channel.OutgoingMessage
	h.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		if channelName != "feishu" {
			t.Fatalf("channelName = %q, want feishu", channelName)
		}
		sent = out
		return nil
	})

	emitter := h.buildIMCardEmitter(context.Background(), "feishu", "chat_1", "msg_1", i18n.LangZhCN)
	if emitter == nil {
		t.Fatal("expected IM card emitter")
	}

	emitter(map[string]interface{}{
		"type":        "browser-progress",
		"step":        "recipe",
		"name":        "Running login recipe",
		"recipe_name": "login recipe",
		"status":      "running",
	})

	want := "⏳ 浏览器 正在运行 login recipe"
	if sent.Content != want {
		t.Fatalf("sent content = %q, want %q", sent.Content, want)
	}
}

func TestFormatIMCard_FinalUIReviewAndDeepResearch(t *testing.T) {
	uiReview := formatIMCard(map[string]interface{}{
		"type":          "ui-review",
		"url":           "https://example.com",
		"overall":       88.0,
		"pass":          true,
		"visual":        map[string]interface{}{"score": 86.0},
		"functional":    map[string]interface{}{"score": 92.0},
		"accessibility": map[string]interface{}{"score": 84.0},
		"suggestions":   []interface{}{"Tighten spacing"},
		"screenshots":   []interface{}{"/api/v1/media/ui-review/a.png"},
	}, i18n.LangEnUS)
	if !strings.Contains(uiReview, "UI Review") || !strings.Contains(uiReview, "Overall: 88") || !strings.Contains(uiReview, "Screenshots:") {
		t.Fatalf("unexpected ui review IM card: %q", uiReview)
	}

	deepResearch := formatIMCard(map[string]interface{}{
		"type":              "deep-research",
		"query":             "ZimaOS market",
		"mode":              "deep",
		"answer":            "Summary body",
		"confidence":        0.91,
		"evidence_count":    4,
		"citation_coverage": 0.85,
		"citations":         []interface{}{map[string]interface{}{"title": "Doc A", "url": "https://example.com/a"}},
		"open_questions":    []interface{}{"What changed recently?"},
	}, i18n.LangEnUS)
	if !strings.Contains(deepResearch, "Research") || !strings.Contains(deepResearch, "Citation Coverage: 85%") || !strings.Contains(deepResearch, "Doc A") {
		t.Fatalf("unexpected deep research IM card: %q", deepResearch)
	}
}

func TestFormatIMCard_DeepResearchKnowledgeBaseIncludesArtifacts(t *testing.T) {
	deepResearch := formatIMCard(map[string]interface{}{
		"type":         "deep-research",
		"query":        "build a knowledge base",
		"mode":         "deep",
		"answer":       "KB summary",
		"report_style": "knowledge_base",
		"coverage_summary": map[string]interface{}{
			"task_count":            4,
			"distinct_domain_count": 2,
		},
		"workflow_phases": []interface{}{
			map[string]interface{}{"label": "scope", "status": "completed"},
			map[string]interface{}{"label": "audit", "status": "current"},
		},
		"object_map": []interface{}{
			map[string]interface{}{"label": "Overview", "task_count": 2},
		},
		"source_inventory": []interface{}{
			map[string]interface{}{"title": "Official docs", "url": "https://example.com/docs"},
		},
	}, i18n.LangEnUS)
	for _, token := range []string{"Report style: knowledge_base", "Official docs", "Overview"} {
		if !strings.Contains(deepResearch, token) {
			t.Fatalf("unexpected deep research IM KB card, missing %q: %q", token, deepResearch)
		}
	}
}

func TestFormatIMCard_DeepResearchProgressIncludesIterationAndGap(t *testing.T) {
	progress := formatIMCard(map[string]interface{}{
		"type":          "deep-research-progress",
		"stage":         "verify",
		"status":        "running",
		"progress":      62,
		"iteration":     2,
		"latest_action": "verification_completed",
		"latest_gap":    "Need primary evidence",
		"query":         "ZimaOS market",
	}, i18n.LangEnUS)
	for _, token := range []string{"Research Verifying", "62%", "Current iteration: 2", "Latest action: Verification completed", "Latest gap: Need primary evidence"} {
		if !strings.Contains(progress, token) {
			t.Fatalf("unexpected deep research progress IM card, missing %q: %q", token, progress)
		}
	}
}

func TestFormatIMCard_DeepResearchIncludesVNextSummary(t *testing.T) {
	deepResearch := formatIMCard(map[string]interface{}{
		"type":              "deep-research",
		"query":             "ZimaOS market",
		"mode":              "deep",
		"answer":            "Summary body",
		"confidence":        0.91,
		"evidence_count":    4,
		"citation_coverage": 0.85,
		"iterations":        2,
		"stop_reason":       "coverage_sufficient",
		"latest_action":     "loop_stopped",
		"latest_gap":        "Need primary evidence",
		"strict_entity":     true,
		"time_windows":      []string{"2024", "2025"},
		"report_style":      "timeline",
		"verification_summary": map[string]interface{}{
			"resolved_count":     1,
			"conflicted_count":   0,
			"insufficient_count": 1,
			"items": []interface{}{
				map[string]interface{}{"focus": "Official", "gap": "Need primary evidence", "status": "insufficient"},
			},
		},
		"research_trace": []interface{}{
			map[string]interface{}{"iteration": 1, "focus": "Official", "gap": "Need primary evidence", "follow_up_query": "topic official source", "evidence_added": 1, "verification_outcome": "insufficient"},
		},
		"citations":      []interface{}{map[string]interface{}{"title": "Doc A", "url": "https://example.com/a"}},
		"open_questions": []interface{}{"What changed recently?"},
	}, i18n.LangEnUS)
	for _, token := range []string{"Iterations: 2", "Stop reason: Coverage target reached", "Latest action: Research loop stopped", "Latest gap: Need primary evidence", "Strict entity matching enabled", "Time windows: 2024, 2025", "Report style: timeline", "Verification:", "Resolved: 1 · Conflicted: 0 · Insufficient: 1", "Official · Need primary evidence · Insufficient", "Research trace:", "#1 · Official · Need primary evidence", "Query: topic official source", "Doc A"} {
		if !strings.Contains(deepResearch, token) {
			t.Fatalf("unexpected deep research IM card, missing %q: %q", token, deepResearch)
		}
	}
}

func TestFormatIMCard_StructuredResearchResultUsesSurfaceTitle(t *testing.T) {
	result := formatIMCard(map[string]interface{}{
		"type":    "result",
		"title":   "deep_research",
		"message": "Summarized",
		"details": []interface{}{"Doc A"},
	}, i18n.LangEnUS)

	if !strings.Contains(result, "Research Update") {
		t.Fatalf("expected structured research result to use research update title, got %q", result)
	}
	if strings.Contains(result, "Deep Research Update") {
		t.Fatalf("expected structured research result not to use legacy deep research title, got %q", result)
	}
}

func TestToolFallbackHumanLabel_ResearchUsesSurfaceLabel(t *testing.T) {
	if got := toolFallbackHumanLabel("deep_research", false); got != "research" {
		t.Fatalf("toolFallbackHumanLabel english = %q, want %q", got, "research")
	}
	if got := toolFallbackHumanLabel("deep-research", true); got != "研究" {
		t.Fatalf("toolFallbackHumanLabel chinese = %q, want %q", got, "研究")
	}
}

func TestBuildIMCardEmitter_DeepResearchProgressDedupesButKeepsMeaningfulUpdates(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent []channel.OutgoingMessage
	h.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		if channelName != "feishu" {
			t.Fatalf("channelName = %q, want feishu", channelName)
		}
		sent = append(sent, out)
		return nil
	})

	emitter := h.buildIMCardEmitter(context.Background(), "feishu", "chat_1", "msg_1", i18n.LangEnUS)
	if emitter == nil {
		t.Fatal("expected IM card emitter")
	}

	base := map[string]interface{}{
		"type":          "deep-research-progress",
		"stage":         "verify",
		"name":          "Verifying",
		"status":        "running",
		"progress":      62,
		"iteration":     2,
		"latest_action": "verification_completed",
		"latest_gap":    "Need primary evidence",
		"query":         "ZimaOS market",
	}
	emitter(base)
	emitter(base)
	emitter(map[string]interface{}{
		"type":          "deep-research-progress",
		"stage":         "planning",
		"name":          "Planning",
		"status":        "running",
		"progress":      68,
		"iteration":     3,
		"latest_action": "followup_planned",
		"latest_gap":    "Need fresher sources",
		"query":         "ZimaOS market",
	})

	if len(sent) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sent))
	}
	if sent[0].ChatID != "chat_1" || sent[0].ReplyToID != "msg_1" {
		t.Fatalf("unexpected routing: %+v", sent[0])
	}
	if sent[0].Format != "markdown" {
		t.Fatalf("format = %q, want markdown", sent[0].Format)
	}
	if got, _ := sent[0].Metadata["show_details"].(bool); !got {
		t.Fatalf("expected show_details metadata, got %#v", sent[0].Metadata)
	}
	for _, token := range []string{"Current iteration: 2", "Latest action: Verification completed", "Latest gap: Need primary evidence"} {
		if !strings.Contains(sent[0].Content, token) {
			t.Fatalf("first emitted content missing %q: %q", token, sent[0].Content)
		}
	}
	for _, token := range []string{"Current iteration: 3", "Latest action: Follow-up planned", "Latest gap: Need fresher sources"} {
		if !strings.Contains(sent[1].Content, token) {
			t.Fatalf("second emitted content missing %q: %q", token, sent[1].Content)
		}
	}
}

func TestSendIMToolResultCards_DeepResearchPreservesVNextSummaryAndMetadata(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent []channel.OutgoingMessage
	h.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		if channelName != "slack" {
			t.Fatalf("channelName = %q, want slack", channelName)
		}
		sent = append(sent, out)
		return nil
	})

	h.sendIMToolResultCards(context.Background(), "slack", "chat_dr", "msg_root", i18n.LangEnUS,
		[]llm.ToolCall{{Name: "deep_research"}},
		[]llm.Message{{Content: `{"query":"ZimaOS","mode":"deep","answer":"summary","confidence":0.9,"evidence_count":2,"iterations":2,"stop_reason":"coverage_sufficient","latest_action":"loop_stopped","latest_gap":"Need primary evidence","strict_entity":true,"time_windows":["2024","2025"],"report_style":"timeline","research_trace":[{"iteration":1,"focus":"Official","gap":"Need primary evidence","follow_up_query":"topic official source","evidence_added":1,"verification_outcome":"insufficient"}],"verification_summary":{"resolved_count":1,"conflicted_count":0,"insufficient_count":1,"items":[{"focus":"Official","gap":"Need primary evidence","status":"insufficient"}]},"citations":[{"title":"Doc A","url":"https://example.com/a"}]}`}},
	)

	if len(sent) != 1 {
		t.Fatalf("sent messages = %d, want 1", len(sent))
	}
	msg := sent[0]
	if msg.ChatID != "chat_dr" || msg.ReplyToID != "msg_root" {
		t.Fatalf("unexpected routing: %+v", msg)
	}
	if msg.Format != "markdown" {
		t.Fatalf("format = %q, want markdown", msg.Format)
	}
	if got, _ := msg.Metadata["show_details"].(bool); !got {
		t.Fatalf("expected show_details metadata, got %#v", msg.Metadata)
	}
	for _, token := range []string{"Research", "Iterations: 2", "Stop reason: Coverage target reached", "Latest action: Research loop stopped", "Latest gap: Need primary evidence", "Verification:", "Research trace:", "Doc A"} {
		if !strings.Contains(msg.Content, token) {
			t.Fatalf("sent content missing %q: %q", token, msg.Content)
		}
	}
}

func TestSendIMToolResultCards_SendsStructuredResults(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent []channel.OutgoingMessage
	h.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		if channelName != "feishu" {
			t.Fatalf("channelName = %q, want feishu", channelName)
		}
		sent = append(sent, out)
		return nil
	})

	h.sendIMToolResultCards(context.Background(), "feishu", "chat_1", "msg_1", i18n.LangEnUS,
		[]llm.ToolCall{{Name: "ui_reviewer"}, {Name: "deep_research"}},
		[]llm.Message{
			{Content: `{"url":"https://example.com","overall":82,"pass":true,"visual":{"score":80},"functional":{"score":84},"accessibility":{"score":81}}`},
			{Content: `{"query":"ZimaOS","mode":"deep","answer":"summary","confidence":0.9,"evidence_count":2,"citations":[{"title":"Doc A","url":"https://example.com/a"}]}`},
		},
	)

	if len(sent) != 2 {
		t.Fatalf("sent messages = %d, want 2", len(sent))
	}
	if !strings.Contains(sent[0].Content, "UI Review") {
		t.Fatalf("first IM content = %q, want UI Review summary", sent[0].Content)
	}
	if !strings.Contains(sent[1].Content, "Research") {
		t.Fatalf("second IM content = %q, want Research summary", sent[1].Content)
	}
}

func TestUpsertIMTodoChecklist_UpdatesExistingIMMessage(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent []channel.OutgoingMessage
	var updates []struct {
		channelName string
		chatID      string
		messageID   string
		content     string
	}

	h.SetChannelSenderWithID(func(_ context.Context, channelName string, out channel.OutgoingMessage) (string, error) {
		sent = append(sent, out)
		if channelName != "feishu" {
			t.Fatalf("channelName = %q, want feishu", channelName)
		}
		return "todo-msg-1", nil
	})
	h.SetChannelMessageUpdater(func(_ context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error {
		updates = append(updates, struct {
			channelName string
			chatID      string
			messageID   string
			content     string
		}{channelName: channelName, chatID: chatID, messageID: messageID, content: out.Content})
		return nil
	})

	state := &imTodoMessageState{}
	h.upsertIMTodoChecklist(context.Background(), state, "feishu", "chat-1", "msg-root", "conv-1", "- [ ] gather facts\n- [ ] write summary")
	h.upsertIMTodoChecklist(context.Background(), state, "feishu", "chat-1", "msg-root", "conv-1", "- [x] gather facts\n- [ ] write summary")

	if len(sent) != 1 {
		t.Fatalf("initial sends = %d, want 1", len(sent))
	}
	if len(updates) != 1 {
		t.Fatalf("updates = %d, want 1", len(updates))
	}
	if state.ChannelMessageID != "todo-msg-1" {
		t.Fatalf("state.ChannelMessageID = %q, want todo-msg-1", state.ChannelMessageID)
	}
	if updates[0].messageID != "todo-msg-1" {
		t.Fatalf("updated message id = %q, want todo-msg-1", updates[0].messageID)
	}
	if !strings.Contains(updates[0].content, "[x] gather facts") {
		t.Fatalf("updated content = %q, want completed checklist", updates[0].content)
	}
}

func TestTodoChecklistCardID_UsesStableEncodedMessageID(t *testing.T) {
	if got := todoChecklistCardID(" msg 1/2 "); got != "todo-checklist-msg+1%2F2" {
		t.Fatalf("todoChecklistCardID = %q, want %q", got, "todo-checklist-msg+1%2F2")
	}
	if got := todoChecklistCardID("   "); got != "" {
		t.Fatalf("todoChecklistCardID for blank id = %q, want empty string", got)
	}
}

func TestUpsertIMTodoChecklist_FallsBackToResendWhenNoEditableMessageID(t *testing.T) {
	h := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer h.Close()

	var sent []channel.OutgoingMessage
	h.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		sent = append(sent, out)
		if channelName != "slack" {
			t.Fatalf("channelName = %q, want slack", channelName)
		}
		return nil
	})

	state := &imTodoMessageState{}
	h.upsertIMTodoChecklist(context.Background(), state, "slack", "chat-1", "msg-root", "conv-1", "- [ ] gather facts\n- [ ] write summary")
	h.upsertIMTodoChecklist(context.Background(), state, "slack", "chat-1", "msg-root", "conv-1", "- [x] gather facts\n- [ ] write summary")

	if len(sent) != 2 {
		t.Fatalf("resend count = %d, want 2", len(sent))
	}
	if state.ChannelMessageID != "" {
		t.Fatalf("state.ChannelMessageID = %q, want empty", state.ChannelMessageID)
	}
	if !strings.Contains(sent[1].Content, "[x] gather facts") {
		t.Fatalf("second send content = %q, want updated checklist", sent[1].Content)
	}
}

func TestBrowserCheckpointRequesterWeb_LocalizesPendingQuestion(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	broker := sse.NewBroker()
	client := broker.Subscribe("user-zh")
	defer broker.Unsubscribe("user-zh", client)

	questionMgr := tools.NewQuestionManager(broker, func() bool { return false }, 2*time.Minute)
	handler.SetQuestionManager(questionMgr)

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"web",
		"user-zh",
		"session-zh",
		"",
		"",
		i18n.LangZhCN,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	resultCh := make(chan tools.BrowserCheckpointResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
			Required:  true,
			RiskLevel: "high",
			Step:      "act",
			Action:    "click",
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	pending := waitPendingQuestionForUser(t, questionMgr, "user-zh", 2*time.Second)
	if got := pending.Questions[0].Question; got != "浏览器操作需要确认。是否继续？" {
		t.Fatalf("question = %q", got)
	}
	if got := pending.Questions[0].Detail; got != "步骤: 交互 | 动作: 点击" {
		t.Fatalf("detail = %q", got)
	}
	if got := pending.Questions[0].Options[0].Label; got != "继续" {
		t.Fatalf("first option = %q", got)
	}
	if got, _ := pending.Context["step"].(string); got != "交互" {
		t.Fatalf("context step = %q", got)
	}
	if got, _ := pending.Context["action"].(string); got != "点击" {
		t.Fatalf("context action = %q", got)
	}
	if !questionMgr.ResolveAnswer(pending.ID, []tools.QuestionAnswerResult{{
		QuestionID: "browser_checkpoint",
		Selected:   []string{"continue"},
	}}) {
		t.Fatalf("failed to resolve question id=%s", pending.ID)
	}

	select {
	case err := <-errCh:
		t.Fatalf("requester returned error: %v", err)
	case result := <-resultCh:
		if result.Decision != tools.BrowserCheckpointApprove {
			t.Fatalf("decision = %s, want %s", result.Decision, tools.BrowserCheckpointApprove)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for requester result")
	}
}

func TestBrowserCheckpointRequesterIM_LocalizesConfirmMessage(t *testing.T) {
	handler := NewChatHandler(nil, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()
	handler.SetBrowserCheckpointManager(tools.NewBrowserCheckpointManager(2 * time.Minute))

	var sent channel.OutgoingMessage
	handler.SetChannelSender(func(_ context.Context, channelName string, out channel.OutgoingMessage) error {
		if channelName != "feishu" {
			t.Fatalf("channelName = %q, want feishu", channelName)
		}
		sent = out
		return nil
	})

	requester := handler.buildBrowserCheckpointRequester(
		context.Background(),
		"feishu",
		"user-im-zh",
		"session-im-zh",
		"chat-im-zh",
		"msg-im-zh",
		i18n.LangZhCN,
	)
	if requester == nil {
		t.Fatal("expected browser checkpoint requester")
	}

	result, err := requester(context.Background(), tools.BrowserCheckpointRequest{
		Required:  true,
		RiskLevel: "high",
		Step:      "act",
		Action:    "click",
		URL:       "https://example.com",
	})
	if err != nil {
		t.Fatalf("requester returned error: %v", err)
	}
	if !result.Pending {
		t.Fatalf("expected pending checkpoint result")
	}
	want := "高风险浏览器操作需要确认。\n步骤: 交互\n动作: 点击\nURL: https://example.com\n回复 `1` 继续，或回复 `2` 取消。"
	if result.Message != want {
		t.Fatalf("result message = %q, want %q", result.Message, want)
	}
	if sent.Content != want {
		t.Fatalf("sent content = %q, want %q", sent.Content, want)
	}
}

func TestChatHandlerListToolsUsesSettingsLocaleForExamples(t *testing.T) {
	store, _ := memory.NewStore(":memory:")
	defer store.Close()

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	toolRegistry.Register(&staticToolMock{def: tools.ToolDefinition{
		Name:        "localized_tool",
		Description: "test tool",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"lang": map[string]interface{}{
					"type":        "string",
					"description": "Optional locale for command execution (e.g. en-US, zh-CN, ja-JP). When omitted, uses the system/default environment locale.",
				},
			},
		},
	}})

	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "zh-CN"
	handler.SetSettingsHandler(settingsHandler)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tools", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ListTools(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	var langDesc string
	for _, tool := range resp {
		if tool["name"] != "localized_tool" {
			continue
		}
		params, _ := tool["parameters"].(map[string]interface{})
		props, _ := params["properties"].(map[string]interface{})
		langProp, _ := props["lang"].(map[string]interface{})
		langDesc, _ = langProp["description"].(string)
		break
	}

	if langDesc == "" {
		t.Fatal("expected lang description in tools list")
	}
	if !strings.Contains(langDesc, "zh-CN") {
		t.Fatalf("lang description = %q, want zh-CN example", langDesc)
	}
	if strings.Contains(langDesc, "en-US") || strings.Contains(langDesc, "ja-JP") {
		t.Fatalf("lang description should use only the configured locale example: %q", langDesc)
	}
}

func TestChatHandlerSendMessage_NoProviderDeepResearchFallbackIncludesVNextFields(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Fallback Conv VNext")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.Locale = "en-US"
	handler.SetSettingsHandler(settings)
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer":         "Deep research vNext fallback answer.",
			"citations":      []map[string]interface{}{{"title": "Doc A", "url": "https://example.com/a"}},
			"iteration":      2,
			"iterations":     2,
			"latest_gap":     "Need primary evidence",
			"latest_action":  "completed",
			"stop_reason":    "coverage_sufficient",
			"strict_entity":  true,
			"time_windows":   []string{"2024", "2025"},
			"report_style":   "timeline",
			"research_trace": []map[string]interface{}{{"iteration": 1, "focus": "Official", "gap": "Need primary evidence", "follow_up_query": "topic official source", "evidence_added": 1, "verification_outcome": "insufficient"}},
			"verification_summary": map[string]interface{}{
				"resolved_count":     1,
				"conflicted_count":   0,
				"insufficient_count": 1,
				"items":              []map[string]interface{}{{"focus": "Official", "status": "insufficient", "gap": "Need primary evidence"}},
			},
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"please deep research topic and show the verification summary","provider":"","model":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.SendMessage(c); err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	content, _ := resp["content"].(string)
	for _, token := range []string{
		`"iteration":2`,
		`"iterations":2`,
		`"latest_gap":"Need primary evidence"`,
		`"latest_action":"completed"`,
		`"stop_reason":"coverage_sufficient"`,
		`"strict_entity":true`,
		`"time_windows":["2024","2025"]`,
		`"report_style":"timeline"`,
		`"research_trace":[`,
		`"verification_summary":{`,
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("response content missing %q: %s", token, content)
		}
	}
}
