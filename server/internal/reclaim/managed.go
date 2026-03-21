package reclaim

import (
	"context"
	"sync"
	"time"
)

// Managed lazily creates a resource and reclaims it after a period of inactivity.
// Acquire keeps the resource alive until the returned release function is called.
// Get is a lighter-weight touch for callers that cannot hold a lease explicitly.
type Managed[T any] struct {
	mu        sync.Mutex
	factory   func() (T, error)
	reclaimer func(context.Context, T) error
	onCreate  func(T)

	idleAfter time.Duration
	timer     *time.Timer

	value T
	ready bool
	inUse int
}

// NewManaged creates a new managed lazy resource.
func NewManaged[T any](idleAfter time.Duration, factory func() (T, error), reclaimer func(context.Context, T) error) *Managed[T] {
	return &Managed[T]{
		idleAfter: idleAfter,
		factory:   factory,
		reclaimer: reclaimer,
	}
}

// SetIdleAfter updates the idle reclaim timeout.
func (m *Managed[T]) SetIdleAfter(idleAfter time.Duration) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.idleAfter = idleAfter
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	if m.ready && m.inUse == 0 {
		m.scheduleLocked()
	}
}

// SetOnCreate configures a callback invoked whenever a fresh resource is created.
func (m *Managed[T]) SetOnCreate(fn func(T)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.onCreate = fn
	value := m.value
	ready := m.ready
	m.mu.Unlock()

	if ready && fn != nil {
		fn(value)
	}
}

// SetOnCreateSilently updates the callback without invoking it for the current resource.
func (m *Managed[T]) SetOnCreateSilently(fn func(T)) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onCreate = fn
}

// Peek returns the current resource without creating it.
func (m *Managed[T]) Peek() (T, bool) {
	var zero T
	if m == nil {
		return zero, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.ready {
		return zero, false
	}
	return m.value, true
}

// Get returns the current resource, creating it if needed.
func (m *Managed[T]) Get() (T, error) {
	var zero T
	if m == nil {
		return zero, nil
	}

	value, created, onCreate, err := m.getLocked(false)
	if err != nil {
		return zero, err
	}
	if created && onCreate != nil {
		onCreate(value)
	}
	return value, nil
}

// Acquire returns the current resource, creating it if needed, and a release
// callback that must be called once the caller is done using it.
func (m *Managed[T]) Acquire() (T, func(), error) {
	var zero T
	if m == nil {
		return zero, func() {}, nil
	}

	value, created, onCreate, err := m.getLocked(true)
	if err != nil {
		return zero, nil, err
	}
	if created && onCreate != nil {
		onCreate(value)
	}

	var once sync.Once
	return value, func() {
		once.Do(func() {
			m.release()
		})
	}, nil
}

func (m *Managed[T]) getLocked(acquire bool) (T, bool, func(T), error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}

	if !m.ready {
		value, err := m.factory()
		if err != nil {
			var zero T
			return zero, false, nil, err
		}
		m.value = value
		m.ready = true
		if acquire {
			m.inUse++
		} else {
			m.scheduleLocked()
		}
		return value, true, m.onCreate, nil
	}

	value := m.value
	if acquire {
		m.inUse++
	} else {
		m.scheduleLocked()
	}
	return value, false, nil, nil
}

func (m *Managed[T]) release() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.inUse > 0 {
		m.inUse--
	}
	if m.inUse == 0 && m.ready {
		m.scheduleLocked()
	}
}

func (m *Managed[T]) scheduleLocked() {
	if m.idleAfter <= 0 || !m.ready {
		return
	}
	if m.timer != nil {
		m.timer.Stop()
	}
	m.timer = time.AfterFunc(m.idleAfter, m.reclaimIdle)
}

func (m *Managed[T]) reclaimIdle() {
	var (
		value T
		ok    bool
	)

	m.mu.Lock()
	if m.ready && m.inUse == 0 {
		value = m.value
		var zero T
		m.value = zero
		m.ready = false
		ok = true
	}
	m.timer = nil
	m.mu.Unlock()

	if ok && m.reclaimer != nil {
		_ = m.reclaimer(context.Background(), value)
	}
}

// Close reclaims the resource immediately if it exists.
func (m *Managed[T]) Close(ctx context.Context) error {
	var (
		value T
		ok    bool
	)

	if m == nil {
		return nil
	}

	m.mu.Lock()
	if m.timer != nil {
		m.timer.Stop()
		m.timer = nil
	}
	if m.ready {
		value = m.value
		var zero T
		m.value = zero
		m.ready = false
		ok = true
	}
	m.inUse = 0
	m.mu.Unlock()

	if ok && m.reclaimer != nil {
		return m.reclaimer(ctx, value)
	}
	return nil
}
