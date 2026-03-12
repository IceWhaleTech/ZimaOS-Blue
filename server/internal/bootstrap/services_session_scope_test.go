package bootstrap

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestSessionListAdapterScopesToContextUser(t *testing.T) {
	store, err := memory.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	convA, err := store.CreateConversation(ctx, "A", "user-a")
	if err != nil {
		t.Fatalf("CreateConversation(user-a): %v", err)
	}
	convB, err := store.CreateConversation(ctx, "B", "user-b")
	if err != nil {
		t.Fatalf("CreateConversation(user-b): %v", err)
	}
	if _, err := store.AddMessage(ctx, convA.ID, memory.Message{Role: "user", Content: "from-a"}); err != nil {
		t.Fatalf("AddMessage(user-a): %v", err)
	}
	if _, err := store.AddMessage(ctx, convB.ID, memory.Message{Role: "user", Content: "from-b"}); err != nil {
		t.Fatalf("AddMessage(user-b): %v", err)
	}

	adapter := sessionListAdapter{store: store}
	scopedCtx := tools.WithUserID(context.Background(), "user-a")

	sessions, err := adapter.ListSessions(scopedCtx, 10, 0, "user-b")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != convA.ID {
		t.Fatalf("sessions = %#v, want only user-a session", sessions)
	}

	session, err := adapter.GetSession(scopedCtx, convB.ID)
	if err != nil {
		t.Fatalf("GetSession(other user): %v", err)
	}
	if session != nil {
		t.Fatalf("GetSession(other user) = %#v, want nil", session)
	}

	if _, err := adapter.GetSessionMessages(scopedCtx, convB.ID, 10, 0); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("GetSessionMessages(other user) err = %v, want ErrNotFound", err)
	}
	if _, err := adapter.AppendSessionMessage(scopedCtx, convB.ID, tools.SessionMessage{Role: "user", Content: "forbidden"}); !errors.Is(err, memory.ErrNotFound) {
		t.Fatalf("AppendSessionMessage(other user) err = %v, want ErrNotFound", err)
	}

	created, err := adapter.CreateSession(scopedCtx, "Scoped Session", "user-b", true)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if created.UserID != "user-a" {
		t.Fatalf("created.UserID = %q, want user-a", created.UserID)
	}
}
