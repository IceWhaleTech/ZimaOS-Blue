package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrNotFound is returned when a memory is not found.
var ErrNotFound = errors.New("memory not found")

// MemoryChunk represents a unit of stored memory.
type MemoryChunk struct {
	ID        string            `json:"id"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SearchResult represents a memory search result.
type SearchResult struct {
	Chunk        MemoryChunk `json:"chunk"`
	Score        float32     `json:"score"`
	KeywordScore float32     `json:"keyword_score,omitempty"`
	MatchTypes   []string    `json:"match_types"`
}

// MemoryBackend defines the interface for memory storage backends.
type MemoryBackend interface {
	// Remember stores a new memory.
	Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error)
	// Recall retrieves relevant memories.
	Recall(ctx context.Context, query string, limit int) ([]SearchResult, error)
	// Forget removes a memory by ID.
	Forget(ctx context.Context, id string) error
	// ForgetAll removes all memories.
	ForgetAll(ctx context.Context) error
	// Get retrieves a memory by ID.
	Get(ctx context.Context, id string) (*MemoryChunk, error)
	// Prune removes old memories.
	Prune(ctx context.Context) (int, error)
	// Stats returns memory statistics.
	Stats(ctx context.Context) (*MemoryStats, error)
	// Name returns the backend name.
	Name() string
}

// MemoryStats holds unified memory statistics.
type MemoryStats struct {
	TotalChunks    int    `json:"total_chunks"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	OldestChunk    string `json:"oldest_chunk,omitempty"`
	NewestChunk    string `json:"newest_chunk,omitempty"`
	Backend        string `json:"backend"`
}

// UnifiedMemoryService provides a unified interface backed by PureMarkdownBackend.
type UnifiedMemoryService struct {
	backend *PureMarkdownBackend
	mu      sync.RWMutex
}

// NewUnifiedMemoryService creates a new unified memory service with a markdown backend.
func NewUnifiedMemoryService(md *PureMarkdownBackend) *UnifiedMemoryService {
	return &UnifiedMemoryService{backend: md}
}

// GetActiveBackend returns the name of the active backend.
func (s *UnifiedMemoryService) GetActiveBackend() string {
	return "markdown"
}

// Remember stores a new memory.
func (s *UnifiedMemoryService) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return b.Remember(ctx, content, tags)
}

// Recall retrieves relevant memories.
func (s *UnifiedMemoryService) Recall(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return b.Recall(ctx, query, limit)
}

// Forget removes a memory by ID.
func (s *UnifiedMemoryService) Forget(ctx context.Context, id string) error {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return fmt.Errorf("no memory backend available")
	}
	return b.Forget(ctx, id)
}

// ForgetAll removes all memories.
func (s *UnifiedMemoryService) ForgetAll(ctx context.Context) error {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return fmt.Errorf("no memory backend available")
	}
	return b.ForgetAll(ctx)
}

// Get retrieves a memory by ID.
func (s *UnifiedMemoryService) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return b.Get(ctx, id)
}

// Prune removes old memories.
func (s *UnifiedMemoryService) Prune(ctx context.Context) (int, error) {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return 0, fmt.Errorf("no memory backend available")
	}
	return b.Prune(ctx)
}

// Stats returns memory statistics.
func (s *UnifiedMemoryService) Stats(ctx context.Context) (*MemoryStats, error) {
	s.mu.RLock()
	b := s.backend
	s.mu.RUnlock()
	if b == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return b.Stats(ctx)
}
