package server

import (
	"context"
	"sync"
)

// StreamController manages active streaming sessions and provides cancellation.
// Extracted for use without the claudecode package (avoids antivirus triggers).
type StreamController struct {
	mu       sync.RWMutex
	sessions map[string]context.CancelFunc
}

// NewStreamController creates a new StreamController.
func NewStreamController() *StreamController {
	return &StreamController{
		sessions: make(map[string]context.CancelFunc),
	}
}

// Register registers a new streaming session with its cancel function.
func (sc *StreamController) Register(sessionID string, cancel context.CancelFunc) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.sessions[sessionID] = cancel
}

// Unregister removes a streaming session.
func (sc *StreamController) Unregister(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.sessions, sessionID)
}

// Cancel cancels a streaming session by ID.
func (sc *StreamController) Cancel(sessionID string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if cancel, ok := sc.sessions[sessionID]; ok {
		cancel()
		delete(sc.sessions, sessionID)
		return true
	}
	return false
}

// CancelAll cancels all active streaming sessions.
func (sc *StreamController) CancelAll() int {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	count := len(sc.sessions)
	for id, cancel := range sc.sessions {
		cancel()
		delete(sc.sessions, id)
	}
	return count
}

// ListActiveSessions returns a list of active session IDs.
func (sc *StreamController) ListActiveSessions() []string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	ids := make([]string, 0, len(sc.sessions))
	for id := range sc.sessions {
		ids = append(ids, id)
	}
	return ids
}
