package tunnel

import (
	"context"
	"strings"
	"testing"
)

func TestNewServeoManager(t *testing.T) {
	m := NewServeoManager()
	if m == nil {
		t.Fatal("NewServeoManager() returned nil")
	}
	if m.GetProvider() != ProviderServeo {
		t.Errorf("GetProvider() = %v, want %v", m.GetProvider(), ProviderServeo)
	}
}

func TestServeoManager_GetStatus_NotRunning(t *testing.T) {
	m := NewServeoManager()
	st := m.GetStatus()
	if st.Active {
		t.Error("expected Active false when not running")
	}
	if st.Provider != ProviderServeo {
		t.Errorf("Provider = %v, want %v", st.Provider, ProviderServeo)
	}
	if st.URL != "" {
		t.Errorf("URL = %q, want empty", st.URL)
	}
}

func TestServeoManager_GetURL_NotRunning(t *testing.T) {
	m := NewServeoManager()
	if u := m.GetURL(); u != "" {
		t.Errorf("GetURL() = %q, want empty", u)
	}
}

func TestServeoManager_Stop_WhenNotRunning(t *testing.T) {
	m := NewServeoManager()
	if err := m.Stop(); err != nil {
		t.Errorf("Stop() when not running: %v", err)
	}
}

// Start with empty subdomain must fail (subdomain required for Serveo).
func TestServeoManager_Start_EmptySubdomainFails(t *testing.T) {
	m := NewServeoManager()
	ctx := context.Background()
	cfg := &Config{Port: 8080, Subdomain: ""}
	err := m.Start(ctx, cfg)
	if err == nil {
		t.Fatal("Start() with empty subdomain should return error")
	}
	if !strings.Contains(err.Error(), "subdomain") {
		t.Errorf("expected subdomain-related error, got: %v", err)
	}
	if m.IsRunning() {
		t.Error("manager should not be running after Start error")
	}
}

func TestServeoManager_Start_AlreadyRunning(t *testing.T) {
	m := NewServeoManager()
	ctx := context.Background()
	// Use exec path (non-Windows) or native path with subdomain so Start is attempted.
	cfg := &Config{Port: 8080, Subdomain: "echo-alreadyrunning"}
	err := m.Start(ctx, cfg)
	if err != nil {
		// In CI we may get dial/listen errors; then we can't test "already running".
		t.Skipf("first Start failed (e.g. no network): %v", err)
	}
	defer m.Stop()
	// Second Start should fail with "tunnel already running" (exec path)
	// or with dial/listen error if first Start actually failed after setting running.
	err2 := m.Start(ctx, cfg)
	if err2 == nil {
		t.Error("second Start() should fail")
	}
	if err2 != nil && !strings.Contains(err2.Error(), "already running") {
		// If we're on Windows and first Start failed after setting running state,
		// second Start might hit "tunnel already running" once running is true.
		if m.IsRunning() && !strings.Contains(err2.Error(), "already running") {
			t.Errorf("second Start() error = %v, want 'already running' when manager is running", err2)
		}
	}
}
