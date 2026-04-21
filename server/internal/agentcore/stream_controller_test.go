package agentcore

import (
	"context"
	"testing"
	"time"
)

func TestStreamController_RegisterAndUnregister(t *testing.T) {
	sc := NewStreamController()

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sc.Register("session1", cancel)

	if sc.ActiveSessions() != 1 {
		t.Errorf("Expected 1 active session, got %d", sc.ActiveSessions())
	}

	sc.Unregister("session1")

	if sc.ActiveSessions() != 0 {
		t.Errorf("Expected 0 active sessions, got %d", sc.ActiveSessions())
	}
}

func TestStreamController_Cancel(t *testing.T) {
	sc := NewStreamController()

	ctx, cancel := context.WithCancel(context.Background())
	sc.Register("session1", cancel)

	// Cancel should return true for existing session
	if !sc.Cancel("session1") {
		t.Error("Expected Cancel to return true for existing session")
	}

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("Context was not cancelled")
	}

	// Cancel should return false for non-existing session
	if sc.Cancel("session1") {
		t.Error("Expected Cancel to return false for already cancelled session")
	}

	if sc.Cancel("nonexistent") {
		t.Error("Expected Cancel to return false for non-existing session")
	}
}

func TestStreamController_CancelWithReasonStoresReasonUntilConsumed(t *testing.T) {
	sc := NewStreamController()

	ctx, cancel := context.WithCancel(context.Background())
	sc.Register("session1", cancel)

	if !sc.CancelWithReason("session1", "user_cancel") {
		t.Fatal("expected CancelWithReason to return true for existing session")
	}

	select {
	case <-ctx.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context was not cancelled")
	}

	if got := sc.CancelReason("session1"); got != "user_cancel" {
		t.Fatalf("CancelReason() = %q, want %q", got, "user_cancel")
	}
	if got := sc.ConsumeCancelReason("session1"); got != "user_cancel" {
		t.Fatalf("ConsumeCancelReason() = %q, want %q", got, "user_cancel")
	}
	if got := sc.CancelReason("session1"); got != "" {
		t.Fatalf("CancelReason() after consume = %q, want empty", got)
	}
}

func TestStreamController_CancelAll(t *testing.T) {
	sc := NewStreamController()

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	ctx3, cancel3 := context.WithCancel(context.Background())

	sc.Register("session1", cancel1)
	sc.Register("session2", cancel2)
	sc.Register("session3", cancel3)

	if sc.ActiveSessions() != 3 {
		t.Errorf("Expected 3 active sessions, got %d", sc.ActiveSessions())
	}

	count := sc.CancelAll()

	if count != 3 {
		t.Errorf("Expected CancelAll to return 3, got %d", count)
	}

	if sc.ActiveSessions() != 0 {
		t.Errorf("Expected 0 active sessions after CancelAll, got %d", sc.ActiveSessions())
	}

	// Verify all contexts were cancelled
	for i, ctx := range []context.Context{ctx1, ctx2, ctx3} {
		select {
		case <-ctx.Done():
			// Expected
		case <-time.After(100 * time.Millisecond):
			t.Errorf("Context %d was not cancelled", i+1)
		}
	}
}

func TestStreamController_ListActiveSessions(t *testing.T) {
	sc := NewStreamController()

	_, cancel1 := context.WithCancel(context.Background())
	_, cancel2 := context.WithCancel(context.Background())

	sc.Register("session1", cancel1)
	sc.Register("session2", cancel2)

	sessions := sc.ListActiveSessions()

	if len(sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(sessions))
	}

	// Check that both sessions are in the list
	found := make(map[string]bool)
	for _, s := range sessions {
		found[s] = true
	}

	if !found["session1"] || !found["session2"] {
		t.Error("Expected both session1 and session2 in the list")
	}
}

func TestStreamController_Concurrent(t *testing.T) {
	sc := NewStreamController()

	// Test concurrent access
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			ctx, cancel := context.WithCancel(context.Background())
			sessionID := string(rune('a' + id))
			sc.Register(sessionID, cancel)
			time.Sleep(10 * time.Millisecond)
			sc.Cancel(sessionID)
			_ = ctx.Done()
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if sc.ActiveSessions() != 0 {
		t.Errorf("Expected 0 active sessions after concurrent test, got %d", sc.ActiveSessions())
	}
}
