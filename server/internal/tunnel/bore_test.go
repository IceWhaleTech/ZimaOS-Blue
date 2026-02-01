package tunnel

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCheckBoreAvailable(t *testing.T) {
	if !CheckBoreAvailable() {
		t.Error("CheckBoreAvailable() should be true (native Go client)")
	}
}

func TestNewBoreManager(t *testing.T) {
	m := NewBoreManager()
	if m == nil {
		t.Fatal("NewBoreManager() returned nil")
	}
	if m.GetProvider() != ProviderBore {
		t.Errorf("GetProvider() = %v, want %v", m.GetProvider(), ProviderBore)
	}
}

func TestBoreManager_GetStatus_NotRunning(t *testing.T) {
	m := NewBoreManager()
	st := m.GetStatus()
	if st.Active {
		t.Error("expected Active false when not running")
	}
	if st.Provider != ProviderBore {
		t.Errorf("Provider = %v, want %v", st.Provider, ProviderBore)
	}
	if st.URL != "" {
		t.Errorf("URL = %q, want empty", st.URL)
	}
}

func TestBoreManager_Stop_WhenNotRunning(t *testing.T) {
	m := NewBoreManager()
	if err := m.Stop(); err != nil {
		t.Errorf("Stop() when not running: %v", err)
	}
}

func TestBoreManager_Start_AlreadyRunning(t *testing.T) {
	m := NewBoreManager()
	ctx := context.Background()
	cfg := &Config{Port: 9999}
	if err := m.Start(ctx, cfg); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	defer m.Stop()
	// Give goroutine time to set running and maybe try dial
	time.Sleep(50 * time.Millisecond)
	err := m.Start(ctx, cfg)
	if err == nil {
		t.Error("second Start() should fail with tunnel already running")
	}
	if err != nil && err.Error() != "tunnel already running" {
		t.Errorf("Start() error = %v, want 'tunnel already running'", err)
	}
}

func TestBoreManager_Start_DefaultPort(t *testing.T) {
	m := NewBoreManager()
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &Config{Port: 0}
	if err := m.Start(ctx, cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	// boreClient will fail to dial bore.pub or get cancelled; either way manager should eventually stop
	m.Stop()
	if m.IsRunning() {
		t.Error("expected not running after Stop")
	}
}

func TestBoreManager_OnURLChange(t *testing.T) {
	m := NewBoreManager()
	var gotURL string
	var mu sync.Mutex
	m.SetOnURLChange(func(url string) {
		mu.Lock()
		gotURL = url
		mu.Unlock()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cfg := &Config{Port: 23456}
	if err := m.Start(ctx, cfg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		cancel()
		m.Stop()
	}()
	// boreClient connects to bore.pub; if it succeeds we get onURL callback. Short wait.
	time.Sleep(500 * time.Millisecond)
	mu.Lock()
	url := gotURL
	mu.Unlock()
	if url != "" {
		if m.GetURL() != url {
			t.Errorf("GetURL() = %q, onURLChange got %q", m.GetURL(), url)
		}
	}
	// If we didn't get URL (e.g. network unreachable), that's ok for unit test
}
