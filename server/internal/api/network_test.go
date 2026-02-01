package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNetworkHandler_GetAddresses(t *testing.T) {
	e := echo.New()
	handler := NewNetworkHandler(23456)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/addresses", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetAddresses(c)
	if err != nil {
		t.Fatalf("GetAddresses returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check required fields
	if _, ok := response["local"]; !ok {
		t.Error("Response should contain 'local' field")
	}
	if _, ok := response["port"]; !ok {
		t.Error("Response should contain 'port' field")
	}
	if _, ok := response["preferred"]; !ok {
		t.Error("Response should contain 'preferred' field")
	}
	if _, ok := response["lan"]; !ok {
		t.Error("Response should contain 'lan' field")
	}

	t.Logf("Response: %s", rec.Body.String())
}

func TestNetworkHandler_GetStatus(t *testing.T) {
	e := echo.New()
	handler := NewNetworkHandler(23456)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetStatus(c)
	if err != nil {
		t.Fatalf("GetStatus returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check required fields
	if _, ok := response["status"]; !ok {
		t.Error("Response should contain 'status' field")
	}
	if _, ok := response["interfaces_count"]; !ok {
		t.Error("Response should contain 'interfaces_count' field")
	}
	if _, ok := response["has_lan_access"]; !ok {
		t.Error("Response should contain 'has_lan_access' field")
	}

	t.Logf("Response: %s", rec.Body.String())
}

func TestNetworkHandler_GetPreferred(t *testing.T) {
	e := echo.New()
	handler := NewNetworkHandler(23456)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/network/preferred", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler.GetPreferred(c)
	if err != nil {
		t.Fatalf("GetPreferred returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// Check required fields
	if _, ok := response["address"]; !ok {
		t.Error("Response should contain 'address' field")
	}

	address := response["address"].(string)
	if address == "" {
		t.Error("Address should not be empty")
	}

	t.Logf("Preferred address: %s", address)
}

func TestNetworkHandler_RegisterRoutes(t *testing.T) {
	e := echo.New()
	handler := NewNetworkHandler(23456)

	handler.RegisterRoutes(e)

	// Check that routes are registered
	routes := e.Routes()
	expectedPaths := []string{
		"/api/v1/network/addresses",
		"/api/v1/network/status",
		"/api/v1/network/preferred",
	}

	for _, path := range expectedPaths {
		found := false
		for _, route := range routes {
			if route.Path == path {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Route %s not found", path)
		}
	}
}
