package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

func newTCP4Server(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skip test server setup (tcp4 unavailable): %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	return srv
}

func TestNew(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:         "127.0.0.1",
		Port:         80,
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
		Port: 80,
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
		Port: 80,
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
		Port: 80,
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
		Port: 80,
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

func splitHostPortFromURL(t *testing.T, rawURL string) (string, int) {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse URL %q: %v", rawURL, err)
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split host/port from %q: %v", u.Host, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("invalid port %q: %v", portStr, err)
	}
	return host, port
}

func TestCheckExistingServer(t *testing.T) {
	t.Run("returns true for zimaos-blue health response", func(t *testing.T) {
		srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/health" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","service":"zimaos-blue"}`))
		}))
		defer srv.Close()

		host, port := splitHostPortFromURL(t, srv.URL)
		if !checkExistingServer(host, port) {
			t.Fatal("checkExistingServer returned false, want true")
		}
	})

	t.Run("returns false for non-blue service response", func(t *testing.T) {
		srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/health" {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok","service":"other-service"}`))
		}))
		defer srv.Close()

		host, port := splitHostPortFromURL(t, srv.URL)
		if checkExistingServer(host, port) {
			t.Fatal("checkExistingServer returned true, want false")
		}
	})
}

func TestRequestGracefulShutdown(t *testing.T) {
	var called bool
	srv := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/shutdown" {
			http.NotFound(w, r)
			return
		}
		called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	host, port := splitHostPortFromURL(t, srv.URL)
	if !requestGracefulShutdown(host, port) {
		t.Fatal("requestGracefulShutdown returned false, want true")
	}
	if !called {
		t.Fatal("expected shutdown endpoint to be called")
	}
}

func TestStartInvokesStartupURLHandlerWithActualPort(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:         "127.0.0.1",
		Port:         0,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Second,
	}
	s := New(cfg)
	s.RegisterHealthRoutes()

	urlCh := make(chan string, 1)
	SetStartupURLHandler(func(rawURL string) {
		urlCh <- rawURL
	})
	defer SetStartupURLHandler(nil)

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.Start()
	}()

	var gotURL string
	select {
	case gotURL = <-urlCh:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for startup URL handler")
	}

	parsed, err := url.Parse(gotURL)
	if err != nil {
		t.Fatalf("parse startup URL %q: %v", gotURL, err)
	}
	if parsed.Scheme != "http" {
		t.Fatalf("startup URL scheme = %q, want %q", parsed.Scheme, "http")
	}
	if parsed.Hostname() != "127.0.0.1" {
		t.Fatalf("startup URL hostname = %q, want %q", parsed.Hostname(), "127.0.0.1")
	}
	if parsed.Port() == "" || parsed.Port() == "0" {
		t.Fatalf("startup URL port = %q, want actual listening port", parsed.Port())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("shutdown server: %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil && !strings.Contains(err.Error(), "Server closed") {
			t.Fatalf("server start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server goroutine to exit")
	}
}
