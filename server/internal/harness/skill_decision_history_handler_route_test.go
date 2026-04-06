package harness

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandler_RegisterRoutesIncludesSkillDecisionHistoryEndpoint(t *testing.T) {
	controller := newMinimalTestController(t)
	handler := NewHandler(controller)
	e := echo.New()
	group := e.Group("/harness")

	handler.RegisterRoutes(group)

	for _, route := range e.Routes() {
		if route.Method == http.MethodGet && route.Path == "/harness/skills/:skill_id/decision-history" {
			return
		}
	}
	t.Fatalf("expected skill decision-history route to be registered, got %#v", e.Routes())
}
