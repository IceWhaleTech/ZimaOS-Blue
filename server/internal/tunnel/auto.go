package tunnel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// AutoManager tries multiple tunnel providers in order until one succeeds.
type AutoManager struct {
	mu            sync.RWMutex
	running       bool
	connecting    bool
	activeManager Manager
	url           string
	startedAt     time.Time
	expiresAt     time.Time
	renewedCount  int
	lastError     error

	onURLChange func(url string)
	onError     func(err error)

	// Providers to try in order (no auth required first)
	providerOrder []Provider
}

// NewAutoManager creates a new auto tunnel manager.
// Auto uses Cloudflare Quick Tunnel.
func NewAutoManager() *AutoManager {
	return &AutoManager{
		providerOrder: []Provider{
			ProviderCloudflare,
		},
	}
}

type connectResult struct {
	manager Manager
	url     string
}

// Start tries to start a tunnel by attempting all providers in parallel.
// The first one to connect successfully is used; the others are stopped.
func (m *AutoManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running || m.connecting {
		// If active manager has stopped, clean up and allow restart
		if m.activeManager == nil || !m.activeManager.IsRunning() {
			m.running = false
			m.activeManager = nil
		} else {
			m.mu.Unlock()
			return fmt.Errorf("tunnel already running")
		}
	}
	m.connecting = true
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	successChan := make(chan connectResult, 1)
	errChan := make(chan error, len(m.providerOrder))
	var wg sync.WaitGroup

	// Forward URL to Auto's callback so handler gets notified as soon as any provider has URL
	onURL := m.onURLChange
	for _, provider := range m.providerOrder {
		var manager Manager
		switch provider {
		case ProviderCloudflare:
			manager = NewCloudflareManager()
		default:
			continue
		}
		if onURL != nil {
			manager.SetOnURLChange(onURL)
		}

		wg.Add(1)
		go func(prov Provider, mgr Manager) {
			defer wg.Done()

			providerCfg := &Config{
				Provider:  prov,
				Port:      cfg.Port,
				Subdomain: cfg.Subdomain,
			}

			err := mgr.Start(ctx, providerCfg)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("%s: %w", prov, err):
				case <-ctx.Done():
				}
				return
			}

			// Poll for URL; first to get one wins
			for i := 0; i < 30; i++ {
				select {
				case <-ctx.Done():
					mgr.Stop()
					return
				case <-time.After(500 * time.Millisecond):
				}

				if mgr.IsRunning() && mgr.GetURL() != "" {
					url := mgr.GetURL()
					select {
					case successChan <- connectResult{mgr, url}:
						return // we won, main will use our manager
					default:
						mgr.Stop() // someone else won
						return
					}
				}

				if !mgr.IsRunning() {
					select {
					case errChan <- fmt.Errorf("%s: connection failed or timed out", prov):
					case <-ctx.Done():
					}
					return
				}
			}

			mgr.Stop()
			select {
			case errChan <- fmt.Errorf("%s: connection failed or timed out", prov):
			case <-ctx.Done():
			}
		}(provider, manager)
	}

	// Wait for first success or all failures
	var errs []error
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	for {
		select {
		case r := <-successChan:
			cancel()
			<-done
			m.mu.Lock()
			m.running = true
			m.connecting = false
			m.activeManager = r.manager
			m.url = r.url
			m.startedAt = time.Now()
			m.expiresAt = m.startedAt.Add(8 * time.Hour)
			m.mu.Unlock()
			// Note: onURLChange already fired by sub-manager (line 81 forwards it)
			return nil

		case err := <-errChan:
			errs = append(errs, err)

		case <-done:
			// Drain any remaining errors
			for {
				select {
				case err := <-errChan:
					errs = append(errs, err)
				default:
					goto exitLoop
				}
			}
		exitLoop:
			m.mu.Lock()
			m.connecting = false
			m.lastError = errors.Join(errs...)
			m.mu.Unlock()
			if m.onError != nil && len(errs) > 0 {
				m.onError(m.lastError)
			}
			if len(errs) > 0 {
				msgs := make([]string, len(errs))
				for i, e := range errs {
					msgs[i] = e.Error()
				}
				return fmt.Errorf("all providers failed: %s", strings.Join(msgs, "; "))
			}
			return fmt.Errorf("no providers available")

		case <-ctx.Done():
			<-done
			return ctx.Err()
		}
	}
}

// Stop stops the active tunnel.
func (m *AutoManager) Stop() error {
	m.mu.Lock()
	activeManager := m.activeManager
	m.mu.Unlock()

	if activeManager != nil {
		if err := activeManager.Stop(); err != nil {
			return err
		}
	}

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.url = ""
	m.activeManager = nil
	m.mu.Unlock()

	return nil
}

// IsRunning returns true if a tunnel is running.
func (m *AutoManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running && m.activeManager != nil && m.activeManager.IsRunning()
}

// GetStatus returns the current tunnel status.
func (m *AutoManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.connecting {
		return Status{
			Active:     false,
			Connecting: true,
			Provider:   ProviderAuto,
		}
	}

	if !m.running || m.activeManager == nil {
		return Status{Active: false, Provider: ProviderAuto}
	}

	// Get status from active manager (URL comes from e.g. Bore's m.url)
	status := m.activeManager.GetStatus()
	// Override provider to show "auto" but include actual provider info
	status.Provider = ProviderAuto
	// Use AutoManager's own timing info (sub-managers may not track these)
	status.StartedAt = m.startedAt
	status.ExpiresAt = m.expiresAt

	return status
}

// GetURL returns the current tunnel URL.
func (m *AutoManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeManager != nil {
		return m.activeManager.GetURL()
	}
	return m.url
}

// GetProvider returns the provider type.
func (m *AutoManager) GetProvider() Provider {
	return ProviderAuto
}

// GetActiveProvider returns the actual active provider.
func (m *AutoManager) GetActiveProvider() Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.activeManager != nil {
		return m.activeManager.GetProvider()
	}
	return ProviderAuto
}

// SetOnURLChange sets a callback for URL changes.
func (m *AutoManager) SetOnURLChange(fn func(url string)) {
	m.onURLChange = fn
}

// SetOnError sets a callback for errors.
func (m *AutoManager) SetOnError(fn func(err error)) {
	m.onError = fn
}
