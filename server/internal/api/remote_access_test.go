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
)

func setupRemoteAccessHandler(t *testing.T) (*RemoteAccessHandler, *echo.Echo) {
	tm := ngrok.NewTunnelManager()
	h := NewRemoteAccessHandler(tm, 23456)

	e := echo.New()
	h.RegisterRoutes(e)

	return h, e
}

func TestRemoteAccessHandler_GetNgrokStatus(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/ngrok/status", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var status ngrok.NgrokStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should not be installed in test environment
	if status.Installed {
		t.Log("ngrok is installed in test environment")
	}
}

func TestRemoteAccessHandler_GetRemoteAccessStatus(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/status", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have tunnel status
	if _, ok := resp["tunnel"]; !ok {
		t.Error("Response should contain tunnel status")
	}
}

func TestRemoteAccessHandler_StartRemoteAccess_NgrokNotInstalled(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	body := `{"port": 23456}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/remote-access/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should return precondition failed when ngrok not installed
	if rec.Code != http.StatusPreconditionFailed {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusPreconditionFailed)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["error_code"] != "NGROK_NOT_INSTALLED" {
		t.Errorf("error_code = %v, want NGROK_NOT_INSTALLED", resp["error_code"])
	}
}

func TestRemoteAccessHandler_StopRemoteAccess(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/remote-access/stop", nil)
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

func TestRemoteAccessHandler_StartRemoteAccess_InvalidBody(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/remote-access/start", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func setupRemoteAccessHandlerWithRepo(t *testing.T) (*RemoteAccessHandler, *echo.Echo, *ngrok.Repository) {
	tempDir := filepath.Join(os.TempDir(), "ngrok-api-test-repo")
	os.MkdirAll(tempDir, 0755)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	dbPath := filepath.Join(tempDir, "remote_access.db")
	repo, err := ngrok.NewRepository(dbPath)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	tm := ngrok.NewTunnelManagerWithRepo(repo)
	h := NewRemoteAccessHandlerWithRepo(tm, repo, 23456)

	e := echo.New()
	h.RegisterRoutes(e)

	return h, e, repo
}

func TestRemoteAccessHandler_GetRemoteAccessConfig(t *testing.T) {
	_, e, _ := setupRemoteAccessHandlerWithRepo(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/config", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Should have config data
	if _, ok := resp["config"]; !ok {
		t.Error("Response should contain config")
	}
}

func TestRemoteAccessHandler_UpdateRemoteAccessConfig(t *testing.T) {
	_, e, _ := setupRemoteAccessHandlerWithRepo(t)

	body := `{"enabled": true, "notification_email": "test@example.com", "notify_on_url_change": true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/remote-access/config", strings.NewReader(body))
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

	// Verify config was saved by fetching it
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/config", nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	var resp2 map[string]interface{}
	json.Unmarshal(rec2.Body.Bytes(), &resp2)

	config := resp2["config"].(map[string]interface{})
	if config["enabled"] != true {
		t.Error("Config enabled should be true")
	}
	if config["notification_email"] != "test@example.com" {
		t.Errorf("notification_email = %v, want test@example.com", config["notification_email"])
	}
}

func TestRemoteAccessHandler_UpdateRemoteAccessConfig_InvalidBody(t *testing.T) {
	_, e, _ := setupRemoteAccessHandlerWithRepo(t)

	body := `invalid json`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/remote-access/config", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestRemoteAccessHandler_GetRemoteAccessLogs(t *testing.T) {
	_, e, repo := setupRemoteAccessHandlerWithRepo(t)

	// Add some test logs
	ctx := context.Background()
	repo.AddLog(ctx, "", "test_event", "Test message 1", nil)
	repo.AddLog(ctx, "", "test_event", "Test message 2", map[string]interface{}{"key": "value"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/logs", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	logs, ok := resp["logs"].([]interface{})
	if !ok {
		t.Fatal("Response should contain logs array")
	}

	if len(logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(logs))
	}
}

func TestRemoteAccessHandler_GetRemoteAccessLogs_WithPagination(t *testing.T) {
	_, e, repo := setupRemoteAccessHandlerWithRepo(t)

	// Add multiple test logs
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		repo.AddLog(ctx, "", "test_event", "Test message", nil)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/logs?limit=2&offset=1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &resp)

	logs := resp["logs"].([]interface{})
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs with limit=2, got %d", len(logs))
	}
}

func TestRemoteAccessHandler_GetRemoteAccessConfig_NoRepo(t *testing.T) {
	_, e := setupRemoteAccessHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/remote-access/config", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should return service unavailable when no repo configured
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}
