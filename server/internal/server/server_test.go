package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Echo/server/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:         "127.0.0.1",
		Port:         23456,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s := New(cfg)
	if s == nil {
		t.Fatal("New() returned nil")
	}
	if s.Echo() == nil {
		t.Error("Echo() returned nil")
	}
}

func TestHealthHandler(t *testing.T) {
	cfg := &config.ServerConfig{
		Host: "127.0.0.1",
		Port: 23456,
	}
	s := New(cfg)
	s.RegisterHealthRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	s.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusOK)
	}

	var status HealthStatus
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if status.Status != "ok" {
		t.Errorf("Status = %v, want %v", status.Status, "ok")
	}
	if status.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
}

func TestLivenessHandler(t *testing.T) {
	cfg := &config.ServerConfig{
		Host: "127.0.0.1",
		Port: 23456,
	}
	s := New(cfg)
	s.RegisterHealthRoutes()

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()

	s.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if resp["status"] != "alive" {
		t.Errorf("Status = %v, want %v", resp["status"], "alive")
	}
}

func TestReadinessHandler_Ready(t *testing.T) {
	cfg := &config.ServerConfig{
		Host: "127.0.0.1",
		Port: 23456,
	}
	s := New(cfg)
	s.RegisterHealthRoutes()

	SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	s.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusOK)
	}
}

func TestReadinessHandler_NotReady(t *testing.T) {
	cfg := &config.ServerConfig{
		Host: "127.0.0.1",
		Port: 23456,
	}
	s := New(cfg)
	s.RegisterHealthRoutes()

	SetReady(false)
	defer SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()

	s.Echo().ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("Status code = %v, want %v", rec.Code, http.StatusServiceUnavailable)
	}
}
