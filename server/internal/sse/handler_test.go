package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func newSSETestContext() echo.Context {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec)
}

func TestResolveUserID_FromAuthContext(t *testing.T) {
	c := newSSETestContext()
	ctx := context.WithValue(c.Request().Context(), auth.UserContextKey, &auth.UserClaims{UserID: "user-123"})
	c.SetRequest(c.Request().WithContext(ctx))

	if got := resolveUserID(c); got != "user-123" {
		t.Fatalf("resolveUserID() = %q, want %q", got, "user-123")
	}
}

func TestResolveUserID_FallbackToEchoContextUserID(t *testing.T) {
	c := newSSETestContext()
	c.Set("user_id", "user-echo")

	if got := resolveUserID(c); got != "user-echo" {
		t.Fatalf("resolveUserID() = %q, want %q", got, "user-echo")
	}
}

func TestResolveUserID_Default(t *testing.T) {
	c := newSSETestContext()

	if got := resolveUserID(c); got != "default" {
		t.Fatalf("resolveUserID() = %q, want %q", got, "default")
	}
}
