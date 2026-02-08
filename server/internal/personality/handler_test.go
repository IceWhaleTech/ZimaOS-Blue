package personality

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestHandlerCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	handler := NewHandler(service)

	e := echo.New()
	body := map[string]string{
		"name":            "Test",
		"description":     "Test desc",
		"system_prompt":   "Test prompt",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/personalities", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Act
	err := handler.Create(c)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rec.Code)
	}
}

func TestHandlerList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	service.Create("P1", "Desc1", "Prompt1")
	service.Create("P2", "Desc2", "Prompt2")

	handler := NewHandler(service)
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/personalities", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Act
	err := handler.List(c)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}
