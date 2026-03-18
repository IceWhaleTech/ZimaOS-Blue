package bootstrap

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type stubPreviewModeService struct {
	enabled bool
}

func (s stubPreviewModeService) IsPreviewMode(context.Context) (bool, error) {
	return s.enabled, nil
}

func newCurrentUserRoutesTestHarness(t *testing.T) (*echo.Echo, *user.Handler, *user.Service, *auth.JWTService) {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("new user repo: %v", err)
	}

	service := user.NewService(repo, nil, nil, nil)
	handler := user.NewHandler(service)
	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	middleware := auth.NewAuthMiddleware(jwtSvc, nil)

	e := echo.New()
	v1 := e.Group("/api/v1")
	registerCurrentUserRoutes(v1, middleware, handler)

	return e, handler, service, jwtSvc
}

func mustCreateCurrentUser(t *testing.T, service *user.Service, username, email string) *user.User {
	t.Helper()

	req := &user.CreateUserRequest{
		Username: username,
		Password: "SecurePass123!",
	}
	if email != "" {
		req.Email = &email
	}

	created, err := service.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("create user %s: %v", username, err)
	}
	return created
}

func mustCurrentUserAccessToken(t *testing.T, jwtSvc *auth.JWTService, u *user.User) string {
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

func TestRegisterCurrentUserRoutesOptionalAuthenticateCarriesUserContext(t *testing.T) {
	e, _, service, jwtSvc := newCurrentUserRoutesTestHarness(t)
	created := mustCreateCurrentUser(t, service, "admin", "admin@example.com")
	token := mustCurrentUserAccessToken(t, jwtSvc, created)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got user.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("user id=%s, want %s", got.ID, created.ID)
	}
}

func TestRegisterCurrentUserRoutesPreservesPreviewAccessWithoutToken(t *testing.T) {
	e, handler, _, _ := newCurrentUserRoutesTestHarness(t)
	handler.SetModeService(stubPreviewModeService{enabled: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/me", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got user.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Username != "preview" {
		t.Fatalf("username=%q, want %q", got.Username, "preview")
	}
}

func TestRegisterCurrentUserRoutesAuthenticateCarriesUserContextForUpdate(t *testing.T) {
	e, _, service, jwtSvc := newCurrentUserRoutesTestHarness(t)
	created := mustCreateCurrentUser(t, service, "member", "old@example.com")
	token := mustCurrentUserAccessToken(t, jwtSvc, created)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/me", strings.NewReader(`{"email":"new@example.com"}`))
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200, body=%s", rec.Code, rec.Body.String())
	}

	var got user.User
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Email == nil || *got.Email != "new@example.com" {
		t.Fatalf("email=%v, want new@example.com", got.Email)
	}
}
