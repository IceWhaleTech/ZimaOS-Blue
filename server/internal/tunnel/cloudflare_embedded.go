//go:build embedded_cloudflared

package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wizzard0/trycloudflared"
	"golang.org/x/sync/singleflight"
)

type CloudflareManager struct {
	mu        sync.RWMutex
	running   bool
	url       string
	startedAt time.Time
	cancel    context.CancelFunc
	sf        singleflight.Group
	onURLChange func(url string)
	onError     func(err error)
}

func NewCloudflareManager() *CloudflareManager { return &CloudflareManager{} }

func (m *CloudflareManager) Start(ctx context.Context, cfg *Config) error {
	m.mu.Lock()
	if m.running { m.mu.Unlock(); return fmt.Errorf("tunnel already running") }
	m.mu.Unlock()
	port := cfg.Port
	if port == 0 { port = 23456 }
	_, err, _ := m.sf.Do("cloudflare-tunnel", func() (interface{}, error) {
		return m.startTunnelInternal(port)
	})
	return err
}

func (m *CloudflareManager) startTunnelInternal(port int) (interface{}, error) {
	m.mu.Lock()
	if m.running { m.mu.Unlock(); return nil, nil }
	m.mu.Unlock()
	tunnelCtx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.mu.Lock(); m.running = false; m.url = ""; m.mu.Unlock()
				var err error
				switch v := r.(type) {
				case error: err = fmt.Errorf("cloudflare tunnel panic: %w", v)
				case string: err = fmt.Errorf("cloudflare tunnel panic: %s", v)
				default: err = fmt.Errorf("cloudflare tunnel panic: %v", v)
				}
				if m.onError != nil { m.onError(err) }
				cancel()
			}
		}()
		tunnelURL, err := trycloudflared.CreateCloudflareTunnel(tunnelCtx, port)
		if err != nil {
			m.mu.Lock(); m.running = false; m.mu.Unlock()
			if m.onError != nil { m.onError(fmt.Errorf("failed to create cloudflare tunnel: %w", err)) }
			resultCh <- err; cancel(); return
		}
		m.mu.Lock(); m.url = tunnelURL; m.mu.Unlock()
		resultCh <- nil
		if m.onURLChange != nil { m.onURLChange(tunnelURL) }
		<-tunnelCtx.Done()
		m.mu.Lock(); m.running = false; m.url = ""; m.mu.Unlock()
	}()
	m.mu.Lock(); m.running = true; m.cancel = cancel; m.startedAt = time.Now(); m.mu.Unlock()
	select {
	case err := <-resultCh:
		if err != nil { m.mu.Lock(); m.running = false; m.cancel = nil; m.mu.Unlock(); return nil, err }
		return nil, nil
	case <-time.After(30 * time.Second):
		return nil, nil
	}
}

func (m *CloudflareManager) Stop() error {
	m.mu.Lock(); defer m.mu.Unlock()
	if !m.running { return nil }
	m.running = false; m.url = ""; return nil
}

func (m *CloudflareManager) IsRunning() bool { m.mu.RLock(); defer m.mu.RUnlock(); return m.running }
func (m *CloudflareManager) GetStatus() Status {
	m.mu.RLock(); defer m.mu.RUnlock()
	return Status{Active: m.running && m.url != "", URL: m.url, StartedAt: m.startedAt, Provider: ProviderCloudflare}
}
func (m *CloudflareManager) GetURL() string { m.mu.RLock(); defer m.mu.RUnlock(); return m.url }
func (m *CloudflareManager) GetProvider() Provider { return ProviderCloudflare }
func (m *CloudflareManager) SetOnURLChange(fn func(string)) { m.onURLChange = fn }
func (m *CloudflareManager) SetOnError(fn func(error)) { m.onError = fn }
