package permission

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func newPermissionHandlerTestHarness(t *testing.T) (*echo.Echo, *Handler, *user.Service, *auth.JWTService) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("new user repo: %v", err)
	}
	userSvc := user.NewService(userRepo, nil, nil, nil)

	permRepo, err := NewRepository(db)
	if err != nil {
		t.Fatalf("new permission repo: %v", err)
	}
	permSvc := NewService(permRepo, userRepo)
	handler := NewHandler(permSvc, userRepo)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})

	return echo.New(), handler, userSvc, jwtSvc
}

func mustCreatePermissionUser(t *testing.T, svc *user.Service, username string, role user.Role) *user.User {
	t.Helper()

	created, err := svc.Create(context.Background(), &user.CreateUserRequest{
		Username: username,
		Password: "SecurePass123!",
		Role:     role,
	})
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return created
}

func mustPermissionAccessToken(t *testing.T, jwtSvc *auth.JWTService, u *user.User) string {
	t.Helper()

	token, err := jwtSvc.GenerateAccessToken(&auth.UserClaims{
		UserID:   u.ID.String(),
		Username: u.Username,
		Role:     string(u.Role),
	})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return token
}

func TestRegisterCurrentUserRoutesServesOwnPermissions(t *testing.T) {
	e, handler, userSvc, jwtSvc := newPermissionHandlerTestHarness(t)
	userRecord := mustCreatePermissionUser(t, userSvc, "member", user.RoleUser)
	token := mustPermissionAccessToken(t, jwtSvc, userRecord)

	group := e.Group("/api/v1", auth.NewAuthMiddleware(jwtSvc, nil).Authenticate())
	handler.RegisterCurrentUserRoutes(group)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/permissions", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got PermissionsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.UserID != userRecord.ID {
		t.Fatalf("user id=%s, want %s", got.UserID, userRecord.ID)
	}
	if got.Role != string(userRecord.Role) {
		t.Fatalf("role=%q, want %q", got.Role, userRecord.Role)
	}
	if len(got.Permissions) == 0 {
		t.Fatalf("permissions empty, want defaults")
	}
}

func TestRegisterAdminRoutesKeepsBootstrapPermissionEndpointOut(t *testing.T) {
	e, handler, userSvc, jwtSvc := newPermissionHandlerTestHarness(t)
	admin := mustCreatePermissionUser(t, userSvc, "admin", user.RoleAdmin)
	token := mustPermissionAccessToken(t, jwtSvc, admin)

	group := e.Group("/api/v1", auth.NewAuthMiddleware(jwtSvc, nil).Authenticate())
	handler.RegisterAdminRoutes(group)

	availableReq := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/available", nil)
	availableReq.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	availableRec := httptest.NewRecorder()

	e.ServeHTTP(availableRec, availableReq)
	if availableRec.Code != http.StatusOK {
		t.Fatalf("available status=%d, want 200, body=%s", availableRec.Code, availableRec.Body.String())
	}

	selfReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/permissions", nil)
	selfReq.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	selfRec := httptest.NewRecorder()

	e.ServeHTTP(selfRec, selfReq)
	if selfRec.Code != http.StatusBadRequest {
		t.Fatalf("self permissions status=%d, want 400, body=%s", selfRec.Code, selfRec.Body.String())
	}
}
