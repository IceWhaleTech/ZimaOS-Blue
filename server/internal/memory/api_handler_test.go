package memory

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func setupTestHandler(t *testing.T) (*APIHandler, *echo.Echo) {
	t.Helper()
	db := newTestDB(t)
	repo, err := NewMemoryRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	nsStore, err := NewNamespaceStore(db)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAPIHandler(repo, nsStore)
	e := echo.New()
	h.RegisterRoutes(e.Group("/api/v1"))
	return h, e
}

func TestAPIHandler_CreateAndGet(t *testing.T) {
	_, e := setupTestHandler(t)

	// Create
	body := `{"content":"test memory","category":"fact","tags":["a","b"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Namespace", "test-ns")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Create status = %d, want %d, body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var created MemoryEntry
	json.Unmarshal(rec.Body.Bytes(), &created)
	if created.Content != "test memory" {
		t.Errorf("Content = %q", created.Content)
	}

	// Get
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/memories/"+created.ID, nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("Get status = %d, want %d", rec2.Code, http.StatusOK)
	}
}

func TestAPIHandler_List(t *testing.T) {
	_, e := setupTestHandler(t)

	// Create 3 entries
	for i := 0; i < 3; i++ {
		body := `{"content":"entry"}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Namespace", "ns1")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
	}

	// List
	req := httptest.NewRequest(http.MethodGet, "/api/v1/memories?namespace=ns1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("List status = %d", rec.Code)
	}
	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)
	count := int(resp["count"].(float64))
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestAPIHandler_DeleteAndPurge(t *testing.T) {
	_, e := setupTestHandler(t)

	// Create
	body := `{"content":"to delete"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/memories", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var created MemoryEntry
	json.Unmarshal(rec.Body.Bytes(), &created)

	// Delete
	req2 := httptest.NewRequest(http.MethodDelete, "/api/v1/memories/"+created.ID, nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("Delete status = %d, body: %s", rec2.Code, rec2.Body.String())
	}
}

func TestAPIHandler_Namespace(t *testing.T) {
	_, e := setupTestHandler(t)

	// Create namespace
	body := `{"id":"my-ns"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Create NS status = %d, body: %s", rec.Code, rec.Body.String())
	}

	// List namespaces
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces", nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("List NS status = %d", rec2.Code)
	}

	// Delete namespace
	req3 := httptest.NewRequest(http.MethodDelete, "/api/v1/namespaces/my-ns", nil)
	rec3 := httptest.NewRecorder()
	e.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("Delete NS status = %d, body: %s", rec3.Code, rec3.Body.String())
	}
}
