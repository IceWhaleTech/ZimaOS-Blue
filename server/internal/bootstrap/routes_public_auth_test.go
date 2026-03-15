package bootstrap

import (
	"database/sql"
	"slices"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func TestRegisterPublicAuthRoutes(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	repo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("new user repo: %v", err)
	}

	handler := user.NewHandler(user.NewService(repo, nil, nil, nil))
	e := echo.New()
	v1 := e.Group("/api/v1")

	registerPublicAuthRoutes(v1, handler)

	type routeKey struct {
		method string
		path   string
	}

	routes := make([]routeKey, 0, len(e.Routes()))
	for _, route := range e.Routes() {
		routes = append(routes, routeKey{method: route.Method, path: route.Path})
	}

	expected := []routeKey{
		{method: "POST", path: "/api/v1/auth/login"},
		{method: "POST", path: "/api/v1/auth/logout"},
		{method: "POST", path: "/api/v1/auth/refresh"},
		{method: "GET", path: "/api/v1/auth/password-policy"},
	}

	for _, route := range expected {
		if !slices.Contains(routes, route) {
			t.Fatalf("expected route %s %s to be registered; got routes=%v", route.method, route.path, routes)
		}
	}
}
