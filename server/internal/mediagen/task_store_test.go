package mediagen

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestTaskStore_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}

	task := &PersistentTask{
		ID:        "task-001",
		MessageID: "msg-001",
		Status:    TaskStatusPending,
		Type:      MediaTypeImage,
		Category:  "t2i",
		Provider:  "mulerouter",
		Model:     "nano-banana-pro",
		Source:    "web",
		Request:   `{"prompt":"a cat"}`,
	}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("task-001")
	if err != nil {
		t.Fatal(err)
	}
	if got.MessageID != "msg-001" {
		t.Errorf("MessageID = %q, want %q", got.MessageID, "msg-001")
	}
	if got.Status != TaskStatusPending {
		t.Errorf("Status = %q, want %q", got.Status, TaskStatusPending)
	}
}

func TestTaskStore_GetScopedByUser(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Create(&PersistentTask{ID: "task-scope", UserID: "user-a", Status: TaskStatusPending, Type: MediaTypeImage, Category: "t2i"}); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("task-scope", "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.UserID != "user-a" {
		t.Fatalf("got = %#v, want user-a task", got)
	}

	if _, err := store.Get("task-scope", "user-b"); err == nil {
		t.Fatal("expected scoped miss for other user")
	}
	if _, err := store.Get("task-scope", ""); err == nil {
		t.Fatal("expected scoped miss for anonymous/public scope")
	}
}

func TestTaskStore_UpdateStatus(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}

	task := &PersistentTask{
		ID:       "task-002",
		Status:   TaskStatusPending,
		Type:     MediaTypeVideo,
		Category: "t2v",
		Provider: "mulerouter",
		Model:    "wan2.6-t2v",
		Source:   "channel",
	}
	if err := store.Create(task); err != nil {
		t.Fatal(err)
	}

	// Update to processing
	if err := store.UpdateStatus("task-002", TaskStatusProcessing, 0.5, "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ := store.Get("task-002")
	if got.Status != TaskStatusProcessing {
		t.Errorf("Status = %q, want processing", got.Status)
	}
	if got.Progress != 0.5 {
		t.Errorf("Progress = %f, want 0.5", got.Progress)
	}

	// Update to succeeded
	resp := `{"created":1234,"data":[{"url":"http://example.com/video.mp4"}]}`
	if err := store.UpdateStatus("task-002", TaskStatusSucceeded, 1.0, "", resp); err != nil {
		t.Fatal(err)
	}
	got, _ = store.Get("task-002")
	if got.Status != TaskStatusSucceeded {
		t.Errorf("Status = %q, want succeeded", got.Status)
	}
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set for terminal state")
	}
}

func TestTaskStore_ListPending(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}

	// Create 3 tasks: pending, processing, succeeded
	for _, tc := range []struct {
		id     string
		status TaskStatus
	}{
		{"t1", TaskStatusPending},
		{"t2", TaskStatusProcessing},
		{"t3", TaskStatusSucceeded},
	} {
		pt := &PersistentTask{ID: tc.id, Status: tc.status, Type: MediaTypeImage, Category: "t2i"}
		if err := store.Create(pt); err != nil {
			t.Fatal(err)
		}
		// For succeeded, update status to set completed_at
		if tc.status == TaskStatusSucceeded {
			_ = store.UpdateStatus(tc.id, TaskStatusSucceeded, 1.0, "", "")
		}
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 2 {
		t.Errorf("ListPending returned %d tasks, want 2", len(pending))
	}
}

func TestTaskStore_GetByMessageID(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		id, msgID, userID string
	}{
		{"a1", "msg-A", "user-a"},
		{"a2", "msg-A", "user-a"},
		{"a3", "msg-A", "user-b"},
		{"b1", "msg-B", "user-b"},
	} {
		pt := &PersistentTask{ID: tc.id, UserID: tc.userID, MessageID: tc.msgID, Status: TaskStatusPending, Type: MediaTypeImage, Category: "t2i"}
		if err := store.Create(pt); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := store.GetByMessageID("msg-A", "user-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("GetByMessageID returned %d tasks, want 2", len(tasks))
	}

	otherTasks, err := store.GetByMessageID("msg-A", "user-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(otherTasks) != 1 || otherTasks[0].ID != "a3" {
		t.Fatalf("otherTasks = %#v, want only a3", otherTasks)
	}
}

func TestPersistentTask_ToMediaTask(t *testing.T) {
	now := time.Now().UTC()
	pt := &PersistentTask{
		ID:        "task-x",
		MessageID: "msg-x",
		Status:    TaskStatusSucceeded,
		Type:      MediaTypeImage,
		Category:  "t2i",
		Provider:  "mulerouter",
		Model:     "nano-banana-pro",
		Source:    "web",
		Request:   `{"prompt":"hello","type":"image"}`,
		Response:  `{"created":1234,"data":[{"url":"http://example.com/img.png"}]}`,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mt := pt.ToMediaTask()
	if mt.ID != "task-x" {
		t.Errorf("ID = %q, want task-x", mt.ID)
	}
	if mt.MessageID != "msg-x" {
		t.Errorf("MessageID = %q, want msg-x", mt.MessageID)
	}
	if mt.Request == nil {
		t.Error("Request should be deserialized")
	}
	if mt.Response == nil {
		t.Error("Response should be deserialized")
	}
	if mt.Response != nil && len(mt.Response.Data) != 1 {
		t.Errorf("Response.Data length = %d, want 1", len(mt.Response.Data))
	}
}

func TestTaskStore_GetStatsScopedByUser(t *testing.T) {
	db := newTestDB(t)
	store, err := NewTaskStore(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		id, userID, model string
	}{
		{"s1", "user-a", "m1"},
		{"s2", "user-b", "m2"},
	} {
		pt := &PersistentTask{ID: tc.id, UserID: tc.userID, Status: TaskStatusSucceeded, Type: MediaTypeImage, Category: "t2i", Model: tc.model, Response: `{"created":1,"data":[{"url":"http://x"}]}`}
		if err := store.Create(pt); err != nil {
			t.Fatal(err)
		}
		if err := store.UpdateStatus(tc.id, TaskStatusSucceeded, 1, "", pt.Response); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := store.GetStats("user-a")
	if err != nil {
		t.Fatal(err)
	}
	if stats.Succeeded != 1 || stats.TotalTasks != 1 {
		t.Fatalf("stats = %#v, want only one user-a task", stats)
	}
}

func TestTaskStore_UsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "media_tasks.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewTaskStore(writeDB); err != nil {
		t.Fatalf("NewTaskStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewTaskStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewTaskStoreWithReadDB: %v", err)
	}
	if store.readDB == nil || store.readDB == store.db {
		t.Fatal("expected separate reader db")
	}

	pt := &PersistentTask{
		ID:        "task-reader",
		UserID:    "user-reader",
		MessageID: "msg-reader",
		Status:    TaskStatusPending,
		Type:      MediaTypeImage,
		Category:  "t2i",
		Provider:  "reader-provider",
		Model:     "reader-model",
		Source:    "web",
		Request:   `{"prompt":"reader"}`,
	}
	if err := store.Create(pt); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := store.Get("task-reader", "user-reader")
	if err != nil {
		t.Fatalf("Get via reader: %v", err)
	}
	if got == nil || got.ID != "task-reader" {
		t.Fatalf("unexpected task via reader: %+v", got)
	}

	byMessage, err := store.GetByMessageID("msg-reader", "user-reader")
	if err != nil {
		t.Fatalf("GetByMessageID via reader: %v", err)
	}
	if len(byMessage) != 1 || byMessage[0].ID != "task-reader" {
		t.Fatalf("unexpected message tasks via reader: %+v", byMessage)
	}

	pending, err := store.ListPending()
	if err != nil {
		t.Fatalf("ListPending via reader: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != "task-reader" {
		t.Fatalf("unexpected pending tasks via reader: %+v", pending)
	}
}

func TestTaskStore_GetStatsUsesReaderDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "media_tasks_stats.db")

	writeDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("sql.Open(write): %v", err)
	}
	defer writeDB.Close()

	if _, err := NewTaskStore(writeDB); err != nil {
		t.Fatalf("NewTaskStore(bootstrap): %v", err)
	}

	readDB, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatalf("sql.Open(read): %v", err)
	}
	defer readDB.Close()

	store, err := NewTaskStoreWithReadDB(writeDB, readDB)
	if err != nil {
		t.Fatalf("NewTaskStoreWithReadDB: %v", err)
	}

	succeeded := &PersistentTask{
		ID:        "task-stats-success",
		UserID:    "user-stats",
		MessageID: "msg-stats",
		Status:    TaskStatusPending,
		Type:      MediaTypeImage,
		Category:  "t2i",
		Provider:  "stats-provider",
		Model:     "stats-model",
		Source:    "web",
		Response:  `{"created":1,"data":[{"url":"http://x"}]}`,
	}
	if err := store.Create(succeeded); err != nil {
		t.Fatalf("Create(succeeded): %v", err)
	}
	if err := store.UpdateStatus(succeeded.ID, TaskStatusSucceeded, 1, "", succeeded.Response); err != nil {
		t.Fatalf("UpdateStatus(succeeded): %v", err)
	}

	failed := &PersistentTask{
		ID:        "task-stats-failed",
		UserID:    "user-stats",
		MessageID: "msg-stats",
		Status:    TaskStatusPending,
		Type:      MediaTypeVideo,
		Category:  "t2v",
		Provider:  "stats-provider",
		Model:     "stats-model",
		Source:    "web",
	}
	if err := store.Create(failed); err != nil {
		t.Fatalf("Create(failed): %v", err)
	}
	if err := store.UpdateStatus(failed.ID, TaskStatusFailed, 1, "boom", ""); err != nil {
		t.Fatalf("UpdateStatus(failed): %v", err)
	}

	otherUser := &PersistentTask{
		ID:        "task-stats-other",
		UserID:    "user-other",
		MessageID: "msg-other",
		Status:    TaskStatusPending,
		Type:      MediaTypeImage,
		Category:  "t2i",
		Provider:  "other-provider",
		Model:     "other-model",
		Source:    "web",
		Response:  `{"created":1,"data":[{"url":"http://y"}]}`,
	}
	if err := store.Create(otherUser); err != nil {
		t.Fatalf("Create(other): %v", err)
	}
	if err := store.UpdateStatus(otherUser.ID, TaskStatusSucceeded, 1, "", otherUser.Response); err != nil {
		t.Fatalf("UpdateStatus(other): %v", err)
	}

	if err := writeDB.Close(); err != nil {
		t.Fatalf("close writer db: %v", err)
	}
	writeDB = nil

	stats, err := store.GetStats("user-stats")
	if err != nil {
		t.Fatalf("GetStats via reader: %v", err)
	}
	if stats.Succeeded != 1 || stats.Failed != 1 || stats.TotalTasks != 2 {
		t.Fatalf("unexpected stats via reader: %+v", stats)
	}
	if stats.TasksByType[string(MediaTypeImage)] != 1 {
		t.Fatalf("unexpected TasksByType via reader: %+v", stats.TasksByType)
	}
	if stats.TasksByCategory["t2i"] != 1 {
		t.Fatalf("unexpected TasksByCategory via reader: %+v", stats.TasksByCategory)
	}
	if stats.TasksByProvider["stats-provider"] != 1 {
		t.Fatalf("unexpected TasksByProvider via reader: %+v", stats.TasksByProvider)
	}
}
