package mediagen

import (
	"database/sql"
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

	for _, id := range []string{"a1", "a2", "b1"} {
		msgID := "msg-A"
		if id == "b1" {
			msgID = "msg-B"
		}
		pt := &PersistentTask{ID: id, MessageID: msgID, Status: TaskStatusPending, Type: MediaTypeImage, Category: "t2i"}
		if err := store.Create(pt); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := store.GetByMessageID("msg-A")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Errorf("GetByMessageID returned %d tasks, want 2", len(tasks))
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
