package proxy

import (
	stdjson "encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/resilience"
	"github.com/labstack/echo/v4"
)

func TestFailoverAPIHandler_UpdateConfig_ProviderRace(t *testing.T) {
	e := echo.New()
	cfg := DefaultProxyConfig().Routing.Failover
	h := NewFailoverAPIHandler(nil, &cfg)

	saved := false
	raceUpdated := false
	h.SetOnConfigSave(func(c *FailoverConfig) error {
		saved = c.ProviderRace.Enabled
		return nil
	})
	h.SetOnProviderRaceChange(func(pr ProviderRaceConfig) {
		raceUpdated = pr.Enabled
	})

	req := httptest.NewRequest(http.MethodPut, "/config", strings.NewReader(`{"provider_race":{"enabled":true}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.UpdateConfig(c); err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !cfg.ProviderRace.Enabled {
		t.Fatal("provider race should be enabled")
	}
	if !saved {
		t.Fatal("onConfigSave callback was not invoked with updated config")
	}
	if !raceUpdated {
		t.Fatal("onProviderRaceChange callback was not invoked")
	}
}

func TestFailoverAPIHandler_GetOverview_AggregatesSparseFailoverState(t *testing.T) {
	e := echo.New()
	cfg := DefaultProxyConfig().Routing.Failover
	cfg.Enabled = true
	cfg.CircuitBreaker = true
	cfg.StreamingAnomaly.Enabled = true

	smart := NewSmartFailoverHandler(&cfg, nil)
	smart.metrics.RecordFailover("provider-a", "provider-b", true)
	smart.metrics.RecordError("provider-a", &ErrorClassification{Type: ErrorTypeTimeout})

	breaker := smart.FailoverHandler.getBreaker("provider-a")
	now := time.Now().UTC()
	breaker.LoadState(resilience.StateOpen, 2, 0, now, now)

	h := NewFailoverAPIHandler(smart, &cfg)
	h.SetProviderRaceStatsProvider(func() ProviderRaceStatsSnapshot {
		return ProviderRaceStatsSnapshot{
			RequestsTotal:             8,
			SuccessfulRaces:           6,
			Hits:                      3,
			HitRate:                   0.375,
			AvgWinnerLatencyMs:        42.5,
			EstimatedLatencySavedMs:   18.75,
			EstimatedSavingsSamples:   2,
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/overview", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetOverview(c); err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Metrics struct {
			ErrorsByType    map[string]int64            `json:"errors_by_type"`
			FailoverTotal   int64                       `json:"failover_total"`
			FailoverSuccess int64                       `json:"failover_success"`
			ProviderErrors  map[string]map[string]int64 `json:"provider_errors"`
		} `json:"metrics"`
		Config struct {
			Enabled          bool `json:"enabled"`
			CircuitBreaker   bool `json:"circuit_breaker"`
			StreamingAnomaly struct {
				Enabled bool `json:"enabled"`
			} `json:"streaming_anomaly"`
		} `json:"config"`
		ProviderRace struct {
			RequestsTotal           int64   `json:"requests_total"`
			SuccessfulRaces         int64   `json:"successful_races"`
			Hits                    int64   `json:"hits"`
			HitRate                 float64 `json:"hit_rate"`
			AvgWinnerLatencyMs      float64 `json:"avg_winner_latency_ms"`
			EstimatedLatencySavedMs float64 `json:"estimated_latency_saved_ms"`
			EstimatedSavingsSamples int64   `json:"estimated_savings_samples"`
		} `json:"provider_race"`
		CircuitBreakers map[string]struct {
			State    string `json:"state"`
			Failures int    `json:"failures"`
		} `json:"circuit_breakers"`
	}
	if err := stdjson.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if body.Metrics.FailoverTotal != 1 || body.Metrics.FailoverSuccess != 1 {
		t.Fatalf("unexpected metrics payload: %+v", body.Metrics)
	}
	if body.Metrics.ErrorsByType[string(ErrorTypeTimeout)] != 1 {
		t.Fatalf("unexpected error counts: %+v", body.Metrics.ErrorsByType)
	}
	if body.Metrics.ProviderErrors["provider-a"][string(ErrorTypeTimeout)] != 1 {
		t.Fatalf("unexpected provider error counts: %+v", body.Metrics.ProviderErrors)
	}
	if !body.Config.Enabled || !body.Config.CircuitBreaker || !body.Config.StreamingAnomaly.Enabled {
		t.Fatalf("unexpected config payload: %+v", body.Config)
	}
	if body.ProviderRace.RequestsTotal != 8 ||
		body.ProviderRace.SuccessfulRaces != 6 ||
		body.ProviderRace.Hits != 3 ||
		body.ProviderRace.HitRate != 0.375 ||
		body.ProviderRace.AvgWinnerLatencyMs != 42.5 ||
		body.ProviderRace.EstimatedLatencySavedMs != 18.75 ||
		body.ProviderRace.EstimatedSavingsSamples != 2 {
		t.Fatalf("unexpected provider race payload: %+v", body.ProviderRace)
	}
	if body.CircuitBreakers["provider-a"].State != "open" || body.CircuitBreakers["provider-a"].Failures != 2 {
		t.Fatalf("unexpected breaker payload: %+v", body.CircuitBreakers)
	}
}
