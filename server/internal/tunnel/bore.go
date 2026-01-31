package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BoreManager manages Bore tunnels using the native Go implementation of the bore protocol.
// Compatible with bore.pub (https://github.com/ekzhang/bore).
const boreServer = "bore.pub"

// BoreManager implements Manager for bore.
type BoreManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc

	onURLChange func(url string)
	onError     func(err error)
}

// NewBoreManager creates a new Bore tunnel manager.
func NewBoreManager() *BoreManager {
	return &BoreManager{}
}

// Start runs the bore client in-process (native Go protocol); no external binary required.
func (m *BoreManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	m.mu.Unlock()

	port := cfg.Port
	if port == 0 {
		port = 8080
	}

	ctx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.startedAt = time.Now()
	m.mu.Unlock()

	go func() {
		err := boreClient(ctx, boreServer, port, func(urlStr string) {
			m.mu.Lock()
			if m.url == "" {
				m.url = urlStr
				m.mu.Unlock()
				if m.onURLChange != nil {
					m.onURLChange(urlStr)
				}
			} else {
				m.mu.Unlock()
			}
		})
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()
		if err != nil && err != context.Canceled && m.onError != nil {
			m.onError(err)
		}
	}()

	return nil
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
