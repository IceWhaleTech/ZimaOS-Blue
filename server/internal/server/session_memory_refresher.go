package server

import (
	"context"
	"hash/fnv"
	"strings"
	"sync"
)

// SessionMemoryRefresher adapts MemoryHandler to session.MemoryRefresher.
// It persists extracted bullet memories into the existing unified memory backend.
type SessionMemoryRefresher struct {
	memoryHandler *MemoryHandler

	mu          sync.Mutex
	lastByScope map[string]uint64
}

// NewSessionMemoryRefresher creates a session memory refresher.
func NewSessionMemoryRefresher(memoryHandler *MemoryHandler) *SessionMemoryRefresher {
	return &SessionMemoryRefresher{
		memoryHandler: memoryHandler,
		lastByScope:   make(map[string]uint64),
	}
}

// RefreshMemory stores extracted session memory if memory services are available.
func (r *SessionMemoryRefresher) RefreshMemory(ctx context.Context, extracted string, sessionID string) error {
	if r.memoryHandler == nil {
		return nil
	}

	content := strings.TrimSpace(extracted)
	if content == "" || content == "NO_MEMORY_NEEDED" {
		return nil
	}
	if r.isDuplicate(sessionID, content) {
		return nil
	}

	r.memoryHandler.Init()
	svc := r.memoryHandler.GetUnifiedService()
	if svc == nil {
		return nil
	}

	tags := []string{"session-compaction"}
	if sessionID != "" {
		tags = append(tags, "session:"+sessionID)
	}
	_, err := svc.Remember(ctx, content, tags)
	return err
}

func (r *SessionMemoryRefresher) isDuplicate(scope string, content string) bool {
	key := strings.TrimSpace(scope)
	if key == "" {
		key = "_global"
	}

	h := fnv.New64a()
	_, _ = h.Write([]byte(content))
	sum := h.Sum64()

	r.mu.Lock()
	defer r.mu.Unlock()
	if prev, ok := r.lastByScope[key]; ok && prev == sum {
		return true
	}
	r.lastByScope[key] = sum
	if len(r.lastByScope) > 1024 {
		for k := range r.lastByScope {
			if k != key {
				delete(r.lastByScope, k)
				break
			}
		}
	}
	return false
}
