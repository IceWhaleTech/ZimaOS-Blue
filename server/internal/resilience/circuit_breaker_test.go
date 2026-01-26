package resilience

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	tests := []struct {
		name        string
		config      CircuitBreakerConfig
		wantMax     int
		wantTimeout time.Duration
	}{
		{
			name:        "default values",
			config:      CircuitBreakerConfig{},
			wantMax:     5,
			wantTimeout: 30 * time.Second,
		},
		{
			name: "custom values",
			config: CircuitBreakerConfig{
				MaxFailures: 10,
				Timeout:     60 * time.Second,
			},
			wantMax:     10,
			wantTimeout: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.config)
			if cb.config.MaxFailures != tt.wantMax {
				t.Errorf("MaxFailures = %d, want %d", cb.config.MaxFailures, tt.wantMax)
			}
			if cb.config.Timeout != tt.wantTimeout {
				t.Errorf("Timeout = %v, want %v", cb.config.Timeout, tt.wantTimeout)
			}
		})
	}
}

func TestCircuitBreakerState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 3,
		Timeout:     100 * time.Millisecond,
	})

	// Initial state should be closed
	if cb.State() != StateClosed {
		t.Errorf("initial state = %v, want %v", cb.State(), StateClosed)
	}

	// Cause failures to open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 3; i++ {
		cb.Execute(func() error { return testErr })
	}

	if cb.State() != StateOpen {
		t.Errorf("state after failures = %v, want %v", cb.State(), StateOpen)
	}

	// Wait for timeout to transition to half-open
	time.Sleep(150 * time.Millisecond)

	if cb.State() != StateHalfOpen {
		t.Errorf("state after timeout = %v, want %v", cb.State(), StateHalfOpen)
	}
}

func TestCircuitBreakerExecute(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 2,
		Timeout:     100 * time.Millisecond,
	})

	// Successful execution
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Failed executions
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	// Circuit should be open now
	err = cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerRecovery(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Successful request should close the circuit
	err := cb.Execute(func() error { return nil })
	if err != nil {
		t.Errorf("expected no error in half-open, got %v", err)
	}

	if cb.State() != StateClosed {
		t.Errorf("state after success = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreakerHalfOpenFailure(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// Failed request should re-open the circuit
	cb.Execute(func() error { return testErr })

	if cb.State() != StateOpen {
		t.Errorf("state after half-open failure = %v, want %v", cb.State(), StateOpen)
	}
}

func TestCircuitBreakerReset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 2,
	})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want %v", cb.State(), StateOpen)
	}

	// Reset should close the circuit
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("state after reset = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		Name:        "test-breaker",
		MaxFailures: 5,
	})

	// Execute some requests
	cb.Execute(func() error { return nil })
	cb.Execute(func() error { return errors.New("error") })

	stats := cb.Stats()

	if stats["name"] != "test-breaker" {
		t.Errorf("name = %v, want test-breaker", stats["name"])
	}
	if stats["state"] != "closed" {
		t.Errorf("state = %v, want closed", stats["state"])
	}
}

func TestCircuitBreakerOnStateChange(t *testing.T) {
	var (
		mu          sync.Mutex
		stateChanges []struct{ from, to State }
	)

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 2,
		Timeout:     50 * time.Millisecond,
		OnStateChange: func(name string, from, to State) {
			mu.Lock()
			stateChanges = append(stateChanges, struct{ from, to State }{from, to})
			mu.Unlock()
		},
	})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	// Wait for callback
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if len(stateChanges) != 1 {
		t.Errorf("expected 1 state change, got %d", len(stateChanges))
	}
	if len(stateChanges) > 0 && stateChanges[0].to != StateOpen {
		t.Errorf("expected transition to Open, got %v", stateChanges[0].to)
	}
	mu.Unlock()
}

func TestCircuitBreakerIsSuccessful(t *testing.T) {
	// Custom error that should not count as failure
	ignoredErr := errors.New("ignored error")

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 2,
		IsSuccessful: func(err error) bool {
			return err == nil || errors.Is(err, ignoredErr)
		},
	})

	// These should not count as failures
	for i := 0; i < 5; i++ {
		cb.Execute(func() error { return ignoredErr })
	}

	if cb.State() != StateClosed {
		t.Errorf("state = %v, want %v (ignored errors should not open circuit)", cb.State(), StateClosed)
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures: 100,
	})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cb.Execute(func() error { return nil })
		}()
	}
	wg.Wait()

	if cb.State() != StateClosed {
		t.Errorf("state = %v, want %v", cb.State(), StateClosed)
	}
}

func TestCircuitBreakerRegistry(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	// Get creates new breaker
	cb1 := registry.Get("service-a", CircuitBreakerConfig{MaxFailures: 5})
	if cb1 == nil {
		t.Fatal("expected non-nil circuit breaker")
	}

	// Get returns existing breaker
	cb2 := registry.Get("service-a", CircuitBreakerConfig{MaxFailures: 10})
	if cb1 != cb2 {
		t.Error("expected same circuit breaker instance")
	}

	// Different name creates new breaker
	cb3 := registry.Get("service-b", CircuitBreakerConfig{MaxFailures: 3})
	if cb1 == cb3 {
		t.Error("expected different circuit breaker instance")
	}
}

func TestCircuitBreakerRegistryStats(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	registry.Get("service-a", CircuitBreakerConfig{})
	registry.Get("service-b", CircuitBreakerConfig{})

	stats := registry.Stats()

	if len(stats) != 2 {
		t.Errorf("expected 2 entries in stats, got %d", len(stats))
	}
	if _, ok := stats["service-a"]; !ok {
		t.Error("expected service-a in stats")
	}
	if _, ok := stats["service-b"]; !ok {
		t.Error("expected service-b in stats")
	}
}

func TestCircuitBreakerRegistryReset(t *testing.T) {
	registry := NewCircuitBreakerRegistry()

	cb := registry.Get("service-a", CircuitBreakerConfig{MaxFailures: 2})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	if cb.State() != StateOpen {
		t.Errorf("state = %v, want %v", cb.State(), StateOpen)
	}

	// Reset all
	registry.Reset()

	if cb.State() != StateClosed {
		t.Errorf("state after reset = %v, want %v", cb.State(), StateClosed)
	}
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %v, want %v", tt.state, got, tt.want)
		}
	}
}

func TestTooManyRequestsInHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
	})

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error { return testErr })
	}

	// Wait for half-open
	time.Sleep(60 * time.Millisecond)

	// First request in half-open should succeed
	var firstErr error
	go func() {
		firstErr = cb.Execute(func() error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
	}()

	// Give time for first request to start
	time.Sleep(10 * time.Millisecond)

	// Second request should get ErrTooManyRequests
	err := cb.Execute(func() error { return nil })
	if !errors.Is(err, ErrTooManyRequests) {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}

	// Wait for first request to complete
	time.Sleep(50 * time.Millisecond)
	if firstErr != nil {
		t.Errorf("first request should succeed, got %v", firstErr)
	}
}
