package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agent"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

// agentMemoryAdapter adapts LayeredMemoryService to agent.MemoryRecaller.
type agentMemoryAdapter struct {
	svc *memory.LayeredMemoryService
}

func newAgentMemoryAdapter(svc *memory.LayeredMemoryService) *agentMemoryAdapter {
	return &agentMemoryAdapter{svc: svc}
}

func (a *agentMemoryAdapter) Recall(ctx context.Context, query string, limit int) ([]agent.MemoryResult, error) {
	results, err := a.svc.Recall(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]agent.MemoryResult, len(results))
	for i, r := range results {
		out[i] = agent.MemoryResult{
			Content:  r.Chunk.Content,
			Score:    r.Score,
			Metadata: r.Chunk.Metadata,
		}
	}
	return out, nil
}

type agentReflectionMemoryWriter struct {
	svc *memory.LayeredMemoryService
}

func newAgentReflectionMemoryWriter(svc *memory.LayeredMemoryService) *agentReflectionMemoryWriter {
	return &agentReflectionMemoryWriter{svc: svc}
}

func (a *agentReflectionMemoryWriter) Write(ctx context.Context, content string, tags []string) error {
	if a == nil || a.svc == nil {
		return nil
	}
	return a.svc.AppendToDaily(ctx, content, tags)
}
