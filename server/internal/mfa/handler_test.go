package mfa

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func setupTestHandler() (*Handler, *echo.Echo) {
	totp := NewTOTP(nil)
	recovery := NewRecovery(nil)
	handler := NewHandler(totp, recovery)
	e := echo.New()
	return handler, e
}

func TestNewHandler(t *testing.T) {
	handler := NewHandler(nil, nil)
	if handler.totp == nil {
		t.Error("NewHandler() should create default TOTP")
	}
	if handler.recovery == nil {
		t.Error("NewHandler() should create default Recovery")
	}
}

func TestHandler_Setup(t *testing.T) {
	handler, e := setupTestHandler()

	reqBody := `{"include_qr_code":true}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/setup", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())
	c.Set("username", "testuser")

	err := handler.Setup(c)
	if err != nil {
		t.Fatalf("Setup() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Setup() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp SetupResponseDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp.Secret == "" {
		t.Error("Setup() secret is empty")
	}
	if resp.URI == "" {
		t.Error("Setup() URI is empty")
	}
	if resp.QRCode == "" {
		t.Error("Setup() QR code is empty when requested")
	}
}

func TestHandler_Setup_Unauthorized(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/setup", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// No user_id in context

	err := handler.Setup(c)
	if err == nil {
		t.Fatal("Setup() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("Setup() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusUnauthorized {
		t.Errorf("Setup() status = %d, want %d", httpErr.Code, http.StatusUnauthorized)
	}
}

func TestHandler_Verify(t *testing.T) {
	handler, e := setupTestHandler()

	// First, generate a secret
	key, _ := handler.totp.GenerateKey("testuser")
	secret := key.Secret()

	// Generate a valid code
	code, _ := handler.totp.GenerateCode(secret)

	reqBody := `{"code":"` + code + `","secret":"` + secret + `"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.Verify(c)
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Verify() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp VerifyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if !resp.Enabled {
		t.Error("Verify() enabled should be true")
	}
	if len(resp.RecoveryCodes) == 0 {
		t.Error("Verify() should return recovery codes")
	}
}

func TestHandler_Verify_InvalidCode(t *testing.T) {
	handler, e := setupTestHandler()

	reqBody := `{"code":"000000","secret":"JBSWY3DPEHPK3PXP"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/verify", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.Verify(c)
	if err == nil {
		t.Fatal("Verify() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("Verify() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Errorf("Verify() status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestHandler_Disable(t *testing.T) {
	handler, e := setupTestHandler()

	reqBody := `{"password":"testpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/disable", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.Disable(c)
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Disable() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Status(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/mfa/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.Status(c)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Status() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp StatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}
}

func TestHandler_GetRecoveryCodes(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodGet, "/auth/mfa/recovery", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.GetRecoveryCodes(c)
	if err != nil {
		t.Fatalf("GetRecoveryCodes() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetRecoveryCodes() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_RegenerateRecoveryCodes(t *testing.T) {
	handler, e := setupTestHandler()

	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/recovery/regenerate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", uuid.New())

	err := handler.RegenerateRecoveryCodes(c)
	if err != nil {
		t.Fatalf("RegenerateRecoveryCodes() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("RegenerateRecoveryCodes() status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp RecoveryCodesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(resp.Codes) == 0 {
		t.Error("RegenerateRecoveryCodes() should return codes")
	}
}

func TestHandler_ValidateMFA(t *testing.T) {
	handler, e := setupTestHandler()

	reqBody := `{"mfa_token":"test_token","code":"180"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/validate", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ValidateMFA(c)
	if err != nil {
		t.Fatalf("ValidateMFA() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("ValidateMFA() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_ValidateMFA_MissingCode(t *testing.T) {
	handler, e := setupTestHandler()

	reqBody := `{"mfa_token":"test_token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/mfa/validate", strings.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.ValidateMFA(c)
	if err == nil {
		t.Fatal("ValidateMFA() expected error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("ValidateMFA() error type = %T, want *echo.HTTPError", err)
	}
	if httpErr.Code != http.StatusBadRequest {
		t.Errorf("ValidateMFA() status = %d, want %d", httpErr.Code, http.StatusBadRequest)
	}
}

func TestGetUserIDFromContext(t *testing.T) {
	e := echo.New()

	// Test with uuid.UUID
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	id := uuid.New()
	c.Set("user_id", id)

	got := getUserIDFromContext(c)
	if got != id {
		t.Errorf("getUserIDFromContext() = %v, want %v", got, id)
	}

	// Test with string
	c2 := e.NewContext(req, rec)
	c2.Set("user_id", id.String())

	got = getUserIDFromContext(c2)
	if got != id {
		t.Errorf("getUserIDFromContext() with string = %v, want %v", got, id)
	}

	// Test with no user_id
	c3 := e.NewContext(req, rec)
	got = getUserIDFromContext(c3)
	if got != uuid.Nil {
		t.Errorf("getUserIDFromContext() with no user_id = %v, want %v", got, uuid.Nil)
	}
}

func TestGetUsernameFromContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	c.Set("username", "testuser")

	got := getUsernameFromContext(c)
	if got != "testuser" {
		t.Errorf("getUsernameFromContext() = %v, want testuser", got)
	}

	// Test with no username
	c2 := e.NewContext(req, rec)
	got = getUsernameFromContext(c2)
	if got != "" {
		t.Errorf("getUsernameFromContext() with no username = %v, want empty", got)
	}
}
