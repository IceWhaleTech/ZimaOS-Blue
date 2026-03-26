package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appconfig "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/labstack/echo/v4"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, func()) {
	config := DefaultConfig()
	config.DefaultTimeout = 5 * time.Second
	config.MaxTimeout = 30 * time.Second

	manager, err := NewManager(config)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	handler := NewHandler(manager)

	cleanup := func() {
		manager.Cleanup()
	}

	return handler, manager, cleanup
}

func TestNewHandler(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestHandler_Execute(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()

	reqBody := ExecuteRequest{
		Command:     "echo",
		Args:        []string{"hello"},
		TimeoutSecs: 5,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Execute(c)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Execute() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var result ExecutionResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("Execute() failed to decode response: %v", err)
	}

	if result.ID == "" {
		t.Error("Execute() result ID should not be empty")
	}
}

func TestHandler_Execute_MissingCommand(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()

	reqBody := ExecuteRequest{
		Command: "",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Execute(c)
	if err == nil {
		t.Error("Execute(missing command) expected error")
	}
}

func TestHandler_Execute_InvalidBody(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/execute", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Execute(c)
	if err == nil {
		t.Error("Execute(invalid body) expected error")
	}
}

func TestHandler_Execute_WithEnv(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()

	reqBody := ExecuteRequest{
		Command:     "printenv",
		Args:        []string{"TEST_VAR"},
		Env:         map[string]string{"TEST_VAR": "test_value"},
		TimeoutSecs: 5,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Execute(c)
	if err != nil {
		t.Fatalf("Execute(env) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Execute(env) status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Execute_WithMemoryLimit(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()

	reqBody := ExecuteRequest{
		Command:     "echo",
		Args:        []string{"hello"},
		MemoryMB:    128,
		TimeoutSecs: 5,
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Execute(c)
	if err != nil {
		t.Fatalf("Execute(memory) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Execute(memory) status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_GetStatus(t *testing.T) {
	handler, manager, cleanup := setupTestHandler(t)
	defer cleanup()

	// First execute a command
	req := NewExecutionRequest("echo", "hello")
	req.Timeout = 5 * time.Second
	result, _ := manager.Execute(context.Background(), req)

	e := echo.New()
	httpReq := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/status/"+result.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)
	c.SetParamNames("id")
	c.SetParamValues(result.ID)

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetStatus() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_GetStatus_MissingID(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/status/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("")

	err := handler.GetStatus(c)
	if err == nil {
		t.Error("GetStatus(missing id) expected error")
	}
}

func TestHandler_GetStatus_NotFound(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/status/non-existent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("non-existent")

	err := handler.GetStatus(c)
	if err == nil {
		t.Error("GetStatus(not found) expected error")
	}
}

func TestHandler_Kill(t *testing.T) {
	handler, manager, cleanup := setupTestHandler(t)
	defer cleanup()

	// Start a long-running command
	req := NewExecutionRequest("sleep", "60")
	req.Timeout = 60 * time.Second

	// Execute in goroutine
	go func() {
		manager.Execute(context.Background(), req)
	}()

	// Wait for process to start
	time.Sleep(100 * time.Millisecond)

	e := echo.New()
	httpReq := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/kill/"+req.ID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(httpReq, rec)
	c.SetParamNames("id")
	c.SetParamValues(req.ID)

	err := handler.Kill(c)
	if err != nil {
		t.Fatalf("Kill() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Kill() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Kill_MissingID(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/kill/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("")

	err := handler.Kill(c)
	if err == nil {
		t.Error("Kill(missing id) expected error")
	}
}

func TestHandler_Kill_NotFound(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sandbox/kill/non-existent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("non-existent")

	err := handler.Kill(c)
	if err == nil {
		t.Error("Kill(not found) expected error")
	}
}

func TestHandler_Info(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/info", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Info(c)
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Info() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("Info() failed to decode response: %v", err)
	}

	if _, ok := info["supported"]; !ok {
		t.Error("Info() response should contain 'supported' field")
	}

	if _, ok := info["default_timeout"]; !ok {
		t.Error("Info() response should contain 'default_timeout' field")
	}

	if _, ok := info["memory_limit"]; !ok {
		t.Error("Info() response should contain 'memory_limit' field")
	}
}

func TestHandler_Info_UnsupportedIncludesReason(t *testing.T) {
	handler := NewHandler(&Manager{
		config:   DefaultConfig(),
		executor: newUnsupportedExecutor("test sandbox reason"),
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sandbox/info", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Info(c)
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("Info() failed to decode response: %v", err)
	}

	if info["supported"] != false {
		t.Fatalf("Info() supported = %v, want false", info["supported"])
	}
	if info["support_reason"] != "test sandbox reason" {
		t.Fatalf("Info() support_reason = %v, want %q", info["support_reason"], "test sandbox reason")
	}
}

func TestHandler_UpdateConfig_NetworkEnabled(t *testing.T) {
	handler, manager, cleanup := setupTestHandler(t)
	defer cleanup()

	store := appconfig.NewConfigStore(kvstore.NewMemoryStore())
	cfg := &appconfig.Config{}
	cfg.Security.Sandbox.Enabled = true
	cfg.Security.Sandbox.NetworkEnabled = false
	if err := store.Import(cfg); err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	handler.SetConfigStore(store)

	hookCalled := false
	handler.SetNetworkConfigHook(func(networkEnabled bool) {
		hookCalled = true
		if !networkEnabled {
			t.Fatalf("networkEnabled = %v, want true", networkEnabled)
		}
	})

	body := []byte(`{"network_enabled":true}`)
	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sandbox/config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.UpdateConfig(c); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("UpdateConfig() status = %d, want %d", rec.Code, http.StatusOK)
	}

	if !manager.GetConfig().NetworkEnabled {
		t.Fatal("manager config should be updated")
	}
	if store.Config() == nil || !store.Config().Security.Sandbox.NetworkEnabled {
		t.Fatal("config store security.sandbox.network_enabled should be true")
	}
	if !hookCalled {
		t.Fatal("network config hook should be called")
	}

	var info map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&info); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if info["network_enabled"] != true {
		t.Fatalf("response network_enabled = %v, want true", info["network_enabled"])
	}
}

func TestHandler_UpdateConfig_RequiresNetworkEnabled(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/sandbox/config", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.UpdateConfig(c)
	if err == nil {
		t.Fatal("UpdateConfig() expected error")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	g := e.Group("/api/v1/sandbox")

	handler.RegisterRoutes(g)

	// Verify routes are registered
	routes := e.Routes()
	expectedPaths := []string{
		"/api/v1/sandbox/execute",
		"/api/v1/sandbox/status/:id",
		"/api/v1/sandbox/kill/:id",
		"/api/v1/sandbox/config",
		"/api/v1/sandbox/info",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, path := range expectedPaths {
		if !routeMap[path] {
			t.Errorf("RegisterRoutes() missing route %s", path)
		}
	}
}

func TestSecondsToDuration(t *testing.T) {
	tests := []struct {
		secs     int
		expected time.Duration
	}{
		{0, 0},
		{1, time.Second},
		{60, time.Minute},
		{3600, time.Hour},
	}

	for _, tt := range tests {
		result := secondsToDuration(tt.secs)
		if result != tt.expected {
			t.Errorf("secondsToDuration(%d) = %v, want %v", tt.secs, result, tt.expected)
		}
	}
}

func TestExecuteRequest_Fields(t *testing.T) {
	req := ExecuteRequest{
		Command:     "echo",
		Args:        []string{"hello"},
		Env:         map[string]string{"KEY": "value"},
		WorkDir:     "/tmp",
		Stdin:       "input",
		TimeoutSecs: 30,
		MemoryMB:    256,
	}

	if req.Command != "echo" {
		t.Errorf("ExecuteRequest Command = %v, want echo", req.Command)
	}

	if len(req.Args) != 1 {
		t.Errorf("ExecuteRequest Args length = %d, want 1", len(req.Args))
	}

	if req.Env["KEY"] != "value" {
		t.Errorf("ExecuteRequest Env[KEY] = %v, want value", req.Env["KEY"])
	}

	if req.TimeoutSecs != 30 {
		t.Errorf("ExecuteRequest TimeoutSecs = %d, want 30", req.TimeoutSecs)
	}

	if req.MemoryMB != 256 {
		t.Errorf("ExecuteRequest MemoryMB = %d, want 256", req.MemoryMB)
	}
}
