package claudecode

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// CircuitState represents the state of a circuit breaker.
type CircuitState int

const (
	// CircuitClosed indicates the circuit is closed (normal operation).
	CircuitClosed CircuitState = iota
	// CircuitOpen indicates the circuit is open (failing fast).
	CircuitOpen
	// CircuitHalfOpen indicates the circuit is half-open (testing recovery).
	CircuitHalfOpen
)

// String returns the string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half_open"
	default:
		return "unknown"
	}
}

// CircuitBreakerStatus contains the current status of the circuit breaker.
type CircuitBreakerStatus struct {
	// State is the current circuit state.
	State CircuitState `json:"state"`
	// StateString is the string representation of the state.
	StateString string `json:"state_string"`
	// FailureCount is the current failure count.
	FailureCount int `json:"failure_count"`
	// SuccessCount is the current success count (in half-open state).
	SuccessCount int `json:"success_count"`
	// LastFailure is the time of the last failure.
	LastFailure *time.Time `json:"last_failure,omitempty"`
	// LastStateChange is the time of the last state change.
	LastStateChange time.Time `json:"last_state_change"`
	// OpenCount is the total number of times the circuit has opened.
	OpenCount int64 `json:"open_count"`
	// HalfOpenMaxCalls is the maximum calls allowed in half-open state.
	HalfOpenMaxCalls int `json:"half_open_max_calls"`
}

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           CircuitState
	failureCount    int
	successCount    int
	halfOpenCalls   int
	lastFailure     *time.Time
	lastStateChange time.Time
	openCount       int64
	mu              sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:          config,
		state:           CircuitClosed,
		lastStateChange: timeutil.NowTime(),
	}
}

// Execute runs the function through the circuit breaker.
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func(context.Context) error) error {
	if !cb.config.Enabled {
		return fn(ctx)
	}

	// Check if we can proceed
	if err := cb.beforeCall(); err != nil {
		return err
	}

	// Execute the function
	err := fn(ctx)

	// Record the result
	cb.afterCall(err)

	return err
}

// beforeCall checks if the call should proceed.
func (cb *CircuitBreaker) beforeCall() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case CircuitClosed:
		return nil

	case CircuitOpen:
		// Check if timeout has elapsed
		if timeutil.SinceTime(cb.lastStateChange) >= cb.config.Timeout {
			cb.transitionTo(CircuitHalfOpen)
			cb.halfOpenCalls = 1
			return nil
		}
		return ErrCircuitOpen

	case CircuitHalfOpen:
		// Check if we've exceeded max calls in half-open state
		if cb.halfOpenCalls >= cb.config.HalfOpenMaxCalls {
			return ErrCircuitOpen
		}
		cb.halfOpenCalls++
		return nil
	}

	return nil
}

// afterCall records the result of a call.
func (cb *CircuitBreaker) afterCall(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failed call.
func (cb *CircuitBreaker) recordFailure() {
	now := timeutil.NowTime()
	cb.lastFailure = &now
	cb.failureCount++

	switch cb.state {
	case CircuitClosed:
		if cb.failureCount >= cb.config.FailureThreshold {
			cb.transitionTo(CircuitOpen)
		}

	case CircuitHalfOpen:
		// Any failure in half-open state opens the circuit
		cb.transitionTo(CircuitOpen)
	}
}

// recordSuccess records a successful call.
func (cb *CircuitBreaker) recordSuccess() {
	switch cb.state {
	case CircuitClosed:
		// Reset failure count on success
		cb.failureCount = 0

	case CircuitHalfOpen:
		cb.successCount++
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.transitionTo(CircuitClosed)
		}
	}
}

// transitionTo changes the circuit state.
func (cb *CircuitBreaker) transitionTo(state CircuitState) {
	cb.state = state
	cb.lastStateChange = timeutil.NowTime()

	switch state {
	case CircuitClosed:
		cb.failureCount = 0
		cb.successCount = 0
		cb.halfOpenCalls = 0

	case CircuitOpen:
		cb.openCount++
		cb.successCount = 0
		cb.halfOpenCalls = 0

	case CircuitHalfOpen:
		cb.successCount = 0
		cb.halfOpenCalls = 0
	}
}

// Status returns the current circuit breaker status.
func (cb *CircuitBreaker) Status() CircuitBreakerStatus {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStatus{
		State:           cb.state,
		StateString:     cb.state.String(),
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailure:     cb.lastFailure,
		LastStateChange: cb.lastStateChange,
		OpenCount:       cb.openCount,
	}
}

// Reset resets the circuit breaker to closed state.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.halfOpenCalls = 0
	cb.lastStateChange = timeutil.NowTime()
}

// IsOpen returns true if the circuit is open.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state == CircuitOpen
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}
