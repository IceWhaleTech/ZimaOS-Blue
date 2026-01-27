package audit

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestMiddleware(t *testing.T) {
	// Create a mock logger
	logger := &mockLogger{entries: make([]*Entry, 0)}

	config := &MiddlewareConfig{
		Logger:          logger,
		SkipPaths:       []string{"/health"},
		LogRequestBody:  false,
		LogResponseBody: false,
		MaxBodySize:     4096,
		GetUserID:       func(c echo.Context) *uuid.UUID { return nil },
		GetUsername:     func(c echo.Context) string { return "" },
	}

	e := echo.New()
	e.Use(Middleware(config))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Middleware() status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Wait for async logging
	time.Sleep(50 * time.Millisecond)

	if len(logger.entries) == 0 {
		t.Error("Middleware() should log request")
	}
}

func TestMiddleware_SkipPaths(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}

	config := &MiddlewareConfig{
		Logger:    logger,
		SkipPaths: []string{"/health", "/metrics"},
		GetUserID: func(c echo.Context) *uuid.UUID { return nil },
		GetUsername: func(c echo.Context) string { return "" },
	}

	e := echo.New()
	e.Use(Middleware(config))
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
	e.GET("/metrics", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Request to skipped path
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	time.Sleep(50 * time.Millisecond)

	if len(logger.entries) != 0 {
		t.Error("Middleware() should skip /health path")
	}
}

func TestMiddleware_WithUser(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	userID := uuid.New()

	config := &MiddlewareConfig{
		Logger:    logger,
		SkipPaths: []string{},
		GetUserID: func(c echo.Context) *uuid.UUID {
			return &userID
		},
		GetUsername: func(c echo.Context) string {
			return "testuser"
		},
	}

	e := echo.New()
	e.Use(Middleware(config))
	e.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	time.Sleep(50 * time.Millisecond)

	if len(logger.entries) == 0 {
		t.Fatal("Middleware() should log request")
	}

	entry := logger.entries[0]
	if entry.UserID == nil || *entry.UserID != userID {
		t.Error("Middleware() should include user ID")
	}
	if entry.Username != "testuser" {
		t.Errorf("Middleware() Username = %v, want testuser", entry.Username)
	}
}

func TestMiddleware_LogRequestBody(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}

	config := &MiddlewareConfig{
		Logger:         logger,
		SkipPaths:      []string{},
		LogRequestBody: true,
		MaxBodySize:    4096,
		GetUserID:      func(c echo.Context) *uuid.UUID { return nil },
		GetUsername:    func(c echo.Context) string { return "" },
	}

	e := echo.New()
	e.Use(Middleware(config))
	e.POST("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	body := bytes.NewBufferString(`{"test": "data"}`)
	req := httptest.NewRequest(http.MethodPost, "/test", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	time.Sleep(50 * time.Millisecond)

	if len(logger.entries) == 0 {
		t.Fatal("Middleware() should log request")
	}

	entry := logger.entries[0]
	if entry.Details == nil {
		t.Error("Middleware() should include details with request body")
	}
}

func TestMiddleware_FailureStatus(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}

	config := &MiddlewareConfig{
		Logger:      logger,
		SkipPaths:   []string{},
		GetUserID:   func(c echo.Context) *uuid.UUID { return nil },
		GetUsername: func(c echo.Context) string { return "" },
	}

	e := echo.New()
	e.Use(Middleware(config))
	e.GET("/error", func(c echo.Context) error {
		return c.String(http.StatusInternalServerError, "Error")
	})

	req := httptest.NewRequest(http.MethodGet, "/error", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	time.Sleep(50 * time.Millisecond)

	if len(logger.entries) == 0 {
		t.Fatal("Middleware() should log request")
	}

	entry := logger.entries[0]
	if entry.Status != StatusFailure {
		t.Errorf("Middleware() Status = %v, want %v", entry.Status, StatusFailure)
	}
}

func TestDefaultMiddlewareConfig(t *testing.T) {
	config := DefaultMiddlewareConfig()

	if config == nil {
		t.Fatal("DefaultMiddlewareConfig() returned nil")
	}

	if len(config.SkipPaths) == 0 {
		t.Error("DefaultMiddlewareConfig() SkipPaths should not be empty")
	}

	if config.MaxBodySize != 4096 {
		t.Errorf("DefaultMiddlewareConfig() MaxBodySize = %d, want 4096", config.MaxBodySize)
	}
}

func TestMethodToAction(t *testing.T) {
	tests := []struct {
		method   string
		expected Action
	}{
		{http.MethodGet, ActionAPIAccess},
		{http.MethodPost, ActionUserCreate},
		{http.MethodPut, ActionUserUpdate},
		{http.MethodPatch, ActionUserUpdate},
		{http.MethodDelete, ActionUserDelete},
		{"UNKNOWN", ActionAPIAccess},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			result := methodToAction(tt.method)
			if result != tt.expected {
				t.Errorf("methodToAction(%s) = %v, want %v", tt.method, result, tt.expected)
			}
		})
	}
}

func TestResponseWriter_Write(t *testing.T) {
	body := new(bytes.Buffer)
	w := httptest.NewRecorder()

	rw := &responseWriter{
		ResponseWriter: w,
		body:           body,
		logBody:        true,
		maxSize:        100,
	}

	data := []byte("test response data")
	n, err := rw.Write(data)

	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if n != len(data) {
		t.Errorf("Write() n = %d, want %d", n, len(data))
	}

	if body.String() != string(data) {
		t.Errorf("Write() body = %v, want %v", body.String(), string(data))
	}
}

func TestResponseWriter_Write_MaxSize(t *testing.T) {
	body := new(bytes.Buffer)
	w := httptest.NewRecorder()

	rw := &responseWriter{
		ResponseWriter: w,
		body:           body,
		logBody:        true,
		maxSize:        10,
	}

	data := []byte("this is a very long response that exceeds max size")
	rw.Write(data)

	if body.Len() > 10 {
		t.Errorf("Write() body length = %d, should be <= 10", body.Len())
	}
}

func TestResponseWriter_Write_LogBodyDisabled(t *testing.T) {
	body := new(bytes.Buffer)
	w := httptest.NewRecorder()

	rw := &responseWriter{
		ResponseWriter: w,
		body:           body,
		logBody:        false,
		maxSize:        100,
	}

	data := []byte("test response data")
	rw.Write(data)

	if body.Len() != 0 {
		t.Error("Write() should not capture body when logBody is false")
	}
}

func TestAuditAction_LogLogin(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("User-Agent", "TestAgent")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogLogin(c, userID, "testuser", true)

	if len(logger.entries) != 1 {
		t.Fatal("LogLogin() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionLogin {
		t.Errorf("LogLogin() Action = %v, want %v", entry.Action, ActionLogin)
	}
	if entry.Status != StatusSuccess {
		t.Errorf("LogLogin() Status = %v, want %v", entry.Status, StatusSuccess)
	}
}

func TestAuditAction_LogLoginFailed(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogLogin(c, userID, "testuser", false)

	if len(logger.entries) != 1 {
		t.Fatal("LogLogin(failed) should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionLoginFailed {
		t.Errorf("LogLogin(failed) Action = %v, want %v", entry.Action, ActionLoginFailed)
	}
	if entry.Status != StatusFailure {
		t.Errorf("LogLogin(failed) Status = %v, want %v", entry.Status, StatusFailure)
	}
}

func TestAuditAction_LogLogout(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogLogout(c, userID, "testuser")

	if len(logger.entries) != 1 {
		t.Fatal("LogLogout() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionLogout {
		t.Errorf("LogLogout() Action = %v, want %v", entry.Action, ActionLogout)
	}
}

func TestAuditAction_LogPasswordChange(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/password", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogPasswordChange(c, userID, "testuser", true)

	if len(logger.entries) != 1 {
		t.Fatal("LogPasswordChange() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionPasswordChange {
		t.Errorf("LogPasswordChange() Action = %v, want %v", entry.Action, ActionPasswordChange)
	}
}

func TestAuditAction_LogMFASetup(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mfa/setup", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogMFASetup(c, userID, "testuser", true)

	if len(logger.entries) != 1 {
		t.Fatal("LogMFASetup() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionMFASetup {
		t.Errorf("LogMFASetup() Action = %v, want %v", entry.Action, ActionMFASetup)
	}
}

func TestAuditAction_LogMFADisable(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mfa/disable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogMFADisable(c, userID, "testuser")

	if len(logger.entries) != 1 {
		t.Fatal("LogMFADisable() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionMFADisable {
		t.Errorf("LogMFADisable() Action = %v, want %v", entry.Action, ActionMFADisable)
	}
}

func TestAuditAction_LogUserCreate(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	actorID := uuid.New()
	targetID := uuid.New()
	action.LogUserCreate(c, actorID, "admin", targetID, "newuser")

	if len(logger.entries) != 1 {
		t.Fatal("LogUserCreate() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionUserCreate {
		t.Errorf("LogUserCreate() Action = %v, want %v", entry.Action, ActionUserCreate)
	}
	if entry.ResourceType != "user" {
		t.Errorf("LogUserCreate() ResourceType = %v, want user", entry.ResourceType)
	}
	if entry.ResourceID != targetID.String() {
		t.Errorf("LogUserCreate() ResourceID = %v, want %v", entry.ResourceID, targetID.String())
	}
}

func TestAuditAction_LogUserUpdate(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/users/123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	actorID := uuid.New()
	targetID := uuid.New()
	oldValue := map[string]string{"name": "old"}
	newValue := map[string]string{"name": "new"}
	action.LogUserUpdate(c, actorID, "admin", targetID, oldValue, newValue)

	if len(logger.entries) != 1 {
		t.Fatal("LogUserUpdate() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionUserUpdate {
		t.Errorf("LogUserUpdate() Action = %v, want %v", entry.Action, ActionUserUpdate)
	}
	if entry.OldValue == nil {
		t.Error("LogUserUpdate() OldValue should not be nil")
	}
	if entry.NewValue == nil {
		t.Error("LogUserUpdate() NewValue should not be nil")
	}
}

func TestAuditAction_LogUserDelete(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/users/123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	actorID := uuid.New()
	targetID := uuid.New()
	action.LogUserDelete(c, actorID, "admin", targetID, "deleteduser")

	if len(logger.entries) != 1 {
		t.Fatal("LogUserDelete() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionUserDelete {
		t.Errorf("LogUserDelete() Action = %v, want %v", entry.Action, ActionUserDelete)
	}
}

func TestAuditAction_LogUserLock(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users/123/lock", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	actorID := uuid.New()
	targetID := uuid.New()
	action.LogUserLock(c, actorID, "admin", targetID, "lockeduser")

	if len(logger.entries) != 1 {
		t.Fatal("LogUserLock() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionUserLock {
		t.Errorf("LogUserLock() Action = %v, want %v", entry.Action, ActionUserLock)
	}
}

func TestAuditAction_LogUserUnlock(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/users/123/unlock", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	actorID := uuid.New()
	targetID := uuid.New()
	action.LogUserUnlock(c, actorID, "admin", targetID, "unlockeduser")

	if len(logger.entries) != 1 {
		t.Fatal("LogUserUnlock() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionUserUnlock {
		t.Errorf("LogUserUnlock() Action = %v, want %v", entry.Action, ActionUserUnlock)
	}
}

func TestAuditAction_LogConfigChange(t *testing.T) {
	logger := &mockLogger{entries: make([]*Entry, 0)}
	action := NewAuditAction(logger)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/config", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	userID := uuid.New()
	action.LogConfigChange(c, userID, "admin", "security.mfa.enabled", false, true)

	if len(logger.entries) != 1 {
		t.Fatal("LogConfigChange() should log one entry")
	}

	entry := logger.entries[0]
	if entry.Action != ActionConfigChange {
		t.Errorf("LogConfigChange() Action = %v, want %v", entry.Action, ActionConfigChange)
	}
	if entry.ResourceType != "config" {
		t.Errorf("LogConfigChange() ResourceType = %v, want config", entry.ResourceType)
	}
}

// mockLogger implements Logger for testing.
type mockLogger struct {
	entries []*Entry
}

func (m *mockLogger) Log(ctx context.Context, entry *Entry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockLogger) Query(ctx context.Context, params *QueryParams) (*QueryResult, error) {
	return &QueryResult{Entries: m.entries, Total: int64(len(m.entries))}, nil
}

func (m *mockLogger) GetByID(ctx context.Context, id uuid.UUID) (*Entry, error) {
	for _, e := range m.entries {
		if e.ID == id {
			return e, nil
		}
	}
	return nil, nil
}

func (m *mockLogger) Close() error {
	return nil
}
