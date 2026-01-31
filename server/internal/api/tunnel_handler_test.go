package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/ngrok"
	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/tunnel"
)

func setupTunnelHandler(t *testing.T) (*TunnelHandler, *echo.Echo) {
	tempDir := filepath.Join(os.TempDir(), "tunnel-handler-test")
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	dbPath := filepath.Join(tempDir, "tunnel.db")
	repo, err := ngrok.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	h := NewTunnelHandler(repo, 8080)

	e := echo.New()
	h.RegisterRoutes(e)

	return h, e
}

func TestTunnelHandler_GetProviders(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/providers", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	providers, ok := resp["providers"].([]interface{})
	if !ok {
		t.Fatal("Response should contain providers array")
	}

	// Should have 4 providers: auto, ngrok, cloudflare, localtunnel
	if len(providers) != 4 {
		t.Errorf("Expected 4 providers, got %d", len(providers))
	}

	// Verify provider IDs
	providerIDs := make(map[string]bool)
	for _, p := range providers {
		provider := p.(map[string]interface{})
		providerIDs[provider["id"].(string)] = true
	}

	expectedProviders := []string{"auto", "ngrok", "cloudflare", "localtunnel"}
	for _, expected := range expectedProviders {
		if !providerIDs[expected] {
			t.Errorf("Expected provider %s not found", expected)
		}
	}
}

func TestTunnelHandler_GetStatus_NoActiveTunnel(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/status", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	tunnelStatus, ok := resp["tunnel"].(map[string]interface{})
	if !ok {
		t.Fatal("Response should contain tunnel status")
	}

	if tunnelStatus["active"] != false {
		t.Error("Expected tunnel to be inactive")
	}
}

func TestTunnelHandler_StopTunnel_NoActiveTunnel(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tunnel/stop", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestTunnelHandler_StartTunnel_InvalidProvider(t *testing.T) {
	_, e := setupTunnelHandler(t)

	body := `{"provider": "invalid_provider", "port": 8080}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tunnel/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != false {
		t.Error("Expected success to be false")
	}
}

func TestTunnelHandler_StartTunnel_InvalidBody(t *testing.T) {
	_, e := setupTunnelHandler(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tunnel/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTunnelHandler_GetConfig(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/config", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	config, ok := resp["config"].(map[string]interface{})
	if !ok {
		t.Fatal("Response should contain config")
	}

	// Should have tunnel_subdomain auto-generated
	if config["tunnel_subdomain"] == nil || config["tunnel_subdomain"] == "" {
		t.Error("Expected tunnel_subdomain to be auto-generated")
	}

	subdomain := config["tunnel_subdomain"].(string)
	if !strings.HasPrefix(subdomain, "echo-") {
		t.Errorf("tunnel_subdomain should start with 'echo-', got %s", subdomain)
	}
}

func TestTunnelHandler_UpdateConfig(t *testing.T) {
	_, e := setupTunnelHandler(t)

	body := `{"default_provider": "auto", "notify_on_error": true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tunnel/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestTunnelHandler_GetLogs(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/logs", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}
}

func TestTunnelHandler_GetDiagnostics(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/diagnostics", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["success"] != true {
		t.Error("Expected success to be true")
	}

	diagnostics, ok := resp["diagnostics"].(map[string]interface{})
	if !ok {
		t.Fatal("Response should contain diagnostics")
	}

	// Should have basic diagnostic info
	if _, ok := diagnostics["tunnel_running"]; !ok {
		t.Error("Diagnostics should contain tunnel_running")
	}
	if _, ok := diagnostics["ssh_available"]; !ok {
		t.Error("Diagnostics should contain ssh_available")
	}
	if _, ok := diagnostics["hints"]; !ok {
		t.Error("Diagnostics should contain hints")
	}
}

func TestTunnelHandler_GetQRCode_NoActiveTunnel(t *testing.T) {
	_, e := setupTunnelHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tunnel/qrcode", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should return 404 when no active tunnel
	if rec.Code != http.StatusNotFound {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestTunnelHandler_BackwardsCompatibility(t *testing.T) {
	_, e := setupTunnelHandler(t)

	// Test old /api/v1/remote-access/* paths still work
	paths := []string{
		"/api/v1/remote-access/providers",
		"/api/v1/remote-access/status",
		"/api/v1/remote-access/config",
		"/api/v1/remote-access/logs",
		"/api/v1/remote-access/diagnostics",
	}

	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()

		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Path %s: Status code = %d, want %d", path, rec.Code, http.StatusOK)
		}
	}
}

// Test tunnel provider implementations
func TestServeoManager_GetStatus_Initial(t *testing.T) {
	m := tunnel.NewServeoManager()

	status := m.GetStatus()
	if status.Active {
		t.Error("Expected tunnel to be inactive initially")
	}
	if status.Connecting {
		t.Error("Expected tunnel to not be connecting initially")
	}
	if status.Provider != tunnel.ProviderServeo {
		t.Errorf("Expected provider to be serveo, got %s", status.Provider)
	}
}

func TestLocalhostRunManager_GetStatus_Initial(t *testing.T) {
	m := tunnel.NewLocalhostRunManager()

	status := m.GetStatus()
	if status.Active {
		t.Error("Expected tunnel to be inactive initially")
	}
	if status.Connecting {
		t.Error("Expected tunnel to not be connecting initially")
	}
	if status.Provider != tunnel.ProviderLocalhostRun {
		t.Errorf("Expected provider to be localhost_run, got %s", status.Provider)
	}
}

func TestCloudflareManager_GetStatus_Initial(t *testing.T) {
	m := tunnel.NewCloudflareManager()

	status := m.GetStatus()
	if status.Active {
		t.Error("Expected tunnel to be inactive initially")
	}
	if status.Connecting {
		t.Error("Expected tunnel to not be connecting initially")
	}
	if status.Provider != tunnel.ProviderCloudflare {
		t.Errorf("Expected provider to be cloudflare, got %s", status.Provider)
	}
}

func TestProviderInfos(t *testing.T) {
	infos := tunnel.GetProviderInfos()

	// 4 providers: auto, ngrok, cloudflare, localtunnel
	if len(infos) != 4 {
		t.Errorf("Expected 4 provider infos, got %d", len(infos))
	}

	// Check each provider has required fields
	for _, info := range infos {
		if info.ID == "" {
			t.Error("Provider ID should not be empty")
		}
		if info.Name == "" {
			t.Error("Provider Name should not be empty")
		}
		if info.Description == "" {
			t.Error("Provider Description should not be empty")
		}
	}

	// Verify specific providers
	providerMap := make(map[tunnel.Provider]tunnel.ProviderInfo)
	for _, info := range infos {
		providerMap[info.ID] = info
	}

	// auto (Serveo) doesn't require key
	if providerMap[tunnel.ProviderAuto].RequiresKey {
		t.Error("auto should not require key")
	}

	// ngrok requires key
	if !providerMap[tunnel.ProviderNgrok].RequiresKey {
		t.Error("ngrok should require key")
	}

	// cloudflare requires key
	if !providerMap[tunnel.ProviderCloudflare].RequiresKey {
		t.Error("cloudflare should require key")
	}

	// localtunnel does not require key
	if providerMap[tunnel.ProviderLocalTunnel].RequiresKey {
		t.Error("localtunnel should not require key")
	}
}

func TestTunnelSubdomainGeneration(t *testing.T) {
	tempDir := filepath.Join(os.TempDir(), "tunnel-subdomain-test")
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	dbPath := filepath.Join(tempDir, "tunnel.db")
	repo, err := ngrok.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Get config - should have auto-generated subdomain
	config, err := repo.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}

	if config.TunnelSubdomain == "" {
		t.Error("Expected tunnel_subdomain to be auto-generated")
	}

	if !strings.HasPrefix(config.TunnelSubdomain, "echo-") {
		t.Errorf("tunnel_subdomain should start with 'echo-', got %s", config.TunnelSubdomain)
	}

	// Subdomain should be 13 characters: "echo-" (5) + 8 base58 chars
	if len(config.TunnelSubdomain) != 13 {
		t.Errorf("tunnel_subdomain should be 13 characters, got %d", len(config.TunnelSubdomain))
	}
	// Suffix should be base58 (no 0, O, I, l)
	const base58 = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range config.TunnelSubdomain[5:] {
		if !strings.ContainsRune(base58, c) {
			t.Errorf("tunnel_subdomain suffix should be base58, got %q", config.TunnelSubdomain)
		}
	}

	// Create another repo - should get the same subdomain (persisted)
	repo2, err := ngrok.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create second repository: %v", err)
	}
	defer repo2.Close()

	config2, err := repo2.GetConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get config from second repo: %v", err)
	}

	if config2.TunnelSubdomain != config.TunnelSubdomain {
		t.Errorf("Subdomain should be persisted: got %s, want %s", config2.TunnelSubdomain, config.TunnelSubdomain)
	}
}
