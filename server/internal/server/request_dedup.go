package server

import (
	"sync"
	"time"
)

// RequestDeduplicator prevents duplicate concurrent requests from hitting the backend
type RequestDeduplicator struct {
	// Map of request key to in-flight request
	inFlight map[string]*InFlightRequest
	mu       sync.RWMutex

	// Metrics
	deduped   int64
	bypassed  int64
	completed int64
}

// InFlightRequest represents a request currently being processed
type InFlightRequest struct {
	// Result channel - all waiters receive the same result
	resultChan chan interface{}
	// Error channel
	errChan chan error
	// Start time for tracking
	startTime time.Time
	// Number of waiters
	waiters int
}

// NewRequestDeduplicator creates a new request deduplicator
func NewRequestDeduplicator() *RequestDeduplicator {
	return &RequestDeduplicator{
		inFlight: make(map[string]*InFlightRequest),
	}
}

// Do executes a function, deduplicating concurrent identical requests
// key: unique identifier for the request
// fn: function to execute (only called once for identical concurrent requests)
// Returns: (result, error, wasDeduped)
func (rd *RequestDeduplicator) Do(key string, fn func() (interface{}, error)) (interface{}, error, bool) {
	rd.mu.Lock()

	// Check if request is already in flight
	if inFlight, exists := rd.inFlight[key]; exists {
		inFlight.waiters++
		rd.mu.Unlock()

		// Wait for the in-flight request to complete
		select {
		case result := <-inFlight.resultChan:
			rd.mu.Lock()
			inFlight.waiters--
			rd.mu.Unlock()
			return result, nil, true
		case err := <-inFlight.errChan:
			rd.mu.Lock()
			inFlight.waiters--
			rd.mu.Unlock()
			return nil, err, true
		}
	}

	// Create new in-flight request
	inFlight := &InFlightRequest{
		resultChan: make(chan interface{}, 1),
		errChan:    make(chan error, 1),
		startTime:  time.Now(),
		waiters:    1,
	}
	rd.inFlight[key] = inFlight
	rd.mu.Unlock()

	// Execute the function
	result, err := fn()

	// Send result to all waiters
	rd.mu.Lock()
	delete(rd.inFlight, key)
	rd.mu.Unlock()

	if err != nil {
		inFlight.errChan <- err
	} else {
		inFlight.resultChan <- result
	}

	return result, err, false
}

// GetMetrics returns deduplication metrics
func (rd *RequestDeduplicator) GetMetrics() map[string]int64 {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	return map[string]int64{
		"deduped":   rd.deduped,
		"bypassed":  rd.bypassed,
		"completed": rd.completed,
		"in_flight": int64(len(rd.inFlight)),
	}
}
