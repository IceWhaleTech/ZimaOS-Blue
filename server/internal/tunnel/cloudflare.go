package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/prometheus/client_golang/prometheus"
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

	port, err := resolveTargetPort(cfg)
	if err != nil {
		return err
	}

	_, err, _ = m.sf.Do("cloudflare-tunnel", func() (interface{}, error) {
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
	done := make(chan struct{})

	// Install a tolerant Prometheus registerer permanently. We can't
	// restore the original after the tunnel starts because cloudflared
	// calls MustRegister asynchronously from child goroutines.
	installTolerantRegisterer()

	result, err := createCloudflareTunnelSafe(tunnelCtx, port)
	if err != nil {
		cancel()
		if m.onError != nil {
			m.onError(fmt.Errorf("failed to create cloudflare tunnel: %w", err))
		}
		return nil, err
	}

	// Monitor the daemon in a background goroutine.
	go func() {
		defer close(done)
		select {
		case daemonErr := <-result.DaemonErrCh:
			if daemonErr != nil {
				slog.Warn("[tunnel] cloudflare daemon exited", "error", daemonErr)
				if m.onError != nil {
					m.onError(daemonErr)
				}
			}
		case <-tunnelCtx.Done():
		}
		// Grace period for cloudflared shutdown
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		m.running = false
		m.url = ""
		m.mu.Unlock()
	}()

	m.mu.Lock()
	m.running = true
	m.url = result.URL
	m.cancel = cancel
	m.done = done
	m.startedAt = timeutil.NowTime()
	m.mu.Unlock()

	if m.onURLChange != nil {
		m.onURLChange(result.URL)
	}

	return nil, nil
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
