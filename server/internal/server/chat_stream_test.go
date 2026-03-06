package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/claudecode"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxy"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/proxybridge"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
	"github.com/tidwall/gjson"
)

// StreamingMockProvider is a mock provider that properly implements streaming
type StreamingMockProvider struct {
	response    string
	streamError error
	lastReq     llm.ChatRequest
}

type trialToolRoundStreamingProvider struct {
	mu        sync.Mutex
	callCount int
}

// autoContinueFailingProxyHandler simulates:
// 1) first stream round returns a TODO checklist (triggers auto-continue)
// 2) follow-up rounds fail before any chunks with upstream_error 502
type autoContinueFailingProxyHandler struct {
	callCount             int
	requestModels         []string
	requestPinnedProvider []string
	requestDisableCont    []bool
}

// continuationRetryThenContinueProxyHandler simulates:
// 1) first round emits a TODO checklist (toolless, triggers auto-continue)
// 2) second request fails pre-content with a continuation-related 502
// 3) retry succeeds with another pending checklist
// 4) retries and follow-up rounds should keep previous_response_id continuity
type continuationRetryThenContinueProxyHandler struct {
	callCount                int
	requestDisableCont       []bool
	effectiveUpstreamPrevIDs []string
	cachedPreviousResponseID string
}

// continuationStage2RecoveryProxyHandler simulates:
// 1) first round emits a TODO checklist
// 2) second round fails (continuation error)
// 3) stage-1 silent recovery also fails
// 4) stage-2 reduced-payload recovery succeeds with another pending checklist
// 5) next normal round should restore continuation mode
type continuationStage2RecoveryProxyHandler struct {
	callCount                int
	requestDisableCont       []bool
	effectiveUpstreamPrevIDs []string
	cachedPreviousResponseID string
	requestMsgCounts         []int
	requestToolCounts        []int
}

// autoContinuePlanThenCompleteProxyHandler simulates:
// 1) first stream round outputs a plan checklist only
// 2) auto-continue round receives injected execution nudge and returns final summary
type autoContinuePlanThenCompleteProxyHandler struct {
	callCount                int
	sawExecutionNudge        bool
	sawMissingNextStepsNudge bool
	lastRequestMessage       string
}

// autoContinueScriptedProxyHandler simulates a toolless first round and checks
// whether the second round receives the expected auto-continue nudge content.
type autoContinueScriptedProxyHandler struct {
	callCount                int
	firstRoundContent        string
	secondRoundContent       string
	roundContents            []string
	sawExecutionNudge        bool
	sawAgentLoopNudge        bool
	sawPseudoToolNudge       bool
	sawMissingNextStepsNudge bool
	lastRequestMessage       string
	requestModels            []string
}

// toolCallThenTextProxyHandler simulates a round that emits text, then tool_call,
// then more text in the same round. The post-tool-call text should be suppressed
// from user-visible SSE deltas.
type toolCallThenTextProxyHandler struct {
	callCount int
}

// actionPledgeThenToolCallProxyHandler simulates:
// 1) first round returns an action-pledge placeholder (toolless)
// 2) auto-continue round emits a structured tool_call
// 3) post-tool round returns a final summary
type actionPledgeThenToolCallProxyHandler struct {
	callCount          int
	sawExecutionNudge  bool
	lastRequestMessage string
}

// missingTodoAfterToolRoundProxyHandler simulates:
// 1) first round executes a tool call but emits no TODO checklist
// 2) second round returns an intermediate toolless reply (still no TODO)
// 3) auto-continue should inject missing_todo nudge and trigger a final round
type missingTodoAfterToolRoundProxyHandler struct {
	callCount                int
	sawMissingTodoNudge      bool
	sawMissingNextStepsNudge bool
	lastRequestMessage       string
}

func hasMissingNextStepsNudge(s string) bool {
	return strings.Contains(s, "Suggested next steps") ||
		strings.Contains(s, "WITHOUT calling tools")
}

// secondTurnTimeoutProxyHandler simulates a provider that succeeds on the first
// turn, but times out once history expands on the second turn.
// It allows us to verify "second send fails" behavior independent of warmup.
type secondTurnTimeoutProxyHandler struct {
	mu                sync.Mutex
	firstMessageCount int
	requestMsgCounts  []int
}

// missingTerminalMarkerProxyHandler emits a valid delta chunk but omits
// response.completed / [DONE] from upstream SSE payload.
type missingTerminalMarkerProxyHandler struct{}

func (h *secondTurnTimeoutProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Messages []json.RawMessage `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	msgCount := len(body.Messages)

	h.mu.Lock()
	if h.firstMessageCount == 0 {
		h.firstMessageCount = msgCount
	}
	h.requestMsgCounts = append(h.requestMsgCounts, msgCount)
	firstCount := h.firstMessageCount
	h.mu.Unlock()

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_second_turn_timeout"
		rr.Model = "gpt-5.3-codex-spark"
	}

	// Simulate upstream timeout once context grows beyond first turn payload.
	if msgCount > firstCount {
		http.Error(w, "Request timed out. The server may be busy — please try again later.", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"first turn ok"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *secondTurnTimeoutProxyHandler) RequestMsgCounts() []int {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]int, len(h.requestMsgCounts))
	copy(out, h.requestMsgCounts)
	return out
}

func (h *missingTerminalMarkerProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_missing_terminal_marker"
		rr.Model = "gpt-5.3-codex-spark"
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_missing_done","choices":[{"delta":{"content":"stream continues"},"finish_reason":null}],"model":"gpt-5.3-codex-spark"}`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func warmupCachedForConversation(h *ChatHandler, convID string) bool {
	h.warmupMu.Lock()
	defer h.warmupMu.Unlock()
	_, ok := h.warmupCache[convID]
	return ok
}

func newTCP4TestServerOrSkip(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip local HTTP server test: cannot bind tcp4 listener: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}

func (p *trialToolRoundStreamingProvider) Name() string {
	return "trial-tool-stream"
}

func (p *trialToolRoundStreamingProvider) Models() []string {
	return []string{"trial-stream-model"}
}

func (p *trialToolRoundStreamingProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	p.callCount++
	call := p.callCount
	p.mu.Unlock()

	if call == 1 {
		return &llm.ChatResponse{
			ID:         "trial-round-1",
			Model:      req.Model,
			Provider:   "trial",
			ProviderID: providerpool.TrialProviderID,
			Message: llm.Message{
				Role: llm.RoleAssistant,
				ToolCalls: []llm.ToolCall{
					{ID: "tc-1", Name: "dummy_tool", Arguments: "{}"},
				},
			},
			Usage: llm.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}, nil
	}

	return &llm.ChatResponse{
		ID:         "trial-round-2",
		Model:      req.Model,
		Provider:   "trial",
		ProviderID: providerpool.TrialProviderID,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "final answer",
		},
		Usage: llm.Usage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10},
	}, nil
}

func (p *trialToolRoundStreamingProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	resp, err := p.Chat(ctx, req)
	if err != nil {
		close(ch)
		return nil, err
	}
	ch <- llm.StreamChunk{
		ID:         resp.ID,
		Model:      resp.Model,
		Provider:   resp.Provider,
		ProviderID: resp.ProviderID,
		Delta:      resp.Message.Content,
		ToolCalls:  resp.Message.ToolCalls,
		Done:       true,
		Usage:      &resp.Usage,
	}
	close(ch)
	return ch, nil
}

func (p *trialToolRoundStreamingProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}
	return callback(llm.StreamChunk{
		ID:         resp.ID,
		Model:      resp.Model,
		Provider:   resp.Provider,
		ProviderID: resp.ProviderID,
		Delta:      resp.Message.Content,
		ToolCalls:  resp.Message.ToolCalls,
		Done:       true,
		Usage:      &resp.Usage,
	})
}

func runStreamTurn(t *testing.T, h *ChatHandler, convID, reqBody string) string {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+convID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(convID)

	if err := h.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	return rec.Body.String()
}

func extractJSONSSEEvents(t *testing.T, body string) []map[string]interface{} {
	t.Helper()
	events := make([]map[string]interface{}, 0)
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if data == "" || data == "[DONE]" {
			continue
		}
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		events = append(events, event)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to scan SSE body: %v", err)
	}
	return events
}

func newSingleModelOpenAIProxyHandler(t *testing.T, upstreamBaseURL, providerID, modelID string) *proxy.ProxyHandler {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "chat-stream-locale-*")
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
		BaseURL:   strings.TrimSuffix(upstreamBaseURL, "/") + "/v1",
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys: []providerpool.APIKey{
			{ID: "k-" + providerID, Key: "sk-test", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}
	models := []*providerpool.Model{
		{
			ID:           modelID,
			Name:         modelID,
			ProviderID:   provider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	if err := storage.SaveModels(provider.ID, models); err != nil {
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

func (h *autoContinueFailingProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	h.requestPinnedProvider = append(h.requestPinnedProvider, proxy.GetPinnedProvider(r.Context()))
	h.requestDisableCont = append(h.requestDisableCont, strings.TrimSpace(r.Header.Get(proxy.DisableResponsesContinuationHeader)) == "1")

	var body struct {
		Model string `json:"model"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.requestModels = append(h.requestModels, body.Model)

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_ui_reviewer"
		rr.Model = "gpt-5.3-codex-spark"
	}

	// First round succeeds with TODO checklist content + stop.
	if h.callCount == 1 {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if f, ok := w.(http.Flusher); ok {
			fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"auto"}`)
			f.Flush()
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"auto"}`)
		return
	}

	// Next round(s) fail before any SSE chunk.
	http.Error(w, `{"error":{"message":"Upstream request failed","type":"upstream_error"}}`, http.StatusBadGateway)
}

func (h *continuationRetryThenContinueProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	disableCont := strings.TrimSpace(r.Header.Get(proxy.DisableResponsesContinuationHeader)) == "1"
	h.requestDisableCont = append(h.requestDisableCont, disableCont)
	effectivePrevID := ""
	if !disableCont {
		effectivePrevID = h.cachedPreviousResponseID
	}
	h.effectiveUpstreamPrevIDs = append(h.effectiveUpstreamPrevIDs, effectivePrevID)

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_cont_retry"
		rr.Model = "gpt-5.3-codex-spark"
	}

	switch h.callCount {
	case 1:
		h.cachedPreviousResponseID = "resp_round_1"
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	case 2:
		http.Error(w, `{"error":{"message":"previous_response_id stale","type":"upstream_error"}}`, http.StatusBadGateway)
		return
	case 3:
		h.cachedPreviousResponseID = "resp_round_2"
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_2","choices":[{"delta":{"content":"- [ ] still pending"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	default:
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_3","choices":[{"delta":{"content":"任务已完成。完成内容：已执行。使用方法：已验证。下一步建议：无。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
}

func (h *continuationStage2RecoveryProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	disableCont := strings.TrimSpace(r.Header.Get(proxy.DisableResponsesContinuationHeader)) == "1"
	h.requestDisableCont = append(h.requestDisableCont, disableCont)
	effectivePrevID := ""
	if !disableCont {
		effectivePrevID = h.cachedPreviousResponseID
	}
	h.effectiveUpstreamPrevIDs = append(h.effectiveUpstreamPrevIDs, effectivePrevID)
	var body struct {
		Messages []json.RawMessage `json:"messages"`
		Tools    []json.RawMessage `json:"tools"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.requestMsgCounts = append(h.requestMsgCounts, len(body.Messages))
	h.requestToolCounts = append(h.requestToolCounts, len(body.Tools))

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_cont_stage2"
		rr.Model = "gpt-5.3-codex-spark"
	}

	switch h.callCount {
	case 1:
		h.cachedPreviousResponseID = "resp_round_1"
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_1","choices":[{"delta":{"content":"- [ ] run command"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	case 2:
		http.Error(w, `{"error":{"message":"previous_response_id stale","type":"upstream_error"}}`, http.StatusBadGateway)
		return
	case 3:
		http.Error(w, `{"error":{"message":"continuation still rejected","type":"upstream_error"}}`, http.StatusBadGateway)
		return
	case 4:
		h.cachedPreviousResponseID = "resp_round_2"
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_2","choices":[{"delta":{"content":"- [ ] still pending"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	default:
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_round_3","choices":[{"delta":{"content":"任务已完成。完成内容：已执行。使用方法：已验证。下一步建议：无。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}
}

func (h *autoContinueScriptedProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++

	var body struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	h.requestModels = append(h.requestModels, body.Model)

	if h.callCount > 1 {
		for i := len(body.Messages) - 1; i >= 0; i-- {
			if body.Messages[i].Role != "user" {
				continue
			}
			h.lastRequestMessage = body.Messages[i].Content
			break
		}
		h.sawExecutionNudge = strings.Contains(h.lastRequestMessage, "Now actually execute by calling available tools") ||
			strings.Contains(h.lastRequestMessage, "A canonical TODO checklist already exists")
		h.sawAgentLoopNudge = strings.Contains(h.lastRequestMessage, "continuous improvement loop") ||
			strings.Contains(h.lastRequestMessage, "A canonical TODO checklist already exists")
		h.sawPseudoToolNudge = strings.Contains(h.lastRequestMessage, "fake tool-call text")
		h.sawMissingNextStepsNudge = h.sawMissingNextStepsNudge || hasMissingNextStepsNudge(h.lastRequestMessage)
	}

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_auto_continue_scripted"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	content := h.firstRoundContent
	if len(h.roundContents) > 0 {
		idx := h.callCount - 1
		if idx >= len(h.roundContents) {
			idx = len(h.roundContents) - 1
		}
		content = h.roundContents[idx]
	} else if h.callCount > 1 {
		content = h.secondRoundContent
	}
	fmt.Fprintf(
		w,
		"data: %s\n\n",
		fmt.Sprintf(`{"id":"%d","choices":[{"delta":{"content":%q},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`, h.callCount, content),
	)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *toolCallThenTextProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_tool_call_then_text"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	if h.callCount == 1 {
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"tool_round_1","choices":[{"delta":{"content":"前置说明：开始执行。"},"finish_reason":null}],"model":"gpt-5.3-codex-spark"}`)
		flush()
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"tool_round_1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"noop_tool","arguments":"{\"foo\":\"bar\"}"}}]},"finish_reason":null}],"model":"gpt-5.3-codex-spark"}`)
		flush()
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"tool_round_1","choices":[{"delta":{"content":"LEAK_AFTER_TOOL_CALL"},"finish_reason":null}],"model":"gpt-5.3-codex-spark"}`)
		flush()
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"tool_round_1","choices":[{"delta":{},"finish_reason":"tool_calls"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", `{"id":"tool_round_2","choices":[{"delta":{"content":"最终结果已返回。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
	flush()
}

func (h *actionPledgeThenToolCallProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++

	if h.callCount == 2 {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for i := len(body.Messages) - 1; i >= 0; i-- {
			if body.Messages[i].Role != "user" {
				continue
			}
			h.lastRequestMessage = body.Messages[i].Content
			break
		}
		h.sawExecutionNudge = strings.Contains(h.lastRequestMessage, "Now actually execute by calling available tools") ||
			strings.Contains(h.lastRequestMessage, "A canonical TODO checklist already exists")
	}

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_action_pledge_tool_round"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	switch h.callCount {
	case 1:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"action_round_1","choices":[{"delta":{"content":"我先帮你快速查一下 OpenClaw 的最新相关新闻与动态。请稍等，我整理成要点给你。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	case 2:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"action_round_2","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_action_1","type":"function","function":{"name":"noop_tool","arguments":"{\"query\":\"openclaw news\"}"}}]},"finish_reason":"tool_calls"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	default:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"action_round_3","choices":[{"delta":{"content":"已完成查询并整理：这是 OpenClaw 的最新动态摘要。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	}
}

func (h *missingTodoAfterToolRoundProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++

	if h.callCount > 1 {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		for i := len(body.Messages) - 1; i >= 0; i-- {
			if body.Messages[i].Role != "user" {
				continue
			}
			h.lastRequestMessage = body.Messages[i].Content
			break
		}
		h.sawMissingTodoNudge = strings.Contains(h.lastRequestMessage, "checklist bootstrap required")
		h.sawMissingNextStepsNudge = h.sawMissingNextStepsNudge || hasMissingNextStepsNudge(h.lastRequestMessage)
	}

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_missing_todo_tool_round"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	flush := func() {
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	switch h.callCount {
	case 1:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"missing_todo_round_1","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_missing_todo_1","type":"function","function":{"name":"noop_tool","arguments":"{\"task\":\"bootstrap\"}"}}]},"finish_reason":"tool_calls"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	case 2:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"missing_todo_round_2","choices":[{"delta":{"content":"已执行第一步，接下来继续优化实现。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	default:
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"missing_todo_round_3","choices":[{"delta":{"content":"任务已完成。最终总结：实现可用并已验证。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		flush()
	}
}

func TestStreamMessage_PropagatesSettingsLocaleToUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "locale stream turn"
	)

	var mu sync.Mutex
	var capturedLocales []string

	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if strings.HasSuffix(r.URL.Path, "/chat/completions") && bytes.Contains(body, []byte(msgText)) {
			mu.Lock()
			capturedLocales = append(capturedLocales, strings.TrimSpace(r.Header.Get("Accept-Language")))
			mu.Unlock()
		}

		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_locale_settings","choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_locale_settings_sync","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "locale-settings-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "zh-CN"
	handler.SetSettingsHandler(settingsHandler)

	conv, err := store.CreateConversation(context.Background(), "Test stream locale from settings")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	respBody := runStreamTurn(t, handler, conv.ID, `{"message":"`+msgText+`","model":"`+modelID+`"}`)
	if strings.Contains(respBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("stream should succeed, body=%s", respBody)
	}
	if !strings.Contains(respBody, `"done":true`) {
		t.Fatalf("stream should contain done marker, body=%s", respBody)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedLocales) == 0 {
		t.Fatal("expected upstream to receive at least one chat request with captured locale")
	}
	for i, got := range capturedLocales {
		if got != "zh-CN" {
			t.Fatalf("captured locale #%d = %q, want %q", i, got, "zh-CN")
		}
	}
}

func TestStreamMessage_ContextLocaleOverridesSettingsForUpstreamAcceptLanguage(t *testing.T) {
	const (
		modelID = "gpt-4o-mini"
		msgText = "context locale turn"
	)

	var mu sync.Mutex
	var capturedLocales []string

	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		if strings.HasSuffix(r.URL.Path, "/chat/completions") && bytes.Contains(body, []byte(msgText)) {
			mu.Lock()
			capturedLocales = append(capturedLocales, strings.TrimSpace(r.Header.Get("Accept-Language")))
			mu.Unlock()
		}

		if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_locale_context","choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"resp_locale_context_sync","choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"model":"gpt-4o-mini"}`))
	}))
	defer upstream.Close()

	proxyHandler := newSingleModelOpenAIProxyHandler(t, upstream.URL, "locale-context-provider", modelID)
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	settingsHandler.settings.Locale = "fr-FR"
	handler.SetSettingsHandler(settingsHandler)

	conv, err := store.CreateConversation(context.Background(), "Test stream locale context override")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(`{"message":"`+msgText+`","model":"`+modelID+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req = req.WithContext(proxy.WithLocale(req.Context(), "ja-JP"))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	respBody := rec.Body.String()
	if strings.Contains(respBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("stream should succeed, body=%s", respBody)
	}
	if !strings.Contains(respBody, `"done":true`) {
		t.Fatalf("stream should contain done marker, body=%s", respBody)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(capturedLocales) == 0 {
		t.Fatal("expected upstream to receive at least one chat request with captured locale")
	}
	for i, got := range capturedLocales {
		if got != "ja-JP" {
			t.Fatalf("captured locale #%d = %q, want %q", i, got, "ja-JP")
		}
	}
}

func (h *autoContinuePlanThenCompleteProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.callCount++
	var body struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if len(body.Messages) > 0 {
		last := body.Messages[len(body.Messages)-1]
		h.lastRequestMessage = last.Content
		if last.Role == "user" && (strings.Contains(last.Content, "Now actually execute by calling available tools") ||
			strings.Contains(last.Content, "A canonical TODO checklist already exists")) {
			h.sawExecutionNudge = true
		}
		if last.Role == "user" && hasMissingNextStepsNudge(last.Content) {
			h.sawMissingNextStepsNudge = true
		}
	}

	if rr := proxy.GetResolvedRouteFromContext(r.Context()); rr != nil {
		rr.Provider = "MockProxy"
		rr.ProviderID = "prov_plan_then_complete"
		rr.Model = "gpt-5.3-codex-spark"
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)

	if h.callCount == 1 {
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"1","choices":[{"delta":{"content":"- [ ] 创建实现计划\n- [ ] 执行计划步骤\n- [ ] 输出结果说明"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		return
	}

	fmt.Fprintf(w, "data: %s\n\n", `{"id":"2","choices":[{"delta":{"content":"已按计划完成关键步骤并验证结果。\n总结：任务已完成。"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func NewStreamingMockProvider(response string) *StreamingMockProvider {
	return &StreamingMockProvider{response: response}
}

func (p *StreamingMockProvider) Name() string {
	return "streaming-mock"
}

func (p *StreamingMockProvider) Models() []string {
	return []string{"streaming-mock-model"}
}

func (p *StreamingMockProvider) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.lastReq = req
	return &llm.ChatResponse{
		ID:      "mock-id",
		Model:   req.Model,
		Message: llm.Message{Role: llm.RoleAssistant, Content: p.response},
		Usage:   llm.Usage{TotalTokens: 10},
	}, nil
}

func (p *StreamingMockProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 10)
	go func() {
		defer close(ch)
		// Send content in chunks
		words := strings.Split(p.response, " ")
		for i, word := range words {
			if i > 0 {
				word = " " + word
			}
			select {
			case <-ctx.Done():
				return
			case ch <- llm.StreamChunk{Delta: word, Done: false}:
			}
		}
		// Send final chunk
		select {
		case <-ctx.Done():
			return
		case ch <- llm.StreamChunk{Done: true, Usage: &llm.Usage{TotalTokens: 10}}:
		}
	}()
	return ch, nil
}

func (p *StreamingMockProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, callback llm.StreamCallback) error {
	p.lastReq = req
	if p.streamError != nil {
		return p.streamError
	}

	// Send content in chunks
	words := strings.Split(p.response, " ")
	for i, word := range words {
		if i > 0 {
			word = " " + word
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := callback(llm.StreamChunk{Delta: word, Done: false}); err != nil {
			return err
		}
	}

	// Send final chunk with Done=true
	return callback(llm.StreamChunk{
		Done:  true,
		Usage: &llm.Usage{PromptTokens: 5, CompletionTokens: 5, TotalTokens: 10},
	})
}

// TestStreamMessageBasic tests basic streaming functionality
func TestStreamMessageBasic(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Stream Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("Hello this is a streaming response")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!", "provider": "streaming-mock", "model": "streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	// Check response headers
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got '%s'", contentType)
	}

	// Parse SSE response
	body := rec.Body.String()
	t.Logf("Response body:\n%s", body)

	if body == "" {
		t.Error("response body is empty - this is the bug!")
	}

	// Check for SSE data lines
	if !strings.Contains(body, "data: ") {
		t.Error("response does not contain SSE data lines")
	}

	// Parse and verify chunks
	var fullContent string
	var gotDone bool
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				continue
			}

			var chunk map[string]interface{}
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				t.Logf("failed to parse chunk: %s, error: %v", data, err)
				continue
			}

			if delta, ok := chunk["delta"].(string); ok {
				fullContent += delta
			}
			if done, ok := chunk["done"].(bool); ok && done {
				gotDone = true
			}
		}
	}

	t.Logf("Full content received: %s", fullContent)
	t.Logf("Got done signal: %v", gotDone)

	if fullContent == "" {
		t.Error("no content received from stream")
	}

	if !gotDone {
		t.Error("did not receive done signal")
	}
}

func TestStreamMessageTrialUsageAggregatesToolRoundsWithoutMetricsRecorder(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Trial Stream Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	registry.Register(&trialToolRoundStreamingProvider{})

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
	reqBody := `{"message":"run tool flow","provider":"trial-tool-stream","model":"trial-stream-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	status := handler.providerPool.TrialQuotaManager.GetStatus()
	if status.TokensUsed != 25 {
		t.Fatalf("trial tokens_used = %d, want 25", status.TokensUsed)
	}
}

func TestStreamMessageBatchesTinyDeltas(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Stream Batch Tiny Deltas")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// 60 tiny word chunks => should be merged into far fewer SSE delta events.
	// Keep total bytes below adaptive byte thresholds to make behavior deterministic.
	parts := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		parts = append(parts, "a")
	}
	responseText := strings.Join(parts, " ")

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider(responseText)
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message":"hi","provider":"streaming-mock","model":"streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if body == "" {
		t.Fatal("empty streaming body")
	}

	var (
		fullContent    strings.Builder
		deltaEventCnt  int
		streamDoneSeen bool
	)

	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		delta, _ := chunk["delta"].(string)
		done, _ := chunk["done"].(bool)
		if delta != "" {
			deltaEventCnt++
			fullContent.WriteString(delta)
		}
		if done {
			streamDoneSeen = true
		}
	}

	if got := fullContent.String(); got != responseText {
		t.Fatalf("streamed content mismatch:\nwant=%q\ngot=%q", responseText, got)
	}
	if !streamDoneSeen {
		t.Fatal("expected final done chunk")
	}
	t.Logf("tiny-delta stream events: %d", deltaEventCnt)
	// We expect significant batching: much fewer events than 60 tiny chunks.
	// Current adaptive strategy should emit 2 delta events in this test.
	if deltaEventCnt > 3 {
		t.Fatalf("expected batched SSE deltas (<=3), got %d", deltaEventCnt)
	}
}

func TestStreamMessageInjectsConversationAnchor(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Stream Anchor Title")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "user", Content: "Initial stream goal: keep context"})
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{Role: "assistant", Content: "ack"})

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("stream response")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSystemPromptBuilder(claudecode.NewSystemPromptBuilder(&claudecode.ClaudeCodeConfig{}))

	e := echo.New()
	reqBody := `{"message":"B","provider":"streaming-mock","model":"streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	if !hasSystemAnchor(mockProvider.lastReq.Messages, "Stream Anchor Title", "Initial stream goal: keep context") {
		t.Fatalf("expected conversation anchor in stream system messages, got %d messages", len(mockProvider.lastReq.Messages))
	}
}

// TestStreamMessageWithNoProvider tests streaming no-provider fallback behavior.
func TestStreamMessageWithNoProvider(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	// Empty registry - no providers
	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message": "Hello!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"provider_unavailable"`) {
		t.Fatalf("expected provider_unavailable in stream body, got: %s", body)
	}
}

func TestStreamMessageWithNoProvider_NonResearchSkipsDeepResearchFallback(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry() // no provider on purpose
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "should not be used",
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"provider_unavailable"`) {
		t.Fatalf("expected provider_unavailable in stream body, got: %s", body)
	}
	execMock.mu.Lock()
	calls := execMock.calls
	execMock.mu.Unlock()
	if calls != 0 {
		t.Fatalf("deep research fallback calls = %d, want 0", calls)
	}
}

func TestStreamMessageWithNoProvider_AutonomousWebSearchFallback(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry() // no provider on purpose
	toolRegistry := tools.NewRegistry()
	webSearchMock := &webSearchToolMock{
		result: map[string]interface{}{
			"query":       "zimaos release notes",
			"provider":    "mock",
			"total_count": 2,
			"results": []map[string]interface{}{
				{"title": "ZimaOS Release Notes", "url": "https://example.com/release", "description": "release summary"},
				{"title": "ZimaOS Docs", "url": "https://example.com/docs", "description": "documentation"},
			},
		},
	}
	toolRegistry.Register(webSearchMock)
	handler := NewChatHandler(store, registry, toolRegistry)
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	e := echo.New()
	reqBody := `{"message":"zimaos release notes"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"provider":"web_search"`) {
		t.Fatalf("expected web_search provider fallback in stream body, got: %s", body)
	}
	if !strings.Contains(body, `"model":"web-search-fallback"`) {
		t.Fatalf("expected web-search-fallback model in stream body, got: %s", body)
	}
	if !strings.Contains(body, "ZimaOS Release Notes") {
		t.Fatalf("expected web-search title in stream body, got: %s", body)
	}
	if !strings.Contains(body, "```typeless") {
		t.Fatalf("expected typeless card block in stream body, got: %s", body)
	}
	if !strings.Contains(body, `\"type\":\"search\"`) {
		t.Fatalf("expected search typeless card in stream body, got: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected [DONE] marker, got: %s", body)
	}

	webSearchMock.mu.Lock()
	calls := webSearchMock.calls
	query, _ := webSearchMock.last["query"].(string)
	format, _ := webSearchMock.last["format"].(string)
	webSearchMock.mu.Unlock()
	if calls != 1 {
		t.Fatalf("web_search calls = %d, want 1", calls)
	}
	if query != "zimaos release notes" {
		t.Fatalf("web_search query = %q, want zimaos release notes", query)
	}
	if format != "json" {
		t.Fatalf("web_search format = %q, want json", format)
	}
}

func TestStreamMessageWithNoProvider_DeepResearchFallback(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry() // no provider on purpose
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	settings := NewSettingsHandler(kvstore.NewMemoryStore())
	settings.settings.Locale = "zh-CN"
	handler.SetSettingsHandler(settings)
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "Deep research stream fallback answer.",
			"citations": []map[string]interface{}{
				{"title": "Doc A", "url": "https://example.com/a"},
			},
			"confidence":     0.82,
			"evidence_count": 1,
			"support_count":  2,
			"conflict_count": 1,
			"has_conflict":   true,
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"please deep research zimaos latest updates with citations"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Deep research stream fallback answer.") {
		t.Fatalf("expected fallback delta in stream body, got: %s", body)
	}
	if !strings.Contains(body, `"provider":"deepresearch"`) {
		t.Fatalf("expected deepresearch provider in stream body, got: %s", body)
	}
	if !strings.Contains(body, "来源:") {
		t.Fatalf("expected localized sources label in stream body, got: %s", body)
	}
	if !strings.Contains(body, "```typeless") {
		t.Fatalf("expected typeless card block in stream body, got: %s", body)
	}
	if !strings.Contains(body, "support_count") || !strings.Contains(body, "has_conflict") {
		t.Fatalf("expected conflict/support metrics in typeless card, got: %s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected [DONE] marker, got: %s", body)
	}
	execMock.mu.Lock()
	gotLang, _ := execMock.last["lang"].(string)
	execMock.mu.Unlock()
	if gotLang != "zh-CN" {
		t.Fatalf("fallback lang = %q, want zh-CN", gotLang)
	}
}

func TestStreamMessageWithNoProvider_DeepResearchFallbackIncludesV2Fields(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv V2")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry() // no provider on purpose
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer":                "时间线报告完成。",
			"citation_coverage":     0.91,
			"stage_errors":          []string{"retry failed: q2"},
			"entity_disambiguation": map[string]interface{}{"enabled": true, "filtered_count": 1},
			"timeline_sections": []map[string]interface{}{
				{"label": "早期", "summary": "物流观点形成"},
				{"label": "近期", "summary": "AI 观点强化"},
			},
			"citations": []map[string]interface{}{
				{"title": "来源A", "url": "https://example.com/a"},
			},
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"请深度调研蓝驰付强观点时间线并附来源"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}
	body := rec.Body.String()
	for _, token := range []string{
		"citation_coverage",
		"entity_disambiguation",
		"timeline_sections",
		"stage_errors",
		"https://example.com/a",
	} {
		if !strings.Contains(body, token) {
			t.Fatalf("stream body missing %q: %s", token, body)
		}
	}
}

func TestStreamMessageWithNoProvider_DeepResearchFallbackUsesRoutingMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Continuation fallback")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "你可以帮我查一下 ZimaOS 的信息吗",
	}); err != nil {
		t.Fatalf("add user message failed: %v", err)
	}
	if _, err := store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "- [ ] 产品定位与功能概览\n- [ ] 最新动态\n\n如果你不想选，我可以默认按「功能概览 + 最新动态」先查一版。",
	}); err != nil {
		t.Fatalf("add assistant message failed: %v", err)
	}

	registry := llm.NewProviderRegistry() // no provider on purpose
	handler := NewChatHandler(store, registry, tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))
	execMock := &deepResearchExecMock{
		result: map[string]interface{}{
			"answer": "Deep research stream fallback answer.",
		},
	}
	handler.deepResearchExec = execMock

	e := echo.New()
	reqBody := `{"message":"好的"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage failed: %v", err)
	}

	execMock.mu.Lock()
	query, _ := execMock.last["query"].(string)
	execMock.mu.Unlock()
	if query != "你可以帮我查一下 ZimaOS 的信息吗" {
		t.Fatalf("deep research fallback query = %q, want routed objective", query)
	}
}

// TestMockProviderChatStreamCallback tests the mock provider's ChatStreamCallback directly
func TestMockProviderChatStreamCallback(t *testing.T) {
	provider := NewStreamingMockProvider("Hello world test")

	var chunks []llm.StreamChunk
	err := provider.ChatStreamCallback(context.Background(), llm.ChatRequest{
		Model: "test",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk)
		t.Logf("Received chunk: delta=%q, done=%v", chunk.Delta, chunk.Done)
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStreamCallback error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("no chunks received")
	}

	// Check that we got a done chunk
	var gotDone bool
	var fullContent string
	for _, chunk := range chunks {
		fullContent += chunk.Delta
		if chunk.Done {
			gotDone = true
		}
	}

	if !gotDone {
		t.Error("did not receive done chunk")
	}

	t.Logf("Full content: %s", fullContent)
	if fullContent == "" {
		t.Error("no content in chunks")
	}
}

// TestStreamMessageEmptyModel tests streaming when model is empty
func TestStreamMessageEmptyModel(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Conv")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("Response with empty model")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	// Request without model specified
	reqBody := `{"message": "Hello!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)

	body := rec.Body.String()
	t.Logf("Response with empty model: %s", body)
	t.Logf("Status code: %d", rec.Code)
	t.Logf("Error: %v", err)

	// Should still work - provider should use default model
	if body == "" && err == nil {
		t.Error("empty response body with no error - this is the bug!")
	}
}

// TestDefaultMockProviderChatStreamCallback tests the default llm.MockProvider
func TestDefaultMockProviderChatStreamCallback(t *testing.T) {
	provider := llm.NewMockProvider()
	provider.SetResponse(llm.ChatResponse{
		ID:      "test-id",
		Model:   "test-model",
		Message: llm.Message{Role: llm.RoleAssistant, Content: "Test response content"},
	})

	var chunks []llm.StreamChunk
	err := provider.ChatStreamCallback(context.Background(), llm.ChatRequest{
		Model: "test",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi"},
		},
	}, func(chunk llm.StreamChunk) error {
		chunks = append(chunks, chunk)
		t.Logf("Received chunk: delta=%q, done=%v", chunk.Delta, chunk.Done)
		return nil
	})

	if err != nil {
		t.Fatalf("ChatStreamCallback error: %v", err)
	}

	if len(chunks) == 0 {
		t.Error("no chunks received from default MockProvider")
	}

	var fullContent string
	var gotDone bool
	for _, chunk := range chunks {
		fullContent += chunk.Delta
		if chunk.Done {
			gotDone = true
		}
	}

	t.Logf("Default MockProvider full content: %s", fullContent)
	t.Logf("Default MockProvider got done: %v", gotDone)

	if fullContent == "" {
		t.Error("default MockProvider returned empty content")
	}
}

func TestStreamMessageAutoContinue_PreContent502GracefulCompletion(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Auto Continue 502")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	// Enable agent mode path (`getMaxToolRounds() > maxToolRounds`) so TODO auto-continue can trigger.
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &autoContinueFailingProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"continue the task","model":"auto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected graceful completion without STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fallbackText := streamContinuationFailureText(settingsHandler.GetLocale()); fallbackText != "" && !strings.Contains(body, fallbackText) {
		t.Fatalf("expected continuation failure fallback text in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 5 {
		t.Fatalf("expected exactly 5 proxy calls (initial + bounded pre-content retry + stage1 + stage2 recovery attempts), got %d", fakeProxy.callCount)
	}
	if len(fakeProxy.requestModels) < 2 || len(fakeProxy.requestPinnedProvider) < 2 {
		t.Fatalf("expected captured request metadata for at least 2 calls, models=%d providers=%d", len(fakeProxy.requestModels), len(fakeProxy.requestPinnedProvider))
	}
	if got := fakeProxy.requestModels[1]; got != "gpt-5.3-codex-spark" {
		t.Fatalf("expected auto-continue to pin model from first round, got %q", got)
	}
	if got := fakeProxy.requestPinnedProvider[1]; got != "prov_ui_reviewer" {
		t.Fatalf("expected auto-continue to pin provider from first round, got %q", got)
	}
	last := len(fakeProxy.requestPinnedProvider) - 1
	if last < 0 || fakeProxy.requestPinnedProvider[last] != "prov_ui_reviewer" {
		t.Fatalf("expected silent recovery to keep pinned provider for prev_response continuity, got %q", fakeProxy.requestPinnedProvider[last])
	}
	if len(fakeProxy.requestDisableCont) <= last || fakeProxy.requestDisableCont[last] {
		t.Fatalf("expected silent recovery to keep continuation enabled, got %v", fakeProxy.requestDisableCont)
	}
}

func TestStreamMessage_MissingTerminalMarkerStillEmitsDoneChunk(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Missing Terminal Marker")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(&missingTerminalMarkerProxyHandler{}))

	body := runStreamTurn(t, handler, conv.ID, `{"message":"hello","model":"auto"}`)
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected successful stream despite missing terminal marker, got body=%s", body)
	}
	if !strings.Contains(body, "stream continues") {
		t.Fatalf("expected streamed delta content, got body=%s", body)
	}
	if !strings.Contains(body, "data: [DONE]") {
		t.Fatalf("expected raw [DONE] sentinel, got body=%s", body)
	}

	events := extractJSONSSEEvents(t, body)
	var doneSeen bool
	for _, event := range events {
		if done, ok := event["done"].(bool); ok && done {
			doneSeen = true
			break
		}
	}
	if !doneSeen {
		t.Fatalf("expected structured done=true SSE event, got body=%s", body)
	}
}

func TestStreamMessageAutoContinue_PreContentRetryKeepsContinuationWithPreviousResponseID(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Continuation Disable Scope")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &continuationRetryThenContinueProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"continue the task","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected graceful completion without STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}

	if fakeProxy.callCount < 4 {
		t.Fatalf("expected at least 4 proxy calls, got %d", fakeProxy.callCount)
	}
	if len(fakeProxy.requestDisableCont) < 4 {
		t.Fatalf("expected disable-continuation markers for at least 4 calls, got %v", fakeProxy.requestDisableCont)
	}
	if len(fakeProxy.effectiveUpstreamPrevIDs) < 4 {
		t.Fatalf("expected effective upstream previous_response_id capture for at least 4 calls, got %v", fakeProxy.effectiveUpstreamPrevIDs)
	}

	if fakeProxy.requestDisableCont[0] {
		t.Fatalf("call 1 should not disable continuation, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[1] {
		t.Fatalf("call 2 should not disable continuation before retry, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[2] {
		t.Fatalf("call 3 (retry) should keep continuation enabled, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[3] {
		t.Fatalf("call 4 should keep continuation enabled, flags=%v", fakeProxy.requestDisableCont)
	}

	if fakeProxy.effectiveUpstreamPrevIDs[0] != "" {
		t.Fatalf("call 1 effective previous_response_id = %q, want empty", fakeProxy.effectiveUpstreamPrevIDs[0])
	}
	if fakeProxy.effectiveUpstreamPrevIDs[1] == "" {
		t.Fatalf("call 2 should include previous_response_id from round 1, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.effectiveUpstreamPrevIDs[2] == "" {
		t.Fatalf("call 3 retry should keep previous_response_id, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.effectiveUpstreamPrevIDs[3] == "" {
		t.Fatalf("call 4 should include previous_response_id from retry-success round, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
}

func TestStreamMessageAutoContinue_Stage2ReducedRecoveryScopedToRecoveryOnly(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Continuation Stage2 Scope")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &continuationStage2RecoveryProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"continue the task","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected graceful completion without STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}

	if fakeProxy.callCount < 5 {
		t.Fatalf("expected at least 5 proxy calls with stage2 recovery, got %d", fakeProxy.callCount)
	}
	if len(fakeProxy.requestDisableCont) < 5 {
		t.Fatalf("expected disable-continuation markers for at least 5 calls, got %v", fakeProxy.requestDisableCont)
	}
	if len(fakeProxy.effectiveUpstreamPrevIDs) < 5 {
		t.Fatalf("expected effective upstream previous_response_id capture for at least 5 calls, got %v", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if len(fakeProxy.requestMsgCounts) < 5 || len(fakeProxy.requestToolCounts) < 5 {
		t.Fatalf("expected request size capture for at least 5 calls, msg_counts=%v tool_counts=%v", fakeProxy.requestMsgCounts, fakeProxy.requestToolCounts)
	}

	if fakeProxy.requestDisableCont[0] {
		t.Fatalf("call 1 should not disable continuation, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[1] {
		t.Fatalf("call 2 should not disable continuation before recovery, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[2] {
		t.Fatalf("call 3 (stage1 recovery) should keep continuation enabled, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[3] {
		t.Fatalf("call 4 (stage2 reduced payload) should keep continuation enabled, flags=%v", fakeProxy.requestDisableCont)
	}
	if fakeProxy.requestDisableCont[4] {
		t.Fatalf("call 5 should restore normal continuation mode, flags=%v", fakeProxy.requestDisableCont)
	}

	if fakeProxy.effectiveUpstreamPrevIDs[0] != "" {
		t.Fatalf("call 1 effective previous_response_id = %q, want empty", fakeProxy.effectiveUpstreamPrevIDs[0])
	}
	if fakeProxy.effectiveUpstreamPrevIDs[1] == "" {
		t.Fatalf("call 2 should include previous_response_id from round 1, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.effectiveUpstreamPrevIDs[2] == "" {
		t.Fatalf("call 3 stage1 recovery should keep previous_response_id, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.effectiveUpstreamPrevIDs[3] == "" {
		t.Fatalf("call 4 stage2 recovery should keep previous_response_id, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.effectiveUpstreamPrevIDs[4] == "" {
		t.Fatalf("call 5 should include previous_response_id from stage2-success round, got empty (prev_ids=%v)", fakeProxy.effectiveUpstreamPrevIDs)
	}
	if fakeProxy.requestMsgCounts[3] >= fakeProxy.requestMsgCounts[2] {
		t.Fatalf("call 4 stage2 payload should have fewer messages than call 3 stage1 payload, msg_counts=%v", fakeProxy.requestMsgCounts)
	}
	if fakeProxy.requestToolCounts[3] > fakeProxy.requestToolCounts[2] {
		t.Fatalf("call 4 stage2 payload should not increase tools vs call 3, tool_counts=%v", fakeProxy.requestToolCounts)
	}
}

func TestStreamMessageAutoContinue_PlanThenExecuteThenSummary(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Auto Continue Plan Execute Summary")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &autoContinuePlanThenCompleteProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请进入agent mode并完成任务","model":"auto"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	err = handler.StreamMessage(c)
	if err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected exactly 2 proxy calls (plan + auto-continue execution), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected second request to include auto-continue execution nudge, last=%q", fakeProxy.lastRequestMessage)
	}
	if fakeProxy.sawMissingNextStepsNudge {
		t.Fatalf("expected no missing_next_steps nudge in plan-execute flow, last=%q", fakeProxy.lastRequestMessage)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected persisted messages, got empty")
	}

	var assistantContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			assistantContent = messages[i].Content
			break
		}
	}
	if assistantContent == "" {
		t.Fatalf("assistant message not found in persisted messages: %+v", messages)
	}
	if !strings.Contains(assistantContent, "总结：任务已完成。") {
		t.Fatalf("expected summary content in persisted assistant message, got=%q", assistantContent)
	}
}

func TestStreamMessageAutoContinue_MissingNextStepsSingleFollowUp(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test missing next steps follow-up")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &autoContinueScriptedProxyHandler{
		roundContents: []string{
			"任务已完成。最终总结：功能可用。",
			"任务已完成。最终总结：功能可用。\nSuggested next steps:\n1. 运行回归测试。\n2. 观察运行日志。",
		},
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请进入agent mode并完成任务","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected 2 proxy calls (completion + next-steps follow-up), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawMissingNextStepsNudge {
		t.Fatalf("expected second request to include missing_next_steps nudge, last=%q", fakeProxy.lastRequestMessage)
	}
	if fakeProxy.sawExecutionNudge {
		t.Fatalf("expected missing_next_steps path not to use execution nudge, last=%q", fakeProxy.lastRequestMessage)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	var assistantContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			assistantContent = messages[i].Content
			break
		}
	}
	if !strings.Contains(assistantContent, "Suggested next steps:") {
		t.Fatalf("expected final assistant message to include suggested next steps, got=%q", assistantContent)
	}
}

func TestStreamMessageAutoContinue_ChecklistRoundsCollapsedIntoSingleMessage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test checklist collapse")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &autoContinueScriptedProxyHandler{
		roundContents: []string{
			"- [ ] 收集官方资料\n- [ ] 对比收益数据\n- [ ] 输出报告",
			"- [ ] 收集官方资料\n- [ ] 对比收益数据\n- [ ] 输出报告\n\nWorking on task 1: 搜索 llama.cpp 关于 MTP 的 issue/PR。",
			"任务已完成。最终总结：已完成调研并给出对比建议。",
		},
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请进入agent模式并深度检索 llama.cpp 的 MTP 支持情况","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	if fakeProxy.callCount != 3 {
		t.Fatalf("expected 3 proxy calls (checklist + checklist update + final), got %d", fakeProxy.callCount)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 50, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	checklistMsgs := 0
	for _, m := range messages {
		if m.Role == "assistant" && strings.Contains(m.Content, "- [ ]") {
			checklistMsgs++
		}
	}
	if checklistMsgs > 1 {
		t.Fatalf("expected checklist rounds to collapse into at most one assistant message, got %d messages", checklistMsgs)
	}
}

func TestStreamMessageAutoContinue_2048Prompt_EnhancedAndNonEnhanced(t *testing.T) {
	const userPrompt = "帮我写一个2048的web版小游戏，需要能直接跑起来的，最后给我一个地址，localhost域名的就可以了。"
	const codexModel = "gpt-5.3-codex-spark"

	tests := []struct {
		name              string
		agentMode         bool
		firstRoundContent string
		wantAgentLoopHint bool
	}{
		{
			name:              "non-enhanced completes via action pledge",
			agentMode:         false,
			firstRoundContent: "我现在就去查并生成一个可运行版本，稍等我几秒。",
			wantAgentLoopHint: false,
		},
		{
			name:              "enhanced triggers agent loop continuation",
			agentMode:         true,
			firstRoundContent: "- [ ] 实现2048网页游戏\n- [ ] 本地运行并给出localhost地址",
			wantAgentLoopHint: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := memory.NewStore(":memory:")
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}
			defer store.Close()

			conv, err := store.CreateConversation(context.Background(), "Test 2048 auto continue")
			if err != nil {
				t.Fatalf("failed to create conversation: %v", err)
			}

			handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
			settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
			settingsHandler.settings.AgentMode = &tt.agentMode
			handler.SetSettingsHandler(settingsHandler)

			fakeProxy := &autoContinueScriptedProxyHandler{
				firstRoundContent:  tt.firstRoundContent,
				secondRoundContent: "已完成，游戏可直接运行，地址：http://localhost:3000",
			}
			handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

			e := echo.New()
			reqBody := fmt.Sprintf(`{"message":%q,"model":%q}`, userPrompt, codexModel)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("id")
			c.SetParamValues(conv.ID)

			if err := handler.StreamMessage(c); err != nil {
				t.Fatalf("StreamMessage error: %v", err)
			}

			body := rec.Body.String()
			if strings.Contains(body, `"error":"STREAM_ERROR"`) {
				t.Fatalf("expected no STREAM_ERROR, body=%s", body)
			}
			if !strings.Contains(body, `"done":true`) {
				t.Fatalf("expected final done chunk, body=%s", body)
			}
			if !strings.Contains(body, "http://localhost:3000") {
				t.Fatalf("expected localhost address in stream body, body=%s", body)
			}
			if fakeProxy.callCount != 2 {
				t.Fatalf("expected 2 proxy calls (initial + auto-continue), got %d", fakeProxy.callCount)
			}
			if len(fakeProxy.requestModels) != 2 {
				t.Fatalf("expected 2 captured request models, got %d", len(fakeProxy.requestModels))
			}
			for i, got := range fakeProxy.requestModels {
				if got != codexModel {
					t.Fatalf("request model[%d] = %q, want %q", i, got, codexModel)
				}
			}
			if !fakeProxy.sawExecutionNudge {
				t.Fatalf("expected second round to include execution nudge, last=%q", fakeProxy.lastRequestMessage)
			}
			if fakeProxy.sawAgentLoopNudge != tt.wantAgentLoopHint {
				t.Fatalf("agent loop hint mismatch: got=%v want=%v last=%q", fakeProxy.sawAgentLoopNudge, tt.wantAgentLoopHint, fakeProxy.lastRequestMessage)
			}

			if tt.agentMode && handler.getMaxToolRounds() <= maxToolRounds {
				t.Fatalf("expected agent mode to use extended tool rounds, got=%d default=%d", handler.getMaxToolRounds(), maxToolRounds)
			}
		})
	}
}

func TestStreamMessageAutoContinue_ActionPledge_ExecutesToolRoundThenSummary(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test action pledge auto continue tool round")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &actionPledgeThenToolCallProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"帮我查询一下openclaw的新闻","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event in stream body, body=%s", body)
	}
	if !strings.Contains(body, "已完成查询并整理：这是 OpenClaw 的最新动态摘要。") {
		t.Fatalf("expected final summary content in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 3 {
		t.Fatalf("expected 3 proxy calls (action pledge + tool round + summary), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected second request to include auto-continue execution nudge, last=%q", fakeProxy.lastRequestMessage)
	}
}

func TestStreamMessageAutoContinue_AgentMode_MissingTodoAfterToolRound(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test missing todo bootstrap")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentModeOn := true
	settingsHandler.settings.AgentMode = &agentModeOn
	handler.SetSettingsHandler(settingsHandler)

	fakeProxy := &missingTodoAfterToolRoundProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"继续完善这个实现直到完成","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 3 {
		t.Fatalf("expected 3 proxy calls (tool round + missing todo bootstrap + completion), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawMissingTodoNudge {
		t.Fatalf("expected third request to include missing_todo checklist bootstrap nudge, last=%q", fakeProxy.lastRequestMessage)
	}
	if fakeProxy.sawMissingNextStepsNudge {
		t.Fatalf("expected no missing_next_steps nudge after missing_todo bootstrap flow, last=%q", fakeProxy.lastRequestMessage)
	}
}

func TestStreamMessageAutoContinue_ActionPledge_DuplicateDebounceStopsLoop(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test action pledge duplicate debounce")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	duplicateActionPledge := "我先帮你快速查一下 OpenClaw 的最新相关新闻与动态。请稍等，我整理成要点给你。"
	fakeProxy := &autoContinueScriptedProxyHandler{
		roundContents: []string{
			duplicateActionPledge,
			duplicateActionPledge,
			duplicateActionPledge,
		},
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"帮我查询一下openclaw的新闻","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected duplicate action_pledge to stop at 2 proxy calls, got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected second request to include auto-continue execution nudge, last=%q", fakeProxy.lastRequestMessage)
	}
}

func TestStreamMessage_ToolCallRound_SuppressesPostToolCallDelta(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test tool-call round delta suppression")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &toolCallThenTextProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请执行并返回结果","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, "前置说明：开始执行。") {
		t.Fatalf("expected pre-tool-call text to remain visible, body=%s", body)
	}
	if strings.Contains(body, "LEAK_AFTER_TOOL_CALL") {
		t.Fatalf("expected post-tool-call delta to be suppressed from stream body, got=%s", body)
	}
	if !strings.Contains(body, "最终结果已返回。") {
		t.Fatalf("expected second-round final content in stream body, got=%s", body)
	}
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool_executing event in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected 2 proxy calls (tool round + final round), got %d", fakeProxy.callCount)
	}
}

func TestStreamMessage_SSEEvents_HaveStrictlyIncreasingSeq(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test SSE seq ordering")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &toolCallThenTextProxyHandler{}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请执行并返回结果","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	events := extractJSONSSEEvents(t, rec.Body.String())
	if len(events) == 0 {
		t.Fatalf("expected at least one JSON SSE event, body=%s", rec.Body.String())
	}

	lastSeq := int64(0)
	for i, event := range events {
		rawSeq, ok := event["seq"]
		if !ok {
			t.Fatalf("event[%d] missing seq: %+v", i, event)
		}
		seqFloat, ok := rawSeq.(float64)
		if !ok {
			t.Fatalf("event[%d] seq is not numeric: %#v", i, rawSeq)
		}
		seq := int64(seqFloat)
		if seq <= lastSeq {
			t.Fatalf("event[%d] seq not strictly increasing: current=%d last=%d event=%+v", i, seq, lastSeq, event)
		}
		lastSeq = seq

		rawStreamID, ok := event["stream_id"]
		if !ok {
			t.Fatalf("event[%d] missing stream_id: %+v", i, event)
		}
		if strings.TrimSpace(fmt.Sprintf("%v", rawStreamID)) == "" {
			t.Fatalf("event[%d] empty stream_id: %+v", i, event)
		}
	}
}

func TestStreamMessageAutoContinue_PseudoToolCall_DiscardsMalformedRound(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test pseudo tool call auto continue")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent:  "{\"cmd\":\"ls -la\"}I’ll inspect now. {\"tool\":\"exec\",\"cmd\":\"ls -la\"}\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>",
		secondRoundContent: "已切换为真实执行路径并完成修复。",
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请修复这个问题","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if strings.Contains(body, "{\"cmd\":\"ls -la\"}") ||
		strings.Contains(body, "```tool") ||
		strings.Contains(strings.ToLower(body), "<exec>") ||
		strings.Contains(body, "to=functions.exec") {
		t.Fatalf("expected pseudo tool-call leakage to be suppressed from streamed body, got=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, "已切换为真实执行路径并完成修复。") {
		t.Fatalf("expected second-round content in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected 2 proxy calls (initial + auto-continue), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected generic execution nudge in second request, last=%q", fakeProxy.lastRequestMessage)
	}
	if fakeProxy.sawPseudoToolNudge {
		t.Fatalf("expected pseudo-specific fake-tool wording removed from continuation prompt, last=%q", fakeProxy.lastRequestMessage)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "{\"cmd\":\"ls -la\"}") ||
			strings.Contains(m.Content, "```tool") ||
			strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from persisted assistant messages, got=%q", m.Content)
		}
	}
}

func TestStreamMessageAutoContinue_PseudoToolCall_EmptySecondRound_NoLeakage(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test pseudo tool call empty second round")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent:  "{\"cmd\":\"ls -la\"}I’ll inspect now. {\"tool\":\"exec\",\"cmd\":\"ls -la\"}\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>",
		secondRoundContent: "",
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"请继续并执行","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if strings.Contains(body, "{\"cmd\":\"ls -la\"}") ||
		strings.Contains(body, "```tool") ||
		strings.Contains(strings.ToLower(body), "<exec>") ||
		strings.Contains(body, "to=functions.exec") {
		t.Fatalf("expected pseudo tool-call leakage to be suppressed from streamed body, got=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected 2 proxy calls (initial + auto-continue), got %d", fakeProxy.callCount)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "{\"cmd\":\"ls -la\"}") ||
			strings.Contains(m.Content, "```tool") ||
			strings.Contains(strings.ToLower(m.Content), "<exec>") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from persisted assistant messages, got=%q", m.Content)
		}
	}
}

func TestStreamMessageAutoContinue_PseudoToolCall_CommandWorkdirJSON(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test pseudo command/workdir tool call auto continue")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	pseudo := "收到，开始设置提醒。\n- [ ] 创建“10秒后喝水”提醒\nWorking on task: add reminder for 10 seconds later.{\"command\":\"blue reminder.add message=\\\"喝水\\\" time=10s\",\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}{\"command\":\"blue help reminder\",...}"
	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent:  pseudo,
		secondRoundContent: "已切换为真实工具调用并完成提醒创建。",
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"提醒我10秒后喝水","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if strings.Contains(body, "{\"command\":\"blue reminder.add") ||
		strings.Contains(body, "Working on task:") ||
		strings.Contains(body, "\"workdir\"") {
		t.Fatalf("expected command/workdir pseudo tool-call leakage to be suppressed from streamed body, got=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, "已切换为真实工具调用并完成提醒创建。") {
		t.Fatalf("expected second-round content in stream body, got=%s", body)
	}
	if fakeProxy.callCount != 2 {
		t.Fatalf("expected 2 proxy calls (initial + auto-continue), got %d", fakeProxy.callCount)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected generic execution nudge in second request, last=%q", fakeProxy.lastRequestMessage)
	}
	if fakeProxy.sawPseudoToolNudge {
		t.Fatalf("expected pseudo-specific fake-tool wording removed from continuation prompt, last=%q", fakeProxy.lastRequestMessage)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	for _, m := range messages {
		if m.Role != "assistant" {
			continue
		}
		if strings.Contains(m.Content, "{\"command\":\"blue reminder.add") ||
			strings.Contains(m.Content, "Working on task:") {
			t.Fatalf("expected malformed pseudo tool-call text to be discarded from persisted assistant messages, got=%q", m.Content)
		}
	}
}

func TestStreamMessage_SanitizesLeakedCommandWorkdirPrefixOnFinalPersist(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test sanitize leaked command/workdir prefix on final persist")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	leaked := "{\"command\":\"blue web_search query=\\\"OpenClaw GitHub release\\\"\"\"workdir\":\"/Users/orca/.zimaos-blue/data/workspace\"}我先帮你搜到一批 OpenClaw 相关最新结果（当前检索到 5 条）：\n- SecurityWeek"
	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent: leaked,
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"帮我查询一下openclaw的新闻","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	var assistantContent string
	for _, m := range messages {
		if m.Role == "assistant" {
			assistantContent = m.Content
		}
	}
	if assistantContent == "" {
		t.Fatal("expected persisted assistant content")
	}
	if strings.Contains(assistantContent, `"command":"blue web_search`) {
		t.Fatalf("expected leaked command json removed from persisted content, got=%q", assistantContent)
	}
	if strings.Contains(assistantContent, `"workdir":`) {
		t.Fatalf("expected leaked workdir removed from persisted content, got=%q", assistantContent)
	}
	if !strings.Contains(assistantContent, "我先帮你搜到一批 OpenClaw 相关最新结果") {
		t.Fatalf("expected user-facing answer retained in persisted content, got=%q", assistantContent)
	}
}

func TestStreamMessageAutoContinue_PseudoToolCall_AgentModeCapsRetries(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test pseudo tool call retry cap")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	settingsHandler := NewSettingsHandler(kvstore.NewMemoryStore())
	agentMode := true
	settingsHandler.settings.AgentMode = &agentMode
	handler.SetSettingsHandler(settingsHandler)

	pseudo := "{\"cmd\":\"ls -la\"}I’ll inspect now. {\"tool\":\"exec\",\"cmd\":\"ls -la\"}\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>"
	fakeProxy := &autoContinueScriptedProxyHandler{
		firstRoundContent:  pseudo,
		secondRoundContent: pseudo,
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"继续修复","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !fakeProxy.sawExecutionNudge {
		t.Fatalf("expected generic execution nudge to be injected before hitting retry cap")
	}
	if fakeProxy.sawPseudoToolNudge {
		t.Fatalf("expected pseudo-specific fake-tool wording removed from continuation prompt")
	}

	wantCalls := maxPseudoToolCallAutoContinueAgent + 1
	if fakeProxy.callCount != wantCalls {
		t.Fatalf("expected call count to stop at pseudo retry cap, got=%d want=%d", fakeProxy.callCount, wantCalls)
	}
}

func TestStreamMessageAutoContinue_PseudoToolCall_SwitchesModelAfterTwoRetries(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test pseudo tool call model switch")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetSettingsHandler(NewSettingsHandler(kvstore.NewMemoryStore()))

	pseudo := "{\"cmd\":\"ls -la\"}I’ll inspect now. {\"tool\":\"exec\",\"cmd\":\"ls -la\"}\n```tool\n{\"name\":\"exec\",\"arguments\":{\"cmd\":\"ls -la\"}}\n```\n<exec>{\"cmd\":\"ls -la\"}</exec>"
	fakeProxy := &autoContinueScriptedProxyHandler{
		roundContents: []string{
			pseudo,
			pseudo,
			"已切换模型并完成真实执行路径。",
		},
	}
	handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

	e := echo.New()
	reqBody := `{"message":"继续修复","model":"gpt-5.3-codex-spark"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if strings.Contains(body, `"error":"STREAM_ERROR"`) {
		t.Fatalf("expected no STREAM_ERROR, body=%s", body)
	}
	if !strings.Contains(body, `"done":true`) {
		t.Fatalf("expected final done chunk, body=%s", body)
	}
	if !strings.Contains(body, "已切换模型并完成真实执行路径。") {
		t.Fatalf("expected final content in stream body, body=%s", body)
	}
	if fakeProxy.callCount != 3 {
		t.Fatalf("expected 3 proxy calls (pseudo + pseudo + switched model), got %d", fakeProxy.callCount)
	}
	if len(fakeProxy.requestModels) != 3 {
		t.Fatalf("expected 3 captured request models, got %d", len(fakeProxy.requestModels))
	}
	if got := fakeProxy.requestModels[0]; got != "gpt-5.3-codex-spark" {
		t.Fatalf("request model[0] = %q, want gpt-5.3-codex-spark", got)
	}
	if got := fakeProxy.requestModels[1]; got != "gpt-5.3-codex-spark" {
		t.Fatalf("request model[1] = %q, want gpt-5.3-codex-spark", got)
	}
	if got := fakeProxy.requestModels[2]; got != "gpt-5.3-codex" {
		t.Fatalf("request model[2] = %q, want gpt-5.3-codex", got)
	}
}

func TestStreamMessageShortAffirmative_InjectsContinuationHint(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Short Affirmative Continuation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "user",
		Content: "你可以帮我查一下 ZimaOS 的信息吗",
	})
	_, _ = store.AddMessage(context.Background(), conv.ID, memory.Message{
		Role:    "assistant",
		Content: "- [ ] 产品定位与功能概览\n- [ ] 最新动态\n\n如果你不想选，我可以默认按「功能概览 + 最新动态」先查一版。",
	})

	registry := llm.NewProviderRegistry()
	capture := &requestCaptureProvider{}
	registry.Register(capture)

	handler := NewChatHandler(store, registry, tools.NewRegistry())

	e := echo.New()
	reqBody := `{"message":"好的","model":"capture-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	if !strings.Contains(rec.Body.String(), `"done":true`) {
		t.Fatalf("expected done marker in stream body, got: %s", rec.Body.String())
	}

	lastReq := capture.LastRequest()
	if len(lastReq.Messages) == 0 {
		t.Fatal("expected captured request messages")
	}

	foundContinuationHint := false
	for _, m := range lastReq.Messages {
		if m.Role != llm.RoleSystem {
			continue
		}
		if strings.Contains(m.Content, "Continuation hint: the user just sent a brief affirmative acknowledgment") {
			foundContinuationHint = true
			break
		}
	}
	if !foundContinuationHint {
		t.Fatalf("expected continuation hint in system messages, got: %+v", lastReq.Messages)
	}
}

func TestStreamMessageSecondSendTimeout_NotWarmupRelated(t *testing.T) {
	tests := []struct {
		name       string
		withWarmup bool
	}{
		{name: "without warmup", withWarmup: false},
		{name: "with warmup", withWarmup: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, err := memory.NewStore(":memory:")
			if err != nil {
				t.Fatalf("failed to create store: %v", err)
			}
			defer store.Close()

			conv, err := store.CreateConversation(context.Background(), "Test second send timeout")
			if err != nil {
				t.Fatalf("failed to create conversation: %v", err)
			}

			registry := llm.NewProviderRegistry()
			toolRegistry := tools.NewRegistry()
			handler := NewChatHandler(store, registry, toolRegistry)
			fakeProxy := &secondTurnTimeoutProxyHandler{}
			handler.SetProxyBridge(proxybridge.NewBridge(fakeProxy))

			if warmupCachedForConversation(handler, conv.ID) {
				t.Fatal("warmup cache should be empty before first turn")
			}

			firstBody := runStreamTurn(t, handler, conv.ID, `{"message":"first turn","model":"gpt-4o-mini"}`)
			if strings.Contains(firstBody, `"error":"STREAM_ERROR"`) {
				t.Fatalf("first turn should succeed, body=%s", firstBody)
			}
			if !strings.Contains(firstBody, `"done":true`) {
				t.Fatalf("first turn should contain done marker, body=%s", firstBody)
			}

			if tc.withWarmup {
				handler.DoChannelWarmup(conv.ID)
				if !warmupCachedForConversation(handler, conv.ID) {
					t.Fatal("expected warmup cache before second turn")
				}
			} else if warmupCachedForConversation(handler, conv.ID) {
				t.Fatal("warmup cache should stay empty when warmup is not triggered")
			}

			secondBody := runStreamTurn(t, handler, conv.ID, `{"message":"second turn","model":"gpt-4o-mini"}`)
			if strings.Contains(secondBody, `"error":"STREAM_ERROR"`) {
				t.Fatalf("expected second turn to avoid STREAM_ERROR, body=%s", secondBody)
			}
			if !strings.Contains(secondBody, `"done":true`) {
				t.Fatalf("second turn should contain done marker, body=%s", secondBody)
			}

			counts := fakeProxy.RequestMsgCounts()
			if len(counts) < 2 {
				t.Fatalf("expected at least 2 proxy calls, got %d", len(counts))
			}
			if counts[1] <= 0 || counts[0] <= 0 {
				t.Fatalf("expected positive message counts, got %v", counts[:2])
			}
		})
	}
}

func TestStreamMessageCodexResponsesSecondTurn_UsesPreviousResponseID(t *testing.T) {
	var mu sync.Mutex
	var requestPaths []string
	var requestPrevIDs []string
	callCount := 0

	upstream := newTCP4TestServerOrSkip(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		prevID := strings.TrimSpace(gjson.GetBytes(body, "previous_response_id").String())
		inputText := strings.TrimSpace(gjson.GetBytes(body, "input.0.content.0.text").String())

		if r.URL.Path != "/v1/responses" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}

		// Ignore non-chat probe traffic (auth/tool probes).
		if inputText != "first turn" && inputText != "second turn" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"resp_probe","object":"response","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"ok"}]}]}`))
			return
		}

		mu.Lock()
		callCount++
		requestPaths = append(requestPaths, r.URL.Path)
		requestPrevIDs = append(requestPrevIDs, prevID)
		mu.Unlock()

		if inputText == "first turn" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_turn_1","choices":[{"delta":{"content":"first ok"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			return
		}

		// Simulate codex relay behavior: second turn must continue from previous_response_id.
		if prevID == "" {
			http.Error(w, "Request timed out. The server may be busy — please try again later.", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "data: %s\n\n", `{"id":"resp_turn_2","choices":[{"delta":{"content":"second ok"},"finish_reason":"stop"}],"model":"gpt-5.3-codex-spark"}`)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer upstream.Close()

	tmpDir, err := os.MkdirTemp("", "chat-codex-responses-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
		ID:        "codex-upstream",
		Name:      "codex-upstream",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   upstream.URL + "/v1",
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys: []providerpool.APIKey{
			{ID: "k-codex", Key: "sk-test", Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}
	models := []*providerpool.Model{
		{
			ID:           "gpt-5.3-codex-spark",
			Name:         "gpt-5.3-codex-spark",
			ProviderID:   provider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	if err := storage.SaveModels(provider.ID, models); err != nil {
		t.Fatalf("failed to save models: %v", err)
	}
	router.RebuildCandidates()

	proxyHandler := proxy.NewProxyHandler(nil, proxy.NewConnectionPool(proxy.DefaultConnectionConfig()), nil)
	proxyHandler.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test codex responses continuation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))

	if warmupCachedForConversation(handler, conv.ID) {
		t.Fatal("warmup cache should be empty before test")
	}

	firstBody := runStreamTurn(t, handler, conv.ID, `{"message":"first turn","model":"gpt-5.3-codex-spark"}`)
	if strings.Contains(firstBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("first turn should succeed, body=%s", firstBody)
	}

	secondBody := runStreamTurn(t, handler, conv.ID, `{"message":"second turn","model":"gpt-5.3-codex-spark"}`)
	if strings.Contains(secondBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("second turn should succeed with responses continuation, body=%s", secondBody)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(requestPaths) != 2 {
		t.Fatalf("expected exactly 2 upstream calls, got %d (paths=%v prev_ids=%v)", len(requestPaths), requestPaths, requestPrevIDs)
	}
	if requestPaths[0] != "/v1/responses" || requestPaths[1] != "/v1/responses" {
		t.Fatalf("expected codex requests to route to /v1/responses, got %v", requestPaths)
	}
	if requestPrevIDs[0] != "" {
		t.Fatalf("first turn previous_response_id = %q, want empty", requestPrevIDs[0])
	}
	if requestPrevIDs[1] == "" {
		t.Fatalf("second turn previous_response_id should be injected, got empty (prev_ids=%v)", requestPrevIDs)
	}
}

func TestStreamMessageCodexResponsesSecondTurn_RealProvider(t *testing.T) {
	if strings.TrimSpace(os.Getenv("ZIMA_RUN_REAL_CODEX")) != "1" {
		t.Skip("set ZIMA_RUN_REAL_CODEX=1 to run against real codex provider")
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

	tmpDir, err := os.MkdirTemp("", "chat-codex-real-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

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
		ID:        "real-codex",
		Name:      "real-codex",
		Type:      providerpool.ProviderTypeCustom,
		BaseURL:   baseURL,
		Enabled:   true,
		Status:    providerpool.ProviderStatusActive,
		Priority:  10,
		APIFormat: providerpool.APIFormatOpenAI,
		APIKeys: []providerpool.APIKey{
			{ID: "key-real-codex", Key: apiKey, Enabled: true},
		},
	}
	if err := registry.Register(provider); err != nil {
		t.Fatalf("failed to register provider: %v", err)
	}
	models := []*providerpool.Model{
		{
			ID:           modelID,
			Name:         modelID,
			ProviderID:   provider.ID,
			Enabled:      true,
			Capabilities: providerpool.ModelCapabilities{Chat: true, Streaming: true},
		},
	}
	if err := storage.SaveModels(provider.ID, models); err != nil {
		t.Fatalf("failed to save models: %v", err)
	}
	router.RebuildCandidates()

	proxyHandler := proxy.NewProxyHandler(nil, proxy.NewConnectionPool(proxy.DefaultConnectionConfig()), nil)
	proxyHandler.SetProviderPool(&providerpool.Pool{
		Registry:  registry,
		Discovery: discovery,
		Router:    router,
	})

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create memory store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test real codex responses continuation")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	handler := NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	handler.SetProxyBridge(proxybridge.NewBridge(proxyHandler))

	firstBody := runStreamTurn(t, handler, conv.ID, fmt.Sprintf(`{"message":"Reply with ONLY: OK","model":"%s"}`, modelID))
	if strings.Contains(firstBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("real provider first turn failed: %s", firstBody)
	}

	secondBody := runStreamTurn(t, handler, conv.ID, fmt.Sprintf(`{"message":"Reply with ONLY: NEXT","model":"%s"}`, modelID))
	if strings.Contains(secondBody, `"error":"STREAM_ERROR"`) {
		t.Fatalf("real provider second turn failed: %s", secondBody)
	}

	if !strings.Contains(secondBody, `"done":true`) {
		t.Fatalf("real provider second turn missing done marker: %s", secondBody)
	}
}

func TestStreamMessageSmoke_AskGateAwaitingAndSanitizedPersistence(t *testing.T) {
	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Test Ask Gate Smoke")
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	registry := llm.NewProviderRegistry()
	mockProvider := NewStreamingMockProvider("Before <ask_gate>请选择执行策略 A. 快速 B. 平衡</ask_gate> <awaiting_user_input>true</awaiting_user_input> After")
	registry.Register(mockProvider)

	toolRegistry := tools.NewRegistry()
	handler := NewChatHandler(store, registry, toolRegistry)

	e := echo.New()
	reqBody := `{"message":"请继续","provider":"streaming-mock","model":"streaming-mock-model"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations/"+conv.ID+"/messages/stream", bytes.NewBufferString(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(conv.ID)

	if err := handler.StreamMessage(c); err != nil {
		t.Fatalf("StreamMessage error: %v", err)
	}

	body := rec.Body.String()
	if count := strings.Count(body, `"awaiting_user_input":true`); count != 1 {
		t.Fatalf("expected exactly one awaiting_user_input event, got %d; body=%s", count, body)
	}

	messages, err := store.GetMessages(context.Background(), conv.ID, 20, 0)
	if err != nil {
		t.Fatalf("failed to load persisted messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected persisted messages, got empty")
	}

	var assistantContent string
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			assistantContent = messages[i].Content
			break
		}
	}
	if assistantContent == "" {
		t.Fatalf("assistant message not found in persisted messages: %+v", messages)
	}
	if strings.Contains(assistantContent, "<ask_gate>") || strings.Contains(assistantContent, "<awaiting_user_input>") {
		t.Fatalf("assistant content still contains internal marker(s): %q", assistantContent)
	}
	if !strings.Contains(assistantContent, "Before") || !strings.Contains(assistantContent, "After") {
		t.Fatalf("assistant content was over-sanitized, got: %q", assistantContent)
	}
}
