package resilience

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
)

// ErrBulkheadFull is returned when the bulkhead is at capacity
var ErrBulkheadFull = errors.New("bulkhead is full")

// BulkheadConfig holds configuration for bulkhead pattern
type BulkheadConfig struct {
	// Name is the bulkhead name for identification
	Name string

	// MaxConcurrent is the maximum number of concurrent requests
	MaxConcurrent int

	// MaxWaiting is the maximum number of requests waiting in queue
	MaxWaiting int

	// WaitTimeout is the maximum time to wait for a slot
	WaitTimeout time.Duration

	// OnRejected is called when a request is rejected
	OnRejected func(c echo.Context)

	// Skipper defines a function to skip middleware
	Skipper func(c echo.Context) bool
}

// Bulkhead implements the bulkhead pattern for resource isolation
type Bulkhead struct {
	config BulkheadConfig

	semaphore chan struct{}
	waiting   int64
	rejected  int64
	completed int64

	mu sync.RWMutex
}

// NewBulkhead creates a new bulkhead
func NewBulkhead(cfg BulkheadConfig) *Bulkhead {
	if cfg.MaxConcurrent <= 0 {
		cfg.MaxConcurrent = 10
	}
	if cfg.MaxWaiting <= 0 {
		cfg.MaxWaiting = 100
	}
	if cfg.WaitTimeout <= 0 {
		cfg.WaitTimeout = 30 * time.Second
	}

	return &Bulkhead{
		config:    cfg,
		semaphore: make(chan struct{}, cfg.MaxConcurrent),
	}
}

// Execute runs the given function with bulkhead protection
func (b *Bulkhead) Execute(fn func() error) error {
	// Check if we can wait
	currentWaiting := atomic.AddInt64(&b.waiting, 1)
	if currentWaiting > int64(b.config.MaxWaiting) {
		atomic.AddInt64(&b.waiting, -1)
		atomic.AddInt64(&b.rejected, 1)
		return ErrBulkheadFull
	}

	// Try to acquire a slot
	select {
	case b.semaphore <- struct{}{}:
		atomic.AddInt64(&b.waiting, -1)
	case <-time.After(b.config.WaitTimeout):
		atomic.AddInt64(&b.waiting, -1)
		atomic.AddInt64(&b.rejected, 1)
		return ErrBulkheadFull
	}

	// Execute the function
	defer func() {
		<-b.semaphore
		atomic.AddInt64(&b.completed, 1)
	}()

	return fn()
}

// Middleware returns the Echo middleware function
func (b *Bulkhead) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if b.config.Skipper != nil && b.config.Skipper(c) {
				return next(c)
			}

			err := b.Execute(func() error {
				return next(c)
			})

			if errors.Is(err, ErrBulkheadFull) {
				if b.config.OnRejected != nil {
					b.config.OnRejected(c)
				}
				return echo.NewHTTPError(http.StatusServiceUnavailable, "Service temporarily unavailable")
			}

			return err
		}
	}
}

// Stats returns bulkhead statistics
func (b *Bulkhead) Stats() map[string]interface{} {
	return map[string]interface{}{
		"name":           b.config.Name,
		"max_concurrent": b.config.MaxConcurrent,
		"max_waiting":    b.config.MaxWaiting,
		"current_active": len(b.semaphore),
		"current_waiting": atomic.LoadInt64(&b.waiting),
		"total_rejected": atomic.LoadInt64(&b.rejected),
		"total_completed": atomic.LoadInt64(&b.completed),
	}
}

// Reset resets the bulkhead statistics
func (b *Bulkhead) Reset() {
	atomic.StoreInt64(&b.waiting, 0)
	atomic.StoreInt64(&b.rejected, 0)
	atomic.StoreInt64(&b.completed, 0)
}

// BulkheadRegistry manages multiple bulkheads
type BulkheadRegistry struct {
	mu        sync.RWMutex
	bulkheads map[string]*Bulkhead
}

// NewBulkheadRegistry creates a new bulkhead registry
func NewBulkheadRegistry() *BulkheadRegistry {
	return &BulkheadRegistry{
		bulkheads: make(map[string]*Bulkhead),
	}
}

// Get returns a bulkhead by name, creating one if it doesn't exist
func (r *BulkheadRegistry) Get(name string, cfg BulkheadConfig) *Bulkhead {
	r.mu.RLock()
	b, exists := r.bulkheads[name]
	r.mu.RUnlock()

	if exists {
		return b
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if b, exists = r.bulkheads[name]; exists {
		return b
	}

	cfg.Name = name
	b = NewBulkhead(cfg)
	r.bulkheads[name] = b
	return b
}

// Stats returns statistics for all bulkheads
func (r *BulkheadRegistry) Stats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]interface{})
	for name, b := range r.bulkheads {
		stats[name] = b.Stats()
	}
	return stats
}
