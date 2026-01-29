package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/stats"
)

func TestStatsHandler_GetStats_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, false)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	handler := NewStatsHandler(collector, consentManager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetStats(c); err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp["enabled"].(bool) {
		t.Error("expected enabled to be false")
	}
}

func TestStatsHandler_GetStats_Enabled(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, true)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	// Set consent to enable collection (ConsentManager disables by default)
	consentManager.SetConsent(true)
	handler := NewStatsHandler(collector, consentManager)

	// Record some events
	collector.Record(&stats.APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet",
		InputTokens:  100,
		OutputTokens: 50,
		LatencyMs:    500,
		Success:      true,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats?period=day", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetStats(c); err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp["enabled"].(bool) {
		t.Error("expected enabled to be true")
	}

	if resp["period"] != "day" {
		t.Errorf("expected period 'day', got '%s'", resp["period"])
	}
}

func TestStatsHandler_GetRecentEvents(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, true)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	// Set consent to enable collection (ConsentManager disables by default)
	consentManager.SetConsent(true)
	handler := NewStatsHandler(collector, consentManager)

	// Record some events
	collector.Record(&stats.APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet",
		InputTokens:  100,
		OutputTokens: 50,
		Success:      true,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/recent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetRecentEvents(c); err != nil {
		t.Fatalf("GetRecentEvents failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp["enabled"].(bool) {
		t.Error("expected enabled to be true")
	}

	count := int(resp["count"].(float64))
	if count != 1 {
		t.Errorf("expected count 1, got %d", count)
	}
}

func TestStatsHandler_ClearStats(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, true)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	// Set consent to enable collection (ConsentManager disables by default)
	consentManager.SetConsent(true)
	handler := NewStatsHandler(collector, consentManager)

	// Record some events
	collector.Record(&stats.APICallEvent{
		Provider: "anthropic",
		Success:  true,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ClearStats(c); err != nil {
		t.Fatalf("ClearStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify events cleared
	if collector.GetEventCount() != 0 {
		t.Error("expected events to be cleared")
	}
}

func TestStatsHandler_GetConsentStatus(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, false)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	handler := NewStatsHandler(collector, consentManager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/consent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetConsentStatus(c); err != nil {
		t.Fatalf("GetConsentStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp stats.ConsentStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Consented {
		t.Error("expected Consented to be false initially")
	}
}

func TestStatsHandler_SetConsentStatus(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, false)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	handler := NewStatsHandler(collector, consentManager)

	e := echo.New()
	body := `{"consented": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stats/consent", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SetConsentStatus(c); err != nil {
		t.Fatalf("SetConsentStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify consent was set
	if !consentManager.IsConsented() {
		t.Error("expected consent to be true")
	}

	// Verify collector is enabled
	if !collector.IsEnabled() {
		t.Error("expected collector to be enabled")
	}
}

func TestStatsHandler_RevokeConsent(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, true)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	handler := NewStatsHandler(collector, consentManager)

	// First give consent
	consentManager.SetConsent(true)

	// Record some events (after consent is given)
	collector.Record(&stats.APICallEvent{
		Provider: "anthropic",
		Success:  true,
	})

	e := echo.New()
	body := `{"consented": false, "clear_data": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/stats/consent", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SetConsentStatus(c); err != nil {
		t.Fatalf("SetConsentStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify consent was revoked
	if consentManager.IsConsented() {
		t.Error("expected consent to be false")
	}

	// Verify data was cleared
	if collector.GetEventCount() != 0 {
		t.Error("expected events to be cleared")
	}
}

func TestStatsHandler_GetConsentInfo(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, false)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	handler := NewStatsHandler(collector, consentManager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/consent/info", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetConsentInfo(c); err != nil {
		t.Fatalf("GetConsentInfo failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp stats.ConsentInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Title == "" {
		t.Error("expected Title to be non-empty")
	}
	if len(resp.DataTypes) == 0 {
		t.Error("expected DataTypes to be non-empty")
	}
}

func TestStatsHandler_ExportStats(t *testing.T) {
	tmpDir := t.TempDir()
	collector := stats.NewStatisticsCollector(tmpDir, true)
	consentManager := stats.NewConsentManager(tmpDir, collector)
	// Set consent to enable collection (ConsentManager disables by default)
	consentManager.SetConsent(true)
	handler := NewStatsHandler(collector, consentManager)

	// Record some events
	collector.Record(&stats.APICallEvent{
		Provider:     "anthropic",
		Model:        "claude-3-5-sonnet",
		InputTokens:  100,
		OutputTokens: 50,
		Success:      true,
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/export?format=json", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ExportStats(c); err != nil {
		t.Fatalf("ExportStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify content disposition header
	contentDisposition := rec.Header().Get("Content-Disposition")
	if contentDisposition == "" {
		t.Error("expected Content-Disposition header")
	}
}
