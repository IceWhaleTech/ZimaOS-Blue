package bootstrap

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
	skillpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestRuntimeToolSurfacesGo_DelegatesSurfaceAssembly(t *testing.T) {
	surfaceContent, err := os.ReadFile(filepath.Join("runtime_tool_surfaces.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_surfaces.go: %v", err)
	}
	surfaceSource := string(surfaceContent)

	builderContent, err := os.ReadFile(filepath.Join("runtime_tool_surface_builder.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_surface_builder.go: %v", err)
	}
	builderSource := string(builderContent)

	typeContent, err := os.ReadFile(filepath.Join("runtime_tool_surface_types.go"))
	if err != nil {
		t.Fatalf("read runtime_tool_surface_types.go: %v", err)
	}
	typeSource := string(typeContent)

	if lines := strings.Count(surfaceSource, "\n") + 1; lines > 20 {
		t.Fatalf("expected runtime_tool_surfaces.go to stay below 20 lines after extraction, got %d", lines)
	}

	requiredSurface := []string{
		"func activateRuntimeToolSurfaces(",
		"newRouteToolRuntimeBinding(",
		"newRuntimeToolSurfacesResult(",
	}
	for _, token := range requiredSurface {
		if !strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_tool_surfaces.go to keep token %q", token)
		}
	}

	forbiddenSurface := []string{
		"type runtimeToolSurfacesOptions struct {",
		"type runtimeToolSurfacesResult struct {",
		"func newRuntimeToolSurfacesResult(",
	}
	for _, token := range forbiddenSurface {
		if strings.Contains(surfaceSource, token) {
			t.Fatalf("expected runtime_tool_surfaces.go to delegate token %q", token)
		}
	}

	requiredBuilder := []string{
		"func newRuntimeToolSurfacesResult(",
		"activateAgentSurface(",
		"registerMCPSurface(",
	}
	for _, token := range requiredBuilder {
		if !strings.Contains(builderSource, token) {
			t.Fatalf("expected runtime_tool_surface_builder.go to contain token %q", token)
		}
	}

	requiredTypes := []string{
		"type runtimeToolSurfacesOptions struct {",
		"type runtimeToolSurfacesResult struct {",
	}
	for _, token := range requiredTypes {
		if !strings.Contains(typeSource, token) {
			t.Fatalf("expected runtime_tool_surface_types.go to contain token %q", token)
		}
	}
}

func TestActivateRuntimeToolSurfaces_RegistersAgentAndMCPRoutes(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "tool-surfaces.db"))
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

	result := activateRuntimeToolSurfaces(runtimeToolSurfacesOptions{
		protected:      protected,
		services:       &Services{ToolRegistry: registry, SkillRegistry: skills},
		cfg:            &ServerConfig{DataDir: tmp},
		deps:           deps,
		logger:         zap.NewNop(),
		agentLLMCaller: &fakeAgentLLMCaller{},
		reflectService: selfreflect.NewService(nil, nil),
		skillRegistry:  skills,
	})
	if result.agentRunner == nil {
		t.Fatal("expected runtime tool surfaces to return agent runner")
	}
	defer result.agentRunner.Shutdown()
	if !result.mcpRegistered {
		t.Fatal("expected runtime tool surfaces to register MCP routes")
	}
	if !routeExists(e, http.MethodPost, "/api/v1/agent/tasks") {
		t.Fatalf("expected agent routes, got %#v", e.Routes())
	}
	if routeExists(e, http.MethodGet, "/api/v1/agent/tasks") {
		t.Fatalf("did not expect standalone agent task list route to remain registered, got %#v", e.Routes())
	}
	if !routeExists(e, http.MethodPost, "/api/v1/mcp/message") {
		t.Fatalf("expected mcp routes, got %#v", e.Routes())
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/message?session_id=test-session", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/v1/mcp/message status=%d body=%s", rec.Code, rec.Body.String())
	}

	resultPayload, execErr := reflectSkill.Execute(context.Background(), map[string]any{
		"goal":         "verify reflection binding",
		"final_status": "completed",
	})
	if execErr != nil {
		t.Fatalf("self_reflect skill execute error: %v", execErr)
	}
	if resultPayload == nil || !resultPayload.Success {
		t.Fatalf("expected self_reflect executor to be wired, got %#v", resultPayload)
	}
}
