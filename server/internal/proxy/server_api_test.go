package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func createTestProxyServer(t *testing.T) *ProxyServer {
	config := DefaultProxyConfig()
	config.Port.Value = 0 // Dynamic port
	config.Routing.Providers = []*ProviderConfig{
		{
			Name:        "anthropic",
			Endpoint:    "https://api.anthropic.com",
			Priority:    1,
			Enabled:     true,
			HealthCheck: "/v1/health",
		},
		{
			Name:        "openai",
			Endpoint:    "https://api.openai.com",
			Priority:    2,
			Enabled:     true,
			HealthCheck: "/v1/health",
		},
	}

	ps, err := NewProxyServer(config)
	if err != nil {
		t.Fatalf("Failed to create proxy server: %v", err)
	}
	return ps
}

func TestHandleStatus(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/status", nil)
	w := httptest.NewRecorder()

	ps.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "running" {
		t.Errorf("Expected status 'running', got '%v'", result["status"])
	}
	if result["version"] != "0.10.5.1" {
		t.Errorf("Expected version '0.10.5.1', got '%v'", result["version"])
	}
}

func TestHandleStatus_MethodNotAllowed(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy/status", nil)
	w := httptest.NewRecorder()

	ps.handleStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleHealth(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/health", nil)
	w := httptest.NewRecorder()

	ps.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", result["status"])
	}
}

func TestHandleProviders(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/providers", nil)
	w := httptest.NewRecorder()

	ps.handleProviders(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	providers, ok := result["providers"].([]interface{})
	if !ok {
		t.Fatal("Expected providers array")
	}
	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}
}

func TestHandleProviderByName(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/providers/anthropic", nil)
	w := httptest.NewRecorder()

	ps.handleProviderByName(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["name"] != "anthropic" {
		t.Errorf("Expected name 'anthropic', got '%v'", result["name"])
	}
	if result["endpoint"] != "https://api.anthropic.com" {
		t.Errorf("Expected endpoint 'https://api.anthropic.com', got '%v'", result["endpoint"])
	}
}

func TestHandleProviderByName_NotFound(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/providers/nonexistent", nil)
	w := httptest.NewRecorder()

	ps.handleProviderByName(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleProviderByName_MissingName(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/providers/", nil)
	w := httptest.NewRecorder()

	ps.handleProviderByName(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleModels(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/models", nil)
	w := httptest.NewRecorder()

	ps.handleModels(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := result["families"]; !ok {
		t.Error("Expected 'families' in response")
	}
}

func TestHandleModelRoute(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/models/route?model=claude-3-opus", nil)
	w := httptest.NewRecorder()

	ps.handleModelRoute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["original_model"] != "claude-3-opus" {
		t.Errorf("Expected original_model 'claude-3-opus', got '%v'", result["original_model"])
	}
}

func TestHandleModelRoute_MissingModel(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/models/route", nil)
	w := httptest.NewRecorder()

	ps.handleModelRoute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleModelRoute_Background(t *testing.T) {
	ps := createTestProxyServer(t)

	// Set up a TierResolver so background downgrade works
	tr := NewTierResolver()
	tr.Resolve([]*providerpool.Model{
		{ID: "claude-3-opus", ProviderID: "anthropic", Enabled: true, InputPrice: 15.0, OutputPrice: 75.0},
		{ID: "claude-3-haiku", ProviderID: "anthropic", Enabled: true, InputPrice: 0.25, OutputPrice: 1.25},
	})
	ps.modelRouter.SetTierResolver(tr)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/models/route?model=claude-3-opus&background=true", nil)
	w := httptest.NewRecorder()

	ps.handleModelRoute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should be downgraded for background task
	if result["downgraded"] != true {
		t.Errorf("Expected downgraded=true for background task, got '%v'", result["downgraded"])
	}
}

func TestHandleModelRules(t *testing.T) {
	ps := createTestProxyServer(t)

	body := `{"pattern": "^test-model$", "target": "target-model", "provider": "test", "priority": 1, "description": "Test rule"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy/models/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ps.handleModelRules(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["success"] != true {
		t.Errorf("Expected success=true, got '%v'", result["success"])
	}
}

func TestHandleModelRules_InvalidRegex(t *testing.T) {
	ps := createTestProxyServer(t)

	body := `{"pattern": "[invalid", "target": "target-model", "provider": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/proxy/models/rules", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	ps.handleModelRules(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleQuotas(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/quotas", nil)
	w := httptest.NewRecorder()

	ps.handleQuotas(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := result["total_providers"]; !ok {
		t.Error("Expected 'total_providers' in response")
	}
}

func TestHandleQuotas_ByProvider(t *testing.T) {
	ps := createTestProxyServer(t)

	// First record some usage
	ps.quotaMonitor.RecordRequest("anthropic")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/quotas?provider=anthropic", nil)
	w := httptest.NewRecorder()

	ps.handleQuotas(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["provider"] != "anthropic" {
		t.Errorf("Expected provider 'anthropic', got '%v'", result["provider"])
	}
}

func TestHandleQuotas_ProviderNotFound(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/quotas?provider=nonexistent", nil)
	w := httptest.NewRecorder()

	ps.handleQuotas(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleQuotasBest(t *testing.T) {
	ps := createTestProxyServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/proxy/quotas/best", nil)
	w := httptest.NewRecorder()

	ps.handleQuotasBest(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if _, ok := result["best_provider"]; !ok {
		t.Error("Expected 'best_provider' in response")
	}
}

func TestProxyServerStats(t *testing.T) {
	ps := createTestProxyServer(t)

	stats := ps.Stats()

	if stats["status"] != "running" {
		t.Errorf("Expected status 'running', got '%v'", stats["status"])
	}
	if _, ok := stats["router"]; !ok {
		t.Error("Expected 'router' in stats")
	}
	if _, ok := stats["connection_pool"]; !ok {
		t.Error("Expected 'connection_pool' in stats")
	}
	if _, ok := stats["model_router"]; !ok {
		t.Error("Expected 'model_router' in stats")
	}
	if _, ok := stats["quota_monitor"]; !ok {
		t.Error("Expected 'quota_monitor' in stats")
	}
}

func TestGetModelRouter(t *testing.T) {
	ps := createTestProxyServer(t)

	mr := ps.GetModelRouter()
	if mr == nil {
		t.Error("Expected non-nil model router")
	}
}

func TestGetQuotaMonitor(t *testing.T) {
	ps := createTestProxyServer(t)

	qm := ps.GetQuotaMonitor()
	if qm == nil {
		t.Error("Expected non-nil quota monitor")
	}
}
