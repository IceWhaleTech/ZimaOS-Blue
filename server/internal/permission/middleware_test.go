package permission

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/labstack/echo/v4"
)

func TestRequirePagePermission(t *testing.T) {
	t.Run("rejects unauthenticated requests", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)

		handler := RequirePagePermission(nil, PageChat)(func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})
		if err := handler(c); err != nil {
			c.Echo().DefaultHTTPErrorHandler(err, c)
		}

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("allows preview user", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withClaims(httptest.NewRequest(http.MethodGet, "/", nil), &auth.UserClaims{
			UserID: "preview-user",
			Role:   "user",
		})
		c := echo.New().NewContext(req, rec)

		handler := RequirePagePermission(nil, PageSecurity)(func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})
		if err := handler(c); err != nil {
			c.Echo().DefaultHTTPErrorHandler(err, c)
		}

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})

	t.Run("allows role default permission fallback", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withClaims(httptest.NewRequest(http.MethodGet, "/", nil), &auth.UserClaims{
			UserID: "0d4e2f2a-09a8-4d9f-96d5-507460c1f4c0",
			Role:   "user",
		})
		c := echo.New().NewContext(req, rec)

		handler := RequirePagePermission(nil, PageChat)(func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})
		if err := handler(c); err != nil {
			c.Echo().DefaultHTTPErrorHandler(err, c)
		}

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})

	t.Run("denies missing permission", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := withClaims(httptest.NewRequest(http.MethodGet, "/", nil), &auth.UserClaims{
			UserID: "0d4e2f2a-09a8-4d9f-96d5-507460c1f4c0",
			Role:   "user",
		})
		c := echo.New().NewContext(req, rec)

		handler := RequirePagePermission(nil, PageSecurity)(func(c echo.Context) error {
			return c.NoContent(http.StatusNoContent)
		})
		if err := handler(c); err != nil {
			c.Echo().DefaultHTTPErrorHandler(err, c)
		}

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})
}

func withClaims(req *http.Request, claims *auth.UserClaims) *http.Request {
	ctx := context.WithValue(req.Context(), auth.UserContextKey, claims)
	return req.WithContext(ctx)
}
