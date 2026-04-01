package bootstrap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type stubAgentLLMCaller struct{}

func (stubAgentLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	return &llm.ChatResponse{}, nil
}

func TestRegisterRuntimeAgentRoutesUsesReadDBFromServices(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-routes.db")
	dbConn, err := dbutil.OpenSQLite(dbPath, nil)
	if err != nil {
		t.Fatalf("OpenSQLite: %v", err)
	}
	defer dbConn.Close()

	store, err := agent.NewStore(dbConn.Writer)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	task := &agent.Task{
		ID:     "task-reader",
		UserID: "user-1",
		Goal:   "verify route reads can use reader pool",
	}
	if err := store.Create(context.Background(), task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	e := echo.New()
	deps := &RoutesDeps{
		DB:        dbConn.Writer,
		Services:  &Services{DBConn: dbConn},
		SSEBroker: sse.NewBroker(),
	}
	runner := registerRuntimeAgentRoutes(
		e.Group("/api/v1"),
		routeToolRuntimeBinding{workspaceDir: t.TempDir()},
		deps,
		zap.NewNop(),
		stubAgentLLMCaller{},
		nil,
	)
	if runner == nil {
		t.Fatal("expected agent runner to be registered")
	}

	if err := dbConn.Writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	dbConn.Writer = nil

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent/tasks/"+task.ID, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentMCPBindingsGo_DelegatesProxyBridgeSurface(t *testing.T) {
	agentContent, err := os.ReadFile(filepath.Join("agent_mcp_bindings.go"))
	if err != nil {
		t.Fatalf("read agent_mcp_bindings.go: %v", err)
	}
	agentSource := string(agentContent)

	surfaceContent, err := os.ReadFile(filepath.Join("runtime_proxy_bridge_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_proxy_bridge_surface.go: %v", err)
	}
	surfaceSource := string(surfaceContent)
	mcpSurfaceContent, err := os.ReadFile(filepath.Join("runtime_mcp_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_mcp_surface.go: %v", err)
	}
	mcpSurfaceSource := string(mcpSurfaceContent)
	agentSurfaceContent, err := os.ReadFile(filepath.Join("runtime_agent_surface.go"))
	if err != nil {
		t.Fatalf("read runtime_agent_surface.go: %v", err)
	}
	agentSurfaceSource := string(agentSurfaceContent)

	requiredAgent := []string{
		"type routeToolRuntimeBinding struct {",
		"func newRouteToolRuntimeBinding(",
	}
	for _, token := range requiredAgent {
		if !strings.Contains(agentSource, token) {
			t.Fatalf("expected agent_mcp_bindings.go to keep token %q", token)
		}
	}

	forbiddenAgent := []string{
		"type runtimeProxyBridgeSurface struct {",
		"func newRuntimeProxyBridgeSurface(",
		"func (surface runtimeProxyBridgeSurface) bind(",
		"func (runtime routeToolRuntimeBinding) registerMCPSurface(",
		"func registerRuntimeMCPRoutes(",
		"func newRuntimeMCPServer(",
		"func newRuntimeMCPGenerativeRunner(",
		"func (runtime routeToolRuntimeBinding) activateAgentSurface(",
		"func registerRuntimeAgentRoutes(",
	}
	for _, token := range forbiddenAgent {
		if strings.Contains(agentSource, token) {
			t.Fatalf("expected agent_mcp_bindings.go to delegate token %q", token)
		}
	}

	requiredSurface := []string{
		"type runtimeProxyBridgeSurface struct {",
		"func newRuntimeProxyBridgeSurface(",
		"func (surface runtimeProxyBridgeSurface) bind(",
	}
	for _, token := range requiredSurface {
		if !strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_proxy_bridge_surface.go to contain token %q", token)
		}
	}

	forbiddenSurface := []string{
		"type routeToolRuntimeBinding struct {",
		"func registerRuntimeAgentRoutes(",
		"func registerRuntimeMCPRoutes(",
	}
	for _, token := range forbiddenSurface {
		if strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_proxy_bridge_surface.go to delegate token %q", token)
		}
	}

	requiredMCPSurface := []string{
		"func (runtime routeToolRuntimeBinding) registerMCPSurface(",
		"func registerRuntimeMCPRoutes(",
		"func newRuntimeMCPServer(",
		"func newRuntimeMCPGenerativeRunner(",
	}
	for _, token := range requiredMCPSurface {
		if !strings.Contains(mcpSurfaceSource, token) {
			t.Fatalf("expected runtime_mcp_surface.go to contain token %q", token)
		}
	}

	forbiddenMCPSurface := []string{
		"type routeToolRuntimeBinding struct {",
		"func newRouteToolRuntimeBinding(",
		"func registerRuntimeAgentRoutes(",
		"type runtimeProxyBridgeSurface struct {",
	}
	for _, token := range forbiddenMCPSurface {
		if strings.Contains(mcpSurfaceSource, token) {
			t.Fatalf("expected runtime_mcp_surface.go to delegate token %q", token)
		}
	}

	requiredAgentSurface := []string{
		"func (runtime routeToolRuntimeBinding) activateAgentSurface(",
		"func registerRuntimeAgentRoutes(",
	}
	for _, token := range requiredAgentSurface {
		if !strings.Contains(agentSurfaceSource, token) {
			t.Fatalf("expected runtime_agent_surface.go to contain token %q", token)
		}
	}

	forbiddenAgentSurface := []string{
		"type routeToolRuntimeBinding struct {",
		"func newRouteToolRuntimeBinding(",
		"func (runtime routeToolRuntimeBinding) registerMCPSurface(",
		"type runtimeProxyBridgeSurface struct {",
	}
	for _, token := range forbiddenAgentSurface {
		if strings.Contains(agentSurfaceSource, token) {
			t.Fatalf("expected runtime_agent_surface.go to delegate token %q", token)
		}
	}
}
