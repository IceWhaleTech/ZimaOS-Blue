package ngrok

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRenewalService_NewRenewalService(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	if rs == nil {
		t.Fatal("NewRenewalService() returned nil")
	}

	if rs.tunnelManager != tm {
		t.Error("tunnelManager not set correctly")
	}

	if rs.checkInterval != 30*time.Minute {
		t.Errorf("checkInterval = %v, want 30m", rs.checkInterval)
	}

	if rs.renewalThreshold != 1*time.Hour {
		t.Errorf("renewalThreshold = %v, want 1h", rs.renewalThreshold)
	}
}

func TestRenewalService_ShouldRenew(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	tests := []struct {
		name      string
		expiresAt time.Time
		expected  bool
	}{
		{
			name:      "8 hours remaining - should not renew",
			expiresAt: time.Now().Add(8 * time.Hour),
			expected:  false,
		},
		{
			name:      "2 hours remaining - should not renew",
			expiresAt: time.Now().Add(2 * time.Hour),
			expected:  false,
		},
		{
			name:      "1 hour remaining - should renew",
			expiresAt: time.Now().Add(1 * time.Hour),
			expected:  true,
		},
		{
			name:      "30 minutes remaining - should renew",
			expiresAt: time.Now().Add(30 * time.Minute),
			expected:  true,
		},
		{
			name:      "already expired - should renew",
			expiresAt: time.Now().Add(-1 * time.Hour),
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rs.ShouldRenew(tt.expiresAt)
			if result != tt.expected {
				t.Errorf("ShouldRenew() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestRenewalService_IsRunning(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	if rs.IsRunning() {
		t.Error("IsRunning() should be false initially")
	}
}

func TestRenewalService_StartStop(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	// Start the service
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rs.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if !rs.IsRunning() {
		t.Error("IsRunning() should be true after Start()")
	}

	// Stop the service
	rs.Stop()

	// Give it a moment to stop
	time.Sleep(100 * time.Millisecond)

	if rs.IsRunning() {
		t.Error("IsRunning() should be false after Stop()")
	}
}

func TestRenewalService_OnRenewal_Callback(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	var callbackCalled bool
	var callbackURL string
	var mu sync.Mutex

	rs.OnRenewal = func(oldURL, newURL string) {
		mu.Lock()
		defer mu.Unlock()
		callbackCalled = true
		callbackURL = newURL
	}

	// Simulate a renewal callback
	if rs.OnRenewal != nil {
		rs.OnRenewal("old-url", "new-url")
	}

	mu.Lock()
	defer mu.Unlock()

	if !callbackCalled {
		t.Error("OnRenewal callback was not called")
	}

	if callbackURL != "new-url" {
		t.Errorf("callbackURL = %s, want new-url", callbackURL)
	}
}

func TestRenewalService_SetCheckInterval(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	newInterval := 15 * time.Minute
	rs.SetCheckInterval(newInterval)

	if rs.checkInterval != newInterval {
		t.Errorf("checkInterval = %v, want %v", rs.checkInterval, newInterval)
	}
}

func TestRenewalService_SetRenewalThreshold(t *testing.T) {
	dm := NewDownloadManager("")
	tm := NewTunnelManager(dm)
	rs := NewRenewalService(tm)

	newThreshold := 2 * time.Hour
	rs.SetRenewalThreshold(newThreshold)

	if rs.renewalThreshold != newThreshold {
		t.Errorf("renewalThreshold = %v, want %v", rs.renewalThreshold, newThreshold)
	}
}
