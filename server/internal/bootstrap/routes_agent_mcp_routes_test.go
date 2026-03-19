package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type fakeAgentLLMCaller struct {
	calls atomic.Int64
}

func (f *fakeAgentLLMCaller) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.calls.Add(1)
	content := "step done"
	if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "deterministic task planner") {
		content = `{"goal":"test","subtasks":[{"description":"noop"}],"success_criteria":["done"]}`
	}
	if len(req.Messages) > 0 && strings.Contains(req.Messages[0].Content, "Summarize the completed agent task") {
		content = "Task completed."
	}
	return &llm.ChatResponse{
		Model: req.Model,
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: content,
		},
	}, nil
}

func TestRegisterAgentAndMCPRoutes_ProxyDisabled(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")
	v1 := e.Group("/api/v1")

	services := &Services{
		ToolRegistry: tools.NewRegistry(),
	}
	cfg := &ServerConfig{
		DataDir: tmp,
	}
	deps := &RoutesDeps{
		DB:        db,
		Config:    &config.Config{},
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	llmCaller := &fakeAgentLLMCaller{}
	runner := registerAgentAndMCPRoutes(protected, v1, services, cfg, deps, zap.NewNop(), llmCaller, nil)
	if runner == nil {
		t.Fatal("expected non-nil agent runner when proxy is disabled")
	}
	defer runner.Shutdown()

	// Agent routes should be real routes, not feature-disabled stubs.
	agentListReq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/tasks", nil)
	agentListRec := httptest.NewRecorder()
	e.ServeHTTP(agentListRec, agentListReq)
	if agentListRec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/agent/tasks status=%d body=%s", agentListRec.Code, agentListRec.Body.String())
	}
	var tasks []map[string]interface{}
	if err := json.Unmarshal(agentListRec.Body.Bytes(), &tasks); err != nil {
		t.Fatalf("expected task list JSON array, got body=%q err=%v", agentListRec.Body.String(), err)
	}

	agentCreateReq := httptest.NewRequest(http.MethodPost, "/api/v1/agent/tasks", strings.NewReader(`{"goal":"verify route works"}`))
	agentCreateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	agentCreateRec := httptest.NewRecorder()
	e.ServeHTTP(agentCreateRec, agentCreateReq)
	if agentCreateRec.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/agent/tasks status=%d body=%s", agentCreateRec.Code, agentCreateRec.Body.String())
	}

	deadline := time.Now().Add(2 * time.Second)
	for llmCaller.calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if llmCaller.calls.Load() == 0 {
		t.Fatal("expected agent runner to invoke fallback LLM caller at least once")
	}

	// MCP route should remain available without proxy.
	mcpReqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	mcpReq := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/message?session_id=test-session", strings.NewReader(mcpReqBody))
	mcpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	mcpRec := httptest.NewRecorder()
	e.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/mcp/message status=%d body=%s", mcpRec.Code, mcpRec.Body.String())
	}

	var mcpResp map[string]interface{}
	if err := json.Unmarshal(mcpRec.Body.Bytes(), &mcpResp); err != nil {
		t.Fatalf("invalid mcp response json: %v", err)
	}
	result, ok := mcpResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcp result object, got: %T", mcpResp["result"])
	}
	if _, ok := result["protocolVersion"].(string); !ok {
		t.Fatalf("expected protocolVersion in mcp initialize result, got: %v", result)
	}
}

func TestRegisterAgentAndMCPRoutes_ProxyDisabled_MCPOrchestratorGenerativeUsesFallbackLLM(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")
	v1 := e.Group("/api/v1")

	services := &Services{
		ToolRegistry: tools.NewRegistry(),
	}
	cfg := &ServerConfig{
		DataDir: tmp,
	}
	deps := &RoutesDeps{
		DB:        db,
		Config:    &config.Config{},
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	llmCaller := &fakeAgentLLMCaller{}
	runner := registerAgentAndMCPRoutes(protected, v1, services, cfg, deps, zap.NewNop(), llmCaller, nil)
	if runner == nil {
		t.Fatal("expected non-nil agent runner when proxy is disabled")
	}
	defer runner.Shutdown()

	var body strings.Builder
	body.WriteString(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"orchestrator.run","arguments":{"goal":"verify fallback llm caller","generative_tasks":[{"id":"draft","prompt":"Generate concise coding note"}],"max_result_chars":1200}}}`)
	mcpReq := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/message?session_id=test-session", strings.NewReader(body.String()))
	mcpReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	mcpRec := httptest.NewRecorder()
	e.ServeHTTP(mcpRec, mcpReq)
	if mcpRec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/mcp/message status=%d body=%s", mcpRec.Code, mcpRec.Body.String())
	}

	var mcpResp map[string]interface{}
	if err := json.Unmarshal(mcpRec.Body.Bytes(), &mcpResp); err != nil {
		t.Fatalf("invalid mcp response json: %v", err)
	}

	result, ok := mcpResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcp result object, got: %T", mcpResp["result"])
	}
	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		t.Fatalf("expected non-empty result.content, got: %#v", result["content"])
	}
	first, ok := content[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected first content block object, got: %T", content[0])
	}
	text, _ := first["text"].(string)
	if strings.TrimSpace(text) == "" {
		t.Fatalf("expected text payload in first content block, got: %#v", first)
	}

	var orchestratorPayload map[string]interface{}
	if err := json.Unmarshal([]byte(text), &orchestratorPayload); err != nil {
		t.Fatalf("expected orchestrator payload JSON in content text, got=%q err=%v", text, err)
	}
	compressed, _ := orchestratorPayload["compressed_result"].(string)
	if !strings.Contains(compressed, "Generative Insights") {
		t.Fatalf("expected compressed result to include generative section, got: %q", compressed)
	}
	if llmCaller.calls.Load() == 0 {
		t.Fatal("expected fallback LLM caller to be invoked by MCP orchestrator generative task")
	}
}
