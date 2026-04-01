package agentsessions

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestSQLiteStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent-sessions.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewSQLiteStore(writeDB); err != nil {
		t.Fatalf("NewSQLiteStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewSQLiteStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewSQLiteStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	profile := &AgentProfile{
		ID:       "profile-reader",
		Protocol: ProtocolACP,
		Name:     "Reader Profile",
	}
	if err := store.SaveProfile(profile); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	session := &ExternalSession{
		ID:        "session-reader",
		ProfileID: profile.ID,
		Protocol:  ProtocolACP,
		UserID:    "user-a",
		Name:      "Reader Session",
		Status:    SessionStatusRunning,
	}
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	run := &ExternalRun{
		ID:        "run-reader",
		SessionID: session.ID,
		Status:    RunStatusRunning,
		Prompt:    "hello",
	}
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	event, err := store.AppendEvent(session.ID, run.ID, "assistant_message", map[string]interface{}{
		"role":    "assistant",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("AppendEvent: %v", err)
	}
	if event.ID == 0 {
		t.Fatal("expected appended event id")
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	gotProfile, err := store.GetProfile(profile.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if gotProfile.Name != profile.Name {
		t.Fatalf("profile name = %q, want %q", gotProfile.Name, profile.Name)
	}

	profiles, err := store.ListProfiles()
	if err != nil {
		t.Fatalf("ListProfiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0].ID != profile.ID {
		t.Fatalf("unexpected profiles: %+v", profiles)
	}

	gotSession, err := store.GetSession(session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if gotSession.Name != session.Name {
		t.Fatalf("session name = %q, want %q", gotSession.Name, session.Name)
	}

	sessions, err := store.ListSessions(10, 0, session.UserID, ProtocolACP)
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != session.ID {
		t.Fatalf("unexpected sessions: %+v", sessions)
	}

	gotRun, err := store.GetRun(run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if gotRun.Prompt != run.Prompt {
		t.Fatalf("run prompt = %q, want %q", gotRun.Prompt, run.Prompt)
	}

	runs, err := store.ListRuns(session.ID, 10)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID {
		t.Fatalf("unexpected runs: %+v", runs)
	}

	activeRun, err := store.LatestActiveRun(session.ID)
	if err != nil {
		t.Fatalf("LatestActiveRun: %v", err)
	}
	if activeRun == nil || activeRun.ID != run.ID {
		t.Fatalf("unexpected latest active run: %+v", activeRun)
	}

	events, err := store.ListEvents(session.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("unexpected events: %+v", events)
	}

	history, err := store.BuildHistory(session.ID, 10)
	if err != nil {
		t.Fatalf("BuildHistory: %v", err)
	}
	if len(history) != 1 || history[0].Content != "hello world" {
		t.Fatalf("unexpected history: %+v", history)
	}
}

func TestSQLiteStore_DeleteProfileNotFound(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	store, err := NewSQLiteStore(db)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}

	if err := store.DeleteProfile("missing"); err != ErrProfileNotFound {
		t.Fatalf("DeleteProfile(missing) err = %v, want %v", err, ErrProfileNotFound)
	}
}
