package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
)

type stubPromptMemoryBackend struct {
	called  bool
	results []memory.SearchResult
}

func newPromptLayeredMemoryServiceForTest(t *testing.T) (*memory.LayeredMemoryService, *memory.UnifiedMemoryService) {
	t.Helper()
	memDir := t.TempDir()
	mdBackend, err := memory.NewPureMarkdownBackend(memDir)
	if err != nil {
		t.Fatalf("create markdown backend: %v", err)
	}
	baseSvc := memory.NewUnifiedMemoryService(mdBackend)
	layeredSvc, err := memory.NewLayeredMemoryService(baseSvc, memory.LayeredMemoryConfig{
		BaseDir:     memDir,
		LongTermDir: memDir,
	})
	if err != nil {
		t.Fatalf("create layered memory service: %v", err)
	}
	return layeredSvc, baseSvc
}

func (s *stubPromptMemoryBackend) Remember(context.Context, string, []string) (*memory.MemoryChunk, error) {
	return nil, nil
}

func (s *stubPromptMemoryBackend) Recall(context.Context, string, int) ([]memory.SearchResult, error) {
	s.called = true
	return s.results, nil
}

func (s *stubPromptMemoryBackend) Forget(context.Context, string) error { return nil }

func (s *stubPromptMemoryBackend) ForgetAll(context.Context) error { return nil }

func (s *stubPromptMemoryBackend) Get(context.Context, string) (*memory.MemoryChunk, error) {
	return nil, nil
}

func (s *stubPromptMemoryBackend) Prune(context.Context) (int, error) { return 0, nil }

func (s *stubPromptMemoryBackend) Stats(context.Context) (*memory.MemoryStats, error) {
	return &memory.MemoryStats{}, nil
}

func (s *stubPromptMemoryBackend) Name() string { return "stub" }

func TestChatRecallMemories_SkipsLatestWebDocsQueries(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "stale docs memory",
				Metadata: map[string]string{"tag_0": "longterm"},
			},
			Score: 0.95,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "搜索 OpenAI Responses API 的最新文档。", MemoryRecallModeBalanced)
	if got != "" {
		t.Fatalf("expected latest web docs query to skip memory recall, got %q", got)
	}
	if backend.called {
		t.Fatal("expected backend recall to be skipped for latest web docs query")
	}
}

func TestChatRecallMemories_SkipsLocalizedWebQueryRoutes(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "stale non-english docs memory",
				Metadata: map[string]string{"tag_0": "longterm"},
			},
			Score: 0.92,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "Vyhledej nejnovejsi dokumentaci k OpenAI Responses API.", MemoryRecallModeBalanced)
	if got != "" {
		t.Fatalf("expected localized web_query request to skip memory recall, got %q", got)
	}
	if backend.called {
		t.Fatal("expected backend recall to be skipped for localized web_query request")
	}
}

func TestChatRecallMemories_SkipsSessionCompactionForGenericPrompt(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "Earlier session summary that may be stale.",
				Metadata: map[string]string{"tag_0": "session-compaction", "tag_1": "session:abc"},
			},
			Score: 0.96,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "Explain Rust borrowing in simple terms.", MemoryRecallModeBalanced)
	if got != "" {
		t.Fatalf("expected session-compaction memory to be skipped for generic prompt, got %q", got)
	}
	if !backend.called {
		t.Fatal("expected backend recall call so result filtering is exercised")
	}
}

func TestChatRecallMemories_SkipsKnowledgeTaggedPromptMemory(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "Blue docs say the workspace keeps compiled knowledge pages.",
				Metadata: map[string]string{"tag_0": "knowledge", "tag_1": "knowledge-page", "tag_2": "blue-workspace"},
			},
			Score: 0.94,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "What preference did I mention for release timelines?", MemoryRecallModeBalanced)
	if got != "" {
		t.Fatalf("expected knowledge-tagged prompt memory to be skipped, got %q", got)
	}
	if !backend.called {
		t.Fatal("expected backend recall call so result filtering is exercised")
	}
}

func TestChatRecallMemories_LabelsLongTermMemorySource(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:   "The repo prefers the lowest-risk migration path.",
				Metadata:  map[string]string{"tag_0": "longterm", "tag_1": "project"},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Score: 0.82,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "Inspect the repository and implement the minimal fix.", MemoryRecallModeBalanced)
	if !strings.Contains(got, "<memory_context>") {
		t.Fatalf("expected memory context wrapper, got %q", got)
	}
	if !strings.Contains(got, "[memory source=long_term trust=medium relevance=0.82]") {
		t.Fatalf("expected long-term source label, got %q", got)
	}
}

func TestChatRecallMemories_FiltersSessionCompactionForRetrospectiveWeekPromptWithoutTimeWindowSpecialCase(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	backend := &stubPromptMemoryBackend{
		results: []memory.SearchResult{{
			Chunk: memory.MemoryChunk{
				Content:  "上周主要在 server/internal/server/chat.go 和 server/internal/server/chat_context.go 里写了长会话压缩与记忆注入。",
				Metadata: map[string]string{"tag_0": "session-compaction", "tag_1": "session:weekly-review"},
			},
			Score: 0.93,
		}},
	}
	baseSvc.SetBackend(backend)
	handler := &ChatHandler{layeredMemory: layered}

	got := handler.recallMemories(context.Background(), "梳理下我过去一周具体写了什么。", MemoryRecallModeBalanced)
	if !backend.called {
		t.Fatal("expected recall path to run so session-compaction filtering is exercised")
	}
	if got != "" {
		t.Fatalf("expected retrospective week prompt to stop using session-compaction special handling, got %q", got)
	}
}

func TestShouldSkipPromptMemoryRecall_DoesNotTreatRecentAsFreshPublicSignalByItself(t *testing.T) {
	if shouldSkipPromptMemoryRecall("网页最近发生了什么？") {
		t.Fatal("expected 最近 to stop triggering fresh-public skip heuristics by itself")
	}
}

func TestIsKnowledgeFirstPrompt_TightensGenericWorkspaceAndMemoryTerms(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    bool
	}{
		{
			name:    "generic workspace preference request stays out of knowledge-first",
			message: "Remember my workspace preference for release timelines.",
			want:    false,
		},
		{
			name:    "generic memory follow-up stays out of knowledge-first",
			message: "What did I say my memory preference was before?",
			want:    false,
		},
		{
			name:    "product memory architecture stays knowledge-first",
			message: "Explain Blue memory architecture.",
			want:    true,
		},
		{
			name:    "docs request stays knowledge-first",
			message: "Show me the ZimaOS documentation for knowledge compile.",
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isKnowledgeFirstPrompt(tc.message)
			if got != tc.want {
				t.Fatalf("isKnowledgeFirstPrompt(%q) = %v, want %v", tc.message, got, tc.want)
			}
		})
	}
}

func TestChatRecallMemories_RealBackendProjectFactStillInjects(t *testing.T) {
	layered, baseSvc := newPromptLayeredMemoryServiceForTest(t)
	if _, err := baseSvc.Remember(context.Background(), "Alpha project timeline April 2026", []string{"project"}); err != nil {
		t.Fatalf("Remember: %v", err)
	}
	handler := &ChatHandler{layeredMemory: layered}
	if shouldSkipPromptMemoryRecall("Alpha project timeline April 2026") {
		t.Fatal("project fact query should not be skipped by prompt memory recall heuristic")
	}

	got := handler.recallMemories(context.Background(), "Alpha project timeline April 2026", MemoryRecallModeBalanced)
	if !strings.Contains(got, "<memory_context>") {
		t.Fatalf("expected memory context from real backend, got %q", got)
	}
	if !strings.Contains(got, "Alpha project timeline April 2026") {
		t.Fatalf("expected stored project fact in memory context, got %q", got)
	}
}
