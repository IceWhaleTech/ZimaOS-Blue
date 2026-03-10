package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServeHTTPBlocksResponsesPathWhenDisabled(t *testing.T) {
	ph := NewProxyHandler(nil, nil, nil)
	ph.SetResponsesIntegrationEnabled(false)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	rec := httptest.NewRecorder()

	ph.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
}
