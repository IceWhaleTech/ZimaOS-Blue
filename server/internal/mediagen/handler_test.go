package mediagen

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func newTestMediaHandler() *Handler {
	manager := NewManager(nil, nil, "")
	return NewHandler(manager, nil, "")
}

func TestHandlerGenerateImageNoProviderReturnsActionableError(t *testing.T) {
	h := newTestMediaHandler()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/images/generations", bytes.NewBufferString(`{"prompt":"draw a cat"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GenerateImage(c); err != nil {
		t.Fatalf("GenerateImage returned error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "no media provider is configured and enabled" {
		t.Fatalf("error = %q", body["error"])
	}
	if body["hint"] == "" {
		t.Fatal("expected hint in response")
	}
}

func TestHandlerDirectGenerateNoProviderReturnsActionableError(t *testing.T) {
	h := newTestMediaHandler()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/media/generate", bytes.NewBufferString(`{"category":"t2i","prompt":"draw a cat"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.DirectGenerate(c); err != nil {
		t.Fatalf("DirectGenerate returned error: %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "no media provider is configured and enabled" {
		t.Fatalf("error = %q", body["error"])
	}
	if body["hint"] == "" {
		t.Fatal("expected hint in response")
	}
}
