package agentsessions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandlerListSessionsFiltersByRuntime(t *testing.T) {
	service, _ := newTestAgentSessionService(t, nil)

	if _, err := service.CreateSession(t.Context(), CreateSessionParams{
		ProfileID: "codex",
		Name:      "Codex Session",
		UserID:    "user-a",
	}); err != nil {
		t.Fatalf("CreateSession(codex) error = %v", err)
	}
	if _, err := service.CreateSession(t.Context(), CreateSessionParams{
		ProfileID: "generic-a2a",
		Name:      "A2A Session",
		UserID:    "user-a",
	}); err != nil {
		t.Fatalf("CreateSession(generic-a2a) error = %v", err)
	}

	e := echo.New()
	NewHandler(service).RegisterSessionRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/sessions?runtime=a2a", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Sessions []ExternalSession `json:"sessions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if len(payload.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(payload.Sessions))
	}
	if payload.Sessions[0].Protocol != ProtocolA2A {
		t.Fatalf("session protocol = %s, want %s", payload.Sessions[0].Protocol, ProtocolA2A)
	}
}

func TestHandlerListSessionsRejectsInvalidRuntimeFilter(t *testing.T) {
	service, _ := newTestAgentSessionService(t, nil)

	e := echo.New()
	NewHandler(service).RegisterSessionRoutes(e.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agent-sessions/sessions?runtime=bogus", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
