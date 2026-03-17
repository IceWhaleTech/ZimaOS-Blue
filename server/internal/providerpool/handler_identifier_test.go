package providerpool

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTestProviderHandler(t *testing.T) (*Handler, *echo.Echo) {
	t.Helper()

	storage, err := NewFileStorage(t.TempDir())
	if err != nil {
		t.Fatalf("NewFileStorage failed: %v", err)
	}
	registry, err := NewRegistry(storage)
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	handler := NewHandler(&Pool{Registry: registry})
	e := echo.New()
	handler.RegisterRoutes(e.Group("/providers"))
	return handler, e
}

func TestHandlerGetProviderRejectsInvalidProviderID(t *testing.T) {
	_, e := newTestProviderHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/providers/..%2Fescape", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerAddProviderRejectsInvalidProviderID(t *testing.T) {
	_, e := newTestProviderHandler(t)

	body := `{"id":"../escape","name":"Bad Provider","base_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}
