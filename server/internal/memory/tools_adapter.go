package memory

import (
	"context"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// ToolsAdapter adapts UnifiedMemoryService to tools.MemoryServiceInterface.
type ToolsAdapter struct {
	service             *UnifiedMemoryService
	progressiveSearcher *ProgressiveSearcher
}

// NewToolsAdapter creates a new tools adapter for the memory service.
func NewToolsAdapter(service *UnifiedMemoryService) *ToolsAdapter {
	return &ToolsAdapter{service: service}
}

// Recall searches memories and returns results in the tools interface format.
func (a *ToolsAdapter) Recall(ctx context.Context, query string, limit int) ([]tools.MemorySearchResult, error) {
	results, err := a.service.Recall(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	toolsResults := make([]tools.MemorySearchResult, len(results))
	for i, r := range results {
		toolsResults[i] = tools.MemorySearchResult{
			Chunk: tools.MemoryChunkResult{
				ID:        r.Chunk.ID,
				Content:   r.Chunk.Content,
				Metadata:  r.Chunk.Metadata,
				CreatedAt: r.Chunk.CreatedAt,
				UpdatedAt: r.Chunk.UpdatedAt,
			},
			VectorScore:   r.VectorScore,
			KeywordScore:  r.KeywordScore,
			CombinedScore: r.CombinedScore,
			MatchTypes:    r.MatchTypes,
		}
	}

	return toolsResults, nil
}

// Get retrieves a memory by ID and returns it in the tools interface format.
func (a *ToolsAdapter) Get(ctx context.Context, id string) (*tools.MemoryChunkResult, error) {
	chunk, err := a.service.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return &tools.MemoryChunkResult{
		ID:        chunk.ID,
		Content:   chunk.Content,
		Metadata:  chunk.Metadata,
		CreatedAt: chunk.CreatedAt,
		UpdatedAt: chunk.UpdatedAt,
	}, nil
}

// Stats returns memory statistics in the tools interface format.
func (a *ToolsAdapter) Stats(ctx context.Context) (*tools.MemoryStatsResult, error) {
	stats, err := a.service.Stats(ctx)
	if err != nil {
		return nil, err
	}

	return &tools.MemoryStatsResult{
		TotalChunks:    stats.TotalChunks,
		TotalSizeBytes: stats.TotalSizeBytes,
		OldestChunk:    stats.OldestChunk,
		NewestChunk:    stats.NewestChunk,
		Backend:        stats.Backend,
	}, nil
}

// GetActiveBackend returns the name of the active backend.
func (a *ToolsAdapter) GetActiveBackend() string {
	return a.service.GetActiveBackend()
}

// Remember stores a new memory and returns it in the tools interface format.
func (a *ToolsAdapter) Remember(ctx context.Context, content string, tags []string) (*tools.MemoryChunkResult, error) {
	chunk, err := a.service.Remember(ctx, content, tags)
	if err != nil {
		return nil, err
	}

	return &tools.MemoryChunkResult{
		ID:        chunk.ID,
		Content:   chunk.Content,
		Metadata:  chunk.Metadata,
		CreatedAt: chunk.CreatedAt,
		UpdatedAt: chunk.UpdatedAt,
	}, nil
}

// Forget deletes a memory by ID.
func (a *ToolsAdapter) Forget(ctx context.Context, id string) error {
	return a.service.Forget(ctx, id)
}

// SetProgressiveSearcher sets the progressive searcher for the adapter.
func (a *ToolsAdapter) SetProgressiveSearcher(ps *ProgressiveSearcher) {
	a.progressiveSearcher = ps
}

// ProgressiveSearch implements tools.ProgressiveSearchInterface.
func (a *ToolsAdapter) ProgressiveSearch(ctx context.Context, query string, depth int, ids []string, limit int) (interface{}, error) {
	if a.progressiveSearcher == nil {
		return nil, fmt.Errorf("progressive search not available")
	}

	req := ProgressiveSearchRequest{
		Query: query,
		Depth: SearchDepth(depth),
		IDs:   ids,
		Limit: limit,
	}

	return a.progressiveSearcher.Search(ctx, req)
}

// Ensure ToolsAdapter implements tools.MemoryServiceInterface
var _ tools.MemoryServiceInterface = (*ToolsAdapter)(nil)

// Ensure ToolsAdapter implements tools.ProgressiveSearchInterface
var _ tools.ProgressiveSearchInterface = (*ToolsAdapter)(nil)
