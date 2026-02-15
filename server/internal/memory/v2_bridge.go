package memory

import (
	"context"
	"log/slog"
)

// V2Bridge listens to the existing memory system and mirrors entries
// into the v2 MemoryRepository for structured browsing and management.
type V2Bridge struct {
	repo *MemoryRepository
}

// NewV2Bridge creates a bridge that writes to the v2 repository.
func NewV2Bridge(repo *MemoryRepository) *V2Bridge {
	return &V2Bridge{repo: repo}
}

// OnRemember is called when the existing memory system stores a new memory.
// It creates a corresponding v2 entry with the given content and tags.
func (b *V2Bridge) OnRemember(ctx context.Context, content string, tags []string, source string) {
	e := NewMemoryEntry("default", content)
	e.Tags = tags
	e.Source = source
	e.Category = "auto"
	if err := b.repo.Create(ctx, e); err != nil {
		slog.Warn("v2 bridge: failed to create entry", "error", err)
	}
}

// OnSessionEnd mirrors a session summary into the v2 repository.
func (b *V2Bridge) OnSessionEnd(ctx context.Context, summary string, tags []string) {
	e := NewMemoryEntry("default", summary)
	e.Tags = tags
	e.Source = "session"
	e.Category = "session"
	if err := b.repo.Create(ctx, e); err != nil {
		slog.Warn("v2 bridge: failed to create session entry", "error", err)
	}
}
