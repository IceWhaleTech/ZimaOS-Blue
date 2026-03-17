package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestResolveLocalFileRejectsRemoteClient(t *testing.T) {
	file := tempFileWithContents(t, "report.txt", []byte("ok"))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/local-file?path="+url.QueryEscape(file), nil)
	req.RemoteAddr = "203.0.113.5:12345"
	req = req.WithContext(context.WithValue(req.Context(), auth.UserContextKey, &auth.UserClaims{
		UserID: "preview-user",
		Role:   "user",
	}))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var handler SystemHandler
	err := handler.ResolveLocalFile(c)
	if err == nil {
		t.Fatalf("expected HTTP error, got nil")
	}
	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", httpErr.Code, http.StatusForbidden)
	}
}

func tempFileWithContents(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := t.TempDir()
	path := dir + "/" + name
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}
