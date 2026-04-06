package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func performAgentJSONRequestAsUser(e *echo.Echo, method, path, body, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	if userID != "" {
		req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{UserID: userID, Role: "user"}))
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestHandlerDeleteTaskRejectsCrossUserAccess(t *testing.T) {
	store := testStore(t)
	if err := store.Create(context.Background(), &Task{ID: "t1", UserID: "user-a", Goal: "secret"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	handler := NewHandler(store, nil)
	e := echo.New()
	handler.RegisterRoutes(e.Group("/api/v1/agent"))

	rec := performAgentJSONRequestAsUser(e, http.MethodDelete, "/api/v1/agent/tasks/t1", "", "user-b")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", rec.Code, rec.Body.String())
	}

	ownerRec := performAgentJSONRequestAsUser(e, http.MethodDelete, "/api/v1/agent/tasks/t1", "", "user-a")
	if ownerRec.Code != http.StatusNoContent {
		t.Fatalf("owner delete status = %d, want 204, body=%s", ownerRec.Code, ownerRec.Body.String())
	}
}
