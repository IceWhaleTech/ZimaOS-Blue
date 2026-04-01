package convert

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

func TestStoreUsesReaderDBForReads(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "convert.db")

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

	task := &ConvertTask{
		ID:             "task-reader",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionConvert,
		Status:         StatusPending,
		Request: &TaskRequest{
			Action:  ActionConvert,
			Sources: []string{"att:reader"},
		},
	}
	if err := store.CreateTask(task); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	source := &StoredSource{
		ID:             "source-reader",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Name:           "reader.txt",
		MimeType:       "text/plain",
		Path:           filepath.Join(t.TempDir(), "reader.txt"),
		SizeBytes:      42,
		CreatedAt:      time.Now().UTC(),
	}
	if err := store.CreateSource(source); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	gotTask, err := store.GetTask("task-reader")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if gotTask == nil || len(gotTask.Sources) != 1 || gotTask.Sources[0] != "att:reader" {
		t.Fatalf("unexpected task via reader: %+v", gotTask)
	}

	tasks, err := store.ListTasks("user-1", "conv-1", 10)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task-reader" {
		t.Fatalf("unexpected tasks via reader: %+v", tasks)
	}

	gotSource, err := store.GetSource("source-reader")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if gotSource == nil || gotSource.Name != "reader.txt" {
		t.Fatalf("unexpected source via reader: %+v", gotSource)
	}

	sources, err := store.ListSources("user-1", "conv-1", 10)
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 1 || sources[0].ID != "source-reader" {
		t.Fatalf("unexpected sources via reader: %+v", sources)
	}
}

func TestStoreCleanupExpired(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	store, err := NewStore(db)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	oldTime := time.Now().UTC().Add(-48 * time.Hour)
	newTime := time.Now().UTC()

	if err := store.CreateTask(&ConvertTask{
		ID:             "task-old",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionConvert,
		Status:         StatusFailed,
		CreatedAt:      oldTime,
		UpdatedAt:      oldTime,
	}); err != nil {
		t.Fatalf("CreateTask(old): %v", err)
	}
	if err := store.CreateTask(&ConvertTask{
		ID:             "task-new",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Action:         ActionConvert,
		Status:         StatusSucceeded,
		CreatedAt:      newTime,
		UpdatedAt:      newTime,
	}); err != nil {
		t.Fatalf("CreateTask(new): %v", err)
	}

	oldPath := filepath.Join(t.TempDir(), "old.txt")
	newPath := filepath.Join(t.TempDir(), "new.txt")
	if err := store.CreateSource(&StoredSource{
		ID:             "source-old",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Name:           "old.txt",
		Path:           oldPath,
		CreatedAt:      oldTime,
	}); err != nil {
		t.Fatalf("CreateSource(old): %v", err)
	}
	if err := store.CreateSource(&StoredSource{
		ID:             "source-new",
		UserID:         "user-1",
		ConversationID: "conv-1",
		Name:           "new.txt",
		Path:           newPath,
		CreatedAt:      newTime,
	}); err != nil {
		t.Fatalf("CreateSource(new): %v", err)
	}

	taskIDs, _, sourcePaths, err := store.CleanupExpired(time.Now().UTC().Add(-24 * time.Hour))
	if err != nil {
		t.Fatalf("CleanupExpired: %v", err)
	}
	if len(taskIDs) != 1 || taskIDs[0] != "task-old" {
		t.Fatalf("unexpected expired task ids: %+v", taskIDs)
	}
	if len(sourcePaths) != 1 || sourcePaths[0] != oldPath {
		t.Fatalf("unexpected expired source paths: %+v", sourcePaths)
	}

	if _, err := store.GetTask("task-old"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetTask(task-old) err = %v, want sql.ErrNoRows", err)
	}
	if _, err := store.GetSource("source-old"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetSource(source-old) err = %v, want sql.ErrNoRows", err)
	}

	if _, err := store.GetTask("task-new"); err != nil {
		t.Fatalf("GetTask(task-new): %v", err)
	}
	if _, err := store.GetSource("source-new"); err != nil {
		t.Fatalf("GetSource(source-new): %v", err)
	}
}
