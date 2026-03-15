package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/labstack/echo/v4"
)

func newUserDataHandlerForTest(t *testing.T) (*UserDataHandler, func()) {
	t.Helper()

	store, err := memory.NewStore(":memory:")
	if err != nil {
		t.Fatalf("memory.NewStore: %v", err)
	}

	handler := NewUserDataHandler(store)
	cleanup := func() {
		_ = store.Close()
	}
	return handler, cleanup
}

func encodeLegacyUserDataImportPayload(t *testing.T) string {
	t.Helper()

	export := map[string]interface{}{
		"version":     "1.0",
		"exported_at": "2026-03-12T00:00:00Z",
		"data_type":   "json",
		"settings": map[string]interface{}{
			"theme":       "dark",
			"locale":      "en-US",
			"theme_style": "minimal",
		},
	}

	raw, err := json.Marshal(export)
	if err != nil {
		t.Fatalf("marshal legacy export: %v", err)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

func buildUserDataImportRequestBody(t *testing.T, payload string) []byte {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"password": "test-password",
		"data":     payload,
	})
	if err != nil {
		t.Fatalf("marshal import request: %v", err)
	}
	return body
}

func TestImportPreviewLegacyThemeStyleIgnored(t *testing.T) {
	handler, cleanup := newUserDataHandlerForTest(t)
	defer cleanup()

	e := echo.New()
	body := buildUserDataImportRequestBody(t, encodeLegacyUserDataImportPayload(t))
	req := httptest.NewRequest(http.MethodPost, "/api/userdata/import/preview", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.ImportPreview(c); err != nil {
		t.Fatalf("ImportPreview failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	settingsPreview, ok := resp["settings_preview"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings_preview missing or invalid: %v", resp["settings_preview"])
	}
	if settingsPreview["theme"] != "dark" {
		t.Fatalf("settings_preview.theme = %v, want dark", settingsPreview["theme"])
	}
	if settingsPreview["locale"] != "en-US" {
		t.Fatalf("settings_preview.locale = %v, want en-US", settingsPreview["locale"])
	}
	if _, exists := settingsPreview["theme_style"]; exists {
		t.Fatalf("legacy field theme_style should be ignored: %+v", settingsPreview)
	}
}

func TestImportLegacyThemeStyleIgnored(t *testing.T) {
	handler, cleanup := newUserDataHandlerForTest(t)
	defer cleanup()

	e := echo.New()
	body := buildUserDataImportRequestBody(t, encodeLegacyUserDataImportPayload(t))
	req := httptest.NewRequest(http.MethodPost, "/api/userdata/import", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.Import(c); err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if success, _ := resp["success"].(bool); !success {
		t.Fatalf("success = %v, want true", resp["success"])
	}

	settings, ok := resp["settings"].(map[string]interface{})
	if !ok {
		t.Fatalf("settings missing or invalid: %v", resp["settings"])
	}
	if settings["theme"] != "dark" {
		t.Fatalf("settings.theme = %v, want dark", settings["theme"])
	}
	if settings["locale"] != "en-US" {
		t.Fatalf("settings.locale = %v, want en-US", settings["locale"])
	}
	if _, exists := settings["theme_style"]; exists {
		t.Fatalf("legacy field theme_style should be ignored: %+v", settings)
	}
}
