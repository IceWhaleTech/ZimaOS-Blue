package ngrok

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// SDKTunnelManager manages ngrok tunnels using the ngrok-go SDK.
// This replaces the binary-based approach with a native Go SDK.
type SDKTunnelManager struct {
	repository *Repository
	authtoken  string

	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	sessionID    string
	tunnel       ngrok.Tunnel
	cancelFunc   context.CancelFunc

	// Callbacks
	OnURLChange func(url string)
	OnError     func(err error)
}

// NewSDKTunnelManager creates a new SDK-based tunnel manager.
func NewSDKTunnelManager(repo *Repository) *SDKTunnelManager {
	return &SDKTunnelManager{
		repository: repo,
	}
}

// Start starts the ngrok tunnel using the SDK.
func (tm *SDKTunnelManager) Start(ctx context.Context, port int, authtoken string) error {
	tm.mu.Lock()
	if tm.running {
		tm.mu.Unlock()
		return ErrTunnelAlreadyRunning
	}
	tm.mu.Unlock()

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	// Configure ngrok session
	opts := []ngrok.ConnectOption{}
	if authtoken != "" {
		opts = append(opts, ngrok.WithAuthtoken(authtoken))
	}

	// Start listening
	tunnel, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(
			config.WithForwardsTo(fmt.Sprintf("localhost:%d", port)),
		),
		opts...,
	)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start ngrok tunnel: %w", err)
	}

	// Get tunnel URL
	url := tunnel.URL()

	tm.mu.Lock()
	tm.running = true
	tm.connecting = false // SDK returns URL immediately
	tm.url = url
	tm.authtoken = authtoken
	tm.tunnel = tunnel
	tm.cancelFunc = cancel
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
	tm.mu.Unlock()

	// Create session record in database if repository is available
	if tm.repository != nil {
		session := &RemoteAccessSession{
			TunnelURL:    url,
			StartedAt:    tm.startedAt,
			ExpiresAt:    tm.expiresAt,
			RenewedCount: 0,
			Status:       "active",
		}
		sessionID, err := tm.repository.CreateSession(ctx, session)
		if err == nil {
			tm.mu.Lock()
			tm.sessionID = sessionID
			tm.mu.Unlock()
		}
	}

	// Notify URL change
	if tm.OnURLChange != nil {
		tm.OnURLChange(url)
	}

	// Monitor tunnel in background
	go tm.monitorTunnel(ctx)

	return nil
}

// monitorTunnel monitors the tunnel and handles disconnection.
func (tm *SDKTunnelManager) monitorTunnel(ctx context.Context) {
	// Wait for context cancellation or listener close
	<-ctx.Done()

	tm.mu.Lock()
	sessionID := tm.sessionID
	tm.running = false
	tm.connecting = false
	tm.mu.Unlock()

	// Mark session as ended in database
	if tm.repository != nil && sessionID != "" {
		tm.repository.EndSession(context.Background(), sessionID, "stopped", "")
	}
}

// Stop stops the ngrok tunnel.
func (tm *SDKTunnelManager) Stop() error {
	tm.mu.Lock()
	sessionID := tm.sessionID
	wasRunning := tm.running
	tunnel := tm.tunnel
	cancelFunc := tm.cancelFunc
	tm.mu.Unlock()

	if !wasRunning {
		return nil // Not an error to stop when not running
	}

	// Cancel context
	if cancelFunc != nil {
		cancelFunc()
	}

	// Close tunnel
	if tunnel != nil {
		tunnel.Close()
	}

	tm.mu.Lock()
	tm.running = false
	tm.connecting = false
	tm.url = ""
	tm.tunnel = nil
	tm.cancelFunc = nil
	tm.mu.Unlock()

	// Mark session as stopped in database
	if tm.repository != nil && sessionID != "" {
		tm.repository.EndSession(context.Background(), sessionID, "stopped", "")
	}

	return nil
}

// IsRunning returns true if the tunnel is running.
func (tm *SDKTunnelManager) IsRunning() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.running
}

// GetStatus returns the current tunnel status.
func (tm *SDKTunnelManager) GetStatus() TunnelStatus {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	if !tm.running {
		return TunnelStatus{Active: false}
	}

	remaining := calculateRemainingTime(tm.expiresAt)

	return TunnelStatus{
		Active:        tm.running,
		Connecting:    tm.connecting,
		URL:           tm.url,
		StartedAt:     tm.startedAt,
		ExpiresAt:     tm.expiresAt,
		RemainingTime: formatRemainingTime(remaining),
		RenewedCount:  tm.renewedCount,
	}
}

// GetURL returns the current tunnel URL.
func (tm *SDKTunnelManager) GetURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.url
}

// IncrementRenewedCount increments the renewal counter.
func (tm *SDKTunnelManager) IncrementRenewedCount() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.renewedCount++
}

// ResetExpiry resets the expiry time (called after renewal).
func (tm *SDKTunnelManager) ResetExpiry() {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.startedAt = time.Now()
	tm.expiresAt = calculateExpiresAt(tm.startedAt)
}
