package lifecycle

import (
	"context"
	"sync"
)

type Manager struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan struct{}
	mu         sync.Mutex
	hooks      []func(context.Context) error
}

func New() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		ctx:        ctx,
		cancel:     cancel,
		shutdownCh: make(chan struct{}),
		hooks:      make([]func(context.Context) error, 0),
	}
}

func (m *Manager) Context() context.Context {
	return m.ctx
}

func (m *Manager) Done() <-chan struct{} {
	return m.ctx.Done()
}

func (m *Manager) ShutdownCh() <-chan struct{} {
	return m.shutdownCh
}

func (m *Manager) RegisterShutdownHook(hook func(context.Context) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook)
}

func (m *Manager) Go(fn func(context.Context)) {
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		fn(m.ctx)
	}()
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.cancel()

	// Run shutdown hooks in reverse order
	m.mu.Lock()
	hooks := make([]func(context.Context) error, len(m.hooks))
	copy(hooks, m.hooks)
	m.mu.Unlock()

	var lastErr error
	for i := len(hooks) - 1; i >= 0; i-- {
		if err := hooks[i](ctx); err != nil {
			lastErr = err
		}
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(m.shutdownCh)
		return lastErr
	case <-ctx.Done():
		close(m.shutdownCh)
		return ctx.Err()
	}
}

func (m *Manager) Wait() {
	m.wg.Wait()
}
