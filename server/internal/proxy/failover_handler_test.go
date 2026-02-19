package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

// helper: create echo context with JSON body for PUT request.
func newUpdateConfigCtx(e *echo.Echo, body string) (echo.Context, *httptest.ResponseRecorder) {
	req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

// decodeConfig decodes the response body into FailoverConfig.
func decodeConfig(t *testing.T, rec *httptest.ResponseRecorder) FailoverConfig {
	t.Helper()
	var cfg FailoverConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return cfg
}

func TestUpdateConfig_PartialMerge_BoolFieldsIndependent(t *testing.T) {
	e := echo.New()

	// Start with all four toggles ON.
	cfg := &FailoverConfig{
		Enabled:        true,
		CircuitBreaker: true,
		ErrorClassification: ErrorClassificationConfig{
			Enabled: true,
		},
		StreamingAnomaly: StreamingAnomalyConfig{
			Enabled: true,
		},
		ContextWindowCheck: true,
	}
	h := NewFailoverAPIHandler(nil, cfg)

	// Toggle only circuit_breaker OFF — others must stay ON.
	c, rec := newUpdateConfigCtx(e, `{"circuit_breaker": false}`)
	if err := h.UpdateConfig(c); err != nil {
		t.Fatal(err)
	}
	got := decodeConfig(t, rec)

	if got.CircuitBreaker != false {
		t.Error("circuit_breaker should be false")
	}
	if got.ErrorClassification.Enabled != true {
		t.Error("error_classification.enabled should remain true")
	}
	if got.StreamingAnomaly.Enabled != true {
		t.Error("streaming_anomaly.enabled should remain true")
	}
	if got.ContextWindowCheck != true {
		t.Error("context_window_check should remain true")
	}
}

func TestUpdateConfig_PartialMerge_ToggleErrorClassification(t *testing.T) {
	e := echo.New()

	cfg := &FailoverConfig{
		CircuitBreaker:     true,
		ContextWindowCheck: true,
		ErrorClassification: ErrorClassificationConfig{
			Enabled:        false,
			FailoverErrors: []string{"rate_limit"},
		},
		StreamingAnomaly: StreamingAnomalyConfig{Enabled: true},
	}
	h := NewFailoverAPIHandler(nil, cfg)

	// Enable error_classification only — others must not change.
	c, rec := newUpdateConfigCtx(e, `{"error_classification": {"enabled": true}}`)
	if err := h.UpdateConfig(c); err != nil {
		t.Fatal(err)
	}
	got := decodeConfig(t, rec)

	if !got.ErrorClassification.Enabled {
		t.Error("error_classification.enabled should be true")
	}
	// Nested slice must survive partial merge.
	if len(got.ErrorClassification.FailoverErrors) != 1 || got.ErrorClassification.FailoverErrors[0] != "rate_limit" {
		t.Errorf("failover_errors should be preserved, got %v", got.ErrorClassification.FailoverErrors)
	}
	if !got.CircuitBreaker {
		t.Error("circuit_breaker should remain true")
	}
	if !got.ContextWindowCheck {
		t.Error("context_window_check should remain true")
	}
	if !got.StreamingAnomaly.Enabled {
		t.Error("streaming_anomaly.enabled should remain true")
	}
}

func TestUpdateConfig_PartialMerge_ToggleStreamingAnomaly(t *testing.T) {
	e := echo.New()

	cfg := &FailoverConfig{
		CircuitBreaker:      true,
		ContextWindowCheck:  true,
		ErrorClassification: ErrorClassificationConfig{Enabled: true},
		StreamingAnomaly: StreamingAnomalyConfig{
			Enabled:          true,
			WindowSize:       50,
			RecoveryStrategy: "failover",
		},
	}
	h := NewFailoverAPIHandler(nil, cfg)

	// Disable streaming_anomaly only.
	c, rec := newUpdateConfigCtx(e, `{"streaming_anomaly": {"enabled": false}}`)
	if err := h.UpdateConfig(c); err != nil {
		t.Fatal(err)
	}
	got := decodeConfig(t, rec)

	if got.StreamingAnomaly.Enabled {
		t.Error("streaming_anomaly.enabled should be false")
	}
	// Other nested fields must survive.
	if got.StreamingAnomaly.WindowSize != 50 {
		t.Errorf("window_size should be preserved, got %d", got.StreamingAnomaly.WindowSize)
	}
	if got.StreamingAnomaly.RecoveryStrategy != "failover" {
		t.Errorf("recovery_strategy should be preserved, got %s", got.StreamingAnomaly.RecoveryStrategy)
	}
	if !got.CircuitBreaker {
		t.Error("circuit_breaker should remain true")
	}
	if !got.ErrorClassification.Enabled {
		t.Error("error_classification.enabled should remain true")
	}
}

func TestUpdateConfig_PartialMerge_EmptyBodyChangesNothing(t *testing.T) {
	e := echo.New()

	cfg := &FailoverConfig{
		Enabled:             true,
		MaxRetries:          3,
		CircuitBreaker:      true,
		ContextWindowCheck:  true,
		ErrorClassification: ErrorClassificationConfig{Enabled: true},
		StreamingAnomaly:    StreamingAnomalyConfig{Enabled: true},
	}
	h := NewFailoverAPIHandler(nil, cfg)

	c, rec := newUpdateConfigCtx(e, `{}`)
	if err := h.UpdateConfig(c); err != nil {
		t.Fatal(err)
	}
	got := decodeConfig(t, rec)

	if !got.Enabled || !got.CircuitBreaker || !got.ContextWindowCheck {
		t.Error("empty body should not change any bool fields")
	}
	if !got.ErrorClassification.Enabled || !got.StreamingAnomaly.Enabled {
		t.Error("empty body should not change nested enabled fields")
	}
	if got.MaxRetries != 3 {
		t.Errorf("max_retries should be preserved, got %d", got.MaxRetries)
	}
}

func TestUpdateConfig_InvalidJSON(t *testing.T) {
	e := echo.New()

	cfg := &FailoverConfig{Enabled: true}
	h := NewFailoverAPIHandler(nil, cfg)

	c, rec := newUpdateConfigCtx(e, `not json`)
	if err := h.UpdateConfig(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
