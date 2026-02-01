package tunnel

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LocalTunnelManager manages LocalTunnel (localtunnel.me / loca.lt) via native Go client.
// No Node.js or npm required; implements the localtunnel protocol in Go.
// For loca.lt tunnels, visitors need the "password" (creator's public IP) from https://loca.lt/mytunnelpassword.
// See https://github.com/localtunnel/localtunnel
type LocalTunnelManager struct {
	mu             sync.RWMutex
	running        bool
	url            string
	tunnelPassword string // For loca.lt: IP from mytunnelpassword
	startedAt      time.Time
	cancel         context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewLocalTunnelManager creates a new LocalTunnel manager (pure Go).
func NewLocalTunnelManager() *LocalTunnelManager {
	return &LocalTunnelManager{}
}

// Start starts the LocalTunnel using the native Go protocol: GET tunnel info, then TCP proxy.
func (m *LocalTunnelManager) Start(ctx context.Context, cfg *Config) error {
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
	subdomain := cfg.Subdomain
	host := localtunnelDefaultHost

	// Use passed ctx only for initial request (to respect caller's timeout/cancel during setup)
	info, err := requestLocaltunnel(ctx, host, subdomain)
	if err != nil {
		return fmt.Errorf("localtunnel request: %w", err)
	}

	// Create independent context for long-running tunnel operation
	// This prevents parent context cancellation from stopping the tunnel
	tunnelCtx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.running = true
	m.url = info.URL
	m.startedAt = time.Now()
	m.cancel = cancel
	m.mu.Unlock()

	// For loca.lt, fetch the "password" (creator's public IP) for visitors
	if strings.Contains(info.URL, "loca.lt") {
		go m.fetchLocaLtPassword()
	}

	if m.onURLChange != nil {
		m.onURLChange(info.URL)
	}

	go func() {
		err := runLocaltunnelClient(tunnelCtx, info, port, func(u string) {
			m.mu.Lock()
			if m.url == "" {
				m.url = u
				m.mu.Unlock()
				if m.onURLChange != nil {
					m.onURLChange(u)
				}
			} else {
				m.mu.Unlock()
			}
		})
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.tunnelPassword = ""
		m.mu.Unlock()
		if err != nil && err != context.Canceled && m.onError != nil {
			m.onError(err)
		}
	}()

	return nil
}

// Stop stops the tunnel.
func (m *LocalTunnelManager) Stop() error {
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
	m.tunnelPassword = ""
	return nil
}

// IsRunning returns true if the tunnel is running.
func (m *LocalTunnelManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *LocalTunnelManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:         m.running && m.url != "",
		Connecting:     m.running && m.url == "",
		URL:            m.url,
		TunnelPassword: m.tunnelPassword,
		StartedAt:      m.startedAt,
		Provider:       ProviderLocalTunnel,
	}
}

// GetURL returns the current tunnel URL.
func (m *LocalTunnelManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *LocalTunnelManager) GetProvider() Provider {
	return ProviderLocalTunnel
}

// SetOnURLChange sets a callback for URL changes.
func (m *LocalTunnelManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *LocalTunnelManager) SetOnError(fn func(err error)) {
	m.onError = fn
}

// CheckLocaltunnelAvailable reports whether the native Go client can run (no external deps).
func CheckLocaltunnelAvailable() bool {
	return true
}

// fetchLocaLtPassword fetches the tunnel creator's public IP from loca.lt/mytunnelpassword.
// Visitors must enter this IP to access the tunnel.
func (m *LocalTunnelManager) fetchLocaLtPassword() {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://loca.lt/mytunnelpassword")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return
	}
	password := strings.TrimSpace(string(body))
	if password != "" {
		m.mu.Lock()
		m.tunnelPassword = password
		m.mu.Unlock()
	}
}
