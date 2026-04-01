package agent

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "agent.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewStore(writeDB); err != nil {
		t.Fatalf("NewStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	ctx := context.Background()
	task := &Task{
		ID:     "task-reader",
		UserID: "user-a",
		Goal:   "reader path",
		Status: TaskStatusExecuting,
	}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := store.AppendRuntimeEvent(ctx, RuntimeEvent{
		ID:           "event-reader",
		TaskID:       task.ID,
		StepIndex:    1,
		PlannerRound: 1,
		EventType:    "tool_call",
		PayloadJSON:  `{"ok":true}`,
		CreatedAt:    time.Now(),
	}); err != nil {
		t.Fatalf("AppendRuntimeEvent: %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}

	got, err := store.Get(ctx, task.ID, task.UserID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got == nil || got.Goal != task.Goal {
		t.Fatalf("unexpected task via reader: %+v", got)
	}

	list, err := store.ListByUser(ctx, task.UserID, 10)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 || list[0].ID != task.ID {
		t.Fatalf("unexpected list via reader: %+v", list)
	}

	count, err := store.CountRunning(ctx, task.UserID)
	if err != nil {
		t.Fatalf("CountRunning: %v", err)
	}
	if count != 1 {
		t.Fatalf("CountRunning = %d, want 1", count)
	}

	events, err := store.ListRuntimeEvents(ctx, task.ID)
	if err != nil {
		t.Fatalf("ListRuntimeEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != "event-reader" {
		t.Fatalf("unexpected runtime events via reader: %+v", events)
	}
}

func TestStore_UserScopeIsolation(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()

	task := &Task{
		ID:     "task-scope",
		UserID: "user-a",
		Goal:   "scope check",
		Status: TaskStatusPending,
	}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, err := store.Get(ctx, task.ID, "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get(wrong user) err = %v, want sql.ErrNoRows", err)
	}

	task.Goal = "mutated"
	if err := store.Update(ctx, task, "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Update(wrong user) err = %v, want sql.ErrNoRows", err)
	}

	if err := store.SetStatus(ctx, task.ID, TaskStatusFailed, "oops", "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("SetStatus(wrong user) err = %v, want sql.ErrNoRows", err)
	}

	if err := store.Delete(ctx, task.ID, "user-b"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Delete(wrong user) err = %v, want sql.ErrNoRows", err)
	}

	got, err := store.Get(ctx, task.ID, "user-a")
	if err != nil {
		t.Fatalf("Get(correct user): %v", err)
	}
	if got == nil || got.Status != TaskStatusPending || got.Goal != "scope check" {
		t.Fatalf("unexpected task after scoped failures: %+v", got)
	}
}
