package memory

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
)

// ErrNotFound is returned when a memory is not found.
var ErrNotFound = errors.New("memory not found")

// MemoryBackend defines the interface for memory storage backends.
type MemoryBackend interface {
	// Remember stores a new memory.
	Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error)
	// Recall retrieves relevant memories.
	Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error)
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

// UnifiedMemoryService provides a unified interface that can switch between backends.
type UnifiedMemoryService struct {
	localBackend  *MemoryService
	activeBackend MemoryBackend
	mu            sync.RWMutex
}

// NewUnifiedMemoryService creates a new unified memory service.
func NewUnifiedMemoryService(localService *MemoryService, cfg config.MemoryConfig) *UnifiedMemoryService {
	svc := &UnifiedMemoryService{
		localBackend: localService,
	}

	if localService != nil {
		svc.activeBackend = &LocalBackendAdapter{service: localService}
	}

	return svc
}

// SetBackend switches the active backend.
func (s *UnifiedMemoryService) SetBackend(backend string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch backend {
	case "local":
		if s.localBackend == nil {
			return fmt.Errorf("local backend not available")
		}
		s.activeBackend = &LocalBackendAdapter{service: s.localBackend}
	default:
		return fmt.Errorf("unknown backend: %s", backend)
	}

	return nil
}

// GetActiveBackend returns the name of the active backend.
func (s *UnifiedMemoryService) GetActiveBackend() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.activeBackend == nil {
		return ""
	}
	return s.activeBackend.Name()
}

// Remember stores a new memory using the active backend.
func (s *UnifiedMemoryService) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return backend.Remember(ctx, content, tags)
}

// Recall retrieves relevant memories using the active backend.
func (s *UnifiedMemoryService) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return backend.Recall(ctx, query, limit)
}

// Forget removes a memory by ID using the active backend.
func (s *UnifiedMemoryService) Forget(ctx context.Context, id string) error {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return fmt.Errorf("no memory backend available")
	}
	return backend.Forget(ctx, id)
}

// ForgetAll removes all memories using the active backend.
func (s *UnifiedMemoryService) ForgetAll(ctx context.Context) error {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return fmt.Errorf("no memory backend available")
	}
	return backend.ForgetAll(ctx)
}

// Get retrieves a memory by ID using the active backend.
func (s *UnifiedMemoryService) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return backend.Get(ctx, id)
}

// Prune removes old memories using the active backend.
func (s *UnifiedMemoryService) Prune(ctx context.Context) (int, error) {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return 0, fmt.Errorf("no memory backend available")
	}
	return backend.Prune(ctx)
}

// Stats returns memory statistics from the active backend.
func (s *UnifiedMemoryService) Stats(ctx context.Context) (*MemoryStats, error) {
	s.mu.RLock()
	backend := s.activeBackend
	s.mu.RUnlock()

	if backend == nil {
		return nil, fmt.Errorf("no memory backend available")
	}
	return backend.Stats(ctx)
}

// LocalBackendAdapter adapts MemoryService to MemoryBackend interface.
type LocalBackendAdapter struct {
	service *MemoryService
}

func (a *LocalBackendAdapter) Name() string {
	return "local"
}

func (a *LocalBackendAdapter) Remember(ctx context.Context, content string, tags []string) (*MemoryChunk, error) {
	return a.service.Remember(ctx, content, tags)
}

func (a *LocalBackendAdapter) Recall(ctx context.Context, query string, limit int) ([]HybridSearchResult, error) {
	return a.service.Recall(ctx, query, limit)
}

func (a *LocalBackendAdapter) Forget(ctx context.Context, id string) error {
	return a.service.Forget(ctx, id)
}

func (a *LocalBackendAdapter) ForgetAll(ctx context.Context) error {
	return a.service.ForgetAll(ctx)
}

func (a *LocalBackendAdapter) Get(ctx context.Context, id string) (*MemoryChunk, error) {
	return a.service.Get(ctx, id)
}

func (a *LocalBackendAdapter) Prune(ctx context.Context) (int, error) {
	return a.service.Prune(ctx)
}

func (a *LocalBackendAdapter) Stats(ctx context.Context) (*MemoryStats, error) {
	stats, err := a.service.Stats(ctx)
	if err != nil {
		return nil, err
	}
	oldestStr := ""
	newestStr := ""
	if !stats.VectorStoreStats.OldestChunk.IsZero() {
		oldestStr = stats.VectorStoreStats.OldestChunk.Format("2006-01-02T15:04:05Z07:00")
	}
	if !stats.VectorStoreStats.NewestChunk.IsZero() {
		newestStr = stats.VectorStoreStats.NewestChunk.Format("2006-01-02T15:04:05Z07:00")
	}
	return &MemoryStats{
		TotalChunks:    stats.VectorStoreStats.ChunkCount,
		TotalSizeBytes: 0, // Not tracked in local backend
		OldestChunk:    oldestStr,
		NewestChunk:    newestStr,
		Backend:        "local",
	}, nil
}
