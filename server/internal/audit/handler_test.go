package audit

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestHandler(t *testing.T) (*Handler, *SQLiteLogger, func()) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	config := &LoggerConfig{
		BatchSize:     10,
		FlushInterval: 100 * time.Millisecond,
		BufferSize:    100,
	}

	logger, err := NewSQLiteLogger(db, config)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	handler := NewHandler(logger)

	cleanup := func() {
		logger.Close()
		db.Close()
	}

	return handler, logger, cleanup
}

func TestNewHandler(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}
}

func TestHandler_List(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	// Add some entries
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		entry := NewEntry(ActionLogin, StatusSuccess).
			WithUser(uuid.New(), "testuser")
		_ = logger.Log(ctx, entry)
	}

	// Wait for flush
	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("List() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_List_WithFilters(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	userID := uuid.New()

	// Add entries with different actions
	entry1 := NewEntry(ActionLogin, StatusSuccess).WithUser(userID, "testuser")
	entry2 := NewEntry(ActionLogout, StatusSuccess).WithUser(userID, "testuser")
	_ = logger.Log(ctx, entry1)
	_ = logger.Log(ctx, entry2)

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?action=login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	if err != nil {
		t.Fatalf("List(filtered) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("List(filtered) status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_List_InvalidUserID(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?user_id=invalid-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.List(c)
	if err == nil {
		t.Error("List(invalid user_id) expected error")
	}
}

func TestHandler_GetByID(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess).WithUser(uuid.New(), "testuser")
	_ = logger.Log(ctx, entry)

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/"+entry.ID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(entry.ID.String())

	err := handler.GetByID(c)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("GetByID() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_GetByID_InvalidID(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/invalid-uuid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("invalid-uuid")

	err := handler.GetByID(c)
	if err == nil {
		t.Error("GetByID(invalid) expected error")
	}
}

func TestHandler_GetByID_NotFound(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/"+nonExistentID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(nonExistentID.String())

	err := handler.GetByID(c)
	if err == nil {
		t.Error("GetByID(not found) expected error")
	}
}

func TestHandler_Export_JSON(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess).WithUser(uuid.New(), "testuser")
	_ = logger.Log(ctx, entry)

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/export?format=json", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Export(c)
	if err != nil {
		t.Fatalf("Export(json) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Export(json) status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Export(json) Content-Type = %v, want application/json", contentType)
	}

	disposition := rec.Header().Get("Content-Disposition")
	if disposition == "" {
		t.Error("Export(json) should set Content-Disposition header")
	}
}

func TestHandler_Export_CSV(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess).WithUser(uuid.New(), "testuser")
	_ = logger.Log(ctx, entry)

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/export?format=csv", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Export(c)
	if err != nil {
		t.Fatalf("Export(csv) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Export(csv) status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/csv" {
		t.Errorf("Export(csv) Content-Type = %v, want text/csv", contentType)
	}
}

func TestHandler_Export_InvalidFormat(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/export?format=xml", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Export(c)
	if err == nil {
		t.Error("Export(invalid format) expected error")
	}
}

func TestHandler_Export_DefaultFormat(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	entry := NewEntry(ActionLogin, StatusSuccess)
	_ = logger.Log(ctx, entry)

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/export", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Export(c)
	if err != nil {
		t.Fatalf("Export(default) error = %v", err)
	}

	// Default should be JSON
	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Export(default) Content-Type = %v, want application/json", contentType)
	}
}

func TestHandler_Stats(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	// Add various entries
	_ = logger.Log(ctx, NewEntry(ActionLogin, StatusSuccess))
	_ = logger.Log(ctx, NewEntry(ActionLogin, StatusFailure))
	_ = logger.Log(ctx, NewEntry(ActionLogout, StatusSuccess))

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Stats(c)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Stats() status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Stats_WithTimeRange(t *testing.T) {
	handler, logger, cleanup := setupTestHandler(t)
	defer cleanup()

	ctx := context.Background()
	_ = logger.Log(ctx, NewEntry(ActionLogin, StatusSuccess))

	time.Sleep(200 * time.Millisecond)

	e := echo.New()
	startTime := url.QueryEscape(time.Now().Add(-1 * time.Hour).Format(time.RFC3339))
	endTime := url.QueryEscape(time.Now().Add(1 * time.Hour).Format(time.RFC3339))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats?start_time="+startTime+"&end_time="+endTime, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Stats(c)
	if err != nil {
		t.Fatalf("Stats(time range) error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Stats(time range) status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_Stats_InvalidStartTime(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit/stats?start_time=invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.Stats(c)
	if err == nil {
		t.Error("Stats(invalid start_time) expected error")
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	handler, _, cleanup := setupTestHandler(t)
	defer cleanup()

	e := echo.New()
	g := e.Group("/api/v1/audit")

	handler.RegisterRoutes(g)

	// Verify routes are registered
	routes := e.Routes()
	expectedPaths := []string{
		"/api/v1/audit",
		"/api/v1/audit/export",
		"/api/v1/audit/stats",
		"/api/v1/audit/:id",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Path] = true
	}

	for _, path := range expectedPaths {
		if !routeMap[path] {
			t.Errorf("RegisterRoutes() missing route %s", path)
		}
	}
}

func TestParseIntParam(t *testing.T) {
	e := echo.New()

	tests := []struct {
		name         string
		queryParam   string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid integer",
			queryParam:   "10",
			defaultValue: 5,
			expected:     10,
		},
		{
			name:         "empty string",
			queryParam:   "",
			defaultValue: 5,
			expected:     5,
		},
		{
			name:         "invalid string",
			queryParam:   "abc",
			defaultValue: 5,
			expected:     5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?param="+tt.queryParam, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			result := parseIntParam(c, "param", tt.defaultValue)
			if result != tt.expected {
				t.Errorf("parseIntParam() = %d, want %d", result, tt.expected)
			}
		})
	}
}
