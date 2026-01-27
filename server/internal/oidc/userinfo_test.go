package oidc

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// mockUserInfoProvider implements UserInfoProvider for testing.
type mockUserInfoProvider struct {
	users map[string]*UserInfoClaims
}

func newMockUserInfoProvider() *mockUserInfoProvider {
	return &mockUserInfoProvider{
		users: map[string]*UserInfoClaims{
			"user-123": {
				Sub:               "user-123",
				Name:              "Test User",
				PreferredUsername: "testuser",
				Email:             "test@example.com",
				EmailVerified:     true,
			},
		},
	}
}

func (m *mockUserInfoProvider) GetUserInfo(userID string) (*UserInfoClaims, error) {
	if user, ok := m.users[userID]; ok {
		return user, nil
	}
	return nil, ErrUserNotFound
}

func setupTestUserInfoHandler(t *testing.T) (*UserInfoHandler, *TokenService, func()) {
	tmpDir, err := os.MkdirTemp("", "oidc-userinfo-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	keyPath := filepath.Join(tmpDir, "test.key")
	km, err := NewKeyManager(keyPath, 90)
	if err != nil {
		t.Fatalf("NewKeyManager() error = %v", err)
	}

	ts := NewTokenService(km, "https://example.com", time.Hour, 30*24*time.Hour, time.Hour)
	provider := newMockUserInfoProvider()
	handler := NewUserInfoHandler(ts, provider)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return handler, ts, cleanup
}

func TestNewUserInfoHandler(t *testing.T) {
	handler, _, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	if handler == nil {
		t.Fatal("NewUserInfoHandler() returned nil")
	}
}

func TestUserInfoHandler_ServeHTTP(t *testing.T) {
	handler, ts, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	// Generate a valid token
	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeProfile, ScopeEmail},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+response.AccessToken)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ServeHTTP() status = %d, want %d", w.Code, http.StatusOK)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("ServeHTTP() Content-Type = %v, want application/json", contentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "user-123") {
		t.Error("ServeHTTP() response should contain user ID")
	}
}

func TestUserInfoHandler_ServeHTTP_POST(t *testing.T) {
	handler, ts, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	authCode := &AuthorizationCode{
		Code:        "test-code",
		ClientID:    "test-client",
		UserID:      "user-123",
		Username:    "testuser",
		Scope:       []string{ScopeOpenID, ScopeProfile},
		RedirectURI: "http://localhost:3000/callback",
		ExpiresAt:   time.Now().Add(10 * time.Minute),
	}

	response, _ := ts.GenerateTokens(authCode)

	req := httptest.NewRequest(http.MethodPost, "/oauth/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+response.AccessToken)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("ServeHTTP(POST) status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUserInfoHandler_ServeHTTP_NoToken(t *testing.T) {
	handler, _, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ServeHTTP(no token) status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	wwwAuth := w.Header().Get("WWW-Authenticate")
	if wwwAuth != "Bearer" {
		t.Errorf("ServeHTTP(no token) WWW-Authenticate = %v, want Bearer", wwwAuth)
	}
}

func TestUserInfoHandler_ServeHTTP_InvalidToken(t *testing.T) {
	handler, _, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/oauth/userinfo", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("ServeHTTP(invalid token) status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	wwwAuth := w.Header().Get("WWW-Authenticate")
	if !strings.Contains(wwwAuth, "invalid_token") {
		t.Errorf("ServeHTTP(invalid token) WWW-Authenticate = %v, should contain invalid_token", wwwAuth)
	}
}

func TestUserInfoHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	handler, _, cleanup := setupTestUserInfoHandler(t)
	defer cleanup()

	methods := []string{http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/oauth/userinfo", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("ServeHTTP(%s) status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name: "from Authorization header",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", "Bearer test-token")
			},
			expected: "test-token",
		},
		{
			name: "from query parameter",
			setup: func(r *http.Request) {
				r.URL.RawQuery = "access_token=query-token"
			},
			expected: "query-token",
		},
		{
			name:     "no token",
			setup:    func(r *http.Request) {},
			expected: "",
		},
		{
			name: "wrong auth scheme",
			setup: func(r *http.Request) {
				r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setup(req)

			result := extractBearerToken(req)
			if result != tt.expected {
				t.Errorf("extractBearerToken() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFilterClaimsByScope(t *testing.T) {
	userInfo := &UserInfoClaims{
		Sub:               "user-123",
		Name:              "Test User",
		PreferredUsername: "testuser",
		Email:             "test@example.com",
		EmailVerified:     true,
	}

	tests := []struct {
		name        string
		scope       string
		expectName  bool
		expectEmail bool
	}{
		{
			name:        "openid only",
			scope:       "openid",
			expectName:  false,
			expectEmail: false,
		},
		{
			name:        "openid profile",
			scope:       "openid profile",
			expectName:  true,
			expectEmail: false,
		},
		{
			name:        "openid email",
			scope:       "openid email",
			expectName:  false,
			expectEmail: true,
		},
		{
			name:        "openid profile email",
			scope:       "openid profile email",
			expectName:  true,
			expectEmail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterClaimsByScope(userInfo, tt.scope)

			// Sub should always be present
			if result["sub"] != "user-123" {
				t.Error("filterClaimsByScope() should always include sub")
			}

			_, hasName := result["name"]
			if hasName != tt.expectName {
				t.Errorf("filterClaimsByScope() name present = %v, want %v", hasName, tt.expectName)
			}

			_, hasEmail := result["email"]
			if hasEmail != tt.expectEmail {
				t.Errorf("filterClaimsByScope() email present = %v, want %v", hasEmail, tt.expectEmail)
			}
		})
	}
}
