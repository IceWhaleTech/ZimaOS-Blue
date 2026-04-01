package bootstrap

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func TestBindRouteRuntimeAuthSurface_ProvidesProtectedGroupsAndPermissionRoutes(t *testing.T) {
	tmp := t.TempDir()
	db, err := sql.Open("sqlite3", filepath.Join(tmp, "runtime-auth-surface.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("new user repo: %v", err)
	}
	userService := user.NewService(userRepo, nil, nil, nil)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	authMiddleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	authSurface := bindRouteRuntimeAuthSurface(routeRuntimeContractAuthSurfaceOptions{
		v1:             v1,
		api:            api,
		writeDB:        db,
		readDB:         db,
		authMiddleware: authMiddleware,
		userRepo:       userRepo,
		logger:         zap.NewNop(),
	})
	if authSurface.protected == nil || authSurface.apiProtected == nil {
		t.Fatalf("expected protected groups, got %#v", authSurface)
	}
	if authSurface.authPageV1Group == nil || authSurface.authPageAPIGroup == nil {
		t.Fatalf("expected auth page group resolvers, got %#v", authSurface)
	}
	if authSurface.requirePagePermission == nil || authSurface.authMiddleware == nil || authSurface.permissionHandler == nil {
		t.Fatalf("expected auth surface middleware and permission handler, got %#v", authSurface)
	}

	authSurface.protected.GET("/runtime-protected", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	authSurface.apiProtected.GET("/runtime-compat", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	authSurface.authPageV1Group(permission.PageProfile).GET("/runtime-profile", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	authSurface.permissionHandler.RegisterCurrentUserRoutes(v1.Group("", authSurface.authMiddleware))

	created := mustCreateCurrentUser(t, userService, "member", "member@example.com")
	permRepo, err := permission.NewRepository(db)
	if err != nil {
		t.Fatalf("new permission repo: %v", err)
	}
	if err := permRepo.SetUserPermissions(context.Background(), created.ID, []string{permission.PageProfile}, nil); err != nil {
		t.Fatalf("seed user permissions: %v", err)
	}
	token := mustCurrentUserAccessToken(t, jwtSvc, created)

	assertRouteStatus := func(path, token string, want int) {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		if token != "" {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != want {
			t.Fatalf("GET %s status=%d, want %d, body=%s", path, rec.Code, want, rec.Body.String())
		}
	}

	assertRouteStatus("/api/v1/runtime-protected", "", http.StatusUnauthorized)
	assertRouteStatus("/api/v1/runtime-protected", token, http.StatusNoContent)
	assertRouteStatus("/api/runtime-compat", token, http.StatusNoContent)
	assertRouteStatus("/api/v1/runtime-profile", token, http.StatusNoContent)
	assertRouteStatus("/api/v1/users/me/permissions", token, http.StatusOK)
}

func TestBindRouteRuntimeAuthSurface_UsesReaderDBForPermissionRoutes(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "runtime-auth-surface-reader.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open sqlite writer: %v", err)
	}

	bootstrapUserRepo, err := user.NewSQLiteRepository(writeDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("new bootstrap user repo: %v", err)
	}
	bootstrapUserService := user.NewService(bootstrapUserRepo, nil, nil, nil)

	created := mustCreateCurrentUser(t, bootstrapUserService, "member", "member@example.com")
	bootstrapPermRepo, err := permission.NewRepository(writeDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("new bootstrap permission repo: %v", err)
	}
	if err := bootstrapPermRepo.SetUserPermissions(context.Background(), created.ID, []string{permission.PageProfile}, nil); err != nil {
		_ = writeDB.Close()
		t.Fatalf("seed user permissions: %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("open sqlite reader: %v", err)
	}
	defer readDB.Close()

	userRepo, err := user.NewSQLiteRepositoryWithReadDB(writeDB, readDB)
	if err != nil {
		_ = writeDB.Close()
		t.Fatalf("new split user repo: %v", err)
	}

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	token := mustCurrentUserAccessToken(t, jwtSvc, created)
	authMiddleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	api := e.Group("/api")

	authSurface := bindRouteRuntimeAuthSurface(routeRuntimeContractAuthSurfaceOptions{
		v1:             v1,
		api:            api,
		writeDB:        writeDB,
		readDB:         readDB,
		authMiddleware: authMiddleware,
		userRepo:       userRepo,
		logger:         zap.NewNop(),
	})
	authSurface.authPageV1Group(permission.PageProfile).GET("/runtime-profile", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})
	authSurface.permissionHandler.RegisterCurrentUserRoutes(v1.Group("", authSurface.authMiddleware))

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	assertRouteStatus := func(path string, want int) {
		t.Helper()

		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != want {
			t.Fatalf("GET %s status=%d, want %d, body=%s", path, rec.Code, want, rec.Body.String())
		}
	}

	assertRouteStatus("/api/v1/runtime-profile", http.StatusNoContent)
	assertRouteStatus("/api/v1/users/me/permissions", http.StatusOK)
}
