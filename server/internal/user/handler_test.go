package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestHandler(t *testing.T) (*Handler, *echo.Echo, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	repo, _ := NewSQLiteRepository(db)
	service := NewService(repo, nil, nil, nil)
	handler := NewHandler(service)

	e := echo.New()

	cleanup := func() {
		db.Close()
	}

	return handler, e, cleanup
}

func TestHandler_Login(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	_, _ = handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Test login
	reqBody := `{"username":"testuser","password":"SecurePass123!"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Login() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.AccessToken == "" {
		t.Error("Login() access_token is empty")
	}
	if resp.RefreshToken == "" {
		t.Error("Login() refresh_token is empty")
	}
	if resp.User == nil {
		t.Error("Login() user is nil")
	}
}

func TestHandler_Login_InvalidCredentials(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	_, _ = handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Test login with wrong password
	reqBody := `{"username":"testuser","password":"WrongPassword!"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Login(c)
	if err == nil {
		t.Fatal("Login() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("Login() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("Login() status = %d, want %d", httpErr.Code, http.StatusUnauthorized)
	}
}

func TestHandler_CreateUser(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	reqBody := `{"username":"newuser","password":"SecurePass123!","email":"new@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateUser(c)
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("CreateUser() status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var user User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if user.Username != "newuser" {
		t.Errorf("CreateUser() username = %v, want newuser", user.Username)
	}
}

func TestHandler_CreateUser_WeakPassword(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	reqBody := `{"username":"newuser","password":"weak"}`
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.CreateUser(c)
	if err == nil {
		t.Fatal("CreateUser() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("CreateUser() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Errorf("CreateUser() status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestHandler_GetUser(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	user, _ := handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	req := httptest.NewRequest(http.MethodGet, "/users/"+user.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	err := handler.GetUser(c)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetUser() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_GetUser_NotFound(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/users/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	err := handler.GetUser(c)
	if err == nil {
		t.Fatal("GetUser() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("GetUser() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusNotFound {
		t.Errorf("GetUser() status = %d, want %d", httpErr.Code, http.StatusNotFound)
	}
}

func TestHandler_UpdateUser(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	user, _ := handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	reqBody := `{"email":"updated@example.com"}`
	req := httptest.NewRequest(http.MethodPut, "/users/"+user.ID.String(), strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	err := handler.UpdateUser(c)
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("UpdateUser() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var updated User
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if *updated.Email != "updated@example.com" {
		t.Errorf("UpdateUser() email = %v, want updated@example.com", *updated.Email)
	}
}

func TestHandler_DeleteUser(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	user, _ := handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	req := httptest.NewRequest(http.MethodDelete, "/users/"+user.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	err := handler.DeleteUser(c)
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("DeleteUser() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHandler_ListUsers(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create some users
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _ = handler.service.Create(ctx, &CreateUserRequest{
			Username: "testuser" + string(rune('a'+i)),
			Password: "SecurePass123!",
		})
	}

	req := httptest.NewRequest(http.MethodGet, "/users?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ListUsers(c)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("ListUsers() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var result ListUsersResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(result.Users) != 5 {
		t.Errorf("ListUsers() returned %d users, want 5", len(result.Users))
	}
}

func TestHandler_LockUnlockUser(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	user, _ := handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	// Lock user
	req := httptest.NewRequest(http.MethodPost, "/users/"+user.ID.String()+"/lock", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	err := handler.LockUser(c)
	if err != nil {
		t.Fatalf("LockUser() error = %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("LockUser() status = %d, want %d", rec.Code, http.StatusNoContent)
	}

	// Unlock user
	req = httptest.NewRequest(http.MethodPost, "/users/"+user.ID.String()+"/unlock", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(user.ID.String())

	err = handler.UnlockUser(c)
	if err != nil {
		t.Fatalf("UnlockUser() error = %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("UnlockUser() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHandler_ChangePassword(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	// Create a user first
	ctx := context.Background()
	user, _ := handler.service.Create(ctx, &CreateUserRequest{
		Username: "testuser",
		Password: "SecurePass123!",
	})

	reqBody := `{"current_password":"SecurePass123!","new_password":"NewSecurePass456!"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", user.ID)

	err := handler.ChangePassword(c)
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("ChangePassword() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestHandler_ChangePassword_Unauthorized(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	reqBody := `{"current_password":"SecurePass123!","new_password":"NewSecurePass456!"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/password", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No user_id in context

	err := handler.ChangePassword(c)
	if err == nil {
		t.Fatal("ChangePassword() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("ChangePassword() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("ChangePassword() status = %d, want %d", httpErr.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Logout(t *testing.T) {
	handler, e, cleanup := setupTestHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Logout(c)
	if err != nil {
		t.Fatalf("Logout() error = %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Errorf("Logout() status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
