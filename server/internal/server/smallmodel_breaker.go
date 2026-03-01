package server

import (
	"sync"
	"time"
)

const (
	defaultSmallModelCircuitFailureThreshold = 5
	defaultSmallModelCircuitOpenDuration     = 2 * time.Minute
)

// smallModelCircuitBreaker opens after repeated failures to protect latency and
// quickly route traffic to fallback paths (IR/cloud LLM).
type smallModelCircuitBreaker struct {
	mu sync.Mutex

	failureThreshold int
	openDuration     time.Duration

	consecutiveFailures int
	openUntil           time.Time
	openedTotal         int64
}

func newSmallModelCircuitBreaker(failureThreshold int, openDuration time.Duration) *smallModelCircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = defaultSmallModelCircuitFailureThreshold
	}
	if openDuration <= 0 {
		openDuration = defaultSmallModelCircuitOpenDuration
	}
	return &smallModelCircuitBreaker{
		failureThreshold: failureThreshold,
		openDuration:     openDuration,
	}
}

func (b *smallModelCircuitBreaker) Allow(now time.Time) bool {
	if b == nil {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeCloseLocked(now)
	return b.openUntil.IsZero()
}

func (b *smallModelCircuitBreaker) RecordSuccess(now time.Time) {
	if b == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeCloseLocked(now)
	if b.openUntil.IsZero() {
		b.consecutiveFailures = 0
	}
}

// RecordFailure returns true when this failure opens the breaker.
func (b *smallModelCircuitBreaker) RecordFailure(now time.Time) bool {
	if b == nil {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.maybeCloseLocked(now)
	if !b.openUntil.IsZero() {
		return false
	}
	b.consecutiveFailures++
	if b.consecutiveFailures < b.failureThreshold {
		return false
	}
	b.openUntil = now.Add(b.openDuration)
	b.consecutiveFailures = 0
	b.openedTotal++
	return true
}

func (b *smallModelCircuitBreaker) maybeCloseLocked(now time.Time) {
	if b.openUntil.IsZero() {
		return
	}
	if now.Before(b.openUntil) {
		return
	}
	b.openUntil = time.Time{}
	b.consecutiveFailures = 0
}
