package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServeoManager manages Serveo tunnels via native Go SSH (no system ssh required).
type ServeoManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cleanup   func()

	onURLChange func(url string)
	onError     func(err error)
}

// NewServeoManager creates a new Serveo tunnel manager.
func NewServeoManager() *ServeoManager {
	return &ServeoManager{}
}

// Start starts the Serveo tunnel using native Go SSH (works on all platforms).
func (m *ServeoManager) Start(ctx context.Context, cfg *Config) error {
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

	// Create independent context for long-running tunnel operation
	// This prevents parent context cancellation from stopping the tunnel
	tunnelCtx, cancel := context.WithCancel(context.Background())
	url, cleanup, err := startServeoNativeSSH(tunnelCtx, port, cfg.Subdomain)
	if err != nil {
		cancel()
		return err
	}

	m.mu.Lock()
	m.running = true
	m.url = url
	m.startedAt = time.Now()
	m.cleanup = func() {
		cleanup()
		cancel()
	}
	m.mu.Unlock()

	if m.onURLChange != nil {
		m.onURLChange(url)
	}
	return nil
}

// Stop stops the tunnel.
func (m *ServeoManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.cleanup != nil {
		m.cleanup()
		m.cleanup = nil
	}

	m.running = false
	m.url = ""
	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *ServeoManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *ServeoManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:     m.running && m.url != "",
		Connecting: m.running && m.url == "",
		URL:        m.url,
		StartedAt:  m.startedAt,
		Provider:   ProviderServeo,
	}
}

// GetURL returns the current tunnel URL.
func (m *ServeoManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *ServeoManager) GetProvider() Provider {
	return ProviderServeo
}

// SetOnURLChange sets a callback for URL changes.
func (m *ServeoManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *ServeoManager) SetOnError(fn func(err error)) {
	m.onError = fn
}
