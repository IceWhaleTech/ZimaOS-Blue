package tunnel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.ngrok.com/ngrok/v2"
)

// NgrokManager manages ngrok tunnels via the ngrok Go SDK.
type NgrokManager struct {
	mu           sync.RWMutex
	running      bool
	connecting   bool
	url          string
	startedAt    time.Time
	expiresAt    time.Time
	renewedCount int
	cancelFunc   context.CancelFunc
	forwarder    ngrok.EndpointForwarder

	onURLChange func(url string)
	onError     func(err error)
}

func NewNgrokManager() *NgrokManager { return &NgrokManager{} }

func (m *NgrokManager) Start(ctx context.Context, cfg *Config) error {
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

	tunnelCtx, cancel := context.WithCancel(ctx)

	// Create agent with authtoken
	agentOpts := []ngrok.AgentOption{}
	if cfg.NgrokAuthtoken != "" {
		agentOpts = append(agentOpts, ngrok.WithAuthtoken(cfg.NgrokAuthtoken))
	}
	agent, err := ngrok.NewAgent(agentOpts...)
	if err != nil {
		cancel()
		return fmt.Errorf("ngrok agent: %w", err)
	}

	// Build endpoint options
	endpointOpts := []ngrok.EndpointOption{}
	if cfg.NgrokDomain != "" {
		endpointOpts = append(endpointOpts, ngrok.WithURL(fmt.Sprintf("https://%s", cfg.NgrokDomain)))
	}

	upstream := ngrok.WithUpstream(fmt.Sprintf("localhost:%d", port))
	fwd, err := agent.Forward(tunnelCtx, upstream, endpointOpts...)
	if err != nil {
		cancel()
		return fmt.Errorf("ngrok forward: %w", err)
	}

	tunnelURL := fwd.URL().String()

	m.mu.Lock()
	m.running = true
	m.connecting = false
	m.url = tunnelURL
	m.forwarder = fwd
	m.cancelFunc = cancel
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour)
	m.mu.Unlock()

	if m.onURLChange != nil {
		m.onURLChange(tunnelURL)
	}

	// Monitor forwarder lifecycle
	go func() {
		<-fwd.Done()
		m.mu.Lock()
		m.running = false
		m.connecting = false
		m.url = ""
		m.mu.Unlock()
	}()

	return nil
}

func (m *NgrokManager) Stop() error {
	m.mu.Lock()
	wasRunning := m.running
	cancelFunc := m.cancelFunc
	m.mu.Unlock()

	if !wasRunning {
		return nil
	}
	if cancelFunc != nil {
		cancelFunc()
	}

	m.mu.Lock()
	m.running = false
	m.connecting = false
	m.url = ""
	m.forwarder = nil
	m.cancelFunc = nil
	m.mu.Unlock()
	return nil
}

func (m *NgrokManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

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

func (m *NgrokManager) GetURL() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.url
}

func (m *NgrokManager) GetProvider() Provider          { return ProviderNgrok }
func (m *NgrokManager) SetOnURLChange(fn func(string)) { m.onURLChange = fn }
func (m *NgrokManager) SetOnError(fn func(error))      { m.onError = fn }

func (m *NgrokManager) IncrementRenewedCount() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.renewedCount++
}

func (m *NgrokManager) ResetExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startedAt = time.Now()
	m.expiresAt = m.startedAt.Add(8 * time.Hour)
}
