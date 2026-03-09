package memory

import (
	"context"
	"log/slog"
	"sync"
)

// DualWriteBackend implements MemoryBackend by writing to both
// Markdown (primary, sync, source of truth) and SQLite/MemoryService
// (secondary, async, for hybrid search).
// Reads prefer secondary (hybrid search); degrade to primary if unavailable.
type DualWriteBackend struct {
	primary   *PureMarkdownBackend // Markdown: sync, source of truth
	secondary *MemoryService       // SQLite hybrid search: async

	writeCh  chan writeOp
	wg       sync.WaitGroup
	stopOnce sync.Once
	stopCh   chan struct{}
}

type writeOp struct {
	kind     string // "remember", "forget", "forgetall", "prune"
	content  string
	tags     []string
	id       string
	sourceID string
}

const writeQueueCap = 256

// NewDualWriteBackend creates a dual-write backend.
func NewDualWriteBackend(primary *PureMarkdownBackend, secondary *MemoryService) *DualWriteBackend {
	d := &DualWriteBackend{
		primary:   primary,
		secondary: secondary,
		writeCh:   make(chan writeOp, writeQueueCap),
		stopCh:    make(chan struct{}),
	}
	d.wg.Add(1)
	go d.asyncWriter()
	return d
}

func (d *DualWriteBackend) Name() string {
	return "dual"
}

// Remember writes to Markdown (sync) and enqueues async write to secondary.
func (d *DualWriteBackend) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	chunk, err := d.primary.Remember(ctx, content, tags)
	if err != nil {
		return nil, err
	}

	d.enqueue(writeOp{kind: "remember", content: content, tags: tags, sourceID: chunk.ID})
	return chunk, nil
}

// Recall prefers secondary (hybrid search); degrades to primary (markdown keyword).
func (d *DualWriteBackend) Recall(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if d.secondary != nil {
		results, err := d.secondary.Recall(ctx, query, limit)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			slog.Warn("dual-write: secondary recall failed, falling back to markdown", "error", err)
		}
	}
	return d.primary.Recall(ctx, query, limit)
}

// Forget deletes from Markdown (sync) and enqueues async delete on secondary.
func (d *DualWriteBackend) Forget(ctx context.Context, id string) error {
	err := d.primary.Forget(ctx, id)
	if err != nil {
		return err
	}
	d.enqueue(writeOp{kind: "forget", id: id})
	return nil
}

// ForgetAll clears Markdown (sync) and enqueues async clear on secondary.
func (d *DualWriteBackend) ForgetAll(ctx context.Context) error {
	err := d.primary.ForgetAll(ctx)
	if err != nil {
		return err
	}
	d.enqueue(writeOp{kind: "forgetall"})
	return nil
}

// Get reads from Markdown (source of truth).
func (d *DualWriteBackend) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return d.primary.Get(ctx, id)
}

// Prune prunes both backends.
func (d *DualWriteBackend) Prune(ctx context.Context) (int, error) {
	count, err := d.primary.Prune(ctx)
	if err != nil {
		return 0, err
	}
	d.enqueue(writeOp{kind: "prune"})
	return count, nil
}

// Stats returns primary stats with backend name "dual".
func (d *DualWriteBackend) Stats(ctx context.Context) (*MemoryStats, error) {
	stats, err := d.primary.Stats(ctx)
	if err != nil {
		return nil, err
	}
	stats.Backend = "dual"
	return stats, nil
}

// Close stops the async writer and waits for pending ops to drain.
func (d *DualWriteBackend) Close() {
	d.stopOnce.Do(func() {
		close(d.stopCh)
		d.wg.Wait()
	})
}

// enqueue sends a write op to the async channel. Drops if full (bounded queue).
func (d *DualWriteBackend) enqueue(op writeOp) {
	select {
	case d.writeCh <- op:
	default:
		slog.Warn("dual-write: async queue full, dropping op", "kind", op.kind)
	}
}

// asyncWriter processes write ops from the channel.
func (d *DualWriteBackend) asyncWriter() {
	defer d.wg.Done()

	for {
		select {
		case op := <-d.writeCh:
			d.processOp(op)
		case <-d.stopCh:
			// Drain remaining ops
			for {
				select {
				case op := <-d.writeCh:
					d.processOp(op)
				default:
					return
				}
			}
		}
	}
}

func (d *DualWriteBackend) processOp(op writeOp) {
	if d.secondary == nil {
		return
	}
	ctx := context.Background()

	switch op.kind {
	case "remember":
		metadata := metadataFromTags(op.tags)
		if op.sourceID != "" {
			metadata["source_id"] = op.sourceID
		}
		if _, err := d.secondary.RememberWithMetadata(ctx, op.content, metadata); err != nil {
			slog.Warn("dual-write: secondary remember failed", "error", err)
		}
	case "forget":
		if err := d.secondary.Forget(ctx, op.id); err != nil {
			slog.Warn("dual-write: secondary forget failed", "error", err)
		}
	case "forgetall":
		if err := d.secondary.ForgetAll(ctx); err != nil {
			slog.Warn("dual-write: secondary forgetall failed", "error", err)
		}
	case "prune":
		if _, err := d.secondary.Prune(ctx); err != nil {
			slog.Warn("dual-write: secondary prune failed", "error", err)
		}
	}
}

// Ensure DualWriteBackend implements MemoryBackend.
var _ MemoryBackend = (*DualWriteBackend)(nil)
