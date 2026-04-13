package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/labstack/echo/v4"
)

type blockingCancelAwareE2ETool struct {
	started     chan struct{}
	release     chan struct{}
	startOnce   sync.Once
	releaseOnce sync.Once
}

func (t *blockingCancelAwareE2ETool) Definition() tools.ToolDefinition {
	return tools.ToolDefinition{
		Name:        "slow_tool",
		Description: "blocks until cancelled",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"action": map[string]interface{}{"type": "string"},
			},
			"additionalProperties": true,
		},
	}
}

func (t *blockingCancelAwareE2ETool) Execute(ctx context.Context, _ map[string]interface{}) (interface{}, error) {
	t.startOnce.Do(func() {
		if t.started != nil {
			close(t.started)
		}
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.release:
		return map[string]interface{}{"released": true}, nil
	}
}

func (t *blockingCancelAwareE2ETool) Release() {
	t.releaseOnce.Do(func() {
		if t.release != nil {
			close(t.release)
		}
	})
}

type cancelToolCallProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *cancelToolCallProvider) Name() string { return "stop-flow-e2e" }

func (p *cancelToolCallProvider) Models() []string { return []string{"gpt-5.3-codex-spark"} }

func (p *cancelToolCallProvider) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	callNum := p.calls

	if callNum == 1 {
		return &llm.ChatResponse{
			ID:    "stop-flow-round-1",
			Model: req.Model,
			Message: llm.Message{
				Role:    llm.RoleAssistant,
				Content: "我先执行这个工具。",
				ToolCalls: []llm.ToolCall{{
					ID:        "call_slow_stop_e2e_1",
					Name:      "slow_tool",
					Arguments: `{"action":"wait"}`,
				}},
			},
		}, nil
	}

	return &llm.ChatResponse{
		ID:    "stop-flow-round-2",
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: "工具已经执行完成。",
		},
	}, nil
}

func (p *cancelToolCallProvider) ChatStream(ctx context.Context, req llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 2)
	go func() {
		defer close(ch)
		_ = p.ChatStreamCallback(ctx, req, func(chunk llm.StreamChunk) error {
			ch <- chunk
			return nil
		})
	}()
	return ch, nil
}

func (p *cancelToolCallProvider) ChatStreamCallback(ctx context.Context, req llm.ChatRequest, cb llm.StreamCallback) error {
	resp, err := p.Chat(ctx, req)
	if err != nil {
		return err
	}

	if len(resp.Message.ToolCalls) > 0 {
		if err := cb(llm.StreamChunk{
			ID:        resp.ID,
			Model:     resp.Model,
			ToolCalls: resp.Message.ToolCalls,
		}); err != nil {
			return err
		}
		return cb(llm.StreamChunk{
			ID:    resp.ID,
			Model: resp.Model,
			Done:  true,
		})
	}

	if err := cb(llm.StreamChunk{
		ID:    resp.ID,
		Model: resp.Model,
		Delta: resp.Message.Content,
	}); err != nil {
		return err
	}
	return cb(llm.StreamChunk{
		ID:    resp.ID,
		Model: resp.Model,
		Done:  true,
	})
}

func (p *cancelToolCallProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func doRequestAsUser(e *echo.Echo, method, path, userID, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method != http.MethodGet {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if strings.TrimSpace(userID) != "" {
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
			UserID: userID,
			Role:   "user",
		}))
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func doRequestAsUserAsync(e *echo.Echo, method, path, userID, body string) (*httptest.ResponseRecorder, <-chan error) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if method != http.MethodGet {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if strings.TrimSpace(userID) != "" {
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
			UserID: userID,
			Role:   "user",
		}))
	}

	done := make(chan error, 1)
	go func() {
		e.ServeHTTP(rec, req)
		done <- nil
	}()
	return rec, done
}

func waitForSingleActiveStreamID(t *testing.T, e *echo.Echo) string {
	t.Helper()

	type activeStreamsResponse struct {
		ActiveStreams []string `json:"active_streams"`
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec := doRequestAsUser(e, http.MethodGet, "/api/v1/streams/active", "", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("list active streams status = %d body=%s", rec.Code, rec.Body.String())
		}

		var payload activeStreamsResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode active streams response: %v body=%s", err, rec.Body.String())
		}
		if len(payload.ActiveStreams) > 0 {
			return strings.TrimSpace(payload.ActiveStreams[0])
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatal("timed out waiting for active stream id")
	return ""
}

func TestStreamMessageE2E_CancelEndpointStopsInFlightToolExecution(t *testing.T) {
	const userID = "cancel-e2e-user"

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	conv, err := store.CreateConversation(context.Background(), "Tool cancel e2e", userID)
	if err != nil {
		t.Fatalf("failed to create conversation: %v", err)
	}

	providers := llm.NewProviderRegistry()
	provider := &cancelToolCallProvider{}
	providers.Register(provider)

	toolRegistry := tools.NewRegistry()
	slowTool := &blockingCancelAwareE2ETool{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	defer slowTool.Release()
	toolRegistry.Register(slowTool)

	handler := serverpkg.NewChatHandler(store, providers, toolRegistry)
	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1"))

	streamRec, streamDone := doRequestAsUserAsync(
		e,
		http.MethodPost,
		"/api/v1/conversations/"+conv.ID+"/messages/stream",
		userID,
		`{"message":"请执行慢工具","provider":"stop-flow-e2e","model":"gpt-5.3-codex-spark"}`,
	)

	select {
	case <-slowTool.started:
	case <-time.After(750 * time.Millisecond):
		t.Fatal("tool did not start before cancellation")
	}

	streamID := waitForSingleActiveStreamID(t, e)
	if streamID == "" {
		t.Fatal("expected a non-empty active stream id")
	}

	cancelRec := doRequestAsUser(
		e,
		http.MethodPost,
		"/api/v1/conversations/"+conv.ID+"/messages/cancel",
		userID,
		fmt.Sprintf(`{"stream_id":%q}`, streamID),
	)
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d body=%s", cancelRec.Code, cancelRec.Body.String())
	}
	if !strings.Contains(cancelRec.Body.String(), `"success":true`) {
		t.Fatalf("expected cancel success payload, got: %s", cancelRec.Body.String())
	}
	if !strings.Contains(cancelRec.Body.String(), streamID) {
		t.Fatalf("expected cancel payload to include stream id %q, got: %s", streamID, cancelRec.Body.String())
	}

	select {
	case err := <-streamDone:
		if err != nil {
			t.Fatalf("stream route returned error: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stream did not stop promptly after cancellation")
	}

	body := streamRec.Body.String()
	if !strings.Contains(body, `"tool_executing":true`) {
		t.Fatalf("expected tool execution event before stop, got: %s", body)
	}
	if !strings.Contains(body, `"cancelled":true`) {
		t.Fatalf("expected cancelled event in stream body, got: %s", body)
	}
	if strings.Contains(body, "tool_execution_cancelled") {
		t.Fatalf("expected stream body to avoid leaked tool cancellation payloads, got: %s", body)
	}

	msgs, err := store.GetMessages(context.Background(), conv.ID, 1000, 0)
	if err != nil {
		t.Fatalf("failed to fetch messages: %v", err)
	}

	var assistant *memory.Message
	for i := range msgs {
		if msgs[i].Role == "assistant" {
			assistant = &msgs[i]
		}
	}
	if assistant == nil {
		t.Fatal("expected assistant message to be persisted")
	}
	if !strings.Contains(assistant.Content, "[Response stopped]") {
		t.Fatalf("expected stopped marker in assistant message, got %q", assistant.Content)
	}
	if strings.Contains(assistant.Content, "[Response interrupted]") {
		t.Fatalf("expected explicit stop marker instead of interrupted marker, got %q", assistant.Content)
	}
	if strings.Contains(assistant.Content, "tool_execution_cancelled") {
		t.Fatalf("expected persisted assistant message to avoid tool cancel payload leakage, got %q", assistant.Content)
	}

	if calls := provider.CallCount(); calls != 1 {
		t.Fatalf("provider call count = %d, want 1 to confirm follow-up round was skipped after cancel", calls)
	}
}
