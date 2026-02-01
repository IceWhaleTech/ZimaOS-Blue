package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.ngrok.com/ngrok"
	"golang.ngrok.com/ngrok/config"
)

// NgrokManager manages ngrok tunnels using the ngrok-go SDK.
type NgrokManager struct {
	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	tunnel       ngrok.Tunnel
	cancelFunc   context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewNgrokManager creates a new ngrok tunnel manager.
func NewNgrokManager() *NgrokManager {
	return &NgrokManager{}
}

// Start starts the ngrok tunnel.
func (m *NgrokManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	m.mu.Unlock()

	port := cfg.Port
	if port == 0 {
		port = 23456
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	// Configure ngrok session
	opts := []ngrok.ConnectOption{}
	if cfg.NgrokAuthtoken != "" {
		opts = append(opts, ngrok.WithAuthtoken(cfg.NgrokAuthtoken))
	}

	// Configure HTTP endpoint options
	endpointOpts := []config.HTTPEndpointOption{
		config.WithForwardsTo(fmt.Sprintf("localhost:%d", port)),
	}

	// Add custom domain if provided (requires paid ngrok plan or free static domain)
	if cfg.NgrokDomain != "" {
		endpointOpts = append(endpointOpts, config.WithDomain(cfg.NgrokDomain))
	}

	// Start listening
	tunnel, err := ngrok.Listen(ctx,
		config.HTTPEndpoint(endpointOpts...),
		opts...,
	)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to start ngrok tunnel: %w", err)
	}

	// Get tunnel URL
	url := tunnel.URL()

	m.mu.Lock()
	m.running = true
	m.connecting = false
	m.url = url
	m.tunnel = tunnel
	m.cancelFunc = cancel
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour) // ngrok free tier expires after 8 hours
	m.mu.Unlock()

	// Notify URL change
	if m.onURLChange != nil {
		m.onURLChange(url)
	}

	// Monitor tunnel in background
	go m.monitorTunnel(ctx)

	return nil
}

// monitorTunnel monitors the tunnel and handles disconnection.
func (m *NgrokManager) monitorTunnel(ctx context.Context) {
	<-ctx.Done()

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.mu.Unlock()
}

// Stop stops the tunnel.
func (m *NgrokManager) Stop() error {
	m.mu.Lock()
	wasRunning := m.running
	tunnel := m.tunnel
	cancelFunc := m.cancelFunc
	m.mu.Unlock()

	if !wasRunning {
		return nil
	}

	if cancelFunc != nil {
		cancelFunc()
	}

	if tunnel != nil {
		tunnel.Close()
	}

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.url = ""
	m.tunnel = nil
	m.cancelFunc = nil
	m.mu.Unlock()

	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *NgrokManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *NgrokManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return Status{Active: false, Provider: ProviderNgrok}
	}

	remaining := m.expiresAt.Sub(time.Now())
	remainingStr := ""
	if remaining > 0 {
		hours := int(remaining.Hours())
		minutes := int(remaining.Minutes()) % 60
		if hours > 0 {
			remainingStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			remainingStr = fmt.Sprintf("%dm", minutes)
		}
	} else {
		remainingStr = "expired"
	}

	return Status{
		Active:        m.running,
		Connecting:    m.connecting,
		URL:           m.url,
		StartedAt:     m.startedAt,
		ExpiresAt:     m.expiresAt,
		RemainingTime: remainingStr,
		RenewedCount:  m.renewedCount,
		Provider:      ProviderNgrok,
	}
}

// GetURL returns the current tunnel URL.
func (m *NgrokManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *NgrokManager) GetProvider() Provider {
	return ProviderNgrok
}

// SetOnURLChange sets a callback for URL changes.
func (m *NgrokManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *NgrokManager) SetOnError(fn func(err error)) {
	m.onError = fn
}

// IncrementRenewedCount increments the renewal counter.
func (m *NgrokManager) IncrementRenewedCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renewedCount++
}

// ResetExpiry resets the expiry time.
func (m *NgrokManager) ResetExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour)
}
