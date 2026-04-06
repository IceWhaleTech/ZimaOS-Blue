package browser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func performBrowserJSONRequest(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestBrowserRelayInfoRoute(t *testing.T) {
	h := NewHandler(nil)
	h.SetRelayInfoProvider(func() RelayInfo {
		return RelayInfo{
			Enabled:      true,
			BaseURL:      "http://127.0.0.1:18792",
			CDPURL:       "http://127.0.0.1:18792?token=test-token",
			Token:        "test-token",
			ExtensionDir: "/tmp/blue-browser-relay-extension",
		}
	})

	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	rec := performBrowserJSONRequest(e, http.MethodGet, "/browser/relay/info", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var info RelayInfo
	if err := json.Unmarshal(rec.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode relay info failed: %v", err)
	}
	if !info.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	if info.Token != "test-token" {
		t.Fatalf("Token = %q, want %q", info.Token, "test-token")
	}
}
