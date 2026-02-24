package resilience

import (
	"errors"
	"sync"
	"time"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota
	StateOpen
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// ErrCircuitOpen is returned when the circuit breaker is open
var ErrCircuitOpen = errors.New("circuit breaker is open")

// ErrTooManyRequests is returned when too many requests are made in half-open state
var ErrTooManyRequests = errors.New("too many requests in half-open state")

// CircuitBreakerConfig holds circuit breaker configuration
type CircuitBreakerConfig struct {
	// Name is the circuit breaker name for identification
	Name string

	// MaxFailures is the number of failures before opening the circuit
	MaxFailures int

	// Timeout is the duration the circuit stays open before transitioning to half-open
	Timeout time.Duration

	// MaxHalfOpenRequests is the max number of requests allowed in half-open state
	MaxHalfOpenRequests int

	// OnStateChange is called when the circuit breaker state changes
	OnStateChange func(name string, from, to State)

	// IsSuccessful determines if an error should be counted as a failure
	// If nil, any non-nil error is counted as a failure
	IsSuccessful func(err error) bool
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	config CircuitBreakerConfig

	mu                sync.Mutex
	state             State
	failures          int
	successes         int
	halfOpenRequests  int
	lastFailureTime   time.Time
	lastStateChange   time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = 5
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if cfg.MaxHalfOpenRequests <= 0 {
		cfg.MaxHalfOpenRequests = 1
	}

	return &CircuitBreaker{
		config:          cfg,
		state:           StateClosed,
		lastStateChange: timeutil.NowTime(),
	}
}

// Execute runs the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err)
	return err
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentState()
}

// Stats returns circuit breaker statistics
func (cb *CircuitBreaker) Stats() map[string]interface{} {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return map[string]interface{}{
		"name":             cb.config.Name,
		"state":            cb.currentState().String(),
		"failures":         cb.failures,
		"successes":        cb.successes,
		"last_failure":     cb.lastFailureTime,
		"last_state_change": cb.lastStateChange,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.setState(StateClosed)
	cb.failures = 0
	cb.successes = 0
	cb.halfOpenRequests = 0
}

// LoadState restores circuit breaker state from persisted data.
func (cb *CircuitBreaker) LoadState(state State, failures, successes int, lastFailure, lastChange time.Time) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = state
	cb.failures = failures
	cb.successes = successes
	cb.lastFailureTime = lastFailure
	cb.lastStateChange = lastChange
	cb.halfOpenRequests = 0
}

// ParseState converts a string to a State value.
func ParseState(s string) State {
	switch s {
	case "open":
		return StateOpen
	case "half-open":
		return StateHalfOpen
	default:
		return StateClosed
	}
}

func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	state := cb.currentState()

	switch state {
	case StateClosed:
		return nil
	case StateOpen:
		return ErrCircuitOpen
	case StateHalfOpen:
		cb.halfOpenRequests++
		if cb.halfOpenRequests > cb.config.MaxHalfOpenRequests {
			return ErrTooManyRequests
		}
		return nil
	}

	return nil
}

func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	isSuccess := err == nil
	if cb.config.IsSuccessful != nil {
		isSuccess = cb.config.IsSuccessful(err)
	}

	state := cb.currentState()

	switch state {
	case StateClosed:
		if isSuccess {
			cb.successes++
			cb.failures = 0
		} else {
			cb.failures++
			cb.lastFailureTime = timeutil.NowTime()
			if cb.failures >= cb.config.MaxFailures {
				cb.setState(StateOpen)
			}
		}
	case StateHalfOpen:
		if isSuccess {
			cb.successes++
			cb.failures = 0
			cb.halfOpenRequests = 0
			cb.setState(StateClosed)
		} else {
			cb.failures++
			cb.lastFailureTime = timeutil.NowTime()
			cb.halfOpenRequests = 0
			cb.setState(StateOpen)
		}
	}
}

func (cb *CircuitBreaker) currentState() State {
	switch cb.state {
	case StateClosed:
		return StateClosed
	case StateOpen:
		if timeutil.SinceTime(cb.lastStateChange) >= cb.config.Timeout {
			cb.setState(StateHalfOpen)
			return StateHalfOpen
		}
		return StateOpen
	case StateHalfOpen:
		return StateHalfOpen
	}
	return StateClosed
}

func (cb *CircuitBreaker) setState(state State) {
	if cb.state == state {
		return
	}

	from := cb.state
	cb.state = state
	cb.lastStateChange = timeutil.NowTime()

	if cb.config.OnStateChange != nil {
		go cb.config.OnStateChange(cb.config.Name, from, state)
	}
}

// CircuitBreakerRegistry manages multiple circuit breakers
type CircuitBreakerRegistry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// NewCircuitBreakerRegistry creates a new registry
func NewCircuitBreakerRegistry() *CircuitBreakerRegistry {
	return &CircuitBreakerRegistry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Get returns a circuit breaker by name, creating one if it doesn't exist
func (r *CircuitBreakerRegistry) Get(name string, cfg CircuitBreakerConfig) *CircuitBreaker {
	r.mu.RLock()
	cb, exists := r.breakers[name]
	r.mu.RUnlock()

	if exists {
		return cb
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if cb, exists = r.breakers[name]; exists {
		return cb
	}

	cfg.Name = name
	cb = NewCircuitBreaker(cfg)
	r.breakers[name] = cb
	return cb
}

// Stats returns statistics for all circuit breakers
func (r *CircuitBreakerRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, cb := range r.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

// Reset resets all circuit breakers
func (r *CircuitBreakerRegistry) Reset() {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, cb := range r.breakers {
		cb.Reset()
	}
}
