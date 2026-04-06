package security

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNewHandler_InitializesScanCacheOnDemand(t *testing.T) {
	handler := NewHandler(nil)
	if handler.scanCache != nil {
		t.Fatal("expected security scan cache to start nil")
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/security/scan/run", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.RunSecurityScan(c); err != nil {
		t.Fatalf("RunSecurityScan() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if handler.scanCache == nil {
		t.Fatal("expected security scan cache to initialize on first scan")
	}
	if _, ok := handler.scanCache.Get("security_scan_result"); !ok {
		t.Fatal("expected security scan result to be stored in the lazy cache")
	}
}
