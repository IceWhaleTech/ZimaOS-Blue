package heartbeat

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
)

// Runner manages the periodic heartbeat loop.
type Runner struct {
	mu       sync.RWMutex
	cfg      *Config
	deps     RunDeps
	dedup    *DedupCache
	lastEvt  *HeartbeatEvent
	nextDue  time.Time
	wakeCh   chan string
	stopOnce sync.Once
	stopped  chan struct{}
}

// RunnerDeps holds dependencies for creating a Runner.
type RunnerDeps struct {
	Config   *Config
	ChatFn   ChatFunc
	Channels *channel.Manager
	Streamer *companion.EventStreamer
	Logger   *zap.Logger
}

// NewRunner creates a new heartbeat runner.
func NewRunner(d RunnerDeps) *Runner {
	dedup := NewDedupCache(24 * time.Hour)
	return &Runner{
		cfg:   d.Config,
		dedup: dedup,
		deps: RunDeps{
			Config:   d.Config,
			ChatFn:   d.ChatFn,
			Channels: d.Channels,
			Streamer: d.Streamer,
			Dedup:    dedup,
			Logger:   d.Logger,
		},
		wakeCh:  make(chan string, 1),
		stopped: make(chan struct{}),
	}
}

// Run starts the heartbeat loop. Blocks until ctx is cancelled.
func (r *Runner) Run(ctx context.Context) {
	defer r.stopOnce.Do(func() { close(r.stopped) })

	for {
		r.mu.RLock()
		enabled := r.cfg.Enabled
		r.mu.RUnlock()

		if !enabled {
			r.deps.Logger.Info("heartbeat: disabled, waiting for enable signal")
			// Wait for wake signal (enable toggle) or context cancellation
			select {
			case <-ctx.Done():
				return
			case <-r.wakeCh:
				continue // Re-check enabled state
			}
		}

		r.runLoop(ctx)

		// If runLoop returned but context is not done, it means we were disabled
		if ctx.Err() != nil {
			return
		}
	}
}

// runLoop runs the ticker loop while enabled. Returns when disabled or ctx cancelled.
func (r *Runner) runLoop(ctx context.Context) {
	r.mu.RLock()
	cfg := r.cfg
	r.mu.RUnlock()

	interval := cfg.Interval
	if interval <= 0 {
		interval = DefaultInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.mu.Lock()
	r.nextDue = time.Now().Add(interval)
	r.mu.Unlock()

	r.deps.Logger.Info("heartbeat: started", zap.Duration("interval", interval))

	for {
		// Check if still enabled
		r.mu.RLock()
		enabled := r.cfg.Enabled
		r.mu.RUnlock()
		if !enabled {
			r.deps.Logger.Info("heartbeat: disabled at runtime")
			return
		}

		select {
		case <-ctx.Done():
			r.deps.Logger.Info("heartbeat: stopped")
			return
		case <-ticker.C:
			r.tick(ctx)
			r.mu.Lock()
			r.nextDue = time.Now().Add(interval)
			r.mu.Unlock()
		case reason := <-r.wakeCh:
			r.deps.Logger.Debug("heartbeat: wake requested", zap.String("reason", reason))
			// Re-check enabled (might be a disable signal)
			r.mu.RLock()
			stillEnabled := r.cfg.Enabled
			r.mu.RUnlock()
			if !stillEnabled {
				return
			}
			r.tick(ctx)
			ticker.Reset(interval)
			r.mu.Lock()
			r.nextDue = time.Now().Add(interval)
			r.mu.Unlock()
		}
	}
}

// RequestNow triggers an immediate heartbeat run. Non-blocking.
func (r *Runner) RequestNow(reason string) {
	select {
	case r.wakeCh <- reason:
	default:
		// Already pending, coalesce
	}
}

// UpdateConfig updates the heartbeat configuration (for hot-reload).
func (r *Runner) UpdateConfig(cfg *Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cfg = cfg
	r.deps.Config = cfg
}

// LastEvent returns the most recent heartbeat event.
func (r *Runner) LastEvent() *HeartbeatEvent {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastEvt
}

// NextDue returns when the next heartbeat is scheduled.
func (r *Runner) NextDue() time.Time {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.nextDue
}

func (r *Runner) tick(ctx context.Context) {
	r.mu.RLock()
	deps := r.deps
	r.mu.RUnlock()

	result := RunOnce(ctx, deps)

	evt := &HeartbeatEvent{
		Timestamp:     time.Now(),
		Status:        result.Status,
		Reason:        result.Reason,
		DurationMs:    result.DurationMs,
		IndicatorType: ResolveIndicator(result.Status),
	}
	r.mu.Lock()
	r.lastEvt = evt
	r.mu.Unlock()

	deps.Logger.Debug("heartbeat: tick complete",
		zap.String("status", result.Status),
		zap.String("reason", result.Reason),
		zap.Int64("duration_ms", result.DurationMs))
}
