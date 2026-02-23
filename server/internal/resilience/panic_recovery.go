package resilience

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PanicInfo contains information about a recovered panic
type PanicInfo struct {
	Time       time.Time `json:"time"`
	RequestID  string    `json:"request_id"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Error      string    `json:"error"`
	StackTrace string    `json:"stack_trace"`
}

// PanicRecoveryConfig holds configuration for panic recovery middleware
type PanicRecoveryConfig struct {
	// StackSize is the maximum size of the stack trace to capture (default: 4KB)
	StackSize int

	// DisableStackAll disables capturing stack traces for all goroutines
	DisableStackAll bool

	// DisablePrintStack disables printing stack trace to stderr
	DisablePrintStack bool

	// OnPanic is called when a panic is recovered
	OnPanic func(c echo.Context, err interface{}, stack []byte)

	// LogPanic enables logging of panic information
	LogPanic bool

	// MaxPanicHistory is the maximum number of panics to keep in history
	MaxPanicHistory int
}

// PanicRecoveryMiddleware provides enhanced panic recovery with metrics and history
type PanicRecoveryMiddleware struct {
	config       PanicRecoveryConfig
	mu           sync.RWMutex
	panicCount   int64
	panicHistory []PanicInfo
}

// NewPanicRecoveryMiddleware creates a new panic recovery middleware
func NewPanicRecoveryMiddleware(cfg PanicRecoveryConfig) *PanicRecoveryMiddleware {
	if cfg.StackSize <= 0 {
		cfg.StackSize = 4 << 10 // 4KB
	}
	if cfg.MaxPanicHistory <= 0 {
		cfg.MaxPanicHistory = 100
	}

	return &PanicRecoveryMiddleware{
		config:       cfg,
		panicHistory: make([]PanicInfo, 0, cfg.MaxPanicHistory),
	}
}

// Middleware returns the Echo middleware function
func (m *PanicRecoveryMiddleware) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}

					stack := make([]byte, m.config.StackSize)
					length := runtime.Stack(stack, !m.config.DisableStackAll)
					stack = stack[:length]

					// Record panic info
					info := PanicInfo{
						Time:       timeutil.NowTime(),
						RequestID:  c.Response().Header().Get(echo.HeaderXRequestID),
						Method:     c.Request().Method,
						Path:       c.Request().URL.Path,
						Error:      err.Error(),
						StackTrace: string(stack),
					}

					m.recordPanic(info)

					// Call custom handler if provided
					if m.config.OnPanic != nil {
						m.config.OnPanic(c, r, stack)
					}

					// Return internal server error
					c.Error(echo.NewHTTPError(http.StatusInternalServerError, "Internal Server Error"))
				}
			}()
			return next(c)
		}
	}
}

func (m *PanicRecoveryMiddleware) recordPanic(info PanicInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.panicCount++

	// Add to history with rotation
	if len(m.panicHistory) >= m.config.MaxPanicHistory {
		m.panicHistory = m.panicHistory[1:]
	}
	m.panicHistory = append(m.panicHistory, info)
}

// PanicCount returns the total number of panics recovered
func (m *PanicRecoveryMiddleware) PanicCount() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.panicCount
}

// PanicHistory returns the recent panic history
func (m *PanicRecoveryMiddleware) PanicHistory() []PanicInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history := make([]PanicInfo, len(m.panicHistory))
	copy(history, m.panicHistory)
	return history
}

// Stats returns panic recovery statistics
func (m *PanicRecoveryMiddleware) Stats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastPanic *PanicInfo
	if len(m.panicHistory) > 0 {
		last := m.panicHistory[len(m.panicHistory)-1]
		lastPanic = &last
	}

	return map[string]interface{}{
		"total_panics":  m.panicCount,
		"history_count": len(m.panicHistory),
		"last_panic":    lastPanic,
	}
}

// Reset clears the panic history and count
func (m *PanicRecoveryMiddleware) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.panicCount = 0
	m.panicHistory = make([]PanicInfo, 0, m.config.MaxPanicHistory)
}
