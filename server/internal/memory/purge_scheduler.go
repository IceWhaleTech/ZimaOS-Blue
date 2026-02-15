package memory

import (
	"context"
	"log/slog"
	"time"
)

// PurgeScheduler runs periodic purge of expired memory entries.
type PurgeScheduler struct {
	repo     *MemoryRepository
	interval time.Duration
	cancel   context.CancelFunc
}

// NewPurgeScheduler creates a scheduler that purges expired entries at the given interval.
func NewPurgeScheduler(repo *MemoryRepository, interval time.Duration) *PurgeScheduler {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &PurgeScheduler{repo: repo, interval: interval}
}

// Start begins the background purge loop.
func (s *PurgeScheduler) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	go s.run(ctx)
}

// Stop cancels the background purge loop.
func (s *PurgeScheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *PurgeScheduler) run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n, err := s.repo.PurgeExpired(ctx)
			if err != nil {
				slog.Warn("memory purge failed", "error", err)
			} else if n > 0 {
				slog.Info("memory purge completed", "purged", n)
			}
		}
	}
}
