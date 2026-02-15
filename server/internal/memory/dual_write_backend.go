package memory

import (
	"context"
	"log/slog"
)

// DualWriteBackend implements MemoryBackend by writing to both
// SQLite (for search) and Markdown (for human readability).
// Reads always go through the primary (SQLite) backend.
// Markdown writes are best-effort — failures are logged but don't block.
type DualWriteBackend struct {
	primary   MemoryBackend
	secondary *PureMarkdownBackend
}

// NewDualWriteBackend creates a dual-write backend.
func NewDualWriteBackend(primary MemoryBackend, secondary *PureMarkdownBackend) *DualWriteBackend {
	return &DualWriteBackend{
		primary:   primary,
		secondary: secondary,
	}
}

func (d *DualWriteBackend) Name() string {
	return "mixed"
}

// Remember writes to both backends. Primary must succeed; secondary is best-effort.
func (d *DualWriteBackend) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	chunk, err := d.primary.Remember(ctx, content, tags)
	if err != nil {
		return nil, err
	}

	if d.secondary != nil {
		if _, secErr := d.secondary.Remember(ctx, content, tags); secErr != nil {
			slog.Warn("dual-write: markdown write failed", "error", secErr)
		}
	}

	return chunk, nil
}

// Recall reads from primary only (SQLite has vector search + FTS).
func (d *DualWriteBackend) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	return d.primary.Recall(ctx, query, limit)
}

// Forget deletes from both backends.
func (d *DualWriteBackend) Forget(ctx context.Context, id string) error {
	err := d.primary.Forget(ctx, id)
	if err != nil {
		return err
	}

	if d.secondary != nil {
		if secErr := d.secondary.Forget(ctx, id); secErr != nil {
			slog.Warn("dual-write: markdown delete failed", "error", secErr)
		}
	}

	return nil
}

// ForgetAll clears both backends.
func (d *DualWriteBackend) ForgetAll(ctx context.Context) error {
	err := d.primary.ForgetAll(ctx)
	if err != nil {
		return err
	}

	if d.secondary != nil {
		if secErr := d.secondary.ForgetAll(ctx); secErr != nil {
			slog.Warn("dual-write: markdown clear failed", "error", secErr)
		}
	}

	return nil
}

// Get reads from primary only.
func (d *DualWriteBackend) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return d.primary.Get(ctx, id)
}

// Prune prunes both backends.
func (d *DualWriteBackend) Prune(ctx context.Context) (int, error) {
	count, err := d.primary.Prune(ctx)
	if err != nil {
		return 0, err
	}

	if d.secondary != nil {
		if _, secErr := d.secondary.Prune(ctx); secErr != nil {
			slog.Warn("dual-write: markdown prune failed", "error", secErr)
		}
	}

	return count, nil
}

// Stats returns primary stats with backend name "mixed".
func (d *DualWriteBackend) Stats(ctx context.Context) (*MemoryStats, error) {
	stats, err := d.primary.Stats(ctx)
	if err != nil {
		return nil, err
	}
	stats.Backend = "mixed"
	return stats, nil
}

// Ensure DualWriteBackend implements MemoryBackend.
var _ MemoryBackend = (*DualWriteBackend)(nil)
