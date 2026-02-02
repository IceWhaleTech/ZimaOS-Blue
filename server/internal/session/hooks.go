package session

import (
	"context"
	"log"
)

// SessionHook defines a hook that can be triggered on session events.
type SessionHook interface {
	// OnSessionEnd is called when a session ends (archive or reset).
	OnSessionEnd(ctx context.Context, session *Session, reason EndReason) error
}

// EndReason indicates why a session ended.
type EndReason string

const (
	// EndReasonArchive indicates the session was archived.
	EndReasonArchive EndReason = "archive"
	// EndReasonReset indicates the session was reset.
	EndReasonReset EndReason = "reset"
	// EndReasonDelete indicates the session was deleted.
	EndReasonDelete EndReason = "delete"
	// EndReasonTimeout indicates the session timed out.
	EndReasonTimeout EndReason = "timeout"
	// EndReasonNew indicates a new session was started (via /new command).
	EndReasonNew EndReason = "new"
)

// HookManager manages session hooks.
type HookManager struct {
	hooks []SessionHook
}

// NewHookManager creates a new hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make([]SessionHook, 0),
	}
}

// Register registers a hook.
func (m *HookManager) Register(hook SessionHook) {
	m.hooks = append(m.hooks, hook)
}

// TriggerSessionEnd triggers all hooks for session end.
func (m *HookManager) TriggerSessionEnd(ctx context.Context, session *Session, reason EndReason) {
	for _, hook := range m.hooks {
		if err := hook.OnSessionEnd(ctx, session, reason); err != nil {
			log.Printf("[WARN] session hook failed: %v", err)
		}
	}
}
