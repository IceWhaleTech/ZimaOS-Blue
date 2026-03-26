package providerpool

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

	discovery := NewModelDiscovery(registry, storage, time.Hour)
	handler := NewHandler(&Pool{Registry: registry, Discovery: discovery})
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

func TestHandlerUpdateProviderSwitchesCustomFormatBackToAutoImmediately(t *testing.T) {
	h, e := newTestProviderHandler(t)

	restoreProbe := installProbeTransport(func(r *http.Request) (int, string) {
		if r.URL.Host == "new.example.com" && r.URL.Path == "/v1/messages" {
			return http.StatusOK, `{"id":"msg_123"}`
		}
		return http.StatusNotFound, `{"error":"not found"}`
	})
	defer restoreProbe()

	err := h.pool.Registry.Register(&Provider{
		ID:               "custom-provider",
		Name:             "Custom Provider",
		Type:             ProviderTypeCustom,
		Location:         ProviderLocationCloud,
		Enabled:          true,
		Status:           ProviderStatusActive,
		BaseURL:          "https://old.example.com/v1",
		DetectedEndpoint: "https://old.example.com/v1/responses",
		APIFormat:        APIFormatResponses,
		DetectedFormat:   APIFormatResponses,
		APIFormatMode:    APIFormatModePinned,
		Priority:         10,
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	body := `{"base_url":"https://new.example.com/v1","api_format_mode":"auto"}`
	req := httptest.NewRequest(http.MethodPut, "/providers/custom-provider", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var updated Provider
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if updated.APIFormatMode != APIFormatModeAuto {
		t.Fatalf("api_format_mode = %q, want %q", updated.APIFormatMode, APIFormatModeAuto)
	}
	if updated.APIFormat != APIFormatAnthropic {
		t.Fatalf("api_format = %q, want %q", updated.APIFormat, APIFormatAnthropic)
	}
	if updated.DetectedFormat != APIFormatAnthropic {
		t.Fatalf("detected_format = %q, want %q", updated.DetectedFormat, APIFormatAnthropic)
	}
	if updated.BaseURL != "https://new.example.com/v1" {
		t.Fatalf("base_url = %q, want %q", updated.BaseURL, "https://new.example.com/v1")
	}
	if updated.DetectedEndpoint != "" {
		t.Fatalf("detected_endpoint = %q, want empty", updated.DetectedEndpoint)
	}
}

func TestHandlerUpdateProviderRejectsLocalLocationForBuiltinProvider(t *testing.T) {
	h, e := newTestProviderHandler(t)

	err := h.pool.Registry.Register(&Provider{
		ID:       "openai",
		Name:     "OpenAI",
		Type:     ProviderTypeBuiltin,
		Location: ProviderLocationCloud,
		Enabled:  true,
		Status:   ProviderStatusActive,
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/providers/openai", strings.NewReader(`{"location":"local"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerListProvidersForcesBuiltinLocationToCloud(t *testing.T) {
	h, e := newTestProviderHandler(t)

	err := h.pool.Registry.Register(&Provider{
		ID:       "openai",
		Name:     "OpenAI",
		Type:     ProviderTypeBuiltin,
		Location: ProviderLocationLocal,
		Enabled:  true,
		Status:   ProviderStatusActive,
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/providers", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Providers []struct {
			ID       string           `json:"id"`
			Location ProviderLocation `json:"location"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(resp.Providers) != 1 {
		t.Fatalf("providers len = %d, want 1", len(resp.Providers))
	}
	if resp.Providers[0].Location != ProviderLocationCloud {
		t.Fatalf("location = %q, want %q", resp.Providers[0].Location, ProviderLocationCloud)
	}
}

func TestHandlerDeleteProviderIconRejectsNonCustomProvider(t *testing.T) {
	h, e := newTestProviderHandler(t)

	err := h.pool.Registry.Register(&Provider{
		ID:         "openai",
		Name:       "OpenAI",
		Type:       ProviderTypeBuiltin,
		Location:   ProviderLocationCloud,
		Enabled:    true,
		Status:     ProviderStatusActive,
		CustomIcon: "data:image/png;base64,abc123",
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/providers/openai/icon", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestHandlerDeleteProviderIconClearsCustomProviderIcon(t *testing.T) {
	h, e := newTestProviderHandler(t)

	err := h.pool.Registry.Register(&Provider{
		ID:         "custom-provider",
		Name:       "Custom Provider",
		Type:       ProviderTypeCustom,
		Location:   ProviderLocationCloud,
		Enabled:    true,
		Status:     ProviderStatusActive,
		CustomIcon: "data:image/png;base64,abc123",
	})
	if err != nil {
		t.Fatalf("register provider failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/providers/custom-provider/icon", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusNoContent, rec.Body.String())
	}

	provider, err := h.pool.Registry.Get("custom-provider")
	if err != nil {
		t.Fatalf("get provider failed: %v", err)
	}
	if provider.CustomIcon != "" {
		t.Fatalf("custom_icon = %q, want empty", provider.CustomIcon)
	}
}
