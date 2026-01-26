package profiling

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name           string
		config         *Config
		expectedPrefix string
	}{
		{
			name:           "nil config uses defaults",
			config:         nil,
			expectedPrefix: "/debug/pprof",
		},
		{
			name: "custom prefix",
			config: &Config{
				Enabled:        true,
				EndpointPrefix: "/custom/pprof",
			},
			expectedPrefix: "/custom/pprof",
		},
		{
			name: "prefix without leading slash",
			config: &Config{
				Enabled:        true,
				EndpointPrefix: "debug/pprof",
			},
			expectedPrefix: "/debug/pprof",
		},
		{
			name: "prefix with trailing slash",
			config: &Config{
				Enabled:        true,
				EndpointPrefix: "/debug/pprof/",
			},
			expectedPrefix: "/debug/pprof",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.config)
			if p == nil {
				t.Fatal("expected non-nil Profiler")
			}
			if p.EndpointPrefix() != tt.expectedPrefix {
				t.Errorf("expected prefix %s, got %s", tt.expectedPrefix, p.EndpointPrefix())
			}
		})
	}
}

func TestIsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		expected bool
	}{
		{
			name:     "nil config defaults to disabled",
			config:   nil,
			expected: false,
		},
		{
			name: "explicitly enabled",
			config: &Config{
				Enabled: true,
			},
			expected: true,
		},
		{
			name: "explicitly disabled",
			config: &Config{
				Enabled: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.config)
			if p.IsEnabled() != tt.expected {
				t.Errorf("expected IsEnabled() = %v, got %v", tt.expected, p.IsEnabled())
			}
		})
	}
}

func TestEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		expectNil     bool
		expectedCount int
	}{
		{
			name: "disabled returns nil",
			config: &Config{
				Enabled: false,
			},
			expectNil: true,
		},
		{
			name: "enabled returns endpoints",
			config: &Config{
				Enabled:        true,
				EndpointPrefix: "/debug/pprof",
			},
			expectNil:     false,
			expectedCount: 11, // All pprof endpoints
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.config)
			endpoints := p.Endpoints()

			if tt.expectNil {
				if endpoints != nil {
					t.Error("expected nil endpoints")
				}
				return
			}

			if len(endpoints) != tt.expectedCount {
				t.Errorf("expected %d endpoints, got %d", tt.expectedCount, len(endpoints))
			}
		})
	}
}

func TestRegisterRoutes(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		testEndpoint string
		expectStatus int
	}{
		{
			name: "disabled does not register routes",
			config: &Config{
				Enabled: false,
			},
			testEndpoint: "/debug/pprof/",
			expectStatus: http.StatusNotFound,
		},
		{
			name: "enabled registers routes",
			config: &Config{
				Enabled:        true,
				EndpointPrefix: "/debug/pprof",
				AuthRequired:   false,
			},
			testEndpoint: "/debug/pprof/",
			expectStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.config)
			e := echo.New()
			p.RegisterRoutes(e)

			req := httptest.NewRequest(http.MethodGet, tt.testEndpoint, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tt.expectStatus {
				t.Errorf("expected status %d, got %d", tt.expectStatus, rec.Code)
			}
		})
	}
}

func TestPprofEndpoints(t *testing.T) {
	p := New(&Config{
		Enabled:        true,
		EndpointPrefix: "/debug/pprof",
		AuthRequired:   false,
	})

	e := echo.New()
	p.RegisterRoutes(e)

	endpoints := []struct {
		path   string
		method string
	}{
		{"/debug/pprof/", http.MethodGet},
		{"/debug/pprof/cmdline", http.MethodGet},
		{"/debug/pprof/symbol", http.MethodGet},
		{"/debug/pprof/heap", http.MethodGet},
		{"/debug/pprof/goroutine", http.MethodGet},
		{"/debug/pprof/allocs", http.MethodGet},
		{"/debug/pprof/block", http.MethodGet},
		{"/debug/pprof/mutex", http.MethodGet},
		{"/debug/pprof/threadcreate", http.MethodGet},
	}

	for _, ep := range endpoints {
		t.Run(ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			// pprof endpoints should return 200
			if rec.Code != http.StatusOK {
				t.Errorf("expected status 200 for %s, got %d", ep.path, rec.Code)
			}
		})
	}
}

func TestRegisterRoutesWithAuth(t *testing.T) {
	authCalled := false
	authMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authCalled = true
			return next(c)
		}
	}

	p := New(&Config{
		Enabled:        true,
		EndpointPrefix: "/debug/pprof",
		AuthRequired:   true,
	})

	e := echo.New()
	p.RegisterRoutes(e, authMiddleware)

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if !authCalled {
		t.Error("expected auth middleware to be called")
	}
}
