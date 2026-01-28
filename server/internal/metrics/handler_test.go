package metrics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func setupTestHandler() (*Handler, *echo.Echo) {
	store := NewMockStore()
	config := &WriterConfig{
		MaxSamples:          1000,
		EnableSystemMetrics: true,
	}
	writer := NewMetricsWriter(store, config)

	// Add some test data
	writer.RecordAPICall("sonnet", true, 1500.0, 1000, 500, 100, 50, "")
	writer.RecordAPICall("opus", true, 3000.0, 2000, 1000, 0, 0, "")
	writer.RecordAPICall("sonnet", false, 500.0, 100, 0, 0, 0, "rate_limit")
	writer.RecordSpeed("sonnet", 50.0, 200.0, 55.0)

	handler := NewHandler(writer)
	e := echo.New()

	return handler, e
}

func TestHandler_GetCallStats(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/calls", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetCallStats(c)
	if err != nil {
		t.Fatalf("GetCallStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response CallStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Stats.TotalCalls != 3 {
		t.Errorf("Expected 3 total calls, got %d", response.Stats.TotalCalls)
	}
}

func TestHandler_GetModelStats(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/models", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetModelStats(c)
	if err != nil {
		t.Fatalf("GetModelStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response ModelStatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(response.Models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(response.Models))
	}

	if response.Summary.TotalCalls != 3 {
		t.Errorf("Expected 3 total calls in summary, got %d", response.Summary.TotalCalls)
	}
}

func TestHandler_GetModelStatsByName(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/models/sonnet", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("model")
	c.SetParamValues("sonnet")

	err := handler.GetModelStatsByName(c)
	if err != nil {
		t.Fatalf("GetModelStatsByName failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var stats ModelStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if stats.Model != "sonnet" {
		t.Errorf("Expected model 'sonnet', got '%s'", stats.Model)
	}

	if stats.Calls != 2 {
		t.Errorf("Expected 2 calls for sonnet, got %d", stats.Calls)
	}
}

func TestHandler_GetModelStatsByName_NotFound(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/models/nonexistent", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("model")
	c.SetParamValues("nonexistent")

	err := handler.GetModelStatsByName(c)
	if err == nil {
		t.Error("Expected error for non-existent model")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("Expected HTTPError, got %T", err)
	}

	if httpErr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", httpErr.Code)
	}
}

func TestHandler_GetTokenUsage(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/tokens", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetTokenUsage(c)
	if err != nil {
		t.Fatalf("GetTokenUsage failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response TokenUsageResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// 1000 + 2000 + 100 = 3100 input tokens
	if response.Usage.InputTokens != 3100 {
		t.Errorf("Expected 3100 input tokens, got %d", response.Usage.InputTokens)
	}
}

func TestHandler_GetLatencyStats(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/latency", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetLatencyStats(c)
	if err != nil {
		t.Fatalf("GetLatencyStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var stats LatencyStats
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if stats.Samples != 3 {
		t.Errorf("Expected 3 samples, got %d", stats.Samples)
	}
}

func TestHandler_GetSpeedStats(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/speed", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetSpeedStats(c)
	if err != nil {
		t.Fatalf("GetSpeedStats failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var response SpeedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Current.TokensPerSecond != 50.0 {
		t.Errorf("Expected TPS 50.0, got %.2f", response.Current.TokensPerSecond)
	}
}

func TestHandler_GetPricing(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/pricing", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetPricing(c)
	if err != nil {
		t.Fatalf("GetPricing failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var pricing []TokenPricing
	if err := json.Unmarshal(rec.Body.Bytes(), &pricing); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(pricing) == 0 {
		t.Error("Expected pricing data")
	}
}

func TestHandler_GetPricingForModel(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/pricing/sonnet", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("model")
	c.SetParamValues("sonnet")

	err := handler.GetPricingForModel(c)
	if err != nil {
		t.Fatalf("GetPricingForModel failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var pricing TokenPricing
	if err := json.Unmarshal(rec.Body.Bytes(), &pricing); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if pricing.InputPrice != 3.0 {
		t.Errorf("Expected input price 3.0 for sonnet, got %.2f", pricing.InputPrice)
	}
}

func TestHandler_GetSummary(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/metrics/summary", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetSummary(c)
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var summary map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if _, ok := summary["calls"]; !ok {
		t.Error("Expected 'calls' in summary")
	}

	if _, ok := summary["tokens"]; !ok {
		t.Error("Expected 'tokens' in summary")
	}

	if _, ok := summary["latency"]; !ok {
		t.Error("Expected 'latency' in summary")
	}

	if _, ok := summary["speed"]; !ok {
		t.Error("Expected 'speed' in summary")
	}
}

func TestHandler_ResetMetrics(t *testing.T) {
	handler, e := setupTestHandler()

	// First verify we have data
	stats := handler.writer.GetCallStats()
	if stats.TotalCalls == 0 {
		t.Error("Expected data before reset")
	}

	// Reset
	req := httptest.NewRequest(http.MethodPost, "/metrics/reset", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ResetMetrics(c)
	if err != nil {
		t.Fatalf("ResetMetrics failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify data is reset
	stats = handler.writer.GetCallStats()
	if stats.TotalCalls != 0 {
		t.Errorf("Expected 0 calls after reset, got %d", stats.TotalCalls)
	}
}

func TestHandler_GetSystemMetrics(t *testing.T) {
	handler, e := setupTestHandler()

	// Collect system metrics first
	if handler.writer.systemMonitor != nil {
		handler.writer.systemMonitor.Collect()
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics/system", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetSystemMetrics(c)
	if err != nil {
		t.Fatalf("GetSystemMetrics failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	var metrics SystemResourceMetrics
	if err := json.Unmarshal(rec.Body.Bytes(), &metrics); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if metrics.CPUCount <= 0 {
		t.Errorf("Expected CPUCount > 0, got %d", metrics.CPUCount)
	}
}
