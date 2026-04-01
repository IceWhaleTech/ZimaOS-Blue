package session

import (
	"path/filepath"
	"testing"

	ctxpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/context"
)

func TestSQLiteSessionStoreSaveLoadAndArchive(t *testing.T) {
	store, err := NewSQLiteSessionStore(":memory:", 2048)
	if err != nil {
		t.Fatalf("create sqlite session store: %v", err)
	}
	defer store.Close()

	id := SessionID{
		AgentID:   "agent-1",
		ChannelID: "channel-1",
		PeerID:    "peer-1",
		ThreadID:  "thread-1",
	}
	session := NewSession(id, 2048)
	session.Metadata.Title = "Support Chat"
	session.Metadata.Tags = []string{"support", "priority"}
	session.SetSystemPrompt("You are helpful.")
	session.AddMessage(ctxpkg.Message{Role: ctxpkg.RoleUser, Content: "hello"})
	session.AddMessage(ctxpkg.Message{Role: ctxpkg.RoleAssistant, Content: "hi there"})

	if err := store.Save(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	loaded, err := store.Load(id)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected loaded session")
	}
	if loaded.Metadata.Title != "Support Chat" {
		t.Fatalf("expected title Support Chat, got %q", loaded.Metadata.Title)
	}
	if len(loaded.GetMessages()) != 3 {
		t.Fatalf("expected 3 messages including system prompt, got %d", len(loaded.GetMessages()))
	}
	messages := loaded.Context.Messages()
	if len(messages) == 0 || messages[0].Role != ctxpkg.RoleSystem || messages[0].Content != "You are helpful." {
		t.Fatalf("expected system prompt to round-trip in messages, got %+v", messages)
	}

	sessions, err := store.List(SessionFilter{AgentID: "agent-1"})
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	if err := store.Archive(id); err != nil {
		t.Fatalf("archive session: %v", err)
	}
	archived, err := store.Load(id)
	if err != nil {
		t.Fatalf("load archived session: %v", err)
	}
	if archived == nil || archived.State != SessionStateArchived {
		t.Fatalf("expected archived session state, got %+v", archived)
	}
}

func TestSQLiteSessionStoreUsesReaderPoolForFileDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions.db")
	store, err := NewSQLiteSessionStore(dbPath, 1024)
	if err != nil {
		t.Fatalf("create file-backed sqlite session store: %v", err)
	}
	defer store.Close()

	if store.readDB == nil {
		t.Fatal("expected read db to be initialized")
	}
	if store.readDB == store.db {
		t.Fatal("expected file-backed session store to use a separate read db")
	}

	session := NewSession(SessionID{
		AgentID:   "agent-1",
		ChannelID: "channel-1",
		PeerID:    "peer-1",
	}, 1024)
	session.AddMessage(ctxpkg.Message{Role: ctxpkg.RoleUser, Content: "persist me"})

	if err := store.Save(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("load session through reader pool: %v", err)
	}
	if loaded == nil || len(loaded.GetMessages()) != 1 {
		t.Fatalf("expected persisted session via reader pool, got %+v", loaded)
	}
}

func TestSQLiteSessionStoreReadsStillWorkAfterWriterClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sessions-reader-close.db")
	store, err := NewSQLiteSessionStore(dbPath, 1024)
	if err != nil {
		t.Fatalf("create file-backed sqlite session store: %v", err)
	}
	defer func() {
		if store.readDB != nil && store.readDB != store.db {
			_ = store.readDB.Close()
		}
	}()

	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate read db")
	}

	session := NewSession(SessionID{
		AgentID:   "agent-reader",
		ChannelID: "channel-reader",
		PeerID:    "peer-reader",
		ThreadID:  "thread-reader",
	}, 1024)
	session.Metadata.Title = "Reader path"
	session.AddMessage(ctxpkg.Message{Role: ctxpkg.RoleUser, Content: "persist me"})

	if err := store.Save(session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	if err := store.db.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	loaded, err := store.Load(session.ID)
	if err != nil {
		t.Fatalf("load session after writer close: %v", err)
	}
	if loaded == nil || loaded.Metadata.Title != "Reader path" {
		t.Fatalf("unexpected loaded session via reader db: %+v", loaded)
	}

	sessions, err := store.List(SessionFilter{AgentID: "agent-reader"})
	if err != nil {
		t.Fatalf("list sessions after writer close: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID.String() != session.ID.String() {
		t.Fatalf("unexpected sessions via reader db: %+v", sessions)
	}
}
