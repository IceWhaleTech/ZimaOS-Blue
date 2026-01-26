package persistence

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.DBPath != "./data/scheduler.db" {
		t.Errorf("expected DBPath='./data/scheduler.db', got %s", config.DBPath)
	}
	if !config.Enabled {
		t.Error("expected Enabled=true by default")
	}
	if config.RetentionDays != 7 {
		t.Errorf("expected RetentionDays=7, got %d", config.RetentionDays)
	}
}

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := Config{
		DBPath:        dbPath,
		Enabled:       true,
		RetentionDays: 7,
	}

	store, err := NewStore(config)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestSaveAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create task
	task := &TaskRecord{
		ID:          "task-1",
		Name:        "test-task",
		Priority:    2,
		Status:      "pending",
		ScheduledAt: time.Now(),
		MaxRetries:  3,
		Timeout:     int64(5 * time.Minute),
		Metadata:    map[string]string{"key": "value"},
		HandlerName: "test-handler",
		HandlerData: []byte("test data"),
	}

	// Save task
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Get task
	retrieved, err := store.Get(task.ID)
	if err != nil {
		t.Fatalf("failed to get task: %v", err)
	}

	if retrieved.ID != task.ID {
		t.Errorf("expected ID %s, got %s", task.ID, retrieved.ID)
	}
	if retrieved.Name != task.Name {
		t.Errorf("expected Name %s, got %s", task.Name, retrieved.Name)
	}
	if retrieved.Priority != task.Priority {
		t.Errorf("expected Priority %d, got %d", task.Priority, retrieved.Priority)
	}
	if retrieved.Status != task.Status {
		t.Errorf("expected Status %s, got %s", task.Status, retrieved.Status)
	}
	if retrieved.Metadata["key"] != "value" {
		t.Errorf("expected Metadata[key]=value, got %s", retrieved.Metadata["key"])
	}
	if retrieved.HandlerName != task.HandlerName {
		t.Errorf("expected HandlerName %s, got %s", task.HandlerName, retrieved.HandlerName)
	}
}

func TestGetNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	_, err = store.Get("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create and save task
	task := &TaskRecord{
		ID:          "task-1",
		Name:        "test-task",
		Status:      "pending",
		ScheduledAt: time.Now(),
		HandlerName: "test-handler",
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Delete task
	if err := store.Delete(task.ID); err != nil {
		t.Fatalf("failed to delete task: %v", err)
	}

	// Verify task is gone
	_, err = store.Get(task.ID)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound after delete, got %v", err)
	}
}

func TestDeleteNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	err = store.Delete("non-existent")
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create tasks
	tasks := []*TaskRecord{
		{ID: "task-1", Name: "task-1", Status: "pending", Priority: 1, ScheduledAt: time.Now(), HandlerName: "handler-a"},
		{ID: "task-2", Name: "task-2", Status: "running", Priority: 2, ScheduledAt: time.Now(), HandlerName: "handler-b"},
		{ID: "task-3", Name: "task-3", Status: "pending", Priority: 3, ScheduledAt: time.Now(), HandlerName: "handler-a"},
	}

	for _, task := range tasks {
		if err := store.Save(task); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	// List all tasks
	all, err := store.List(TaskFilter{})
	if err != nil {
		t.Fatalf("failed to list tasks: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(all))
	}

	// List by status
	pending, err := store.List(TaskFilter{Status: "pending"})
	if err != nil {
		t.Fatalf("failed to list pending tasks: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}

	// List by handler
	handlerA, err := store.List(TaskFilter{HandlerName: "handler-a"})
	if err != nil {
		t.Fatalf("failed to list handler-a tasks: %v", err)
	}
	if len(handlerA) != 2 {
		t.Errorf("expected 2 handler-a tasks, got %d", len(handlerA))
	}

	// List with limit
	limited, err := store.List(TaskFilter{Limit: 2})
	if err != nil {
		t.Fatalf("failed to list limited tasks: %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("expected 2 limited tasks, got %d", len(limited))
	}
}

func TestGetPendingTasks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create tasks with different statuses
	tasks := []*TaskRecord{
		{ID: "task-1", Name: "task-1", Status: "pending", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-2", Name: "task-2", Status: "running", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-3", Name: "task-3", Status: "pending", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-4", Name: "task-4", Status: "completed", ScheduledAt: time.Now(), HandlerName: "handler"},
	}

	for _, task := range tasks {
		if err := store.Save(task); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	pending, err := store.GetPendingTasks()
	if err != nil {
		t.Fatalf("failed to get pending tasks: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending tasks, got %d", len(pending))
	}
}

func TestUpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create task
	task := &TaskRecord{
		ID:          "task-1",
		Name:        "test-task",
		Status:      "pending",
		ScheduledAt: time.Now(),
		HandlerName: "test-handler",
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update to running
	if err := store.UpdateStatus(task.ID, "running", nil); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	retrieved, _ := store.Get(task.ID)
	if retrieved.Status != "running" {
		t.Errorf("expected status running, got %s", retrieved.Status)
	}
	if retrieved.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// Update to completed
	if err := store.UpdateStatus(task.ID, "completed", nil); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	retrieved, _ = store.Get(task.ID)
	if retrieved.Status != "completed" {
		t.Errorf("expected status completed, got %s", retrieved.Status)
	}
	if retrieved.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}
}

func TestUpdateStatusNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	err = store.UpdateStatus("non-existent", "running", nil)
	if err != ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestIncrementRetry(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create task
	task := &TaskRecord{
		ID:          "task-1",
		Name:        "test-task",
		Status:      "failed",
		ScheduledAt: time.Now(),
		RetryCount:  0,
		MaxRetries:  3,
		HandlerName: "test-handler",
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Increment retry
	if err := store.IncrementRetry(task.ID); err != nil {
		t.Fatalf("failed to increment retry: %v", err)
	}

	retrieved, _ := store.Get(task.ID)
	if retrieved.RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", retrieved.RetryCount)
	}
	if retrieved.Status != "pending" {
		t.Errorf("expected status pending after retry, got %s", retrieved.Status)
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 1})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create old completed task
	oldTime := time.Now().AddDate(0, 0, -2)
	oldTask := &TaskRecord{
		ID:          "old-task",
		Name:        "old-task",
		Status:      "completed",
		ScheduledAt: oldTime,
		HandlerName: "test-handler",
	}
	if err := store.Save(oldTask); err != nil {
		t.Fatalf("failed to save old task: %v", err)
	}
	// Manually set completed_at to old time
	store.db.Exec("UPDATE tasks SET completed_at = ? WHERE id = ?", oldTime, oldTask.ID)

	// Create recent completed task
	recentTask := &TaskRecord{
		ID:          "recent-task",
		Name:        "recent-task",
		Status:      "completed",
		ScheduledAt: time.Now(),
		HandlerName: "test-handler",
	}
	if err := store.Save(recentTask); err != nil {
		t.Fatalf("failed to save recent task: %v", err)
	}
	store.UpdateStatus(recentTask.ID, "completed", nil)

	// Run cleanup
	deleted, err := store.Cleanup()
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}
	if deleted != 1 {
		t.Errorf("expected 1 deleted, got %d", deleted)
	}

	// Verify old task is gone
	_, err = store.Get(oldTask.ID)
	if err != ErrTaskNotFound {
		t.Error("old task should have been deleted")
	}

	// Verify recent task still exists
	_, err = store.Get(recentTask.ID)
	if err != nil {
		t.Error("recent task should still exist")
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create tasks with different statuses
	tasks := []*TaskRecord{
		{ID: "task-1", Name: "task-1", Status: "pending", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-2", Name: "task-2", Status: "running", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-3", Name: "task-3", Status: "completed", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-4", Name: "task-4", Status: "failed", ScheduledAt: time.Now(), HandlerName: "handler"},
		{ID: "task-5", Name: "task-5", Status: "cancelled", ScheduledAt: time.Now(), HandlerName: "handler"},
	}

	for _, task := range tasks {
		if err := store.Save(task); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}

	stats, err := store.Stats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if stats.TotalTasks != 5 {
		t.Errorf("expected TotalTasks=5, got %d", stats.TotalTasks)
	}
	if stats.PendingTasks != 1 {
		t.Errorf("expected PendingTasks=1, got %d", stats.PendingTasks)
	}
	if stats.RunningTasks != 1 {
		t.Errorf("expected RunningTasks=1, got %d", stats.RunningTasks)
	}
	if stats.CompletedTasks != 1 {
		t.Errorf("expected CompletedTasks=1, got %d", stats.CompletedTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("expected FailedTasks=1, got %d", stats.FailedTasks)
	}
	if stats.CancelledTasks != 1 {
		t.Errorf("expected CancelledTasks=1, got %d", stats.CancelledTasks)
	}
}

func TestStoreClosed(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	store.Close()

	// All operations should return ErrStoreClosed
	_, err = store.Get("task-1")
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed from Get, got %v", err)
	}

	err = store.Save(&TaskRecord{ID: "task-1", HandlerName: "handler"})
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed from Save, got %v", err)
	}

	err = store.Delete("task-1")
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed from Delete, got %v", err)
	}

	_, err = store.List(TaskFilter{})
	if err != ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed from List, got %v", err)
	}
}

func TestUpsert(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create task
	task := &TaskRecord{
		ID:          "task-1",
		Name:        "original-name",
		Status:      "pending",
		ScheduledAt: time.Now(),
		HandlerName: "test-handler",
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Update task with same ID
	task.Name = "updated-name"
	task.Status = "running"
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to update task: %v", err)
	}

	// Verify update
	retrieved, _ := store.Get(task.ID)
	if retrieved.Name != "updated-name" {
		t.Errorf("expected Name='updated-name', got %s", retrieved.Name)
	}
	if retrieved.Status != "running" {
		t.Errorf("expected Status='running', got %s", retrieved.Status)
	}
}

func TestDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	store, err := NewStore(Config{DBPath: dbPath, Enabled: true, RetentionDays: 7})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer store.Close()

	// Create task with dependencies
	task := &TaskRecord{
		ID:           "task-1",
		Name:         "test-task",
		Status:       "pending",
		ScheduledAt:  time.Now(),
		Dependencies: []string{"dep-1", "dep-2", "dep-3"},
		HandlerName:  "test-handler",
	}
	if err := store.Save(task); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Retrieve and verify dependencies
	retrieved, _ := store.Get(task.ID)
	if len(retrieved.Dependencies) != 3 {
		t.Errorf("expected 3 dependencies, got %d", len(retrieved.Dependencies))
	}
	if retrieved.Dependencies[0] != "dep-1" {
		t.Errorf("expected first dependency 'dep-1', got %s", retrieved.Dependencies[0])
	}
}
