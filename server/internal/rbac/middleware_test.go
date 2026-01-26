package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRBACMiddleware_RequirePermission(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"chat", "skills.execute"})
	rbac.DefineRole("readonly", []string{"chat.read"})

	middleware := NewMiddleware(rbac)

	e := echo.New()
	e.GET("/chat", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, middleware.RequirePermission("chat"))

	t.Run("admin can access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "admin")

		handler := middleware.RequirePermission("chat")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("admin should have access: %v", err)
		}
	})

	t.Run("user can access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "user")

		handler := middleware.RequirePermission("chat")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("user should have access: %v", err)
		}
	})

	t.Run("readonly cannot access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "readonly")

		handler := middleware.RequirePermission("chat")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err == nil {
			t.Error("readonly should not have access")
		}
		httpErr, ok := err.(*echo.HTTPError)
		if !ok || httpErr.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %v", err)
		}
	})

	t.Run("no role cannot access", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/chat", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := middleware.RequirePermission("chat")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err == nil {
			t.Error("no role should not have access")
		}
	})
}

func TestRBACMiddleware_RequireAnyPermission(t *testing.T) {
	rbac := New()
	rbac.DefineRole("user", []string{"chat"})
	rbac.DefineRole("skills_user", []string{"skills.execute"})

	middleware := NewMiddleware(rbac)
	e := echo.New()

	t.Run("user with chat permission", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "user")

		handler := middleware.RequireAnyPermission("chat", "skills.execute")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("user should have access: %v", err)
		}
	})

	t.Run("skills_user with skills permission", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "skills_user")

		handler := middleware.RequireAnyPermission("chat", "skills.execute")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("skills_user should have access: %v", err)
		}
	})
}

func TestRBACMiddleware_RequireAllPermissions(t *testing.T) {
	rbac := New()
	rbac.DefineRole("full_user", []string{"chat", "skills.execute"})
	rbac.DefineRole("partial_user", []string{"chat"})

	middleware := NewMiddleware(rbac)
	e := echo.New()

	t.Run("full_user has all permissions", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "full_user")

		handler := middleware.RequireAllPermissions("chat", "skills.execute")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("full_user should have access: %v", err)
		}
	})

	t.Run("partial_user missing permission", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "partial_user")

		handler := middleware.RequireAllPermissions("chat", "skills.execute")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err == nil {
			t.Error("partial_user should not have access")
		}
	})
}

func TestRBACMiddleware_RequireRole(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("user", []string{"chat"})

	middleware := NewMiddleware(rbac)
	e := echo.New()

	t.Run("correct role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "admin")

		handler := middleware.RequireRole("admin")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("admin should have access: %v", err)
		}
	})

	t.Run("wrong role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "user")

		handler := middleware.RequireRole("admin")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err == nil {
			t.Error("user should not have access to admin-only route")
		}
	})
}

func TestRBACMiddleware_RequireAnyRole(t *testing.T) {
	rbac := New()
	rbac.DefineRole("admin", []string{"*"})
	rbac.DefineRole("moderator", []string{"chat.*"})
	rbac.DefineRole("user", []string{"chat"})

	middleware := NewMiddleware(rbac)
	e := echo.New()

	t.Run("admin in allowed roles", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "admin")

		handler := middleware.RequireAnyRole("admin", "moderator")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err != nil {
			t.Errorf("admin should have access: %v", err)
		}
	})

	t.Run("user not in allowed roles", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		SetRoleInContext(c, "user")

		handler := middleware.RequireAnyRole("admin", "moderator")(func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})

		err := handler(c)
		if err == nil {
			t.Error("user should not have access")
		}
	})
}
