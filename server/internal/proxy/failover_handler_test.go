package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
