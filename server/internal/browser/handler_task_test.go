package browser

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

func TestBrowserTaskCancel_RemainsCancelledAfterAsyncCompletion(t *testing.T) {
	h := NewHandler(nil)
	e := echo.New()
	h.RegisterRoutes(e.Group("/browser"))

	createRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/tasks", `{"name":"demo","steps":[]}`)
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status=%d body=%s", createRec.Code, createRec.Body.String())
	}

	var created BrowserTask
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created task id")
	}

	runRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/tasks/"+created.ID+"/run", "")
	if runRec.Code != http.StatusOK {
		t.Fatalf("run status=%d body=%s", runRec.Code, runRec.Body.String())
	}

	time.Sleep(100 * time.Millisecond)
	cancelRec := performBrowserJSONRequest(e, http.MethodPost, "/browser/tasks/"+created.ID+"/cancel", "")
	if cancelRec.Code != http.StatusOK {
		t.Fatalf("cancel status=%d body=%s", cancelRec.Code, cancelRec.Body.String())
	}

	time.Sleep(2200 * time.Millisecond)
	getRec := performBrowserJSONRequest(e, http.MethodGet, "/browser/tasks/"+created.ID, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status=%d body=%s", getRec.Code, getRec.Body.String())
	}

	var got BrowserTask
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode get response failed: %v", err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("status=%q, want %q", got.Status, "cancelled")
	}
	if got.CompletedAt == "" {
		t.Fatal("expected cancelled task to have completed_at set")
	}
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
