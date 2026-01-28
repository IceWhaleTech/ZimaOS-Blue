package claudecode

import (
	"context"
	"testing"
	"time"
)

func TestNewInMemorySessionStore(t *testing.T) {
	store := NewInMemorySessionStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSessionStoreCreate(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	session := &CliSession{
		AgentID:      "agent-1",
		ChannelID:    "channel-1",
		WorkspaceDir: "/tmp/workspace",
		Model:        "opus",
	}

	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// ID should be generated
	if session.ID == "" {
		t.Error("expected session ID to be generated")
	}

	// CreatedAt should be set
	if session.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// LastUsedAt should be set
	if session.LastUsedAt.IsZero() {
		t.Error("expected LastUsedAt to be set")
	}
}

func TestSessionStoreGet(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	// Create a session
	session := &CliSession{
		ID:      "test-session-123",
		AgentID: "agent-1",
	}
	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Get the session
	retrieved, err := store.Get(ctx, "test-session-123")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.ID != "test-session-123" {
		t.Errorf("Get() ID = '%s', want 'test-session-123'", retrieved.ID)
	}

	if retrieved.AgentID != "agent-1" {
		t.Errorf("Get() AgentID = '%s', want 'agent-1'", retrieved.AgentID)
	}

	// Get non-existent session
	_, err = store.Get(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
	if _, ok := err.(ErrSessionNotFound); !ok {
		t.Errorf("expected ErrSessionNotFound, got %T", err)
	}
}

func TestSessionStoreUpdate(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	// Create a session
	session := &CliSession{
		ID:           "test-session-123",
		AgentID:      "agent-1",
		MessageCount: 0,
	}
	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update the session
	session.MessageCount = 5
	err = store.Update(ctx, session)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Verify update
	retrieved, err := store.Get(ctx, "test-session-123")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if retrieved.MessageCount != 5 {
		t.Errorf("MessageCount = %d, want 5", retrieved.MessageCount)
	}

	// Update non-existent session
	nonExistent := &CliSession{ID: "non-existent"}
	err = store.Update(ctx, nonExistent)
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestSessionStoreDelete(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	// Create a session
	session := &CliSession{ID: "test-session-123"}
	err := store.Create(ctx, session)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the session
	err = store.Delete(ctx, "test-session-123")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deletion
	_, err = store.Get(ctx, "test-session-123")
	if err == nil {
		t.Error("expected error after deletion")
	}

	// Delete non-existent session
	err = store.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent session")
	}
}

func TestSessionStoreList(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	// Create multiple sessions
	sessions := []*CliSession{
		{ID: "session-1", AgentID: "agent-1", ChannelID: "channel-1"},
		{ID: "session-2", AgentID: "agent-1", ChannelID: "channel-2"},
		{ID: "session-3", AgentID: "agent-2", ChannelID: "channel-1"},
	}

	for _, s := range sessions {
		err := store.Create(ctx, s)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	// List all sessions
	result, err := store.List(ctx, nil)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result) != 3 {
		t.Errorf("List() returned %d sessions, want 3", len(result))
	}

	// Filter by AgentID
	result, err = store.List(ctx, &SessionFilter{AgentID: "agent-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List(AgentID=agent-1) returned %d sessions, want 2", len(result))
	}

	// Filter by ChannelID
	result, err = store.List(ctx, &SessionFilter{ChannelID: "channel-1"})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List(ChannelID=channel-1) returned %d sessions, want 2", len(result))
	}

	// Filter with limit
	result, err = store.List(ctx, &SessionFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("List(Limit=2) returned %d sessions, want 2", len(result))
	}
}

func TestSessionStoreCleanupExpired(t *testing.T) {
	store := NewInMemorySessionStore()
	ctx := context.Background()

	// Create sessions with different last used times
	now := time.Now()

	oldSession := &CliSession{
		ID:         "old-session",
		LastUsedAt: now.Add(-2 * time.Hour),
	}
	newSession := &CliSession{
		ID:         "new-session",
		LastUsedAt: now,
	}

	store.Create(ctx, oldSession)
	store.Create(ctx, newSession)

	// Cleanup with 1 hour TTL
	count, err := store.CleanupExpired(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("CleanupExpired() error = %v", err)
	}

	if count != 1 {
		t.Errorf("CleanupExpired() removed %d sessions, want 1", count)
	}

	// Verify old session is gone
	_, err = store.Get(ctx, "old-session")
	if err == nil {
		t.Error("expected old session to be deleted")
	}

	// Verify new session still exists
	_, err = store.Get(ctx, "new-session")
	if err != nil {
		t.Error("expected new session to still exist")
	}
}

func TestSessionManager(t *testing.T) {
	store := NewInMemorySessionStore()
	manager := NewSessionManager(store, 24*time.Hour, 1*time.Hour)

	if manager == nil {
		t.Fatal("expected non-nil manager")
	}

	// Test GetOrCreate with new session
	ctx := context.Background()
	session, err := manager.GetOrCreate(ctx, "", &CliSession{
		AgentID: "agent-1",
		Model:   "opus",
	})
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if session.ID == "" {
		t.Error("expected session ID to be generated")
	}
	if session.AgentID != "agent-1" {
		t.Errorf("AgentID = '%s', want 'agent-1'", session.AgentID)
	}

	// Test GetOrCreate with existing session
	session2, err := manager.GetOrCreate(ctx, session.ID, nil)
	if err != nil {
		t.Fatalf("GetOrCreate() error = %v", err)
	}

	if session2.ID != session.ID {
		t.Errorf("expected same session ID, got '%s' and '%s'", session.ID, session2.ID)
	}

	// Test Touch
	err = manager.Touch(ctx, session.ID)
	if err != nil {
		t.Fatalf("Touch() error = %v", err)
	}

	updated, _ := store.Get(ctx, session.ID)
	if updated.MessageCount != 1 {
		t.Errorf("MessageCount = %d, want 1", updated.MessageCount)
	}
}

func TestSessionManagerStartStop(t *testing.T) {
	store := NewInMemorySessionStore()
	manager := NewSessionManager(store, 24*time.Hour, 100*time.Millisecond)

	// Start should not panic
	manager.Start()

	// Give it time to run at least one cleanup cycle
	time.Sleep(200 * time.Millisecond)

	// Stop should not panic
	manager.Stop()
}
