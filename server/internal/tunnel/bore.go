package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BoreManager manages Bore tunnels using the native Go implementation of the bore protocol.
// Compatible with bore.pub (https://github.com/ekzhang/bore).
const (
	boreServer               = "bore.pub"
	boreMaxReconnectDelay    = 60 * time.Second // Maximum delay between reconnection attempts
	boreInitReconnectDelay   = 2 * time.Second  // Initial delay between reconnection attempts
	boreMaxReconnectAttempts = 5                // Maximum reconnection attempts before giving up
)

// BoreManager implements Manager for bore.
type BoreManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc
	port      int // Store port for reconnection

	onURLChange func(url string)
	onError     func(err error)
}

// NewBoreManager creates a new Bore tunnel manager.
func NewBoreManager() *BoreManager {
	return &BoreManager{}
}

// Start runs the bore client in-process (native Go protocol); no external binary required.
// Includes automatic reconnection with exponential backoff.
func (m *BoreManager) Start(ctx context.Context, cfg *Config) error {
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

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.startedAt = time.Now()
	m.port = port
	m.mu.Unlock()

	go m.runWithReconnect(tunnelCtx, port)

	return nil
}

// runWithReconnect runs the bore client with automatic reconnection on failure.
func (m *BoreManager) runWithReconnect(ctx context.Context, port int) {
	reconnectDelay := boreInitReconnectDelay
	reconnectAttempts := 0

	for {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.running = false
			m.url = ""
			m.mu.Unlock()
			return
		default:
		}

		err := boreClient(ctx, boreServer, port, func(urlStr string) {
			m.mu.Lock()
			oldURL := m.url
			m.url = urlStr
			m.mu.Unlock()

			// Notify on URL change (including reconnection with new URL)
			if m.onURLChange != nil && oldURL != urlStr {
				m.onURLChange(urlStr)
			}

			// Reset reconnect attempts on successful connection
			reconnectAttempts = 0
			reconnectDelay = boreInitReconnectDelay
		})

		// Check if context was cancelled (intentional stop)
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.running = false
			m.url = ""
			m.mu.Unlock()
			return
		default:
		}

		// Connection failed or dropped
		m.mu.Lock()
		m.url = "" // Clear URL while disconnected
		m.mu.Unlock()

		reconnectAttempts++
		if reconnectAttempts > boreMaxReconnectAttempts {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			if err != nil && m.onError != nil {
				m.onError(fmt.Errorf("bore: max reconnection attempts (%d) exceeded: %w", boreMaxReconnectAttempts, err))
			}
			return
		}

		// Log reconnection attempt (via error callback for visibility)
		if m.onError != nil {
			m.onError(fmt.Errorf("bore: connection lost, reconnecting in %v (attempt %d/%d): %v",
				reconnectDelay, reconnectAttempts, boreMaxReconnectAttempts, err))
		}

		// Wait before reconnecting with exponential backoff
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.running = false
			m.url = ""
			m.mu.Unlock()
			return
		case <-time.After(reconnectDelay):
		}

		// Exponential backoff: double the delay up to max
		reconnectDelay *= 2
		if reconnectDelay > boreMaxReconnectDelay {
			reconnectDelay = boreMaxReconnectDelay
		}
	}
}

// Stop stops the tunnel.
func (m *BoreManager) Stop() error {
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
func (m *BoreManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetStatus returns the current tunnel status.
func (m *BoreManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return Status{
		Active:     m.running && m.url != "",
		Connecting: m.running && m.url == "",
		URL:        m.url,
		StartedAt:  m.startedAt,
		Provider:   ProviderBore,
	}
}

// GetURL returns the current tunnel URL.
func (m *BoreManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

// GetProvider returns the provider type.
func (m *BoreManager) GetProvider() Provider {
	return ProviderBore
}

// SetOnURLChange sets a callback for URL changes.
func (m *BoreManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *BoreManager) SetOnError(fn func(err error)) {
	m.onError = fn
}

// CheckBoreAvailable reports whether bore is available. With native Go client, always true.
func CheckBoreAvailable() bool {
	return true
}
