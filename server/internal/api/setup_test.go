package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/setup"
)

func TestFirstRunHandler_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	manager := setup.NewFirstRunManager(tmpDir)
	handler := NewFirstRunHandler(manager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/first-run/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.GetStatus(c); err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp FirstRunStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.IsFirstRun {
		t.Error("expected IsFirstRun to be true")
	}
}

func TestFirstRunHandler_MarkComplete(t *testing.T) {
	tmpDir := t.TempDir()
	manager := setup.NewFirstRunManager(tmpDir)
	handler := NewFirstRunHandler(manager)

	e := echo.New()
	body := `{"statistics_opt_in": true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/first-run/complete", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.MarkComplete(c); err != nil {
		t.Fatalf("MarkComplete failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify status changed
	if manager.IsFirstRun() {
		t.Error("expected IsFirstRun to be false after MarkComplete")
	}

	status := manager.GetStatus()
	if !status.StatisticsOptIn {
		t.Error("expected StatisticsOptIn to be true")
	}
}

func TestFirstRunHandler_SkipCLI(t *testing.T) {
	tmpDir := t.TempDir()
	manager := setup.NewFirstRunManager(tmpDir)
	handler := NewFirstRunHandler(manager)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/first-run/skip-cli", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.SkipCLI(c); err != nil {
		t.Fatalf("SkipCLI failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	status := manager.GetStatus()
	if !status.CLISkipped {
		t.Error("expected CLISkipped to be true")
	}
}

func TestFirstRunHandler_Reset(t *testing.T) {
	tmpDir := t.TempDir()
	manager := setup.NewFirstRunManager(tmpDir)
	handler := NewFirstRunHandler(manager)

	// First complete first-run
	manager.MarkComplete()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/first-run/reset", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Reset(c); err != nil {
		t.Fatalf("Reset failed: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify status reset
	if !manager.IsFirstRun() {
		t.Error("expected IsFirstRun to be true after Reset")
	}
}
