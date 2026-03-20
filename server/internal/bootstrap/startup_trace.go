package bootstrap

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StartupTrace emits coarse startup timing marks when tracing is enabled via env.
type StartupTrace struct {
	enabled   bool
	component string
	started   time.Time
	last      time.Time
	logger    *zap.Logger
	mu        sync.Mutex
}

// StartupTraceEnabled returns true when startup timing logs should be emitted.
func StartupTraceEnabled() bool {
	for _, key := range []string{"ZIMAOS_STARTUP_TRACE", "BLUE_STARTUP_TRACE"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "1", "true", "yes", "on":
			return true
		}
	}
	return false
}

// NewStartupTrace creates a new startup trace for a component.
func NewStartupTrace(component string, logger *zap.Logger) *StartupTrace {
	now := time.Now()
	return &StartupTrace{
		enabled:   StartupTraceEnabled(),
		component: component,
		started:   now,
		last:      now,
		logger:    logger,
	}
}

// SetLogger attaches a logger for subsequent trace events.
func (t *StartupTrace) SetLogger(logger *zap.Logger) {
	if t == nil || !t.enabled {
		return
	}
	t.mu.Lock()
	t.logger = logger
	t.mu.Unlock()
}

// Mark records a startup milestone.
func (t *StartupTrace) Mark(label string, fields ...zap.Field) {
	if t == nil || !t.enabled {
		return
	}

	t.mu.Lock()
	now := time.Now()
	step := now.Sub(t.last)
	total := now.Sub(t.started)
	t.last = now
	logger := t.logger
	component := t.component
	t.mu.Unlock()

	if logger != nil {
		traceFields := []zap.Field{
			zap.String("component", component),
			zap.String("label", label),
			zap.Int64("step_ms", step.Milliseconds()),
			zap.Int64("total_ms", total.Milliseconds()),
		}
		logger.Info("startup-trace", append(traceFields, fields...)...)
		return
	}

	fmt.Fprintf(
		os.Stderr,
		"[startup-trace] component=%s label=%s step_ms=%d total_ms=%d\n",
		component,
		label,
		step.Milliseconds(),
		total.Milliseconds(),
	)
}
