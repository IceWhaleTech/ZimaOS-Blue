package preview

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestHandler(t *testing.T) (*Handler, *user.Service, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	repo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}

	hasher := password.NewHasher(nil)
	policy := password.NewPolicy(nil)
	userService := user.NewService(repo, hasher, policy, nil)

	modeService := NewModeService(userService)
	upgradeService := NewUpgradeService(userService, db)

	// Create JWT service for testing
	jwtService := auth.NewJWTService(&auth.JWTConfig{
		Secret:            "test-secret-key-for-testing-only",
		Expiration:        24 * time.Hour,
		RefreshExpiration: 7 * 24 * time.Hour,
		Issuer:            "echo-test",
	})

	handler := NewHandler(modeService, upgradeService, jwtService, userService)

	cleanup := func() {
		db.Close()
	}

	return handler, userService, cleanup
}

func TestHandler_GetSystemMode_Preview(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetSystemMode(c)
	if err != nil {
		t.Fatalf("GetSystemMode() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetSystemMode() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp SystemMode
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Mode != "preview" {
		t.Errorf("GetSystemMode() mode = %v, want preview", resp.Mode)
	}
}

func TestHandler_GetSystemMode_Normal(t *testing.T) {
	handler, userService, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create admin
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "admin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/mode", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.GetSystemMode(c)
	if err != nil {
		t.Fatalf("GetSystemMode() error = %v", err)
	}

	var resp SystemMode
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Mode != "normal" {
		t.Errorf("GetSystemMode() mode = %v, want normal", resp.Mode)
	}
}

func TestHandler_GetPreviewStatus_Active(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/preview/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetPreviewStatus(c)
	if err != nil {
		t.Fatalf("GetPreviewStatus() error = %v", err)
	}

	var resp PreviewStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Active {
		t.Error("GetPreviewStatus() Active = false, want true")
	}
}

func TestHandler_Upgrade_Success(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	body := `{"username":"admin","password":"SecurePass123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/upgrade", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Upgrade(c)
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Upgrade() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp UpgradeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !resp.Success {
		t.Error("Upgrade() Success = false, want true")
	}
	if resp.User.Username != "admin" {
		t.Errorf("Upgrade() User.Username = %v, want admin", resp.User.Username)
	}
}

func TestHandler_Upgrade_EmptyUsername(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	body := `{"username":"","password":"SecurePass123!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/upgrade", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Upgrade(c)
	if err == nil {
		t.Error("Upgrade() expected error, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Errorf("Upgrade() status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestHandler_Upgrade_AdminAlreadyExists(t *testing.T) {
	handler, userService, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create admin first
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "existingadmin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	e := echo.New()
	body := `{"username":"newadmin","password":"SecurePass456!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/upgrade", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.Upgrade(c)
	if err == nil {
		t.Error("Upgrade() expected error, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusConflict {
		t.Errorf("Upgrade() status = %d, want %d", httpErr.Code, http.StatusConflict)
	}
}

func TestHandler_GetPreviewToken_Success(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/token", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetPreviewToken(c)
	if err != nil {
		t.Fatalf("GetPreviewToken() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetPreviewToken() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp PreviewTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if resp.Token == "" {
		t.Error("GetPreviewToken() Token is empty")
	}
	if resp.ExpiresIn != 86400 {
		t.Errorf("GetPreviewToken() ExpiresIn = %d, want 86400", resp.ExpiresIn)
	}
}

func TestHandler_GetPreviewToken_ForbiddenWhenUsersExist(t *testing.T) {
	handler, userService, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	// Create admin - this should disable preview mode
	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "admin",
		Password: "SecurePass123!",
		Role:     user.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("failed to create admin: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/token", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.GetPreviewToken(c)
	if err == nil {
		t.Error("GetPreviewToken() expected error when users exist, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusForbidden {
		t.Errorf("GetPreviewToken() status = %d, want %d (Forbidden)", httpErr.Code, http.StatusForbidden)
	}
}

func TestHandler_GetPreviewToken_ForbiddenWhenRegularUserExists(t *testing.T) {
	handler, userService, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "member",
		Password: "SecurePass123!",
		Role:     user.RoleUser,
	})
	if err != nil {
		t.Fatalf("failed to create regular user: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/token", nil)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.GetPreviewToken(c)
	if err == nil {
		t.Error("GetPreviewToken() expected error when a regular user exists, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusForbidden {
		t.Errorf("GetPreviewToken() status = %d, want %d (Forbidden)", httpErr.Code, http.StatusForbidden)
	}
}

func TestHandler_Upgrade_ConflictWhenRegularUserExists(t *testing.T) {
	handler, userService, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()

	_, err := userService.Create(ctx, &user.CreateUserRequest{
		Username: "member",
		Password: "SecurePass123!",
		Role:     user.RoleUser,
	})
	if err != nil {
		t.Fatalf("failed to create regular user: %v", err)
	}

	e := echo.New()
	body := `{"username":"admin","password":"SecurePass456!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/preview/upgrade", bytes.NewBufferString(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = handler.Upgrade(c)
	if err == nil {
		t.Error("Upgrade() expected error when a regular user exists, got nil")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if httpErr.Code != http.StatusConflict {
		t.Errorf("Upgrade() status = %d, want %d", httpErr.Code, http.StatusConflict)
	}
}
