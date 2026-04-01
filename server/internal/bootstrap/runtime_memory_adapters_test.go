package bootstrap

import (
	"context"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

type stubMemoryBackend struct {
	results []memory.SearchResult
}

func (s *stubMemoryBackend) Remember(context.Context, string, []string) (*memory.MemoryChunk, error) {
	return nil, nil
}

func (s *stubMemoryBackend) Recall(context.Context, string, int) ([]memory.SearchResult, error) {
	return s.results, nil
}

func (s *stubMemoryBackend) Forget(context.Context, string) error { return nil }

func (s *stubMemoryBackend) ForgetAll(context.Context) error { return nil }

func (s *stubMemoryBackend) Get(context.Context, string) (*memory.MemoryChunk, error) {
	return nil, nil
}

func (s *stubMemoryBackend) Prune(context.Context) (int, error) { return 0, nil }

func (s *stubMemoryBackend) Stats(context.Context) (*memory.MemoryStats, error) {
	return &memory.MemoryStats{}, nil
}

func (s *stubMemoryBackend) Name() string { return "stub" }

func TestAgentMemoryAdapter_RecallPreservesMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mdBackend, err := memory.NewPureMarkdownBackend(tmpDir)
	if err != nil {
		t.Fatalf("NewPureMarkdownBackend: %v", err)
	}
	unified := memory.NewUnifiedMemoryService(mdBackend)
	unified.SetBackend(&stubMemoryBackend{
		results: []memory.SearchResult{
			{
				Chunk: memory.MemoryChunk{
					ID:        "mem-1",
					Content:   "remembered repo preference",
					Metadata:  map[string]string{"tag_0": "longterm", "tag_1": "project"},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				Score: 0.88,
			},
		},
	})
	layered, err := memory.NewLayeredMemoryService(unified, memory.LayeredMemoryConfig{
		BaseDir: tmpDir,
	})
	if err != nil {
		t.Fatalf("NewLayeredMemoryService: %v", err)
	}

	adapter := newAgentMemoryAdapter(layered)
	results, err := adapter.Recall(context.Background(), "repo preference", 5)
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("results len = %d, want 1", len(results))
	}
	if results[0].Metadata["tag_0"] != "longterm" || results[0].Metadata["tag_1"] != "project" {
		t.Fatalf("metadata = %#v, want longterm/project tags", results[0].Metadata)
	}
}
