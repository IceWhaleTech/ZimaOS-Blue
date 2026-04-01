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
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
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

func TestRegisterRuntimeAgentAndMCPRoutes_ProxyDisabled(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")

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
	routeRuntime := newRouteToolRuntimeBinding(services, cfg, deps)
	runner := registerRuntimeAgentRoutes(protected, routeRuntime, deps, zap.NewNop(), llmCaller, nil)
	if runner == nil {
		t.Fatal("expected non-nil agent runner when proxy is disabled")
	}
	defer runner.Shutdown()
	if !registerRuntimeMCPRoutes(protected, routeRuntime.registry, routeRuntime.executor, cfg, deps, zap.NewNop(), llmCaller) {
		t.Fatal("expected MCP routes to be registered")
	}

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

func TestRegisterRuntimeAgentRoutes_RegistersAgentRoutesWhenReady(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")

	registry := tools.NewRegistry()
	deps := &RoutesDeps{
		DB:        db,
		Config:    &config.Config{},
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	runner := registerRuntimeAgentRoutes(
		protected,
		routeToolRuntimeBinding{
			registry:     registry,
			executor:     tools.NewExecutor(registry),
			workspaceDir: tmp,
		},
		deps,
		zap.NewNop(),
		&fakeAgentLLMCaller{},
		nil,
	)
	if runner == nil {
		t.Fatal("expected runtime agent routes to return a runner")
	}
	defer runner.Shutdown()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/tasks", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/agent/tasks status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRegisterRuntimeAgentRoutes_RegistersDisabledRoutesWithoutPrerequisites(t *testing.T) {
	e := echo.New()
	protected := e.Group("/api/v1")
	registry := tools.NewRegistry()

	runner := registerRuntimeAgentRoutes(
		protected,
		routeToolRuntimeBinding{
			registry:     registry,
			executor:     tools.NewExecutor(registry),
			workspaceDir: t.TempDir(),
		},
		&RoutesDeps{Config: &config.Config{}},
		zap.NewNop(),
		nil,
		nil,
	)
	if runner != nil {
		t.Fatalf("expected nil runner without prerequisites, got %#v", runner)
	}
	if !routeExists(e, "GET", "/api/v1/agent/*") {
		t.Fatalf("expected disabled catch-all route to be registered, got %#v", e.Routes())
	}
}

func TestRegisterRuntimeAgentAndMCPRoutes_ProxyDisabled_MCPOrchestratorGenerativeUsesFallbackLLM(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")

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
	routeRuntime := newRouteToolRuntimeBinding(services, cfg, deps)
	runner := registerRuntimeAgentRoutes(protected, routeRuntime, deps, zap.NewNop(), llmCaller, nil)
	if runner == nil {
		t.Fatal("expected non-nil agent runner when proxy is disabled")
	}
	defer runner.Shutdown()
	if !registerRuntimeMCPRoutes(protected, routeRuntime.registry, routeRuntime.executor, cfg, deps, zap.NewNop(), llmCaller) {
		t.Fatal("expected MCP routes to be registered")
	}

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

func TestRegisterRuntimeMCPRoutes_RegistersInitializeRoute(t *testing.T) {
	e := echo.New()
	protected := e.Group("/api/v1")
	cfg := &ServerConfig{DataDir: t.TempDir()}
	deps := &RoutesDeps{
		Config: &config.Config{},
	}
	registry := tools.NewRegistry()
	registered := registerRuntimeMCPRoutes(
		protected,
		registry,
		tools.NewExecutor(registry),
		cfg,
		deps,
		zap.NewNop(),
		nil,
	)
	if !registered {
		t.Fatal("expected MCP routes to be registered")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/message?session_id=test-session", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/mcp/message status=%d body=%s", rec.Code, rec.Body.String())
	}

	var mcpResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &mcpResp); err != nil {
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

func TestNewRuntimeMCPGenerativeRunner_UsesFallbackLLM(t *testing.T) {
	llmCaller := &fakeAgentLLMCaller{}
	runner := newRuntimeMCPGenerativeRunner(llmCaller)
	if runner == nil {
		t.Fatal("expected generative runner when llm caller is present")
	}

	out, err := runner(context.Background(), "Generate concise coding note", 1200)
	if err != nil {
		t.Fatalf("generative runner returned error: %v", err)
	}
	if out != "step done" {
		t.Fatalf("runner output = %q, want %q", out, "step done")
	}
	if llmCaller.calls.Load() != 1 {
		t.Fatalf("expected llm caller to be invoked once, got %d", llmCaller.calls.Load())
	}
}

func TestNewRouteToolRuntimeBinding_PreservesRegistryAndWorkspaceDir(t *testing.T) {
	cfg := &ServerConfig{DataDir: t.TempDir()}
	deps := &RoutesDeps{Config: &config.Config{}}
	registry := tools.NewRegistry()

	binding := newRouteToolRuntimeBinding(&Services{ToolRegistry: registry}, cfg, deps)
	if binding.registry != registry {
		t.Fatalf("expected binding to preserve service registry, got %#v", binding.registry)
	}
	if binding.executor == nil {
		t.Fatal("expected binding executor to be initialized")
	}
	wantWorkspace := ResolveWorkspaceDir(cfg.DataDir, deps.Config)
	if binding.workspaceDir != wantWorkspace {
		t.Fatalf("workspaceDir = %q, want %q", binding.workspaceDir, wantWorkspace)
	}
}

func TestRouteToolRuntimeBinding_ActivateAgentSurface_WiresSelfReflectSkill(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "agent-surface.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	e := echo.New()
	protected := e.Group("/api/v1")
	registry := tools.NewRegistry()
	skills := skillpkg.NewRegistry()
	reflectSkill := builtin.NewSelfReflect()
	if err := skills.Register(reflectSkill, true); err != nil {
		t.Fatalf("register self_reflect skill: %v", err)
	}

	deps := &RoutesDeps{
		DB:        db,
		Config:    &config.Config{},
		SSEBroker: sse.NewBroker(),
	}
	defer deps.SSEBroker.Close()

	binding := routeToolRuntimeBinding{
		registry:     registry,
		executor:     tools.NewExecutor(registry),
		workspaceDir: tmp,
	}
	runner := binding.activateAgentSurface(
		protected,
		deps,
		zap.NewNop(),
		&fakeAgentLLMCaller{},
		nil,
		selfreflect.NewService(nil, nil),
		skills,
	)
	if runner == nil {
		t.Fatal("expected activated agent surface to return runner")
	}
	defer runner.Shutdown()
	if !routeExists(e, "GET", "/api/v1/agent/tasks") {
		t.Fatalf("expected agent routes to be registered, got %#v", e.Routes())
	}

	result, execErr := reflectSkill.Execute(context.Background(), map[string]any{
		"goal":         "verify reflection binding",
		"final_status": "completed",
	})
	if execErr != nil {
		t.Fatalf("self_reflect skill execute error: %v", execErr)
	}
	if result == nil || !result.Success {
		t.Fatalf("expected self_reflect executor to be wired, got %#v", result)
	}
}

func TestNewRuntimeProxyBridgeSurface_ResolvesRegistryAndDependencyTargets(t *testing.T) {
	registry := tools.NewRegistry()
	imageTool := tools.NewImageTool(nil, nil, nil)
	registry.Register(imageTool)

	skills := skillpkg.NewRegistry()
	uiSkill := builtin.NewUIReviewer()
	if err := skills.Register(uiSkill, true); err != nil {
		t.Fatalf("register ui_reviewer skill: %v", err)
	}

	uiTool := tools.NewUIReviewerTool()
	mediaMgr := &mediagen.Manager{}
	analyzeTool := &tools.AnalyzeTool{}
	pdfSvc := &pdfextract.Service{}

	surface := newRuntimeProxyBridgeSurface(&Services{
		ToolRegistry:  registry,
		SkillRegistry: skills,
		PDFService:    pdfSvc,
	}, &RoutesDeps{
		UIReviewerTool: uiTool,
		MediaManager:   mediaMgr,
		AnalyzeTool:    analyzeTool,
	})

	if surface.image != imageTool {
		t.Fatalf("expected image tool target, got %#v", surface.image)
	}
	if surface.uiSkill != uiSkill {
		t.Fatalf("expected ui reviewer skill target, got %#v", surface.uiSkill)
	}
	if surface.uiTool != uiTool || surface.media != mediaMgr || surface.pdf != pdfSvc || surface.analyze != analyzeTool {
		t.Fatalf("expected proxy bridge surface to preserve dependency targets, got %#v", surface)
	}
}
