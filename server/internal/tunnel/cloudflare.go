package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wizzard0/trycloudflared"
)

// CloudflareManager manages Cloudflare Tunnel connections using native Go implementation.
type CloudflareManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewCloudflareManager creates a new Cloudflare Tunnel manager.
func NewCloudflareManager() *CloudflareManager {
	return &CloudflareManager{}
}

// Start starts the Cloudflare Tunnel using native Go implementation.
func (m *CloudflareManager) Start(ctx context.Context, cfg *Config) error {
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

	// Create independent context for the tunnel
	// This prevents panics when the parent context is canceled by Auto manager
	tunnelCtx, cancel := context.WithCancel(context.Background())

	// Note: trycloudflared only supports quick tunnel (no token-based auth)
	// For token-based tunnels, users should use the standalone Cloudflare provider
	// with the cloudflared binary

	// Start tunnel in background
	go func() {
		// Recover from panics in the trycloudflared library
		defer func() {
			if r := recover(); r != nil {
				m.mu.Lock()
				m.running = false
				m.url = ""
				m.mu.Unlock()

				var err error
				switch v := r.(type) {
				case error:
					err = fmt.Errorf("cloudflare tunnel panic: %w", v)
				case string:
					err = fmt.Errorf("cloudflare tunnel panic: %s", v)
				default:
					err = fmt.Errorf("cloudflare tunnel panic: %v", v)
				}

				if m.onError != nil {
					m.onError(err)
				}
				cancel()
			}
		}()

		// Create Cloudflare tunnel
		tunnelURL, err := trycloudflared.CreateCloudflareTunnel(tunnelCtx, port)
		if err != nil {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			if m.onError != nil {
				m.onError(fmt.Errorf("failed to create cloudflare tunnel: %w", err))
			}
			cancel()
			return
		}

		m.mu.Lock()
		m.url = tunnelURL
		m.mu.Unlock()

		// Notify URL change
		if m.onURLChange != nil {
			m.onURLChange(tunnelURL)
		}

		// Wait for context cancellation
		<-tunnelCtx.Done()
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()
	}()

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.startedAt = time.Now()
	m.mu.Unlock()

	return nil
}

// Stop stops the tunnel.
func (m *CloudflareManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
	}

	m.running = false
	m.url = ""
	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *CloudflareManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *CloudflareManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:    m.running && m.url != "",
		URL:       m.url,
		StartedAt: m.startedAt,
		Provider:  ProviderCloudflare,
	}
}

// GetURL returns the current tunnel URL.
func (m *CloudflareManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *CloudflareManager) GetProvider() Provider {
	return ProviderCloudflare
}

// SetOnURLChange sets a callback for URL changes.
func (m *CloudflareManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *CloudflareManager) SetOnError(fn func(err error)) {
	m.onError = fn
}
