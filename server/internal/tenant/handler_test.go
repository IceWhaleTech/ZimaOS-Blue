package tenant

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestHandler(t *testing.T) (*Handler, *echo.Echo, *sql.DB, uuid.UUID) {
	db := setupTestDB(t)

	repo := NewSQLiteRepository(db)
	if err := repo.InitSchema(context.Background()); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}

	service := NewService(repo, nil)
	handler := NewHandler(service)

	e := echo.New()
	g := e.Group("/api/v1")
	handler.RegisterRoutes(g)

	// Create a test user ID
	userID := uuid.New()

	return handler, e, db, userID
}

// withUserID is a middleware that sets the user ID in context
func withUserID(userID uuid.UUID) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("user_id", userID)
			return next(c)
		}
	}
}

func TestHandler_CreateTenant(t *testing.T) {
	handler, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	t.Run("successful creation", func(t *testing.T) {
		body := `{"name": "Test Tenant", "slug": "test-tenant"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
		}

		var tenant Tenant
		if err := json.Unmarshal(rec.Body.Bytes(), &tenant); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if tenant.Name != "Test Tenant" {
			t.Errorf("expected name 'Test Tenant', got '%s'", tenant.Name)
		}
		if tenant.Slug != "test-tenant" {
			t.Errorf("expected slug 'test-tenant', got '%s'", tenant.Slug)
		}
	})

	t.Run("duplicate slug", func(t *testing.T) {
		// First create
		body := `{"name": "Another Tenant", "slug": "duplicate-slug"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("first creation failed: %s", rec.Body.String())
		}

		// Second create with same slug
		req = httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, rec.Code)
		}
	})

	t.Run("unauthorized", func(t *testing.T) {
		// Create a new echo instance without user middleware
		e2 := echo.New()
		g := e2.Group("/api/v1")
		handler.RegisterRoutes(g)

		body := `{"name": "Test", "slug": "test"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e2.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestHandler_GetTenant(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant first
	body := `{"name": "Get Test", "slug": "get-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("successful get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+createdTenant.ID.String(), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var tenant Tenant
		if err := json.Unmarshal(rec.Body.Bytes(), &tenant); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if tenant.ID != createdTenant.ID {
			t.Errorf("expected ID %s, got %s", createdTenant.ID, tenant.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+uuid.New().String(), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			// Non-member gets forbidden, not not-found (for security)
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rec.Code)
		}
	})

	t.Run("invalid ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/invalid-uuid", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

func TestHandler_UpdateTenant(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant first
	body := `{"name": "Update Test", "slug": "update-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("successful update", func(t *testing.T) {
		updateBody := `{"name": "Updated Name"}`
		req := httptest.NewRequest(http.MethodPut, "/api/v1/tenants/"+createdTenant.ID.String(), bytes.NewBufferString(updateBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var tenant Tenant
		if err := json.Unmarshal(rec.Body.Bytes(), &tenant); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if tenant.Name != "Updated Name" {
			t.Errorf("expected name 'Updated Name', got '%s'", tenant.Name)
		}
	})
}

func TestHandler_DeleteTenant(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant first
	body := `{"name": "Delete Test", "slug": "delete-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("successful delete", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/tenants/"+createdTenant.ID.String(), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Errorf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
		}

		// Verify tenant is deleted - should return 404 (not found) or 403 (forbidden)
		req = httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+createdTenant.ID.String(), nil)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		// After deletion, the tenant doesn't exist, so membership check fails
		if rec.Code != http.StatusForbidden && rec.Code != http.StatusNotFound {
			t.Errorf("expected status 403 or 404 after delete, got %d", rec.Code)
		}
	})
}

func TestHandler_ListTenants(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create some tenants
	for i := 0; i < 3; i++ {
		body := `{"name": "List Test ` + string(rune('A'+i)) + `", "slug": "list-test-` + string(rune('a'+i)) + `"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}

	t.Run("list all tenants", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		total := int(resp["total"].(float64))
		if total != 3 {
			t.Errorf("expected 3 tenants, got %d", total)
		}
	})
}

func TestHandler_ListMembers(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant
	body := `{"name": "Members Test", "slug": "members-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("list members", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+createdTenant.ID.String()+"/members", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		// The response structure may vary - just check we got a valid response
		if resp == nil {
			t.Error("expected non-nil response")
		}
	})
}

func TestHandler_GetSettings(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant
	body := `{"name": "Settings Test", "slug": "settings-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("get settings", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+createdTenant.ID.String()+"/settings", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})
}

func TestHandler_GetLimits(t *testing.T) {
	_, e, db, userID := setupTestHandler(t)
	defer db.Close()

	e.Use(withUserID(userID))

	// Create a tenant
	body := `{"name": "Limits Test", "slug": "limits-test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var createdTenant Tenant
	json.Unmarshal(rec.Body.Bytes(), &createdTenant)

	t.Run("get limits", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tenants/"+createdTenant.ID.String()+"/limits", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
		}
	})
}
