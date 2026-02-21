package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/wizzard0/trycloudflared"
	"golang.org/x/sync/singleflight"
)

// CloudflareManager manages Cloudflare Quick Tunnel via the trycloudflared SDK.
type CloudflareManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc
	done      chan struct{} // closed when the tunnel goroutine exits
	sf        singleflight.Group

	onURLChange func(url string)
	onError     func(err error)
}

func NewCloudflareManager() *CloudflareManager { return &CloudflareManager{} }

// tolerantRegisterer wraps a prometheus.Registerer to silently ignore
// duplicate collector registrations (AlreadyRegisteredError).
// This is needed because trycloudflared.CreateCloudflareTunnel calls
// metrics.RegisterBuildInfo on every invocation, which panics on restart.
type tolerantRegisterer struct {
	prometheus.Registerer
}

func (t tolerantRegisterer) Register(c prometheus.Collector) error {
	err := t.Registerer.Register(c)
	if err != nil {
		// Ignore AlreadyRegisteredError — the collector is already there
		if _, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return nil
		}
	}
	return err
}

func (t tolerantRegisterer) MustRegister(cs ...prometheus.Collector) {
	for _, c := range cs {
		_ = t.Register(c) // silently ignore duplicates
	}
}

var tolerantOnce sync.Once

// installTolerantRegisterer permanently wraps prometheus.DefaultRegisterer
// so that duplicate collector registrations (from tunnel restarts) are
// silently ignored instead of panicking.
func installTolerantRegisterer() {
	tolerantOnce.Do(func() {
		prometheus.DefaultRegisterer = tolerantRegisterer{prometheus.DefaultRegisterer}
	})
}

func (m *CloudflareManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("tunnel already running")
	}
	m.mu.Unlock()

	port := cfg.Port
	if port == 0 {
		port = 80
	}

	_, err, _ := m.sf.Do("cloudflare-tunnel", func() (interface{}, error) {
		return m.startTunnelInternal(port)
	})
	return err
}

func (m *CloudflareManager) startTunnelInternal(port int) (interface{}, error) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil, nil
	}
	m.mu.Unlock()

	tunnelCtx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
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
				slog.Error("[tunnel] cloudflare panic recovered", "error", err)
				if m.onError != nil {
					m.onError(err)
				}
				cancel()
			}
		}()

		// Install a tolerant Prometheus registerer permanently. We can't
		// restore the original after CreateCloudflareTunnel because it spawns
		// a child goroutine that calls MustRegister asynchronously.
		installTolerantRegisterer()
		tunnelURL, err := trycloudflared.CreateCloudflareTunnel(tunnelCtx, port)
		if err != nil {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
			if m.onError != nil {
				m.onError(fmt.Errorf("failed to create cloudflare tunnel: %w", err))
			}
			resultCh <- err
			cancel()
			return
		}

		m.mu.Lock()
		m.url = tunnelURL
		m.mu.Unlock()
		resultCh <- nil
		if m.onURLChange != nil {
			m.onURLChange(tunnelURL)
		}

		<-tunnelCtx.Done()
		// Wait for cloudflared daemon to finish its grace period shutdown
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()
	}()

	m.mu.Lock()
	m.running = true
	m.cancel = cancel
	m.done = done
	m.startedAt = time.Now()
	m.mu.Unlock()

	select {
	case err := <-resultCh:
		if err != nil {
			m.mu.Lock()
			m.running = false
			m.cancel = nil
			m.mu.Unlock()
			return nil, err
		}
		return nil, nil
	case <-time.After(30 * time.Second):
		return nil, nil
	}
}

func (m *CloudflareManager) Stop() error {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil
	}
	if m.cancel != nil {
		m.cancel()
	}
	done := m.done
	m.mu.Unlock()

	// Wait for the tunnel goroutine to finish
	if done != nil {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
		}
	}

	m.mu.Lock()
	m.running = false
	m.url = ""
	m.mu.Unlock()
	return nil
}

func (m *CloudflareManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *CloudflareManager) GetStatus() Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return Status{Active: m.running && m.url != "", URL: m.url, StartedAt: m.startedAt, Provider: ProviderCloudflare}
}

func (m *CloudflareManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

func (m *CloudflareManager) GetProvider() Provider          { return ProviderCloudflare }
func (m *CloudflareManager) SetOnURLChange(fn func(string)) { m.onURLChange = fn }
func (m *CloudflareManager) SetOnError(fn func(error))      { m.onError = fn }
