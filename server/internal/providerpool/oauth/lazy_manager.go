package oauth

import (
	"context"
	"sync"
	"time"
)

// RuntimeManager is the OAuth manager surface used by provider/pipeline runtime code.
type RuntimeManager interface {
	StartAuth(ctx context.Context, providerID, providerType string) (*AuthStartResult, error)
	HandleCallback(ctx context.Context, state, code string) (*Token, error)
	CompleteDeviceFlow(ctx context.Context, deviceCode string, interval int) (*Token, error)
	GetAccessToken(providerID string) (string, error)
	GetCopilotAccessToken(providerID string) (string, error)
	GetCopilotEndpoint() string
	GetAccessTokenByID(providerID, tokenID string) (string, error)
	GetTokens(providerID string) ([]*Token, error)
	Disconnect(providerID, tokenID string) error
	ImportToken(token *Token) error
}

// LazyManager defers OAuth manager construction until first use or an explicit warmup.
// Initialization retries on failure so transient startup DB contention does not permanently
// disable OAuth features for the process lifetime.
type LazyManager struct {
	initFn func() (*Manager, error)

	mu           sync.Mutex
	cond         *sync.Cond
	manager      *Manager
	initializing bool

	port                  int
	autoRefreshCtx        context.Context
	autoRefreshInterval   time.Duration
	autoRefreshConfigured bool
	onReady               []func(*Manager)
}

// NewLazyManager creates a new lazy OAuth manager wrapper.
func NewLazyManager(initFn func() (*Manager, error)) *LazyManager {
	lm := &LazyManager{initFn: initFn}
	lm.cond = sync.NewCond(&lm.mu)
	return lm
}

func (m *LazyManager) ensure() (*Manager, error) {
	for {
		m.mu.Lock()
		for m.initializing && m.manager == nil {
			m.cond.Wait()
		}
		if m.manager != nil {
			manager := m.manager
			m.mu.Unlock()
			return manager, nil
		}
		m.initializing = true
		m.mu.Unlock()

		manager, err := m.initFn()

		m.mu.Lock()
		m.initializing = false
		if err == nil {
			if m.port > 0 {
				manager.SetPort(m.port)
			}
			if m.autoRefreshConfigured {
				manager.StartAutoRefresh(m.autoRefreshCtx, m.autoRefreshInterval)
			}
			m.manager = manager
			callbacks := append([]func(*Manager){}, m.onReady...)
			m.onReady = nil
			m.cond.Broadcast()
			m.mu.Unlock()

			for _, cb := range callbacks {
				cb(manager)
			}
			return manager, nil
		}
		m.cond.Broadcast()
		m.mu.Unlock()
		return nil, err
	}
}

// WarmAsync starts best-effort initialization in the background.
func (m *LazyManager) WarmAsync() {
	go func() {
		_, _ = m.ensure()
	}()
}

// IsReady reports whether the real OAuth manager has been initialized successfully.
func (m *LazyManager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.manager != nil
}

// OnReady registers a callback that runs once the real manager is available.
func (m *LazyManager) OnReady(cb func(*Manager)) {
	if cb == nil {
		return
	}
	m.mu.Lock()
	if m.manager != nil {
		manager := m.manager
		m.mu.Unlock()
		cb(manager)
		return
	}
	m.onReady = append(m.onReady, cb)
	m.mu.Unlock()
}

// SetPort stores the callback port and applies it immediately when the manager is ready.
func (m *LazyManager) SetPort(port int) {
	m.mu.Lock()
	m.port = port
	manager := m.manager
	m.mu.Unlock()
	if manager != nil && port > 0 {
		manager.SetPort(port)
	}
}

// StartAutoRefresh records the refresh policy and applies it once the manager is ready.
func (m *LazyManager) StartAutoRefresh(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	m.mu.Lock()
	m.autoRefreshCtx = ctx
	m.autoRefreshInterval = interval
	m.autoRefreshConfigured = true
	manager := m.manager
	m.mu.Unlock()
	if manager != nil {
		manager.StartAutoRefresh(ctx, interval)
	}
}

func (m *LazyManager) StartAuth(ctx context.Context, providerID, providerType string) (*AuthStartResult, error) {
	manager, err := m.ensure()
	if err != nil {
		return nil, err
	}
	return manager.StartAuth(ctx, providerID, providerType)
}

func (m *LazyManager) HandleCallback(ctx context.Context, state, code string) (*Token, error) {
	manager, err := m.ensure()
	if err != nil {
		return nil, err
	}
	return manager.HandleCallback(ctx, state, code)
}

func (m *LazyManager) CompleteDeviceFlow(ctx context.Context, deviceCode string, interval int) (*Token, error) {
	manager, err := m.ensure()
	if err != nil {
		return nil, err
	}
	return manager.CompleteDeviceFlow(ctx, deviceCode, interval)
}

func (m *LazyManager) GetAccessToken(providerID string) (string, error) {
	manager, err := m.ensure()
	if err != nil {
		return "", err
	}
	return manager.GetAccessToken(providerID)
}

func (m *LazyManager) GetCopilotAccessToken(providerID string) (string, error) {
	manager, err := m.ensure()
	if err != nil {
		return "", err
	}
	return manager.GetCopilotAccessToken(providerID)
}

func (m *LazyManager) GetCopilotEndpoint() string {
	manager, err := m.ensure()
	if err != nil {
		return ""
	}
	return manager.GetCopilotEndpoint()
}

func (m *LazyManager) GetAccessTokenByID(providerID, tokenID string) (string, error) {
	manager, err := m.ensure()
	if err != nil {
		return "", err
	}
	return manager.GetAccessTokenByID(providerID, tokenID)
}

func (m *LazyManager) GetTokens(providerID string) ([]*Token, error) {
	manager, err := m.ensure()
	if err != nil {
		return nil, err
	}
	return manager.GetTokens(providerID)
}

func (m *LazyManager) Disconnect(providerID, tokenID string) error {
	manager, err := m.ensure()
	if err != nil {
		return err
	}
	return manager.Disconnect(providerID, tokenID)
}

func (m *LazyManager) ImportToken(token *Token) error {
	manager, err := m.ensure()
	if err != nil {
		return err
	}
	return manager.ImportToken(token)
}
