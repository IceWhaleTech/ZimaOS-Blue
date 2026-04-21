package agentcore

import (
	"context"
	"sync"
)

// StreamController manages active streaming sessions and provides cancellation.
type StreamController struct {
	mu            sync.RWMutex
	sessions      map[string]context.CancelFunc
	cancelReasons map[string]string
}

// NewStreamController creates a new StreamController.
func NewStreamController() *StreamController {
	return &StreamController{
		sessions:      make(map[string]context.CancelFunc),
		cancelReasons: make(map[string]string),
	}
}

// Register registers a new streaming session with its cancel function.
func (sc *StreamController) Register(sessionID string, cancel context.CancelFunc) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.sessions[sessionID] = cancel
	delete(sc.cancelReasons, sessionID)
}

// Unregister removes a streaming session.
func (sc *StreamController) Unregister(sessionID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	delete(sc.sessions, sessionID)
	delete(sc.cancelReasons, sessionID)
}

// Cancel cancels a streaming session by ID.
func (sc *StreamController) Cancel(sessionID string) bool {
	return sc.CancelWithReason(sessionID, "")
}

// CancelWithReason cancels a streaming session by ID and stores a reason for later inspection.
func (sc *StreamController) CancelWithReason(sessionID string, reason string) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if cancel, ok := sc.sessions[sessionID]; ok {
		if reason != "" {
			sc.cancelReasons[sessionID] = reason
		}
		cancel()
		delete(sc.sessions, sessionID)
		return true
	}
	return false
}

// CancelAll cancels all active streaming sessions.
func (sc *StreamController) CancelAll() int {
	return sc.CancelAllWithReason("")
}

// CancelAllWithReason cancels all active streaming sessions and stores a shared reason.
func (sc *StreamController) CancelAllWithReason(reason string) int {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	count := len(sc.sessions)
	for id, cancel := range sc.sessions {
		if reason != "" {
			sc.cancelReasons[id] = reason
		}
		cancel()
		delete(sc.sessions, id)
	}
	return count
}

// CancelReason returns the stored cancellation reason for a session, if any.
func (sc *StreamController) CancelReason(sessionID string) string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.cancelReasons[sessionID]
}

// ConsumeCancelReason returns and clears the stored cancellation reason for a session.
func (sc *StreamController) ConsumeCancelReason(sessionID string) string {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	reason := sc.cancelReasons[sessionID]
	delete(sc.cancelReasons, sessionID)
	return reason
}

// ActiveSessions returns the number of active streaming sessions.
func (sc *StreamController) ActiveSessions() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.sessions)
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

// DefaultStreamController is the global stream controller instance.
var DefaultStreamController = NewStreamController()
